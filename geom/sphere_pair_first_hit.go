package geom

import (
	"context"
	"math/big"
)

// ExactSpherePairBoundaryNormal is an outward unnormalized normal at Parameter:
// Constant + t*Linear, in the original model coordinates and raw line units.
// Source is one original operand constraint. Difference cutter normals are
// reversed. All rational values and component Bounds are caller-owned copies.
type ExactSpherePairBoundaryNormal struct {
	Source           BoreBoundaryMask
	Constant, Linear [3]*big.Rat
	Bounds           [3]ExactParameterInterval
	DirectionSign    int
}

// SpherePairFirstHit retains exact exposed support ownership. No hit
// is HasHit=false with nil Parameter. Isolated means isolated in the clipped
// material line, not necessarily an isolated contact on an infinite line.
// Tangent means all active outward normals have zero dot with line direction;
// it is deliberately separate from Isolated and polynomial multiplicity.
type SpherePairFirstHit struct {
	HasHit            bool
	Parameter         *ExactAlgebraicRoot
	Bounds            ExactParameterInterval
	PointBounds       [3]ExactParameterInterval
	Boundary          BoreBoundaryMask
	Normals           []ExactSpherePairBoundaryNormal
	Isolated, Tangent bool
}

// FirstBoundaryHit finds the first exposed boundary on origin+t*direction,
// 0<=t<=maxT, in raw parameter units. It preserves both active sphere normals
// at shared contacts; Difference reverses the second sphere's normal. Exact
// parameter identity is separate from rational model/normal enclosures.
// One bounded ClipLine pass is followed by at most twelve bounded root
// comparisons and two bounded normal sign tests. Failure returns no hit.
func (s SpherePairMaterial) FirstBoundaryHit(ctx context.Context, origin, direction Vec3, maxT float64, o ToroidLineOptions) (SpherePairFirstHit, error) {
	var zero SpherePairFirstHit
	if maxT < 0 {
		return zero, ErrInvalid
	}
	line, e := s.ClipLine(ctx, origin, direction, 0, maxT, o)
	if e != nil {
		return zero, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return zero, e
	}
	ro := ExactCurvePlaneEventOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	var root *ExactAlgebraicRoot
	var mask BoreBoundaryMask
	isolated := false
	add := func(r *ExactAlgebraicRoot, m BoreBoundaryMask, point bool) error {
		if m == 0 {
			return nil
		}
		if root == nil {
			root, mask, isolated = r, m, point
			return nil
		}
		cmp, e := r.Compare(ctx, root, ro)
		if e != nil {
			return e
		}
		if !cmp.Proven {
			return ErrConvergence
		}
		if cmp.Order < 0 {
			root, mask, isolated = r, m, point
		} else if cmp.Order == 0 {
			mask |= m
			isolated = isolated && point
		}
		return nil
	}
	for _, iv := range line.Intervals {
		if e = add(iv.Start, iv.StartBoundary, false); e != nil {
			return zero, e
		}
		if e = add(iv.End, iv.EndBoundary, false); e != nil {
			return zero, e
		}
	}
	for _, p := range line.Points {
		if e = add(p.Parameter, p.Boundary, true); e != nil {
			return zero, e
		}
	}
	if root == nil {
		return zero, nil
	}
	out := SpherePairFirstHit{HasHit: true, Parameter: root, Bounds: root.Interval(), Boundary: mask, Isolated: isolated, Tangent: true}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	org := [3]*big.Rat{q(origin.X), q(origin.Y), q(origin.Z)}
	dir := [3]*big.Rat{q(direction.X), q(direction.Y), q(direction.Z)}
	bounds := func(a, b *big.Rat) (ExactParameterInterval, error) {
		lo := new(big.Rat).Add(a, new(big.Rat).Mul(b, out.Bounds.Lower))
		hi := new(big.Rat).Add(a, new(big.Rat).Mul(b, out.Bounds.Upper))
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		for _, v := range []*big.Rat{lo, hi} {
			if e := budget.check(v); e != nil {
				return ExactParameterInterval{}, e
			}
		}
		return ExactParameterInterval{Lower: lo, Upper: hi}, nil
	}
	for j := 0; j < 3; j++ {
		out.PointBounds[j], e = bounds(org[j], dir[j])
		if e != nil {
			return zero, e
		}
	}
	for owner, source := range []BoreBoundaryMask{SpherePairFirst, SpherePairSecond} {
		if mask&source == 0 {
			continue
		}
		n := ExactSpherePairBoundaryNormal{Source: source}
		derivative := rationalPolynomial{new(big.Rat), new(big.Rat)}
		for j := 0; j < 3; j++ {
			n.Constant[j] = new(big.Rat).Mul(new(big.Rat).Sub(org[j], s.center[owner][j]), big.NewRat(2, 1))
			n.Linear[j] = new(big.Rat).Mul(dir[j], big.NewRat(2, 1))
			if s.op == SphereSetDifference && owner == 1 {
				n.Constant[j].Neg(n.Constant[j])
				n.Linear[j].Neg(n.Linear[j])
			}
			for _, v := range []*big.Rat{n.Constant[j], n.Linear[j]} {
				if e = budget.check(v); e != nil {
					return zero, e
				}
			}
			n.Bounds[j], e = bounds(n.Constant[j], n.Linear[j])
			if e != nil {
				return zero, e
			}
			derivative[0].Add(derivative[0], new(big.Rat).Mul(n.Constant[j], dir[j]))
			derivative[1].Add(derivative[1], new(big.Rat).Mul(n.Linear[j], dir[j]))
		}
		n.DirectionSign, _, e = composedPolynomialRootSign(ctx, root.root, trimPolynomial(derivative), budget, o.MaxSubdivisions)
		if e != nil {
			return zero, e
		}
		out.Tangent = out.Tangent && n.DirectionSign == 0
		out.Normals = append(out.Normals, n)
	}

	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
