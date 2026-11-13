package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
	"slices"
)

// CertifiedWrappedBoreMesh has outer/inner walls (OriginalFace 0/1) and lower/
// upper annular caps (2/3). RimVertices[wall][cap] retains shared display IDs.
// The four source charts are embedded by strict canonical radius nesting and
// exact common angular profile. Per-triangle bounds use normalized parameter
// interpolation in the original wall or constructed exact radial cap chart.
type CertifiedWrappedBoreMesh struct {
	Mesh                                            mesh.Mesh
	Walls                                           [2]geom.WrappedFrustumEmbedding
	OriginalFace                                    []int
	RimVertices                                     [2][2][]int
	SquaredErrors                                   []*big.Rat
	SourceImageSquaredDistance, SignedDisplayVolume *big.Rat
	Collision                                       collision.Report
	Subdivisions, TriangleChecks                    int
}

// TessellateWrappedBoreClosedCertified requires two positive-radius canonical
// frustums with identical center/heights, knot bases and rational angular
// weights up to a common scale. Strict endpoint-radius nesting proves nesting
// at every intermediate height because each radius is affine in height.
// Both inputs must have outward side orientation;
// the inner mesh incidence is reversed. One dyadic grid supplies all exact rim
// samples and both walls. Cap charts are exact radial interpolations between
// those profiles. All triangle checks across retry grids share
// Patch.MaxTriangleChecks; MaxTriangles bounds each candidate and final output.
// Neither tolerance welding nor inferred circularity is used. Failure is atomic.
func TessellateWrappedBoreClosedCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, o CertifiedWrappedFrustumMeshOptions) (out CertifiedWrappedBoreMesh, err error) {
	defer func() {
		if err != nil {
			out = CertifiedWrappedBoreMesh{}
		}
	}()
	if ctx == nil || outer == nil || inner == nil {
		return out, geom.ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 8000000 {
		return out, geom.ErrInvalid
	}
	if o.Patch.MaxTriangleChecks == 0 {
		o.Patch.MaxTriangleChecks = 100000
	}
	if o.Patch.MaxSubdivisions == 0 {
		o.Patch.MaxSubdivisions = 32
	}
	if o.Patch.MaxTriangleChecks < 1 || o.Patch.MaxSubdivisions < 1 || o.Patch.MaxSubdivisions > 128 || o.Patch.MaxSubdivisions&(o.Patch.MaxSubdivisions-1) != 0 || !finite(o.Patch.Deflection) || o.Patch.Deflection <= 0 {
		return out, geom.ErrInvalid
	}
	var proofs [2]geom.WrappedFrustumEmbedding
	var data [2]geom.BSplineSurfaceData
	for i, w := range []*geom.WrappedSurfacePatch{outer, inner} {
		p, e := geom.CertifyWrappedFrustumEmbedding(ctx, w, o.Embedding)
		if e != nil {
			return out, e
		}
		proofs[i] = p
		s, e := w.FundamentalPatch().ToBSplineSurface()
		if e != nil {
			return out, e
		}
		data[i] = s.Data()
	}
	a, b := proofs[0], proofs[1]
	if a.CenterX.Cmp(b.CenterX) != 0 || a.CenterY.Cmp(b.CenterY) != 0 || a.LowerZ.Cmp(b.LowerZ) != 0 || a.UpperZ.Cmp(b.UpperZ) != 0 || a.LowerRadius.Cmp(b.LowerRadius) <= 0 || a.UpperRadius.Cmp(b.UpperRadius) <= 0 {
		return out, geom.ErrConvergence
	}
	if !slices.Equal(data[0].KnotsU, data[1].KnotsU) || !slices.Equal(data[0].KnotsV, data[1].KnotsV) {
		return out, geom.ErrConvergence
	}
	weight := func(d geom.BSplineSurfaceData, i int) *big.Rat {
		if len(d.Weights) == 0 {
			return big.NewRat(1, 1)
		}
		return ratInset(d.Weights[i])
	}
	for i := 0; i < 18; i++ {
		left := new(big.Rat).Mul(weight(data[0], i), weight(data[1], 0))
		right := new(big.Rat).Mul(weight(data[1], i), weight(data[0], 0))
		if left.Cmp(right) != 0 {
			return out, geom.ErrConvergence
		}
	}
	var caps [2]*geom.SurfacePatch
	for cap := 0; cap < 2; cap++ {
		var poles []geom.Vec3
		var weights []float64
		for i := 0; i < 9; i++ {
			for _, wall := range []int{1, 0} {
				poles = append(poles, data[wall].Poles[2*i+cap])
				wf, _ := weight(data[0], 2*i+cap).Float64()
				weights = append(weights, wf)
			}
		}
		s, e := geom.NewBSplineSurface(2, 1, 9, 2, poles, data[0].KnotsU, []float64{0, 0, 1, 1}, weights)
		if e != nil {
			return out, e
		}
		caps[cap], e = geom.NewSurfacePatch(s, o.Embedding)
		if e != nil {
			return out, e
		}
	}
	tolerance := ratInset(o.Patch.Deflection)
	tolerance.Mul(tolerance, tolerance)
	checks := 0
	for minimum := 1; minimum <= o.Patch.MaxSubdivisions; {
		remaining := o.Patch.MaxTriangleChecks - checks
		if remaining < 1 {
			return out, geom.ErrConvergence
		}
		po := o.Patch
		po.MaxTriangleChecks = remaining
		side, e := tessellateCertifiedKnotGridFrom(ctx, outer.FundamentalPatch(), [2]bool{true, false}, po, minimum)
		if e != nil {
			return out, e
		}
		checks += side.TriangleChecks
		if !side.BothDirections {
			return out, geom.ErrConvergence
		}
		nu, nv := len(side.UParameters)-1, len(side.VParameters)-1
		if 2*len(side.Mesh.Triangles)+4*nu > o.MaxTriangles {
			return out, geom.ErrConvergence
		}
		candidate := CertifiedWrappedBoreMesh{Walls: proofs, Subdivisions: side.Subdivisions, SourceImageSquaredDistance: new(big.Rat)}
		candidate.Mesh.Vertices = append(candidate.Mesh.Vertices, side.Mesh.Vertices...)
		for i := 0; i < nu; i++ {
			for j := 0; j <= nv; j++ {
				p, e := inner.FundamentalPatch().EvaluateRational(ctx, ratInset(side.UParameters[i]), ratInset(side.VParameters[j]), o.Patch.Evaluation)
				if e != nil {
					return out, e
				}
				var xyz [3]float64
				for k, r := range p.Point {
					xyz[k], _ = r.Float64()
				}
				v := geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
				if !v.IsFinite() {
					return out, geom.ErrConvergence
				}
				candidate.Mesh.Vertices = append(candidate.Mesh.Vertices, v)
			}
		}
		offset := len(side.Mesh.Vertices)
		accepted := true
		appendTriangle := func(tri [3]int, uv [3]geom.Vec2, patch *geom.SurfacePatch, face int, known *big.Rat) error {
			bound := known
			if bound == nil {
				if checks >= o.Patch.MaxTriangleChecks {
					return geom.ErrConvergence
				}
				var model [3]geom.Vec3
				for i, id := range tri {
					model[i] = candidate.Mesh.Vertices[id]
				}
				q, e := BoundSplineTriangleDeviation(ctx, patch, uv, model)
				checks++
				if e != nil {
					return e
				}
				if !q.BothDirections {
					return geom.ErrConvergence
				}
				bound = q.SquaredDistance
			}
			if bound.Cmp(tolerance) > 0 {
				accepted = false
			}
			if bound.Cmp(candidate.SourceImageSquaredDistance) > 0 {
				candidate.SourceImageSquaredDistance.Set(bound)
			}
			candidate.Mesh.Triangles = append(candidate.Mesh.Triangles, tri)
			candidate.OriginalFace = append(candidate.OriginalFace, face)
			candidate.SquaredErrors = append(candidate.SquaredErrors, new(big.Rat).Set(bound))
			return nil
		}
		for i, tri := range side.Mesh.Triangles {
			if e := appendTriangle(tri, side.UVTriangles[i], outer.FundamentalPatch(), 0, side.SquaredErrors[i]); e != nil {
				return out, e
			}
			it := [3]int{tri[0] + offset, tri[2] + offset, tri[1] + offset}
			uv := side.UVTriangles[i]
			uv[1], uv[2] = uv[2], uv[1]
			if e := appendTriangle(it, uv, inner.FundamentalPatch(), 1, nil); e != nil {
				return out, e
			}
		}
		for wall := 0; wall < 2; wall++ {
			for cap := 0; cap < 2; cap++ {
				for i := 0; i < nu; i++ {
					candidate.RimVertices[wall][cap] = append(candidate.RimVertices[wall][cap], wall*offset+i*(nv+1)+cap*nv)
				}
			}
		}
		for cap := 0; cap < 2; cap++ {
			for i := 0; i < nu; i++ {
				j := (i + 1) % nu
				ids := [4]int{candidate.RimVertices[1][cap][i], candidate.RimVertices[1][cap][j], candidate.RimVertices[0][cap][j], candidate.RimVertices[0][cap][i]}
				uv := [4]geom.Vec2{{X: side.UParameters[i]}, {X: side.UParameters[i+1]}, {X: side.UParameters[i+1], Y: 1}, {X: side.UParameters[i], Y: 1}}
				for _, local := range [][3]int{{0, 1, 2}, {0, 2, 3}} {
					if cap == 1 {
						local[1], local[2] = local[2], local[1]
					}
					tri := [3]int{ids[local[0]], ids[local[1]], ids[local[2]]}
					param := [3]geom.Vec2{uv[local[0]], uv[local[1]], uv[local[2]]}
					if e := appendTriangle(tri, param, caps[cap], cap+2, nil); e != nil {
						return out, e
					}
				}
			}
		}
		if accepted {
			candidate.TriangleChecks = checks
			volume, report, e := checkClosedPeriodicMesh(ctx, candidate.Mesh, o.Collision)
			if e != nil {
				return out, e
			}
			candidate.SignedDisplayVolume, candidate.Collision = volume, report
			return candidate, nil
		}
		minimum = side.Subdivisions * 2
	}
	return out, geom.ErrConvergence
}

func checkClosedPeriodicMesh(ctx context.Context, m mesh.Mesh, o collision.Options) (*big.Rat, collision.Report, error) {
	var empty collision.Report
	if e := m.ValidateSolid(); e != nil {
		return nil, empty, e
	}
	ids := make([]int, len(m.Vertices))
	for i := range ids {
		ids[i] = i
	}
	if e := validateQuadraticJunctionFans(m, ids); e != nil {
		return nil, empty, e
	}
	v := new(big.Rat)
	for _, tri := range m.Triangles {
		if e := ctx.Err(); e != nil {
			return nil, empty, e
		}
		a, b, c := exactBoundPoint(m.Vertices[tri[0]]), exactBoundPoint(m.Vertices[tri[1]]), exactBoundPoint(m.Vertices[tri[2]])
		for i := 0; i < 3; i++ {
			j, k := (i+1)%3, (i+2)%3
			v.Add(v, new(big.Rat).Mul(a[i], new(big.Rat).Sub(new(big.Rat).Mul(b[j], c[k]), new(big.Rat).Mul(b[k], c[j]))))
		}
	}
	v.Quo(v, big.NewRat(6, 1))
	if v.Sign() <= 0 {
		return nil, empty, geom.ErrDegenerate
	}
	report, e := collision.SelfIntersectionsBVHContext(ctx, m, o)
	if e != nil {
		return nil, report, e
	}
	if !report.Complete || len(report.Contacts) > 0 {
		return nil, report, geom.ErrConvergence
	}
	return v, report, nil
}
