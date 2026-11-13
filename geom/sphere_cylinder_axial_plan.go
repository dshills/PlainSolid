package geom

import (
	"context"
	"math/big"
	"sort"
)

// SphereCylinderAxialEvent denotes exact raw axis parameter
// Offset+HeightSign*sqrt(SquaredBase+RadicalSign*sqrt(SquaredRadicand))/AxisScale.
// AxisScale is positive. Supplied finite window ends use HeightSign=0 and
// Offset equal to the represented rational parameter. Bounds are outward.
type SphereCylinderAxialEvent struct {
	Offset, AxisScale, SquaredBase, SquaredRadicand *big.Rat
	HeightSign, RadicalSign                         int
	Bounds                                          [2]*big.Rat
	WindowBoundary                                  bool
}
type SphereCylinderAxialCell struct {
	Lower, Upper     int
	WitnessParameter *big.Rat
	Material         SphereCylinderCapMaterialKind
}
type SphereCylinderAxialPlan struct {
	Complete        bool
	Events          []SphereCylinderAxialEvent
	Cells           []SphereCylinderAxialCell
	ExactOperations int
}

// PlanSphereCylinderAxialCaps exhausts cap-material regimes for a finite raw
// axis window, including empty, full cylinder disk, full sphere disk and
// partial lens cases. Exact circle-tangency and sphere-pole events are retained
// symbolically; bounded outward root intervals produce rational open-cell
// witnesses. Very close distinct algebraic cuts may return ErrConvergence.
// Events themselves require their exact descriptor for topology; witness kinds
// describe open cells only. The axis must be coordinate-aligned.
func PlanSphereCylinderAxialCaps(ctx context.Context, center Vec3, R float64, origin, axis Vec3, radius float64, window [2]*big.Rat, o SurfacePatchEvaluationOptions) (out SphereCylinderAxialPlan, err error) {
	if ctx == nil || window[0] == nil || window[1] == nil || window[0].Cmp(window[1]) >= 0 {
		return out, ErrInvalid
	}
	// Reuse source validation, including raw axis and radius contracts.
	validation, e := IntersectSphereCylinderCap(ctx, center, R, origin, axis, radius, window[0], o)
	if e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: validation.ExactOperations}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.Events = nil
			out.Cells = nil
		}
	}()
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	ax := 0
	if axis.Y != 0 {
		ax = 1
	} else if axis.Z != 0 {
		ax = 2
	}
	scale := new(big.Rat).Abs(r(coordinate(axis, ax)))
	offset := new(big.Rat).Quo(new(big.Rat).Sub(r(coordinate(center, ax)), r(coordinate(origin, ax))), r(coordinate(axis, ax)))
	H := new(big.Rat)
	for i := 0; i < 3; i++ {
		if i != ax {
			d := new(big.Rat).Sub(r(coordinate(center, i)), r(coordinate(origin, i)))
			H.Add(H, new(big.Rat).Mul(d, d))
		}
	}
	R2, r2 := new(big.Rat).Mul(r(R), r(R)), new(big.Rat).Mul(r(radius), r(radius))
	A := new(big.Rat).Sub(new(big.Rat).Sub(R2, H), r2)
	B := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(r2, H))
	for _, q := range []*big.Rat{offset, scale, H, A, B} {
		if e = b.check(q); e != nil {
			return out, e
		}
	}
	// Sign of base+sign*sqrt(B), without evaluating the radical.
	radicalSign := func(base *big.Rat, sign int) int {
		if sign == 0 || B.Sign() == 0 {
			return base.Sign()
		}
		if sign > 0 {
			if base.Sign() >= 0 {
				return 1
			}
			return B.Cmp(new(big.Rat).Mul(base, base))
		}
		if base.Sign() <= 0 {
			return -1
		}
		return new(big.Rat).Mul(base, base).Cmp(B)
	}
	type squared struct {
		base, rad *big.Rat
		sign      int
	}
	heights := []squared{{new(big.Rat), new(big.Rat), 0}, {new(big.Rat).Set(R2), new(big.Rat), 0}}
	for _, s := range []int{-1, 1} {
		if radicalSign(A, s) > 0 {
			if B.Sign() == 0 {
				heights = append(heights, squared{new(big.Rat).Set(A), new(big.Rat), 0})
			} else {
				heights = append(heights, squared{new(big.Rat).Set(A), new(big.Rat).Set(B), s})
			}
		}
	}
	// Equality with zero/poles and duplicate coaxial thresholds is exact.
	unique := map[string]bool{}
	var candidates []SphereCylinderAxialEvent
	for _, h := range heights {
		for _, sgn := range []int{-1, 1} {
			if h.base.Sign() == 0 && h.rad.Sign() == 0 && sgn < 0 {
				continue
			}
			if h.sign != 0 && radicalSign(new(big.Rat).Sub(h.base, R2), h.sign) == 0 {
				h = squared{new(big.Rat).Set(R2), new(big.Rat), 0}
			}
			key := h.base.RatString() + ":" + h.rad.RatString() + ":" + big.NewInt(int64(h.sign)).String() + ":" + big.NewInt(int64(sgn)).String()
			if unique[key] {
				continue
			}
			unique[key] = true
			candidates = append(candidates, SphereCylinderAxialEvent{Offset: new(big.Rat).Set(offset), AxisScale: new(big.Rat).Set(scale), SquaredBase: new(big.Rat).Set(h.base), SquaredRadicand: new(big.Rat).Set(h.rad), HeightSign: sgn, RadicalSign: h.sign})
		}
	}
	enclose := func(ev *SphereCylinderAxialEvent, precision int) error {
		lo, hi := new(big.Rat).Set(ev.SquaredBase), new(big.Rat).Set(ev.SquaredBase)
		if ev.RadicalSign != 0 {
			a, c, e := rationalSqrtBounds(ev.SquaredRadicand, precision, b)
			if e != nil {
				return e
			}
			if ev.RadicalSign > 0 {
				lo.Add(lo, a)
				hi.Add(hi, c)
			} else {
				lo.Sub(lo, c)
				hi.Sub(hi, a)
			}
		}
		if lo.Sign() < 0 {
			lo.SetInt64(0)
		}
		if hi.Sign() < 0 {
			return ErrConvergence
		}
		l, _, e := rationalSqrtBounds(lo, precision, b)
		if e != nil {
			return e
		}
		_, h, e := rationalSqrtBounds(hi, precision, b)
		if e != nil {
			return e
		}
		l.Quo(l, ev.AxisScale)
		h.Quo(h, ev.AxisScale)
		if ev.HeightSign < 0 {
			l, h = new(big.Rat).Neg(h), new(big.Rat).Neg(l)
		}
		ev.Bounds = [2]*big.Rat{new(big.Rat).Add(ev.Offset, l), new(big.Rat).Add(ev.Offset, h)}
		return nil
	}
	// Compare an event to rational t by moving to signed physical height and
	// squaring only after confirming the same sign. This includes exact caps.
	cmp := func(ev SphereCylinderAxialEvent, t *big.Rat) int {
		v := new(big.Rat).Mul(new(big.Rat).Sub(t, offset), scale)
		if ev.HeightSign < 0 {
			v.Neg(v)
		}
		var c int
		if v.Sign() < 0 {
			c = 1
		} else {
			base := new(big.Rat).Sub(ev.SquaredBase, new(big.Rat).Mul(v, v))
			if ev.RadicalSign == 0 {
				c = base.Sign()
			} else {
				c = radicalSign(base, ev.RadicalSign)
			}
		}
		if ev.HeightSign < 0 {
			c = -c
		}
		return c
	}
	for i := range candidates {
		ev := &candidates[i]
		if cmp(*ev, window[0]) <= 0 || cmp(*ev, window[1]) >= 0 {
			continue
		}
		if e = enclose(ev, 128); e != nil {
			return out, e
		}
		out.Events = append(out.Events, *ev)
	}
	for _, v := range window {
		out.Events = append(out.Events, SphereCylinderAxialEvent{Offset: new(big.Rat).Set(v), AxisScale: big.NewRat(1, 1), SquaredBase: new(big.Rat), SquaredRadicand: new(big.Rat), Bounds: [2]*big.Rat{new(big.Rat).Set(v), new(big.Rat).Set(v)}, WindowBoundary: true})
	}
	sort.Slice(out.Events, func(i, j int) bool { return out.Events[i].Bounds[0].Cmp(out.Events[j].Bounds[0]) < 0 })
	for precision := 256; precision <= 1024; precision *= 2 {
		overlap := false
		for i := 0; i+1 < len(out.Events); i++ {
			if out.Events[i].Bounds[1].Cmp(out.Events[i+1].Bounds[0]) >= 0 {
				overlap = true
			}
		}
		if !overlap {
			break
		}
		for i := range out.Events {
			if !out.Events[i].WindowBoundary {
				if e = enclose(&out.Events[i], precision); e != nil {
					return out, e
				}
			}
		}
		sort.Slice(out.Events, func(i, j int) bool { return out.Events[i].Bounds[0].Cmp(out.Events[j].Bounds[0]) < 0 })
	}
	for i := 0; i+1 < len(out.Events); i++ {
		if out.Events[i].Bounds[1].Cmp(out.Events[i+1].Bounds[0]) >= 0 {
			return out, ErrConvergence
		}
		w := new(big.Rat).Mul(new(big.Rat).Add(out.Events[i].Bounds[1], out.Events[i+1].Bounds[0]), big.NewRat(1, 2))
		if b.operations >= ops {
			return out, ErrConvergence
		}
		oo := o
		oo.MaxOperations = ops - b.operations
		q, e := ClassifySphereCylinderCapMaterial(ctx, center, R, origin, axis, radius, w, oo)
		b.operations += q.ExactOperations
		if e != nil {
			return out, e
		}
		out.Cells = append(out.Cells, SphereCylinderAxialCell{i, i + 1, w, q.Kind})
	}
	out.Complete = true
	return out, nil
}
