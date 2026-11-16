package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// TessellateFilletedFrustumThroughBoreNormalsCertified bounds the angle in
// radians between each represented facet and all original outward material
// normals on its source parameter triangle. The corner and seam proofs use
// the original analytic charts. The accepted interval is 0 < angle <= 1.
// Angular refinement shares the supplied distance-grid work limits. A failed
// proof returns no mesh or angular certificate.
func TessellateFilletedFrustumThroughBoreNormalsCertified(ctx context.Context, source Solid, lower, upper float64, center geom.Vec2, radius, angle float64, o CertifiedWrappedFrustumMeshOptions) (CertifiedFilletedFrustumBoreMesh, error) {
	if !finite(angle) || angle <= 0 || angle > 1 {
		return CertifiedFilletedFrustumBoreMesh{}, geom.ErrInvalid
	}
	// atan(t) <= t for t >= 0. Use the represented request itself as
	// a rational lower bound for its tangent.
	limit := ratInset(angle)
	angles := &filletedFrustumFacetAngles{limit: limit.Mul(limit, limit), maximum: new(big.Rat)}
	return tessellateFilletedFrustumThroughBore(ctx, source, lower, upper, center, radius, o, angles)
}

type filletedFrustumFacetAngles struct {
	limit, maximum *big.Rat
}

type filletedFrustumGridNormals struct {
	patch                *geom.SurfacePatch
	center               [2]*big.Rat
	rimRadius            *big.Rat
	rimV                 float64
	height, radialChange *big.Rat
	meridian             *FrustumRollingCircle
	inward               bool
}

func frustumGridNormalSource(ctx context.Context, w *geom.WrappedSurfacePatch, inward bool, o geom.SurfacePatchOptions) (filletedFrustumGridNormals, error) {
	p, err := geom.CertifyWrappedFrustumEmbedding(ctx, w, o)
	if err != nil {
		return filletedFrustumGridNormals{}, err
	}
	if !p.Complete || !p.Circular {
		return filletedFrustumGridNormals{}, geom.ErrConvergence
	}
	patch := w.FundamentalPatch()
	return filletedFrustumGridNormals{
		patch: patch, center: [2]*big.Rat{p.CenterX, p.CenterY}, rimRadius: p.LowerRadius, rimV: patch.Domain().VMin,
		height: new(big.Rat).Sub(p.UpperZ, p.LowerZ), radialChange: new(big.Rat).Sub(p.UpperRadius, p.LowerRadius), inward: inward,
	}, nil
}

// FilletWrappedFrustumRims has just restricted a canonical circular torus
// whose azimuth is shared with the cone. Its retained meridian uses the exact
// rolling circle, with the cap at one end and at most one original quadrant
// break inside. The upper/lower cap contacts have zero radial normal and
// radius equal to the rolling circle's first coordinate.
func filletGridNormalSource(w *geom.WrappedSurfacePatch, center [2]*big.Rat, c *FrustumRollingCircle, upper bool) filletedFrustumGridNormals {
	p := w.FundamentalPatch()
	capStation := p.Domain().VMin
	if upper {
		capStation = p.Domain().VMax
	}
	return filletedFrustumGridNormals{patch: p, center: center, rimRadius: c.Center[0], rimV: capStation, meridian: c}
}

// A minor unit-circle arc in one original quadrant lies in the triangle
// formed by its two endpoint rays and their tangent intersection. All
// coordinates are exact. Upstream authorship proves monotone angular sweep
// in each original chart span, so the entire restricted arc is covered.
type filletArcHull [3][2]*big.Rat

func filletNormalArcHulls(parameters, knots []float64, evaluate func(float64) ([2]*big.Rat, error)) ([]filletArcHull, error) {
	if len(parameters) < 2 || len(knots) < 2 || parameters[0] != knots[0] || parameters[len(parameters)-1] != knots[len(knots)-1] {
		return nil, geom.ErrConvergence
	}
	var prev [2]*big.Rat
	hulls := make([]filletArcHull, len(parameters)-1)
	span := 1
	one := big.NewRat(1, 1)
	dot := func(a, b [2]*big.Rat) *big.Rat {
		return new(big.Rat).Add(new(big.Rat).Mul(a[0], b[0]), new(big.Rat).Mul(a[1], b[1]))
	}
	for i, u := range parameters {
		point, err := evaluate(u)
		if err != nil {
			return nil, err
		}
		if dot(point, point).Cmp(one) != 0 {
			return nil, geom.ErrConvergence
		}
		if i != 0 {
			if u <= parameters[i-1] || span >= len(knots) || u > knots[span] {
				return nil, geom.ErrConvergence
			}
			cosine := dot(prev, point)
			if cosine.Sign() < 0 || cosine.Cmp(one) >= 0 {
				return nil, geom.ErrConvergence
			}
			denominator := new(big.Rat).Add(one, cosine)
			tangent := [2]*big.Rat{
				new(big.Rat).Quo(new(big.Rat).Add(prev[0], point[0]), denominator),
				new(big.Rat).Quo(new(big.Rat).Add(prev[1], point[1]), denominator),
			}
			hulls[i-1] = filletArcHull{prev, point, tangent}
			if u == knots[span] {
				span++
			}
		}
		prev = point
	}
	return hulls, nil
}

func (s filletedFrustumGridNormals) certify(ctx context.Context, grid CertifiedKnotPatchMesh, reversed bool, limit *big.Rat, o geom.SurfacePatchEvaluationOptions) (*big.Rat, error) {
	p := s.patch
	us, vs := p.KnotBreaks()
	azimuth, err := filletNormalArcHulls(grid.UParameters, us, func(u float64) ([2]*big.Rat, error) {
		jet, err := p.EvaluateRational(ctx, ratInset(u), ratInset(s.rimV), o)
		if err != nil {
			return [2]*big.Rat{}, err
		}
		return [2]*big.Rat{
			new(big.Rat).Quo(new(big.Rat).Sub(jet.Point[0], s.center[0]), s.rimRadius),
			new(big.Rat).Quo(new(big.Rat).Sub(jet.Point[1], s.center[1]), s.rimRadius),
		}, nil
	})
	if err != nil {
		return nil, err
	}
	var meridian []filletArcHull
	if c := s.meridian; c != nil {
		meridian, err = filletNormalArcHulls(grid.VParameters, vs, func(v float64) ([2]*big.Rat, error) {
			jet, err := p.EvaluateRational(ctx, ratInset(p.Domain().UMin), ratInset(v), o)
			if err != nil {
				return [2]*big.Rat{}, err
			}
			// At the first canonical U station the exact ray is +X.
			if jet.Point[1].Cmp(s.center[1]) != 0 || jet.Point[0].Cmp(s.center[0]) <= 0 {
				return [2]*big.Rat{}, geom.ErrConvergence
			}
			radial := new(big.Rat).Sub(jet.Point[0], s.center[0])
			radial.Sub(radial, c.Center[0]).Quo(radial, c.Radius)
			axial := new(big.Rat).Sub(jet.Point[2], c.Center[1])
			axial.Quo(axial, c.Radius)
			return [2]*big.Rat{radial, axial}, nil
		})
		if err != nil {
			return nil, err
		}
	}
	nv := len(grid.VParameters) - 1
	if nv < 1 || len(grid.Mesh.Triangles) != 2*len(azimuth)*nv || len(grid.UVTriangles) != len(grid.Mesh.Triangles) {
		return nil, geom.ErrConvergence
	}
	maximum := new(big.Rat)
	var height, axial *big.Rat
	if meridian == nil {
		height, axial = new(big.Rat).Set(s.height), new(big.Rat).Neg(s.radialChange)
		if s.inward {
			height.Neg(height)
			axial.Neg(axial)
		}
	}
	for i, az := range azimuth {
		var wall [3][3]*big.Rat
		if meridian == nil {
			for k, a := range az {
				wall[k] = filletFacetNormalRay(a, height, axial)
			}
		}
		for j := 0; j < nv; j++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			// For the frustum, the normal ray is (h*A, -radialChange).
			// For a torus it is (B.x*A, B.y). Bilinearity gives a convex
			// combination of the nine products of arc corners. All rays
			// must lie in the positive-dot convex cone of the represented
			// facet, including the effects of rounded display vertices.
			var product [9][3]*big.Rat
			rays := wall[:]
			if meridian != nil {
				for k, a := range az {
					for l, b := range meridian[j] {
						product[3*k+l] = filletFacetNormalRay(a, b[0], b[1])
					}
				}
				rays = product[:]
			}
			first := 2 * (i*nv + j)
			for k := first; k < first+2; k++ {
				// Bind each proof to the actual source UV triangle used by
				// the distance certificate, including the final wrapped row.
				uv := grid.UVTriangles[k]
				if min(uv[0].X, uv[1].X, uv[2].X) != grid.UParameters[i] || max(uv[0].X, uv[1].X, uv[2].X) != grid.UParameters[i+1] ||
					min(uv[0].Y, uv[1].Y, uv[2].Y) != grid.VParameters[j] || max(uv[0].Y, uv[1].Y, uv[2].Y) != grid.VParameters[j+1] {
					return nil, geom.ErrConvergence
				}
				facet := frustumMeshFacetNormal(grid.Mesh.Vertices, grid.Mesh.Triangles[k], reversed)
				for _, ray := range rays {
					q, ok := ddTanSquared(facet, ray)
					if !ok || q.Cmp(limit) > 0 {
						return nil, nil // refine the grid
					}
					if q.Cmp(maximum) > 0 {
						maximum.Set(q)
					}
				}
			}
		}
	}
	return maximum, nil
}

func filletFacetNormalRay(a [2]*big.Rat, scale, axial *big.Rat) [3]*big.Rat {
	return [3]*big.Rat{new(big.Rat).Mul(scale, a[0]), new(big.Rat).Mul(scale, a[1]), axial}
}

func frustumMeshFacetNormal(vertices []geom.Vec3, tri [3]int, reversed bool) [3]*big.Rat {
	if reversed {
		tri[1], tri[2] = tri[2], tri[1]
	}
	a, b, c := exactBoundPoint(vertices[tri[0]]), exactBoundPoint(vertices[tri[1]]), exactBoundPoint(vertices[tri[2]])
	return ddCross(ddSub(b, a), ddSub(c, a))
}

func frustumCapFacetNormalMatches(vertices []geom.Vec3, tri [3]int, upper bool) bool {
	n := frustumMeshFacetNormal(vertices, tri, false)
	sign := -1
	if upper {
		sign = 1
	}
	return n[0].Sign() == 0 && n[1].Sign() == 0 && n[2].Sign() == sign
}
