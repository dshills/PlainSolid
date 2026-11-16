package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"

	"plainsolid/geom"
)

type mappedRoofDisplayOptions struct {
	alignSupportKnots      bool
	maxFlips               int
	facetAngle, normalTurn float64
}

type mappedRoofNormalVertex struct {
	cell [2]int
	id   int
}

type mappedRoofNormals struct {
	vertices          map[mappedRoofNormalVertex][3]*big.Rat
	certified         map[[3]int]*big.Rat
	cells             map[[2]int]*geom.SurfacePatch
	limit, trimRadius *big.Rat
	remaining         int
	sourceCentered    bool
	turnLimit         *big.Rat
	turnChart         *geom.AffineXYGraph
}

func newMappedRoofNormals(ctx context.Context, p *geom.SurfacePatch, angle float64, trimDistance *big.Rat) (*mappedRoofNormals, error) {
	r := &mappedRoofNormals{
		cells: make(map[[2]int]*geom.SurfacePatch), vertices: make(map[mappedRoofNormalVertex][3]*big.Rat),
		certified: make(map[[3]int]*big.Rat), remaining: 100000,
	}
	r.limit = mappedAngleTangentLimit(angle)
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

// tan'(x)=1+tan²(x) >= 1+x² on this interval, so x+x³/3 is
// a rational lower bound for tan(x) at the represented request.
func mappedAngleTangentLimit(angle float64) *big.Rat {
	a := ratInset(angle)
	lower := new(big.Rat).Mul(new(big.Rat).Mul(a, a), a)
	lower.Quo(lower, big.NewRat(3, 1))
	lower.Add(lower, a)
	return lower.Mul(lower, lower)
}

func (r *mappedRoofNormals) enableTurn(ctx context.Context, p *geom.SurfacePatch, angle float64) error {
	graph, err := geom.NewAffineXYGraph(ctx, p, geom.SurfacePatchOptions{})
	if err != nil {
		return err
	}
	r.turnChart = graph
	r.sourceCentered = true
	r.turnLimit = mappedNormalTurnLimit(angle)
	return nil
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

// certify returns the per-triangle refinement set when the exact bound
// cannot yet admit the entire roof. It first uses exact vertex normals
// to detect actual errors and only tests continuous hulls once those pass.
func (r *mappedRoofNormals) certify(ctx context.Context, p *geom.SurfacePatch, face CertifiedCurvedQuadraticTrimMesh) (*big.Rat, []bool, error) {
	cells, err := mappedChainTriangleSupportCells(p, face)
	if err != nil {
		return nil, nil, err
	}
	maximum := new(big.Rat)
	directions := make([][3]*big.Rat, len(face.Mesh.Triangles))
	retry := make([]bool, len(directions))
	centerOutside := make([]bool, len(directions))
	needsRefinement := false
	var outsideCount int
	for i, tri := range face.Mesh.Triangles {
		if proof := r.certified[tri]; proof != nil {
			if proof.Cmp(maximum) > 0 {
				maximum.Set(proof)
			}
			continue
		}
		c := r.cells[cells[i]]
		a, b, d := exactBoundPoint(face.Mesh.Vertices[tri[0]]), exactBoundPoint(face.Mesh.Vertices[tri[1]]), exactBoundPoint(face.Mesh.Vertices[tri[2]])
		direction := ddCross(ddSub(b, a), ddSub(d, a))
		if r.sourceCentered {
			if err := r.charge(ctx); err != nil {
				return nil, nil, err
			}
			u, v := new(big.Rat), new(big.Rat)
			for _, id := range tri {
				u.Add(u, face.ExactUV[id][0])
				v.Add(v, face.ExactUV[id][1])
			}
			u.Quo(u, big.NewRat(3, 1))
			v.Quo(v, big.NewRat(3, 1))
			middle, err := c.EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
			if err != nil {
				return nil, nil, err
			}
			direction = ddCross(middle.Du, middle.Dv)
		}
		directions[i] = direction
		var rays [3][3]*big.Rat
		for j, id := range tri {
			key := mappedRoofNormalVertex{cells[i], id}
			n, exists := r.vertices[key]
			if !exists {
				if err := r.charge(ctx); err != nil {
					return nil, nil, err
				}
				uv := face.ExactUV[id]
				j, err := c.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
				if err != nil {
					return nil, nil, err
				}
				n = ddCross(j.Du, j.Dv)
				r.vertices[key] = n
			}
			if t, ok := ddTanSquared(direction, n); !ok || t.Cmp(r.limit) > 0 {
				if !centerOutside[i] {
					centerOutside[i] = true
					outsideCount++
				}
				if !r.sourceCentered {
					retry[i], needsRefinement = true, true
					break
				}
			}
			if r.sourceCentered {
				for _, other := range rays[:j] {
					if q, ok := ddTanSquared(other, n); !ok || q.Cmp(r.turnLimit) > 0 {
						retry[i], needsRefinement = true, true
						break
					}
				}
				if retry[i] {
					break
				}
				rays[j] = n
			}
		}
	}
	if needsRefinement {
		return nil, retry, nil
	}
	// Prefer a cheaper conforming refinement while many triangles still
	// miss their source-centered cone. The direct pairwise check resolves
	// the few borderline facets that remain, including protected edges.
	if r.sourceCentered && outsideCount > 8 {
		return nil, centerOutside, nil
	}
	unproved := 0
	for i, t := range face.Mesh.Triangles {
		if r.certified[t] != nil {
			continue
		}
		uv := [3][2]*big.Rat{face.ExactUV[t[0]], face.ExactUV[t[1]], face.ExactUV[t[2]]}
		var q *big.Rat
		var err error
		cell := r.cells[cells[i]]
		if !centerOutside[i] {
			q, err = r.bound(ctx, cell, directions[i], uv, 3)
			if err != nil {
				return nil, nil, err
			}
		}
		if q == nil {
			retry[i] = true
			unproved++
			continue
		}
		if r.sourceCentered {
			q = doubledNormalTangent(q)
		}
		r.certified[t] = q
		if q.Cmp(maximum) > 0 {
			maximum.Set(q)
		}
	}
	if r.sourceCentered && unproved > 0 && unproved <= 8 {
		for i, missing := range retry {
			if !missing {
				continue
			}
			t := face.Mesh.Triangles[i]
			uv := [3][2]*big.Rat{face.ExactUV[t[0]], face.ExactUV[t[1]], face.ExactUV[t[2]]}
			q, err := r.boundPairwiseTurn(ctx, r.cells[cells[i]], uv)
			if err != nil {
				return nil, nil, err
			}
			if q != nil {
				retry[i] = false
				unproved--
				r.certified[t] = q
				if q.Cmp(maximum) > 0 {
					maximum.Set(q)
				}
			}
		}
	}
	if unproved > 0 {
		return nil, retry, nil
	}
	return maximum, nil, nil
}

func (r *mappedRoofNormals) normalRectangle(p *geom.SurfacePatch, uv [3][2]*big.Rat) geom.ParameterRectangle {
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
	return geom.ParameterRectangle{UMin: bounds[0][0], UMax: bounds[0][1], VMin: bounds[1][0], VMax: bounds[1][1]}
}

func (r *mappedRoofNormals) normalBox(ctx context.Context, p *geom.SurfacePatch, rect geom.ParameterRectangle) ([3]geom.CertifiedScalarRange, bool, error) {
	var empty [3]geom.CertifiedScalarRange
	if err := r.charge(ctx); err != nil {
		return empty, false, err
	}
	child, err := p.Trim(rect)
	if err != nil {
		return empty, false, err
	}
	proof, err := geom.CertifySurfaceRegularity(child, geom.SurfaceRegularityOptions{MaxDepth: 1, MaxCells: 1})
	if err != nil && !errors.Is(err, geom.ErrConvergence) {
		return empty, false, err
	}
	if err != nil || !proof.Complete || len(proof.Certified) != 1 {
		return empty, false, nil
	}
	return proof.Certified[0].ScaledNormal, true, nil
}

func (r *mappedRoofNormals) bound(ctx context.Context, p *geom.SurfacePatch, direction [3]*big.Rat, uv [3][2]*big.Rat, depth int) (*big.Rat, error) {
	c, present, err := r.normalBox(ctx, p, r.normalRectangle(p, uv))
	if err != nil {
		return nil, err
	}
	if present {
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
			q, ok := ddTanSquared(direction, vector)
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
		q, err := r.bound(ctx, p, direction, sub, depth-1)
		if err != nil || q == nil {
			return nil, err
		}
		if q.Cmp(maximum) > 0 {
			maximum.Set(q)
		}
	}
	return maximum, nil
}

// A projectively parameterized straight trim can have a nonzero residual
// against its linear chord clock. Its exact image is still the same segment.
// Prove this from monotone projected controls and collinearity, independently
// of the per-trim point sampling used by the chord winding proof.
func mappedRoofStraightTrimImage(trims []PCurve) bool {
	for _, trim := range trims {
		h, err := mixedExactControls(trim)
		if err != nil {
			return false
		}
		controls := make([][3]*big.Rat, len(h))
		for i, p := range h {
			for axis := 0; axis < 2; axis++ {
				controls[i][axis] = new(big.Rat).Quo(p[axis], p[2])
			}
		}
		start, end := controls[0], controls[len(controls)-1]
		axis := 0
		if start[axis].Cmp(end[axis]) == 0 {
			axis = 1
		}
		_, direction := mappedChainMonotoneSpan(controls, axis)
		if direction == 0 {
			return false
		}
		a, b := [2]*big.Rat{start[0], start[1]}, [2]*big.Rat{end[0], end[1]}
		for _, c := range controls[1 : len(controls)-1] {
			if exactUVSignedArea(a, b, [2]*big.Rat{c[0], c[1]}).Sign() != 0 {
				return false
			}
		}
	}
	return true
}

// The roof proofs are relative to their original chart orientation; reversing
// both source and display for outward winding preserves the angle. Planar
// owners already proved an affine lift, so one exact lifted triangle gives
// their constant normal before display rounding.
func certifyMappedChainClosedNormals(ctx context.Context, out CertifiedMappedChainClosedMesh, angle float64) (*big.Rat, error) {
	limit := mappedAngleTangentLimit(angle)
	maximum := new(big.Rat)
	for _, roof := range out.Curved {
		q := roof.SquaredNormalTangent
		if q == nil || q.Cmp(limit) > 0 {
			return nil, geom.ErrConvergence
		}
		if q.Cmp(maximum) > 0 {
			maximum.Set(q)
		}
	}
	for _, planar := range out.Planar {
		if len(planar.Mesh.Triangles) == 0 {
			return nil, geom.ErrConvergence
		}
		tri := planar.Mesh.Triangles[0]
		a, b, c := planar.ExactPoints[tri[0]], planar.ExactPoints[tri[1]], planar.ExactPoints[tri[2]]
		planeNormal := ddCross(ddSub(b, a), ddSub(c, a))
		for _, t := range planar.Mesh.Triangles {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			a, b, c := exactBoundPoint(planar.Mesh.Vertices[t[0]]), exactBoundPoint(planar.Mesh.Vertices[t[1]]), exactBoundPoint(planar.Mesh.Vertices[t[2]])
			facet := ddCross(ddSub(b, a), ddSub(c, a))
			q, ok := ddTanSquared(facet, planeNormal)
			if !ok || q.Cmp(limit) > 0 {
				return nil, geom.ErrConvergence
			}
			if q.Cmp(maximum) > 0 {
				maximum.Set(q)
			}
		}
	}
	return maximum, nil
}
