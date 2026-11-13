package geom

import (
	"context"
	"math/big"
)

// ExactSphereCylinderBoundaryNormal is an outward unnormalized normal at Parameter:
// Constant + t*Linear, in the original model coordinates and raw line units.
// Source is one original operand constraint. Difference cutter normals are
// reversed. All rational values and component Bounds are caller-owned copies.
type ExactSphereCylinderBoundaryNormal struct {
	Source           BoreBoundaryMask
	Constant, Linear [3]*big.Rat
	Bounds           [3]ExactParameterInterval
	DirectionSign    int
}

// SphereCylinderFirstHit retains exact exposed support ownership. No hit
// is HasHit=false with nil Parameter. Isolated means isolated in the clipped
// material line, not necessarily an isolated contact on an infinite line.
// Tangent means all active outward normals have zero dot with line direction;
// it is deliberately separate from Isolated and polynomial multiplicity.
type SphereCylinderFirstHit struct {
	HasHit            bool
	Parameter         *ExactAlgebraicRoot
	Bounds            ExactParameterInterval
	PointBounds       [3]ExactParameterInterval
	Boundary          BoreBoundaryMask
	Normals           []ExactSphereCylinderBoundaryNormal
	Isolated, Tangent bool
}

// FirstBoundaryHit returns the first exposed boundary on the finite raw ray
// origin+t*direction, 0<=t<=maxT. Normal polynomials and rational enclosures
// retain exact algebraic event provenance. Artificial range endpoints inside
// material and hidden supports are not hits. Each query uses one bounded clip
// plus at most sixteen bounded root comparisons and four normal sign tests.
func (s OffAxisSphereFiniteCylinderDifference) FirstBoundaryHit(ctx context.Context, origin, direction Vec3, maxT float64, o ToroidLineOptions) (SphereCylinderFirstHit, error) {
	if maxT < 0 {
		return SphereCylinderFirstHit{}, ErrInvalid
	}
	line, e := s.ClipLine(ctx, origin, direction, 0, maxT, o)
	if e != nil {
		return SphereCylinderFirstHit{}, e
	}
	return s.model.firstMaterialHit(ctx, origin, direction, line, true, o)
}

// FirstBoundaryHit preserves all exposed original supports at union contacts;
// a touching union is not assigned a fabricated single smooth normal.
func (s OffAxisSphereFiniteCylinderUnion) FirstBoundaryHit(ctx context.Context, origin, direction Vec3, maxT float64, o ToroidLineOptions) (SphereCylinderFirstHit, error) {
	if maxT < 0 {
		return SphereCylinderFirstHit{}, ErrInvalid
	}
	line, e := s.ClipLine(ctx, origin, direction, 0, maxT, o)
	if e != nil {
		return SphereCylinderFirstHit{}, e
	}
	return s.model.firstMaterialHit(ctx, origin, direction, line, false, o)
}

func (s OffAxisSphereFiniteCylinderCommon) firstMaterialHit(ctx context.Context, origin, direction Vec3, line SphereCylinderMaterialLine, difference bool, o ToroidLineOptions) (SphereCylinderFirstHit, error) {
	var zero SphereCylinderFirstHit
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
	out := SphereCylinderFirstHit{HasHit: true, Parameter: root, Bounds: root.Interval(), Boundary: mask, Isolated: isolated, Tangent: true}
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
	for _, source := range []BoreBoundaryMask{BoreSphere, BoreCylinder, BoreLowerCap, BoreUpperCap} {
		if mask&source == 0 {
			continue
		}
		n := ExactSphereCylinderBoundaryNormal{Source: source}
		derivative := rationalPolynomial{new(big.Rat), new(big.Rat)}
		for j := 0; j < 3; j++ {
			n.Constant[j], n.Linear[j] = new(big.Rat), new(big.Rat)
			if source == BoreSphere {
				n.Constant[j].Mul(org[j], big.NewRat(2, 1))
				n.Linear[j].Mul(dir[j], big.NewRat(2, 1))
			} else if source == BoreCylinder && j < 2 {
				center := s.cx
				if j == 1 {
					center = s.cy
				}
				n.Constant[j].Mul(new(big.Rat).Sub(org[j], center), big.NewRat(2, 1))
				n.Linear[j].Mul(dir[j], big.NewRat(2, 1))
			} else if j == 2 && source == BoreLowerCap {
				n.Constant[j].SetInt64(-1)
			} else if j == 2 && source == BoreUpperCap {
				n.Constant[j].SetInt64(1)
			}
			if difference && source != BoreSphere {
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
