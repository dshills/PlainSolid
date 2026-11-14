package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphDirectionalWitness proves an extremal value using one
// original directed cap use. Ties are valid: the chosen witness is the
// first in original cap/wire/use order, without asserting uniqueness.
// ConstantUse means the entire selected use has the extremal value.
type AlgebraicGraphDirectionalWitness struct {
	Value, CurveParameter *geom.ExactAlgebraicRoot
	Cap, Wire, Use        int
	ConstantUse           bool
}

// AlgebraicGraphDirectionalBounds contains the true min/max of the
// supplied unnormalized linear functional on the entire retained solid.
type AlgebraicGraphDirectionalBounds struct {
	Min, Max                      AlgebraicGraphDirectionalWitness
	ExactOperations, Subdivisions int
}

// BoundDirectionRational proves extrema of direction·point on the entire
// stock. Every material fiber is bounded by the two cap graphs, and a
// nonconstant affine function on each planar cap reaches its extremes
// on the exact original trim. Constant cap values have valid trim
// witnesses as well. Curve or root degrees outside the bounded solver
// fail without returning partial extrema.
func (m AlgebraicGraphVerticalMaterial) BoundDirectionRational(ctx context.Context, direction [3]*big.Rat,
	opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphDirectionalBounds, error) {
	return m.boundDirectionRational(ctx, direction, nil, opts)
}

func (m AlgebraicGraphVerticalMaterial) boundDirectionRational(ctx context.Context, direction [3]*big.Rat,
	offset *big.Rat, opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphDirectionalBounds, error) {
	var zero AlgebraicGraphDirectionalBounds
	if m.caps[0].recipe == nil {
		return zero, geom.ErrInvalid
	}
	if !m.lineReady {
		return zero, geom.ErrUnrepresentable
	}
	if opts.MaxCoefficientBits == 0 {
		opts.MaxCoefficientBits = m.maxBits
	}
	b, err := newCapClipBudget(ctx, opts)
	if err != nil {
		return zero, err
	}
	maxWork, maxSubdiv := b.work, b.subdiv
	for _, d := range direction {
		if d == nil {
			return zero, geom.ErrInvalid
		}
		if max(d.Num().BitLen(), d.Denom().BitLen()) > b.bits {
			return zero, geom.ErrConvergence
		}
	}
	if direction[0].Sign() == 0 && direction[1].Sign() == 0 && direction[2].Sign() == 0 {
		return zero, geom.ErrDegenerate
	}
	form := [4]*big.Rat{direction[0], direction[1], direction[2], offset}
	var out AlgebraicGraphDirectionalBounds
	cmp := func(a, z *geom.ExactAlgebraicRoot) (int, error) {
		q, e := a.Compare(ctx, z, b.limits())
		if e != nil {
			return 0, e
		}
		if e = b.charge(q.Operations, q.Subdivisions); e != nil {
			return 0, e
		}
		if !q.Proven {
			return 0, geom.ErrConvergence
		}
		return q.Order, nil
	}
	for ci, cap := range m.caps {
		for wi, wire := range cap.wires {
			for ui, use := range wire {
				stage, err := use.Cell.LinearExtremaCandidates(ctx, form, b.limits())
				if err != nil {
					return zero, err
				}
				if err = b.charge(stage.Operations, stage.Subdivisions); err != nil {
					return zero, err
				}
				for _, v := range stage.Candidates {
					w := AlgebraicGraphDirectionalWitness{v.Value, v.Parameter, ci, wi, ui, stage.Constant}
					if out.Min.Value == nil {
						out.Min, out.Max = w, w
						continue
					}
					c, e := cmp(w.Value, out.Min.Value)
					if e != nil {
						return zero, e
					}
					if c < 0 {
						out.Min = w
					}
					c, e = cmp(w.Value, out.Max.Value)
					if e != nil {
						return zero, e
					}
					if c > 0 {
						out.Max = w
					}
				}
			}
		}
	}
	if out.Min.Value == nil {
		return zero, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.ExactOperations, out.Subdivisions = maxWork-b.work, maxSubdiv-b.subdiv
	return out, nil
}

// BoundDirectionRational proves the support range in world units without
// rebaking the original stock. The occurrence's rotation pulls back the
// functional and its translation contributes the exact constant offset.
func (o AlgebraicGraphStockOccurrence) BoundDirectionRational(ctx context.Context, direction [3]*big.Rat,
	opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphDirectionalBounds, error) {
	var zero AlgebraicGraphDirectionalBounds
	d, e := o.pose.TransformVector(ctx, direction, true, 4096)
	if e != nil {
		return zero, e
	}
	m, e := o.pose.Matrix()
	if e != nil {
		return zero, e
	}
	offset := new(big.Rat)
	for i, q := range direction {
		offset.Add(offset, new(big.Rat).Mul(m[i][3], q))
	}
	return o.model.boundDirectionRational(ctx, d, offset, opts)
}
