package intersect

import (
	"context"
	"errors"
	"math"
	"math/big"

	"plainsolid/geom"
)

// SplineTubeTerminal proposes an exact rational terminal on one original
// face side. Proof verifies the point independently against the tube.
type SplineTubeTerminal struct {
	Operand int
	Side    geom.SurfaceEdge
	Proof   geom.TubeEndpointEdgeCertificate
}

// DiscoverRationalSplineTubeTerminal recovers a bounded rational candidate
// from each certified endpoint interval and checks the result exactly.
// The terminal must be on the tube's free-coordinate face side. General
// algebraic terminals remain unresolved.
func DiscoverRationalSplineTubeTerminal(ctx context.Context, s TubeSegment, end int, a, b Patch,
	o geom.TubeEndpointEdgeOptions) (SplineTubeTerminal, error) {
	var out SplineTubeTerminal
	if ctx == nil || s.pa == nil || s.axis < 0 || s.axis > 3 || end < 0 || end > 1 || !a.valid() || !b.valid() ||
		o.MaxOperations < 0 || o.MaxCoefficientBits < 0 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	upper := (end == 1) != s.reversed
	operand := s.axis / 2
	side := geom.SurfaceEdge(2 * (s.axis % 2))
	if upper {
		side++
	}
	patch := [2]Patch{a, b}[operand]
	d := geom.ParameterRectangle{UMin: patch.Min.X, UMax: patch.Max.X, VMin: patch.Min.Y, VMax: patch.Max.Y}
	free := s.lo
	if upper {
		free = s.hi
	}
	boundary := [4]float64{d.UMin, d.UMax, d.VMin, d.VMax}[side]
	if free != boundary {
		return out, geom.ErrConvergence
	}
	x, e := s.Evaluate(float64(end))
	if e != nil {
		return out, e
	}
	lo := [4]float64{x.ParametersA.Min.X, x.ParametersA.Min.Y, x.ParametersB.Min.X, x.ParametersB.Min.Y}
	hi := [4]float64{x.ParametersA.Max.X, x.ParametersA.Max.Y, x.ParametersB.Max.X, x.ParametersB.Max.Y}
	var candidates [4][]*big.Rat
	for i := range candidates {
		if i == s.axis {
			candidates[i] = []*big.Rat{new(big.Rat).SetFloat64(free)}
			continue
		}
		candidates[i] = smallRationalBracketCandidates(lo[i], hi[i])
		if len(candidates[i]) == 0 {
			return out, geom.ErrConvergence
		}
	}
	limit := o.MaxOperations
	if limit == 0 {
		limit = 1000000
	}
	total := 0
	var hint [4]*big.Rat
	var search func(int) error
	search = func(axis int) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if axis < 4 {
			for _, t := range candidates[axis] {
				hint[axis] = t
				if e := search(axis + 1); e != nil || out.Proof.Proven {
					return e
				}
			}
			return nil
		}
		if total >= limit {
			return geom.ErrConvergence
		}
		proof, e := s.CertifyEndpointOnEdge(ctx, end, operand, d, side, hint,
			geom.TubeEndpointEdgeOptions{MaxOperations: limit - total, MaxCoefficientBits: o.MaxCoefficientBits})
		total += proof.ExactOperations
		if e != nil {
			if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDomain) {
				return nil
			}
			return e
		}
		if proof.Proven {
			out.Operand, out.Side, out.Proof = operand, side, proof
			out.Proof.ExactOperations = total
		}
		return nil
	}
	if e = search(0); e != nil {
		return out, e
	}
	if !out.Proof.Proven {
		return out, geom.ErrConvergence
	}
	return out, nil
}

func smallRationalBracketCandidates(lo, hi float64) []*big.Rat {
	if !finite(lo) || !finite(hi) || lo > hi {
		return nil
	}
	a, b := new(big.Rat).SetFloat64(lo), new(big.Rat).SetFloat64(hi)
	var out []*big.Rat
	add := func(q *big.Rat) {
		if q.Cmp(a) < 0 || q.Cmp(b) > 0 {
			return
		}
		for _, t := range out {
			if t.Cmp(q) == 0 {
				return
			}
		}
		if len(out) < 6 {
			out = append(out, q)
		}
	}
	mid := lo/2 + hi/2
	// Try rational fractions with modest denominators and exact dyadic
	// grids first; rounded candidates are used only as proposals.
	if math.Abs(mid) < 1<<45 {
		for k := int64(1); k <= 256; k++ {
			add(big.NewRat(int64(math.Round(mid*float64(k))), k))
		}
		for k := 9; k <= 40; k++ {
			v := math.Ldexp(math.Round(math.Ldexp(mid, k)), -k)
			if finite(v) {
				add(new(big.Rat).SetFloat64(v))
			}
		}
	}
	add(a)
	add(b)
	add(new(big.Rat).SetFloat64(mid))
	return out
}
