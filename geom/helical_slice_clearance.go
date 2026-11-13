package geom

import (
	"context"
	"math/big"
)

// HelicalAxialSlice is an exact closed axial-offset interval at one radius.
// Edge indices refer to the original input polygon. No slice means that the
// radius is outside the closed polygon projection.
type HelicalAxialSlice struct {
	Present                bool
	Lower, Upper           *big.Rat
	LowerEdges, UpperEdges []int
}

// HelicalSliceClearance compares profiles at equal physical turn, after adding
// TargetOffset to B. SignedClearance=max(B.low-A.high,A.low-B.high): positive
// is strict separation, zero is contact, negative is the negative minimum axial
// translation needed to separate these intervals. In particular it is not
// simply overlap length when one slice contains the other. Comparison applies
// only at Radius and CommonTurns; different turn numbers are not compared.
// All rational outputs and edge lists are caller-owned.
type HelicalSliceClearance struct {
	Complete, Comparable bool
	Radius, TargetOffset *big.Rat
	CommonTurns          [2]*big.Rat
	A, B                 HelicalAxialSlice
	SignedClearance      *big.Rat
	ExactOperations      int
}

// CompareHelicalMeridianSlices requires identical represented frame, lead and
// handedness, so the common lead*turn term cancels exactly. It supports separate
// finite turn windows and rational, including nonbinary, target axial offsets.
// Empty radial slices or disjoint turn windows return Complete but not
// Comparable; this is not a global separation certificate for the whole sweeps.
func CompareHelicalMeridianSlices(ctx context.Context, a, c HelicalPolygon, radius, targetOffset *big.Rat, o SurfacePatchEvaluationOptions) (out HelicalSliceClearance, err error) {
	if ctx == nil || len(a.profile) < 3 || len(c.profile) < 3 || radius == nil || targetOffset == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if a.reference.frame != c.reference.frame || a.reference.lead != c.reference.lead || a.reference.handedness != c.reference.handedness {
		return out, ErrConvergence
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.Comparable = false
		}
	}()
	for _, q := range []*big.Rat{radius, targetOffset} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	out.Radius = new(big.Rat).Set(radius)
	out.TargetOffset = new(big.Rat).Set(targetOffset)
	lower, upper := a.reference.domain.UMin, a.reference.domain.UMax
	if c.reference.domain.UMin > lower {
		lower = c.reference.domain.UMin
	}
	if c.reference.domain.UMax < upper {
		upper = c.reference.domain.UMax
	}
	if lower > upper {
		out.Complete = true
		return out, nil
	}
	out.CommonTurns = [2]*big.Rat{new(big.Rat).SetFloat64(lower), new(big.Rat).SetFloat64(upper)}
	var e error
	out.A, e = exactHelicalPolygonSlice(ctx, a.profile, radius, b)
	if e != nil {
		return out, e
	}
	out.B, e = exactHelicalPolygonSlice(ctx, c.profile, radius, b)
	if e != nil {
		return out, e
	}
	if !out.A.Present || !out.B.Present {
		out.Complete = true
		return out, nil
	}
	out.B.Lower.Add(out.B.Lower, targetOffset)
	out.B.Upper.Add(out.B.Upper, targetOffset)
	forward := new(big.Rat).Sub(out.B.Lower, out.A.Upper)
	reverse := new(big.Rat).Sub(out.A.Lower, out.B.Upper)
	out.SignedClearance = forward
	if reverse.Cmp(forward) > 0 {
		out.SignedClearance = reverse
	}
	for _, q := range []*big.Rat{out.B.Lower, out.B.Upper, out.SignedClearance} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	out.Comparable = true
	out.Complete = true
	return out, nil
}

func exactHelicalPolygonSlice(ctx context.Context, profile []Vec2, radius *big.Rat, b *polynomialBudget) (out HelicalAxialSlice, err error) {
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	add := func(q *big.Rat, edge int) {
		if !out.Present || q.Cmp(out.Lower) < 0 {
			out.Lower = new(big.Rat).Set(q)
			out.LowerEdges = []int{edge}
		} else if q.Cmp(out.Lower) == 0 {
			out.LowerEdges = append(out.LowerEdges, edge)
		}
		if !out.Present || q.Cmp(out.Upper) > 0 {
			out.Upper = new(big.Rat).Set(q)
			out.UpperEdges = []int{edge}
		} else if q.Cmp(out.Upper) == 0 {
			out.UpperEdges = append(out.UpperEdges, edge)
		}
		out.Present = true
	}
	for i, a := range profile {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		c := profile[(i+1)%len(profile)]
		dx := new(big.Rat).Sub(rat(c.X), rat(a.X))
		if dx.Sign() == 0 {
			if radius.Cmp(rat(a.X)) == 0 {
				add(rat(a.Y), i)
				add(rat(c.Y), i)
			}
			continue
		}
		t := new(big.Rat).Quo(new(big.Rat).Sub(radius, rat(a.X)), dx)
		if e := b.check(t); e != nil {
			return out, e
		}
		if t.Sign() < 0 || t.Cmp(big.NewRat(1, 1)) > 0 {
			continue
		}
		q := new(big.Rat).Add(rat(a.Y), new(big.Rat).Mul(t, new(big.Rat).Sub(rat(c.Y), rat(a.Y))))
		if e := b.check(q); e != nil {
			return out, e
		}
		add(q, i)
	}
	if out.Present {
		for _, q := range []*big.Rat{out.Lower, out.Upper} {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
	}
	return out, nil
}
