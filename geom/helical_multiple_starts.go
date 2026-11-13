package geom

import (
	"context"
	"math/big"
	"sort"
)

// SeparatedHelicalStarts is an immutable collection of N copies of a convex
// meridian sweep translated axially by lead*j/N, j=0..N-1. All copies share
// the same finite physical-turn window, rather than separately rotated caps.
// These offsets give uniformly spaced screw phases modulo a turn. A strict
// global axial-span inequality proves different copies never meet, including
// contacts between different turn numbers. No rounded phase/frame is stored.
type SeparatedHelicalStarts struct {
	base         HelicalPolygon
	count        int
	spacing, gap *big.Rat
}

// NewSeparatedHelicalStarts accepts 1..64 starts and proves
// max(profile.Z)-min(profile.Z) < lead/starts using exact represented inputs.
// This sufficient condition can reject thinner radius-dependent slices when
// their global axial span is too large; rejection is not evidence of overlap.
func NewSeparatedHelicalStarts(base HelicalPolygon, starts int) (SeparatedHelicalStarts, error) {
	if len(base.profile) < 3 || starts < 1 || starts > 64 {
		return SeparatedHelicalStarts{}, ErrInvalid
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	low, high := rat(base.profile[0].Y), rat(base.profile[0].Y)
	for _, p := range base.profile {
		q := rat(p.Y)
		if q.Cmp(low) < 0 {
			low = q
		}
		if q.Cmp(high) > 0 {
			high = q
		}
	}
	spacing := new(big.Rat).Quo(rat(base.reference.lead), big.NewRat(int64(starts), 1))
	gap := new(big.Rat).Sub(spacing, new(big.Rat).Sub(high, low))
	if gap.Sign() <= 0 {
		return SeparatedHelicalStarts{}, ErrConvergence
	}
	f := base.reference.frame
	x := [3]float64{f.x.X, f.x.Y, f.x.Z}
	y := [3]float64{f.y.X, f.y.Y, f.y.Z}
	z := [3]float64{f.z.X, f.z.Y, f.z.Z}
	det := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		det.Add(det, new(big.Rat).Mul(rat(x[i]), new(big.Rat).Sub(new(big.Rat).Mul(rat(y[j]), rat(z[k])), new(big.Rat).Mul(rat(y[k]), rat(z[j])))))
	}
	if det.Sign() == 0 {
		return SeparatedHelicalStarts{}, ErrDegenerate
	}
	return SeparatedHelicalStarts{base, starts, spacing, gap}, nil
}

// Separation returns caller-owned exact axial spacing and a strictly positive
// global lower bound on the axial gap at common radius/angular phase. This is
// not a Euclidean closest-distance bound between arbitrary surface points.
func (s SeparatedHelicalStarts) Separation() (starts int, spacing, gap *big.Rat, err error) {
	if s.count < 1 || s.spacing == nil || s.gap == nil {
		return 0, nil, nil, ErrInvalid
	}
	return s.count, new(big.Rat).Set(s.spacing), new(big.Rat).Set(s.gap), nil
}

type HelicalStartsLineOptions struct {
	HelicalPolygonLineOptions
	MaxIntervals int
}

// HelicalStartContributor retains the original turn/profile ancestry plus the
// zero-based start and its exact axial translation in frame units.
type HelicalStartContributor struct {
	Start       int
	AxialOffset *big.Rat
	Meridians   []HelicalMaterialContributor
}
type HelicalStartsInterval struct {
	HelicalMaterialInterval
	Source HelicalStartContributor
}
type HelicalStartsLineResult struct {
	Complete                        bool
	Phase                           [2]*big.Rat
	PhaseScale, Radius              *big.Rat
	Intervals                       []HelicalStartsInterval
	CandidateTurns, ExactOperations int
}

// IntersectAxisLine returns the ordered material union of all proven-separated
// starts in the original caller line units. The common irrational phase term
// cancels when sorting and verifying strict gaps between rational offsets.
// MaxIntervals defaults to8192 and MaxOperations is cumulative across query,
// shifts and enclosures. Errors discard partial results.
func (s SeparatedHelicalStarts) IntersectAxisLine(ctx context.Context, cs, sn *big.Rat, origin [3]*big.Rat, axisScale *big.Rat, o HelicalStartsLineOptions) (out HelicalStartsLineResult, err error) {
	if ctx == nil || s.count < 1 || axisScale == nil || axisScale.Sign() == 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxIntervals == 0 {
		o.MaxIntervals = 8192
	}
	if o.MaxIntervals < 1 {
		return out, ErrInvalid
	}
	max := o.MaxOperations
	if max == 0 {
		max = 1000000
	}
	bits := o.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if max < 1 || bits < 64 {
		return out, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: max, maxBits: bits}
	defer func() {
		out.ExactOperations = budget.operations
		if err != nil {
			out.Complete = false
			out.Intervals = nil
		}
	}()
	r, e := s.base.IntersectAxisLine(ctx, cs, sn, origin, axisScale, o.HelicalPolygonLineOptions)
	budget.operations += r.ExactOperations
	if e != nil {
		return out, e
	}
	out.Phase = r.Phase
	out.PhaseScale = r.PhaseScale
	out.Radius = r.Radius
	out.CandidateTurns = r.CandidateTurns
	if len(r.Intervals) > o.MaxIntervals/s.count {
		return out, ErrConvergence
	}
	for start := 0; start < s.count; start++ {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		axialOffset := new(big.Rat).Mul(s.spacing, big.NewRat(int64(start), 1))
		lineOffset := new(big.Rat).Quo(axialOffset, axisScale)
		if e := budget.check(lineOffset); e != nil {
			return out, e
		}
		for _, original := range r.Intervals {
			interval := HelicalStartsInterval{Source: HelicalStartContributor{Start: start, AxialOffset: new(big.Rat).Set(axialOffset), Meridians: original.Contributors}}
			interval.Contributors = original.Contributors
			interval.Touch = original.Touch
			for end := 0; end < 2; end++ {
				interval.Offsets[end] = new(big.Rat).Add(original.Offsets[end], lineOffset)
				if e := budget.check(interval.Offsets[end]); e != nil {
					return out, e
				}
				for bound := 0; bound < 2; bound++ {
					interval.Bounds[end][bound] = new(big.Rat).Add(original.Bounds[end][bound], lineOffset)
					if e := budget.check(interval.Bounds[end][bound]); e != nil {
						return out, e
					}
				}
			}
			out.Intervals = append(out.Intervals, interval)
		}
	}
	sort.Slice(out.Intervals, func(i, j int) bool { return out.Intervals[i].Offsets[0].Cmp(out.Intervals[j].Offsets[0]) < 0 })
	for i := 1; i < len(out.Intervals); i++ {
		if out.Intervals[i-1].Offsets[1].Cmp(out.Intervals[i].Offsets[0]) >= 0 {
			return out, ErrConvergence
		}
	}
	out.Complete = true
	return out, nil
}
