// Package thread builds bounded coaxial helical parts with independently
// checkable clearances in their original meridian profiles.
package thread

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// PairRecipe specifies finite, blind-ended grooves on a shaft and its nut.
// Both recipes must share their represented chart, lead, hand, turn interval
// and start count. The shaft radius must exceed the nut bore radius. At each
// common material radius the nut groove must strictly contain the retained
// shaft ridge between consecutive shaft groove intervals in the original
// periodic helical chart.
//
// This proves nominal clearance where all neighboring cutter turns exist.
// It does not prove global noninterference at runouts or model entry reliefs.
type PairRecipe struct {
	Shaft, Nut trimmed.HelicalGrooveRecipe
}

type Pair struct {
	Shaft, Nut                trimmed.HelicalGrooveMaterial
	Fit                       FitProof
	ShaftRemovedPi            *big.Rat
	NutRemovedPi              *big.Rat
	shaftStarts, nutStarts    geom.SeparatedHelicalStarts
	radialRange, stockOverlap [2]*big.Rat
	stockBands                [2][2]*big.Rat
	lead                      float64
}

type FlankClearance struct {
	Radius [2]*big.Rat
	Lower  [2]*big.Rat
	Upper  [2]*big.Rat
}

// FitProof describes the two directed axial flank gaps throughout the shared
// radius band, at corresponding helical phase. Periodic copies repeat these
// inequalities at each start. The original turn window is a recipe bound;
// the proof applies only where both bounding shaft grooves are active. The two arrays describe exact values at the
// radial interval endpoints; each value varies affinely within its interval.
// Minimum and all other rational values are caller-owned.
type FitProof struct {
	RadialBand, CommonTurns [2]*big.Rat
	Spacing, Minimum        *big.Rat
	Flanks                  []FlankClearance
}

// BuildPair checks exact profiles and phase placement before constructing both
// native parts. Removed volumes refer to the ideal represented affine shaft
// and nut and equal π times the corresponding returned coefficient.
func BuildPair(ctx context.Context, p PairRecipe, o geom.SurfacePatchEvaluationOptions) (Pair, error) {
	var empty Pair
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	a, b := p.Shaft, p.Nut
	if a.Frame != b.Frame || a.Lead != b.Lead || a.Hand != b.Hand || a.Start != b.Start || a.End != b.End ||
		a.Starts != b.Starts || a.Starts < 1 || a.Starts > 8 || a.BoreRadius != 0 ||
		!isFinite(a.OuterRadius) || !isFinite(b.BoreRadius) || b.BoreRadius <= 0 || b.BoreRadius >= a.OuterRadius ||
		!isFinite(a.Height) || !isFinite(b.Height) || len(a.Profile) < 3 || len(a.Profile) > 32 || len(b.Profile) < 3 || len(b.Profile) > 32 {
		return empty, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return empty, geom.ErrInvalid
	}
	pa, e := geom.NewHelicalPolygon(a.Frame, a.Profile, a.Lead, a.Hand, a.Start, a.End)
	if e != nil {
		return empty, e
	}
	pb, e := geom.NewHelicalPolygon(b.Frame, b.Profile, b.Lead, b.Hand, b.Start, b.End)
	if e != nil {
		return empty, e
	}
	as, e := geom.NewSeparatedHelicalStarts(pa, a.Starts)
	if e != nil {
		return empty, e
	}
	bs, e := geom.NewSeparatedHelicalStarts(pb, b.Starts)
	if e != nil {
		return empty, e
	}
	_, spacing, _, e := as.Separation()
	if e != nil {
		return empty, e
	}
	band := [2]*big.Rat{exact(b.BoreRadius), exact(a.OuterRadius)}
	offset := new(big.Rat)
	report, e := geom.CompareHelicalRadialBand(ctx, pa, pb, band, offset, geom.HelicalClearanceBandOptions{SurfacePatchEvaluationOptions: o, MaxCells: 256})
	if e != nil {
		return empty, e
	}
	if !report.HasBand || report.RadialRange[0].Cmp(band[0]) != 0 || report.RadialRange[1].Cmp(band[1]) != 0 {
		return empty, geom.ErrDomain
	}
	work := report.ExactOperations
	fit := FitProof{RadialBand: report.RadialRange, CommonTurns: report.CommonTurns, Spacing: spacing}
	var last [2]*big.Rat
	for i, ev := range report.Events {
		remaining := o.MaxOperations - work
		if remaining <= 0 {
			return empty, geom.ErrConvergence
		}
		s, e := geom.CompareHelicalMeridianSlices(ctx, pa, pb, ev.Radius, offset,
			geom.SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return empty, e
		}
		if !s.Comparable {
			return empty, geom.ErrDomain
		}
		work += s.ExactOperations
		// On a common ray, retained shaft material follows its removed
		// interval until the start of its next phase copy.
		lower := new(big.Rat).Sub(s.A.Upper, s.B.Lower)
		upper := new(big.Rat).Sub(s.B.Upper, new(big.Rat).Add(s.A.Lower, spacing))
		for _, gap := range []*big.Rat{lower, upper} {
			if gap.Sign() <= 0 {
				return empty, geom.ErrDomain
			}
			if gap.Num().BitLen() > o.MaxCoefficientBits || gap.Denom().BitLen() > o.MaxCoefficientBits {
				return empty, geom.ErrConvergence
			}
			if fit.Minimum == nil || gap.Cmp(fit.Minimum) < 0 {
				fit.Minimum = new(big.Rat).Set(gap)
			}
		}
		if i > 0 {
			fit.Flanks = append(fit.Flanks, FlankClearance{
				Radius: [2]*big.Rat{new(big.Rat).Set(report.Events[i-1].Radius), new(big.Rat).Set(ev.Radius)},
				Lower:  [2]*big.Rat{new(big.Rat).Set(last[0]), new(big.Rat).Set(lower)},
				Upper:  [2]*big.Rat{new(big.Rat).Set(last[1]), new(big.Rat).Set(upper)},
			})
		}
		last = [2]*big.Rat{lower, upper}
	}
	shaft, e := trimmed.BuildHelicalGrooveMaterial(ctx, a)
	if e != nil {
		return empty, e
	}
	nut, e := trimmed.BuildHelicalGrooveMaterial(ctx, b)
	if e != nil {
		return empty, e
	}
	return Pair{
		Shaft: shaft, Nut: nut, Fit: fit, ShaftRemovedPi: removedVolumePi(a), NutRemovedPi: removedVolumePi(b),
		shaftStarts: as, nutStarts: bs,
		radialRange:  [2]*big.Rat{exact(b.BoreRadius), exact(a.OuterRadius)},
		stockOverlap: [2]*big.Rat{new(big.Rat), exact(min(a.Height, b.Height))}, lead: a.Lead,
		stockBands: [2][2]*big.Rat{{new(big.Rat), exact(a.Height)}, {new(big.Rat), exact(b.Height)}},
	}, nil
}

func exact(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
func isFinite(x float64) bool  { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func removedVolumePi(p trimmed.HelicalGrooveRecipe) *big.Rat {
	// ∫ r dA = Σ ((r_i+r_j) * cross(v_i,v_j)) / 6.
	radialMoment := new(big.Rat)
	for i, a := range p.Profile {
		b := p.Profile[(i+1)%len(p.Profile)]
		cross := new(big.Rat).Sub(new(big.Rat).Mul(exact(a.X), exact(b.Y)), new(big.Rat).Mul(exact(b.X), exact(a.Y)))
		radialMoment.Add(radialMoment, cross.Mul(cross, new(big.Rat).Add(exact(a.X), exact(b.X))))
	}
	radialMoment.Abs(radialMoment)
	span := new(big.Rat).Sub(exact(p.End), exact(p.Start))
	x, y, z := p.Frame.XAxis(), p.Frame.YAxis(), p.Frame.ZAxis()
	a, b, c := [3]float64{x.X, x.Y, x.Z}, [3]float64{y.X, y.Y, y.Z}, [3]float64{z.X, z.Y, z.Z}
	det := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		m := new(big.Rat).Sub(new(big.Rat).Mul(exact(b[j]), exact(c[k])), new(big.Rat).Mul(exact(b[k]), exact(c[j])))
		det.Add(det, m.Mul(m, exact(a[i])))
	}
	det.Abs(det)
	return radialMoment.Mul(radialMoment, span).Mul(radialMoment, det).Mul(radialMoment, big.NewRat(int64(p.Starts), 3))
}
