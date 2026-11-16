package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"

	"plainsolid/geom"
)

type mappedRoofNormals struct {
	cells             map[[2]int]*geom.SurfacePatch
	limit, trimRadius *big.Rat
	remaining         int
}

func newMappedRoofNormals(ctx context.Context, p *geom.SurfacePatch, angle float64, trimDistance *big.Rat) (*mappedRoofNormals, error) {
	r := &mappedRoofNormals{cells: make(map[[2]int]*geom.SurfacePatch), remaining: 100000}
	// tan'(x)=1+tan²(x) >= 1+x² on this interval, so x+x³/3 is
	// a rational lower bound for tan(x) at the represented request.
	a := ratInset(angle)
	lower := new(big.Rat).Mul(new(big.Rat).Mul(a, a), a)
	lower.Quo(lower, big.NewRat(3, 1))
	lower.Add(lower, a)
	r.limit = lower.Mul(lower, lower)
	_, radius, err := graphAreaSqrt(trimDistance, 64, 16384)
	if err != nil {
		return nil, err
	}
	r.trimRadius = radius
	us, vs := p.KnotBreaks()
	for i := 0; i+1 < len(us); i++ {
		for j := 0; j+1 < len(vs); j++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			c, err := p.Trim(geom.ParameterRectangle{UMin: us[i], UMax: us[i+1], VMin: vs[j], VMax: vs[j+1]})
			if err != nil {
				return nil, err
			}
			r.cells[[2]int{i, j}] = c
		}
	}
	return r, nil
}

func (r *mappedRoofNormals) charge(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.remaining == 0 {
		return geom.ErrConvergence
	}
	r.remaining--
	return nil
}

// certify returns nil without an error if the triangles need a finer mesh.
// source cell closure uses the same knot on either side and independent
// original derivatives; no smooth normal is inferred across its crease.
func (r *mappedRoofNormals) certify(ctx context.Context, p *geom.SurfacePatch, face CertifiedCurvedQuadraticTrimMesh) (*big.Rat, error) {
	cells, err := mappedChainTriangleSupportCells(p, face)
	if err != nil {
		return nil, err
	}
	type vertex struct {
		cell [2]int
		id   int
	}
	normals := make(map[vertex][3]*big.Rat)
	limit := new(big.Rat)
	facets := make([][3]*big.Rat, len(face.Mesh.Triangles))
	for i, tri := range face.Mesh.Triangles {
		c := r.cells[cells[i]]
		a, b, d := exactBoundPoint(face.Mesh.Vertices[tri[0]]), exactBoundPoint(face.Mesh.Vertices[tri[1]]), exactBoundPoint(face.Mesh.Vertices[tri[2]])
		facet := ddCross(ddSub(b, a), ddSub(d, a))
		facets[i] = facet
		for _, id := range tri {
			key := vertex{cells[i], id}
			n, exists := normals[key]
			if !exists {
				if err := r.charge(ctx); err != nil {
					return nil, err
				}
				uv := face.ExactUV[id]
				j, err := c.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
				if err != nil {
					return nil, err
				}
				n = ddCross(j.Du, j.Dv)
				normals[key] = n
			}
			if t, ok := ddTanSquared(facet, n); !ok || t.Cmp(r.limit) > 0 {
				return nil, nil
			}
		}
	}
	for i, t := range face.Mesh.Triangles {
		uv := [3][2]*big.Rat{face.ExactUV[t[0]], face.ExactUV[t[1]], face.ExactUV[t[2]]}
		q, err := r.bound(ctx, r.cells[cells[i]], facets[i], uv, 3)
		if err != nil || q == nil {
			return nil, err
		}
		if q.Cmp(limit) > 0 {
			limit.Set(q)
		}
	}
	return limit, nil
}

func (r *mappedRoofNormals) bound(ctx context.Context, p *geom.SurfacePatch, facet [3]*big.Rat, uv [3][2]*big.Rat, depth int) (*big.Rat, error) {
	if err := r.charge(ctx); err != nil {
		return nil, err
	}
	// Every trim chord lies in its original convex support cell. The
	// source/chord winding deformation inside that cell lies within the
	// certified UV trim distance of the chord. Include that neighborhood
	// without crossing to the other one-sided support at a knot.
	domain := p.Domain()
	floor := [2]*big.Rat{ratInset(domain.UMin), ratInset(domain.VMin)}
	ceil := [2]*big.Rat{ratInset(domain.UMax), ratInset(domain.VMax)}
	var bounds [2][2]float64
	for axis := range bounds {
		lo, hi := uv[0][axis], uv[0][axis]
		for _, corner := range uv[1:] {
			if corner[axis].Cmp(lo) < 0 {
				lo = corner[axis]
			}
			if corner[axis].Cmp(hi) > 0 {
				hi = corner[axis]
			}
		}
		lower, upper := new(big.Rat).Sub(lo, r.trimRadius), new(big.Rat).Add(hi, r.trimRadius)
		if lower.Cmp(floor[axis]) < 0 {
			lower = floor[axis]
		}
		if upper.Cmp(ceil[axis]) > 0 {
			upper = ceil[axis]
		}
		a, _ := lower.Float64()
		b, _ := upper.Float64()
		if ratInset(a).Cmp(lower) > 0 {
			a = math.Nextafter(a, math.Inf(-1))
		}
		if ratInset(b).Cmp(upper) < 0 {
			b = math.Nextafter(b, math.Inf(1))
		}
		bounds[axis] = [2]float64{a, b}
	}
	rect := geom.ParameterRectangle{UMin: bounds[0][0], UMax: bounds[0][1], VMin: bounds[1][0], VMax: bounds[1][1]}
	child, err := p.Trim(rect)
	if err != nil {
		return nil, err
	}
	proof, err := geom.CertifySurfaceRegularity(child, geom.SurfaceRegularityOptions{MaxDepth: 1, MaxCells: 1})
	if err == nil && proof.Complete && len(proof.Certified) == 1 {
		c := proof.Certified[0].ScaledNormal
		maximum := new(big.Rat)
		for corner := 0; corner < 8; corner++ {
			var vector [3]*big.Rat
			for axis, x := range c {
				value := x.Min
				if corner&(1<<axis) != 0 {
					value = x.Max
				}
				if !finite(value) {
					return nil, geom.ErrConvergence
				}
				vector[axis] = ratInset(value)
			}
			q, ok := ddTanSquared(facet, vector)
			if !ok || q.Cmp(r.limit) > 0 {
				maximum = nil
				break
			}
			if q.Cmp(maximum) > 0 {
				maximum.Set(q)
			}
		}
		// Positive-dot circular cones are convex. Every hull corner
		// inside the exact tangent limit prove the bound for the entire box.
		if maximum != nil {
			return maximum, nil
		}
	} else if err != nil && !errors.Is(err, geom.ErrConvergence) {
		return nil, err
	}
	if depth == 0 {
		return nil, nil
	}
	var mid [3][2]*big.Rat
	for i, a := range uv {
		b := uv[(i+1)%3]
		for axis := range a {
			mid[i][axis] = new(big.Rat).Quo(new(big.Rat).Add(a[axis], b[axis]), big.NewRat(2, 1))
		}
	}
	maximum := new(big.Rat)
	for _, sub := range [4][3][2]*big.Rat{
		{uv[0], mid[0], mid[2]}, {mid[0], uv[1], mid[1]}, {mid[2], mid[1], uv[2]}, mid,
	} {
		q, err := r.bound(ctx, p, facet, sub, depth-1)
		if err != nil || q == nil {
			return nil, err
		}
		if q.Cmp(maximum) > 0 {
			maximum.Set(q)
		}
	}
	return maximum, nil
}
