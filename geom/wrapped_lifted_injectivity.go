package geom

import (
	"context"
	"math/big"
)

// WrappedLiftedInjectivityCell is a certificate on an original fundamental
// restriction. LiftedDomain = Source.Domain + PeriodShift*original period,
// exactly, without rounding the translated knot or subdivision parameters.
// Every cell uses the same fixed projection preconditioner.
type WrappedLiftedInjectivityCell struct {
	Source       SurfaceInjectivityCell
	LiftedDomain [2][2]*big.Rat
	PeriodShift  [2]*big.Int
}

// WrappedLiftedRectangleInjectivity proves injectivity on the full closed,
// convex lifted Domain, including its seams. It does not identify opposite
// boundary points or certify a solid. Fields and exact numbers are caller-owned.
type WrappedLiftedRectangleInjectivity struct {
	ProvenInjective                                  bool
	Domain                                           ParameterRectangle
	ProjectionAxes                                   [2]int
	ProjectionOrientation                            int
	FundamentalPieces, VisitedCells, ExactOperations int
	Preconditioner                                   [2][2]*big.Rat
	Cells                                            []WrappedLiftedInjectivityCell
}

// CertifyLiftedRectangleInjectivity permits true fundamental-seam crossings.
// Each wrapped width must be strictly smaller than its exact period. At most
// four fundamental rectangles cover the lifted domain. Exact C0 seam identity
// comes from this immutable wrapper's construction; original knots are C0.
// One fixed exact inverse Jacobian preconditions ALL restrictions. Uniformly
// positive symmetric parts imply strict monotonicity by integration along any
// lifted straight segment, including across seams/knots. Separate local
// injectivity certificates would not suffice and are never combined here.
// Source restriction cuts must be representable; translated reported cells
// remain rational. Options have SurfaceInjectivityOptions defaults. Failure or
// cancellation returns no partial proof. This is a sufficient, not necessary,
// embedding criterion and does not change other wrapped embedding APIs.
func (w *WrappedSurfacePatch) CertifyLiftedRectangleInjectivity(ctx context.Context, d ParameterRectangle, o SurfaceInjectivityOptions) (WrappedLiftedRectangleInjectivity, error) {
	var zero WrappedLiftedRectangleInjectivity
	if ctx == nil || w == nil || w.patch == nil || !finite(d.UMin) || !finite(d.UMax) || !finite(d.VMin) || !finite(d.VMax) || d.UMin >= d.UMax || d.VMin >= d.VMax {
		return zero, ErrInvalid
	}
	if o.ProjectionAxes == ([2]int{}) {
		o.ProjectionAxes = [2]int{0, 1}
	}
	axes := o.ProjectionAxes
	if axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return zero, ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	base := w.Domain()
	sourceDomain := [2][2]float64{{base.UMin, base.UMax}, {base.VMin, base.VMax}}
	lifted := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	type interval struct {
		lo, hi float64
		turn   *big.Int
		shift  *big.Rat
	}
	var cuts [2][]interval
	for axis := 0; axis < 2; axis++ {
		lo, hi := new(big.Rat).SetFloat64(lifted[axis][0]), new(big.Rat).SetFloat64(lifted[axis][1])
		min, max := new(big.Rat).SetFloat64(sourceDomain[axis][0]), new(big.Rat).SetFloat64(sourceDomain[axis][1])
		for _, q := range []*big.Rat{lo, hi, min, max} {
			if e := budget.check(q); e != nil {
				return zero, e
			}
		}
		period := w.period[axis]
		if period == nil {
			if lo.Cmp(min) < 0 || hi.Cmp(max) > 0 {
				return zero, ErrDomain
			}
			cuts[axis] = []interval{{lifted[axis][0], lifted[axis][1], new(big.Int), new(big.Rat)}}
			continue
		}
		if !w.seam.PositionMatched[axis] {
			return zero, ErrConvergence
		}
		width := new(big.Rat).Sub(hi, lo)
		if e := budget.check(width); e != nil {
			return zero, e
		}
		if width.Cmp(period) >= 0 {
			return zero, ErrConvergence
		}
		quotient := new(big.Rat).Quo(new(big.Rat).Sub(lo, min), period)
		if e := budget.check(quotient); e != nil {
			return zero, e
		}
		turns, rem := new(big.Int), new(big.Int)
		turns.QuoRem(quotient.Num(), quotient.Denom(), rem)
		if rem.Sign() < 0 {
			turns.Sub(turns, big.NewInt(1))
		}
		shift := new(big.Rat).Mul(new(big.Rat).SetInt(turns), period)
		if e := budget.check(shift); e != nil {
			return zero, e
		}
		lo.Sub(lo, shift)
		hi.Sub(hi, shift)
		add := func(a, b *big.Rat, k *big.Int, s *big.Rat) error {
			for _, q := range []*big.Rat{a, b, s} {
				if e := budget.check(q); e != nil {
					return e
				}
			}
			x, e := exactRepresentableFloat(a)
			if e != nil {
				return e
			}
			y, e := exactRepresentableFloat(b)
			if e != nil {
				return e
			}
			if x >= y {
				return ErrConvergence
			}
			cuts[axis] = append(cuts[axis], interval{x, y, new(big.Int).Set(k), new(big.Rat).Set(s)})
			return nil
		}
		if hi.Cmp(max) <= 0 {
			if e := add(lo, hi, turns, shift); e != nil {
				return zero, e
			}
		} else {
			if e := add(lo, max, turns, shift); e != nil {
				return zero, e
			}
			if e := add(min, new(big.Rat).Sub(hi, period), new(big.Int).Add(turns, big.NewInt(1)), new(big.Rat).Add(shift, period)); e != nil {
				return zero, e
			}
		}
	}
	type node struct {
		p     *SurfacePatch
		depth int
		turns [2]*big.Int
		shift [2]*big.Rat
	}
	var pieces []node
	for _, u := range cuts[0] {
		for _, v := range cuts[1] {
			if e := ctx.Err(); e != nil {
				return zero, e
			}
			p, e := trimPatchBudget(w.patch, ParameterRectangle{UMin: u.lo, UMax: u.hi, VMin: v.lo, VMax: v.hi}, budget)
			if e != nil {
				return zero, e
			}
			pieces = append(pieces, node{p: p, turns: [2]*big.Int{u.turn, v.turn}, shift: [2]*big.Rat{u.shift, v.shift}})
		}
	}
	result := WrappedLiftedRectangleInjectivity{Domain: d, ProjectionAxes: axes, FundamentalPieces: len(pieces)}
	// Pick one exact midpoint only to choose a fixed matrix, never as evidence
	// about the sign or injectivity on any interval.
	centerPatch := pieces[0].p
	domain := centerPatch.Domain()
	center := [2]*big.Rat{new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).SetFloat64(domain.UMin), new(big.Rat).SetFloat64(domain.UMax)), big.NewRat(2, 1)), new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).SetFloat64(domain.VMin), new(big.Rat).SetFloat64(domain.VMax)), big.NewRat(2, 1))}
	for {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		t, u, knot := patchSplitChoice(centerPatch)
		if !knot {
			break
		}
		if result.VisitedCells >= o.MaxCells {
			return zero, ErrConvergence
		}
		result.VisitedCells++
		left, right, e := centerPatch.split(t, u, budget)
		if e != nil {
			return zero, e
		}
		axis := 1
		if u {
			axis = 0
		}
		if center[axis].Cmp(new(big.Rat).SetFloat64(t)) < 0 {
			centerPatch = left
		} else {
			centerPatch = right
		}
	}
	_, derivative, e := exactSurfaceJet(centerPatch, center[0], center[1], budget)
	if e != nil {
		return zero, e
	}
	a, b, c, f := derivative[axes[0]][0], derivative[axes[0]][1], derivative[axes[1]][0], derivative[axes[1]][1]
	det := new(big.Rat).Sub(new(big.Rat).Mul(a, f), new(big.Rat).Mul(b, c))
	if e := budget.check(det); e != nil {
		return zero, e
	}
	if det.Sign() == 0 {
		return zero, ErrConvergence
	}
	inverse := [2][2]*big.Rat{{new(big.Rat).Quo(f, det), new(big.Rat).Quo(new(big.Rat).Neg(b), det)}, {new(big.Rat).Quo(new(big.Rat).Neg(c), det), new(big.Rat).Quo(a, det)}}
	for _, row := range inverse {
		for _, q := range row {
			if e := budget.check(q); e != nil {
				return zero, e
			}
		}
	}
	stack := append([]node(nil), pieces...)
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		if result.VisitedCells >= o.MaxCells {
			return zero, ErrConvergence
		}
		result.VisitedCells++
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		t, u, knot := patchSplitChoice(n.p)
		if !knot {
			cell, ok, e := projectedSymmetricCertificate(n.p, axes, inverse, budget)
			if e != nil {
				return zero, e
			}
			if ok {
				cell.Depth = n.depth
				entry := WrappedLiftedInjectivityCell{Source: cell}
				bounds := [2][2]float64{{cell.Domain.UMin, cell.Domain.UMax}, {cell.Domain.VMin, cell.Domain.VMax}}
				for axis := 0; axis < 2; axis++ {
					entry.PeriodShift[axis] = new(big.Int).Set(n.turns[axis])
					for end := 0; end < 2; end++ {
						q := new(big.Rat).Add(new(big.Rat).SetFloat64(bounds[axis][end]), n.shift[axis])
						if e := budget.check(q); e != nil {
							return zero, e
						}
						entry.LiftedDomain[axis][end] = q
					}
				}
				result.Cells = append(result.Cells, entry)
				continue
			}
		}
		d := n.p.Domain()
		if n.depth >= o.MaxDepth || (u && (t == d.UMin || t == d.UMax)) || (!u && (t == d.VMin || t == d.VMax)) {
			return zero, ErrConvergence
		}
		left, right, e := n.p.split(t, u, budget)
		if e != nil {
			return zero, e
		}
		l, r := n, n
		l.p = left
		r.p = right
		l.depth++
		r.depth++
		stack = append(stack, r, l)
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	result.ProvenInjective = true
	result.ProjectionOrientation = det.Sign()
	result.Preconditioner = inverse
	result.ExactOperations = budget.operations
	return result, nil
}
