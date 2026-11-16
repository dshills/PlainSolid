package geom

import (
	"context"
	"math/big"
)

// ProjectiveGraphBoundaryContact is the contact of two proved graph charts
// whose projected rectangles are disjoint or meet only on their boundary.
// A shared analytic edge retains each original parameterized trace, directed
// toward increasing common model projection. Its BClockRatio maps A's
// normalized edge station to B's as in ExactCurveComparisonOptions.
type ProjectiveGraphBoundaryContact struct {
	Complete, Disjoint bool
	Point, Edge        bool
	Projection         [2]ExactParameterInterval
	Endpoints          [2]ProjectiveGraphContactCorner
	Owners             [2]SurfaceAffineCurvePieces
	BClockRatio        *big.Rat
	ComparedSpans      int
	ExactOperations    int
}

// CertifyProjectiveGraphBoundaryContact proves empty, point or entire edge
// contact. It proves both whole projections before comparing the contact.
// A positive-area common projection belongs to the separate graph overlap
// API. An unequal recovered edge may contain isolated contacts and remains
// unresolved with its exact parameterized owner traces.
func CertifyProjectiveGraphBoundaryContact(ctx context.Context, a, c *SurfacePatch, axes [2]int,
	o SurfaceAffineCurveOptions) (r ProjectiveGraphBoundaryContact, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 || o.MaxPieces < 0 ||
		axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8_000_000
	}
	limits, err := patchOptions(SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return r, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	var projected [2][2]projectiveGraphAxis
	patches := [2]*SurfacePatch{a, c}
	for owner, p := range patches {
		projected[owner], err = certifyProjectiveGraphProjection(ctx, p, axes, limits.MaxPoles, b)
		if err != nil {
			return r, err
		}
	}
	var domain [2]exactInterval
	free, varying := 0, 0
	for i := range domain {
		lo, hi := orderedProjectiveEnds(projected[0][i].ends)
		c0, c1 := orderedProjectiveEnds(projected[1][i].ends)
		if c0.Cmp(lo) > 0 {
			lo = c0
		}
		if c1.Cmp(hi) < 0 {
			hi = c1
		}
		if lo.Cmp(hi) > 0 {
			r.Complete, r.Disjoint = true, true
			return r, nil
		}
		if lo.Cmp(hi) < 0 {
			free, varying = i, varying+1
		}
		domain[i] = exactInterval{lo, hi}
	}
	if varying == 2 {
		return r, ErrDomain
	}
	for i, x := range domain {
		r.Projection[i] = ExactParameterInterval{new(big.Rat).Set(x.lo), new(big.Rat).Set(x.hi)}
	}
	arith := boundedRatArithmetic{b: b}
	one := big.NewRat(1, 1)
	var clock [2]*big.Rat
	for owner, chart := range projected {
		domains := projectiveParameterIntervals(patches[owner].Domain())
		for i, m := range chart {
			k := arith.sub(m.ratio, one)
			width := arith.sub(m.ends[1], m.ends[0])
			raw := domains[m.raw]
			rawWidth := arith.sub(raw.hi, raw.lo)
			var factor [2]*big.Rat
			for j, x := range [2]*big.Rat{domain[i].lo, domain[i].hi} {
				y := arith.quo(arith.sub(x, m.ends[0]), width)
				s := arith.quo(y, arith.sub(m.ratio, arith.mul(k, y)))
				factor[j] = arith.add(one, arith.mul(k, s))
				r.Endpoints[j].OriginalUV[owner][m.raw] = arith.add(raw.lo, arith.mul(rawWidth, s))
			}
			if i == free {
				clock[owner] = arith.quo(factor[1], factor[0])
			}
		}
	}
	if arith.err != nil {
		return r, arith.err
	}
	if varying == 1 {
		r.BClockRatio = arith.quo(clock[0], clock[1])
		if arith.err != nil {
			return r, arith.err
		}
		for owner, p := range patches {
			if b.operations >= b.maxOperations {
				return r, ErrConvergence
			}
			o.MaxPoles, o.MaxOperations, o.MaxCoefficientBits = limits.MaxPoles, b.maxOperations-b.operations, b.maxBits
			r.Owners[owner], err = p.AffineParameterCurvePieces(ctx, r.Endpoints[0].OriginalUV[owner], r.Endpoints[1].OriginalUV[owner], o)
			b.operations += r.Owners[owner].ExactOperations
			if err != nil {
				return r, err
			}
		}
		if b.operations >= b.maxOperations {
			return r, ErrConvergence
		}
		proof, err := CompareAffineParameterCurvePieces(ctx, r.Owners[0], r.Owners[1], ExactCurveComparisonOptions{
			BStationRatio: r.BClockRatio, MaxSpans: o.MaxPieces, MaxOperations: b.maxOperations - b.operations, MaxCoefficientBits: b.maxBits})
		b.operations += proof.ExactOperations
		r.ComparedSpans = proof.ComparedSpans
		if err != nil {
			return r, err
		}
		if !proof.Complete || !proof.Equal {
			return r, ErrConvergence
		}
	}
	for i := 0; i <= varying; i++ {
		var point [3]*big.Rat
		for owner, p := range patches {
			if b.operations >= b.maxOperations {
				return r, ErrConvergence
			}
			uv := r.Endpoints[i].OriginalUV[owner]
			jet, err := p.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{
				MaxOperations: b.maxOperations - b.operations, MaxCoefficientBits: b.maxBits})
			b.operations += jet.ExactOperations
			if err != nil {
				return r, err
			}
			if owner == 0 {
				point = jet.Point
				continue
			}
			for axis, x := range jet.Point {
				if x.Cmp(point[axis]) != 0 {
					if varying == 0 {
						r.Complete, r.Disjoint = true, true
						return r, nil
					}
					return r, ErrConvergence
				}
			}
		}
		r.Endpoints[i].Point = point
	}
	r.Complete, r.Point, r.Edge = true, varying == 0, varying == 1
	return r, nil
}
