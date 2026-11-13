package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// HelicalOverlapRelief retains a bounded helical groove with an entry relief
// crossing the first partial turn. Exact rational coefficients multiply pi;
// EntryVolume excludes groove material already removed below the shoulder.
// Coefficients describe the ideal unit-frame construction; native numerical
// sewing and frame roundoff remain separate from these analytic measures.
type HelicalOverlapRelief struct {
	Solid, Original                                     Solid
	ChartRoundoff                                       HelicalChartRoundoff
	ReliefWall, Shoulder                                int
	GrooveVolumePiCoefficient, EntryVolumePiCoefficient *big.Rat
}

// GrooveWithOverlappingEntryRelief supports one finite start on coaxial shaft
// or tube stock. Relief must clear the entire radial groove profile and end
// strictly inside its first axial end profile, away from every original vertex.
// Native clipped helical supports and a polar analytic shoulder are sewn using
// the original tolerance. Original upper cap and rim geometry remain unchanged.
func GrooveWithOverlappingEntryRelief(ctx context.Context, r HelicalGrooveRecipe, reliefRadius, depth, maxChartError float64) (HelicalOverlapRelief, error) {
	return grooveWithOverlappingEntryRelief(ctx, r, reliefRadius, depth, maxChartError, true, true)
}

// SteppedShaftWithGrooveAndEntryRelief returns the final relieved shaft with
// its original head attached. Intermediate boundaries are checked for
// topology and edge agreement, and only the published solid is meshed.
func SteppedShaftWithGrooveAndEntryRelief(ctx context.Context, shaft SteppedShaft, r HelicalGrooveRecipe, reliefRadius, depth, maxChartError float64) (Solid, *big.Rat, error) {
	groove, err := grooveWithOverlappingEntryRelief(ctx, r, reliefRadius, depth, maxChartError, false, false)
	if err != nil {
		return Solid{}, nil, err
	}
	solid, err := AttachSteppedShaftHead(ctx, shaft, groove.Solid)
	if err != nil {
		return Solid{}, nil, err
	}
	return solid, new(big.Rat).Add(groove.GrooveVolumePiCoefficient, groove.EntryVolumePiCoefficient), nil
}

func grooveWithOverlappingEntryRelief(ctx context.Context, r HelicalGrooveRecipe, reliefRadius, depth, maxChartError float64, validateOriginal, validateResult bool) (HelicalOverlapRelief, error) {
	fail := func(e error) (HelicalOverlapRelief, error) { return HelicalOverlapRelief{}, e }
	if ctx == nil || r.Starts != 1 || !finite(reliefRadius) || !finite(depth) || reliefRadius <= 0 || depth <= 0 {
		return fail(geom.ErrInvalid)
	}
	original, proof, e := grooveWithBoundedChart(ctx, r, nil, maxChartError, validateOriginal)
	if e != nil {
		return fail(e)
	}
	return finishOverlapEntry(ctx, r, original, proof, reliefRadius, depth, false, validateResult)
}

// GrooveWithPhasedOverlappingEntryRelief supports phase-staggered starts in
// one common axial window. The relief may pass the complete first end profile,
// but must remain strictly before every far-end profile point.
func GrooveWithPhasedOverlappingEntryRelief(ctx context.Context, r HelicalGrooveRecipe, initialPhase, reliefRadius, depth, maxChartError float64) (HelicalOverlapRelief, error) {
	if ctx == nil || !finite(reliefRadius) || !finite(depth) || reliefRadius <= 0 || depth <= 0 {
		return HelicalOverlapRelief{}, geom.ErrInvalid
	}
	original, proof, e := GrooveWithPhasedStarts(ctx, r, initialPhase, maxChartError)
	if e != nil {
		return HelicalOverlapRelief{}, e
	}
	return finishOverlapEntry(ctx, r, original, proof, reliefRadius, depth, true, true)
}
func finishOverlapEntry(ctx context.Context, r HelicalGrooveRecipe, original Solid, proof HelicalChartRoundoff, reliefRadius, depth float64, phased, validateDisplay bool) (HelicalOverlapRelief, error) {
	fail := func(e error) (HelicalOverlapRelief, error) { return HelicalOverlapRelief{}, e }
	loR, hiR, loZ, hiZ := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, p := range r.Profile {
		loR = min(loR, p.X)
		hiR = max(hiR, p.X)
		loZ = min(loZ, p.Y+r.Lead*r.Start)
		hiZ = max(hiZ, p.Y+r.Lead*r.Start)
	}
	pad := 8 * original.tolerance
	maxDepth := hiZ
	if phased {
		maxDepth = loZ + r.Lead*(r.End-r.Start)
	}
	if depth <= loZ+pad || depth >= maxDepth-pad || depth >= r.Height-pad {
		return fail(geom.ErrDomain)
	}
	if r.BoreRadius == 0 {
		if reliefRadius >= loR-pad {
			return fail(geom.ErrDomain)
		}
	} else {
		if reliefRadius <= hiR+pad || reliefRadius >= r.OuterRadius-pad {
			return fail(geom.ErrDomain)
		}
	}
	for _, v := range original.vertices {
		if math.Abs(r.Frame.ToLocal(v).Z-depth) <= pad {
			return fail(geom.ErrDomain)
		}
	}
	c, e := clipHelicalEntry(ctx, original, r, depth)
	if e != nil {
		return fail(e)
	}
	wall, shoulder, e := c.appendEntryRelief(reliefRadius)
	if e != nil {
		return fail(e)
	}
	solid, e := c.solidWithValidation(validateDisplay)
	if e != nil {
		return fail(e)
	}
	groove, below, e := helicalOverlapVolume(r, depth)
	if e != nil {
		return fail(e)
	}
	if phased {
		groove.Mul(groove, big.NewRat(int64(r.Starts), 1))
		below.Mul(below, big.NewRat(int64(r.Starts), 1))
	}
	stockRadius := r.OuterRadius
	if r.BoreRadius > 0 {
		stockRadius = r.BoreRadius
	}
	annulus := new(big.Rat).Abs(new(big.Rat).Sub(new(big.Rat).Mul(ratInset(reliefRadius), ratInset(reliefRadius)), new(big.Rat).Mul(ratInset(stockRadius), ratInset(stockRadius))))
	entry := new(big.Rat).Sub(new(big.Rat).Mul(annulus, ratInset(depth)), below)
	if entry.Sign() <= 0 {
		return fail(geom.ErrDomain)
	}
	return HelicalOverlapRelief{solid, original, proof, wall, shoulder, groove, entry}, nil
}

type reliefExactPoint [2]*big.Rat

func reliefMoments(p []reliefExactPoint) (*big.Rat, *big.Rat) {
	first, mixed := new(big.Rat), new(big.Rat)
	for i, a := range p {
		b := p[(i+1)%len(p)]
		cross := new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(b[0], a[1]))
		first.Add(first, new(big.Rat).Mul(new(big.Rat).Add(a[0], b[0]), cross))
		sum := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(new(big.Rat).Mul(a[0], a[1]), new(big.Rat).Mul(b[0], b[1])))
		sum.Add(sum, new(big.Rat).Mul(a[0], b[1]))
		sum.Add(sum, new(big.Rat).Mul(b[0], a[1]))
		mixed.Add(mixed, new(big.Rat).Mul(sum, cross))
	}
	first.Quo(first, big.NewRat(6, 1))
	mixed.Quo(mixed, big.NewRat(24, 1))
	return first, mixed
}
func reliefBelow(p []reliefExactPoint, z *big.Rat) []reliefExactPoint {
	var out []reliefExactPoint
	for i, a := range p {
		b := p[(i+1)%len(p)]
		ia, ib := a[1].Cmp(z) <= 0, b[1].Cmp(z) <= 0
		if ia {
			out = append(out, a)
		}
		if ia != ib {
			t := new(big.Rat).Quo(new(big.Rat).Sub(z, a[1]), new(big.Rat).Sub(b[1], a[1]))
			x := new(big.Rat).Add(a[0], new(big.Rat).Mul(t, new(big.Rat).Sub(b[0], a[0])))
			out = append(out, reliefExactPoint{x, new(big.Rat).Set(z)})
		}
	}
	return out
}
func helicalOverlapVolume(r HelicalGrooveRecipe, depth float64) (*big.Rat, *big.Rat, error) {
	return helicalOverlapVolumeExactDepth(r, ratInset(depth))
}
func helicalOverlapVolumeExactDepth(r HelicalGrooveRecipe, depth *big.Rat) (*big.Rat, *big.Rat, error) {
	var p []reliefExactPoint
	for _, v := range r.Profile {
		p = append(p, reliefExactPoint{ratInset(v.X), ratInset(v.Y)})
	}
	moment, _ := reliefMoments(p)
	if moment.Sign() < 0 {
		for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
			p[i], p[j] = p[j], p[i]
		}
		moment, _ = reliefMoments(p)
	}
	if moment.Sign() <= 0 {
		return nil, nil, geom.ErrDomain
	}
	full := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(new(big.Rat).Sub(ratInset(r.End), ratInset(r.Start)), moment))
	integral := func(turn float64) *big.Rat {
		z := new(big.Rat).Sub(depth, new(big.Rat).Mul(ratInset(r.Lead), ratInset(turn)))
		clipped := reliefBelow(p, z)
		if len(clipped) < 3 {
			return new(big.Rat)
		}
		first, mixed := reliefMoments(clipped)
		return new(big.Rat).Sub(new(big.Rat).Mul(z, first), mixed)
	}
	below := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Sub(integral(r.Start), integral(r.End))), ratInset(r.Lead))
	if below.Sign() < 0 || below.Cmp(full) > 0 {
		return nil, nil, geom.ErrDomain
	}
	return full, below, nil
}
