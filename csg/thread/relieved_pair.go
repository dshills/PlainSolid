package thread

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// PairReliefs removes an optional lower shaft tip and optional nut entry and
// exit shoulders. Depths are measured inward from the corresponding cap.
// A zero Radius and Depth leaves that feature unchanged.
type PairReliefs struct {
	ShaftTip, NutLower, NutUpper trimmed.HelicalReliefSpec
}

// RelievedPair retains the nominal pair and its finished native parts. These
// disjoint reliefs do not alter the groove charts or nominal flank proof.
// They can shorten actual material overlap; they do not by themselves prove
// assembly clearance. The original affine frame is shared by both parts.
type RelievedPair struct {
	Pair                Pair
	Shaft, Nut          trimmed.Solid
	Reliefs             PairReliefs
	ShaftTipRemovedPi   *big.Rat
	NutReliefsRemovedPi *big.Rat
}

// BuildRelievedPair verifies the pair and builds all requested disjoint
// reliefs atomically. The two removal coefficients use the represented affine
// volume convention of BuildPair.
func BuildRelievedPair(ctx context.Context, recipe PairRecipe, relief PairReliefs, o geom.SurfacePatchEvaluationOptions) (RelievedPair, error) {
	var empty RelievedPair
	valid := func(x trimmed.HelicalReliefSpec) bool {
		return x == (trimmed.HelicalReliefSpec{}) || isFinite(x.Radius) && isFinite(x.Depth) && x.Radius > 0 && x.Depth > 0
	}
	if !valid(relief.ShaftTip) || !valid(relief.NutLower) || !valid(relief.NutUpper) {
		return empty, geom.ErrInvalid
	}
	p, e := BuildPair(ctx, recipe, o)
	if e != nil {
		return empty, e
	}
	out := RelievedPair{Pair: p, Shaft: p.Shaft.Solid(), Nut: p.Nut.Solid(), Reliefs: relief, ShaftTipRemovedPi: new(big.Rat), NutReliefsRemovedPi: new(big.Rat)}
	if relief.ShaftTip.Depth > 0 {
		x, e := trimmed.GrooveShaftWithTipRelief(ctx, recipe.Shaft, relief.ShaftTip.Radius, relief.ShaftTip.Depth)
		if e != nil {
			return empty, e
		}
		out.Shaft = x.Solid
		out.ShaftTipRemovedPi.Set(x.RemovedVolumePiCoefficient)
	}
	if relief.NutLower.Depth > 0 || relief.NutUpper.Depth > 0 {
		x, e := trimmed.GrooveTubeWithEndReliefs(ctx, recipe.Nut, relief.NutLower, relief.NutUpper)
		if e != nil {
			return empty, e
		}
		out.Nut = x.Solid
		// The relief constructor's coefficient uses its orthonormal
		// convention; transport to the pair's represented affine frame.
		out.NutReliefsRemovedPi.Mul(x.RemovedVolumePiCoefficient, frameDeterminant(recipe.Nut.Frame))
	}
	return out, nil
}

// CompareRunoutQuarter uses the actual relieved stock bands on an exact
// quarter-phase ray. A ray strictly inside both the original common radial
// band and away from the new cylindrical relief radii is required; at a new
// radial wall a regularized 1D comparison cannot classify boundary-only
// material reliably. NutShift applies to its whole finished part.
func (p RelievedPair) CompareRunoutQuarter(ctx context.Context, radius, phase, nutShift *big.Rat, o geom.HelicalStartsLineOptions) (geom.HelicalRetainedPair, error) {
	bands, e := p.bandsAtRadius(radius)
	if e != nil {
		return geom.HelicalRetainedPair{}, e
	}
	return geom.CompareHelicalRetainedQuarter(ctx, p.Pair.shaftStarts, p.Pair.nutStarts, radius, phase, nutShift, bands[0], bands[1], o)
}

func (p RelievedPair) bandsAtRadius(radius *big.Rat) ([2][2]*big.Rat, error) {
	q := p.Pair
	if radius == nil || q.radialRange[0] == nil || radius.Cmp(q.radialRange[0]) <= 0 || radius.Cmp(q.radialRange[1]) >= 0 {
		return [2][2]*big.Rat{}, geom.ErrInvalid
	}
	for _, spec := range []trimmed.HelicalReliefSpec{p.Reliefs.ShaftTip, p.Reliefs.NutLower, p.Reliefs.NutUpper} {
		if spec.Depth > 0 && radius.Cmp(exact(spec.Radius)) == 0 {
			return [2][2]*big.Rat{}, geom.ErrDomain
		}
	}
	return p.stockBandsAtRadius(radius), nil
}

func (p RelievedPair) stockBandsAtRadius(radius *big.Rat) [2][2]*big.Rat {
	bands := p.Pair.stockBands
	if tip := p.Reliefs.ShaftTip; tip.Depth > 0 && radius.Cmp(exact(tip.Radius)) > 0 {
		bands[0][0] = exact(tip.Depth)
	}
	if lower := p.Reliefs.NutLower; lower.Depth > 0 && radius.Cmp(exact(lower.Radius)) < 0 {
		bands[1][0] = exact(lower.Depth)
	}
	if upper := p.Reliefs.NutUpper; upper.Depth > 0 && radius.Cmp(exact(upper.Radius)) < 0 {
		bands[1][1] = new(big.Rat).Sub(bands[1][1], exact(upper.Depth))
	}
	return bands
}

// RelievedRunoutComparison covers the selected closed radial band through a
// full phase turn. At a radial relief wall the longer axial band is tested.
// A witness is a line with positive common length; if WitnessOnReliefWall is
// true, it may belong
// only to a boundary cylinder. A clear report excludes positive axial
// overlap, rather than claiming minimum Euclidean distance.
type RelievedRunoutComparison struct {
	Complete, NoPositiveOverlap, WitnessOnReliefWall bool
	CheckedSlabs, ExactOperations                    int
	Witness                                          geom.HelicalJointWindowComparison
}

// CompareRunoutFullTurn splits at the original relief radii and proves each
// constant stock regime with exact helical endpoint arrangements. It checks
// boundary radii separately using the union of both neighboring axial
// regimes (the longer band), which contains the radial wall itself.
func (p RelievedPair) CompareRunoutFullTurn(ctx context.Context, nutShift *big.Rat, radii [2]*big.Rat, o geom.HelicalStartsLineOptions) (RelievedRunoutComparison, error) {
	var empty RelievedRunoutComparison
	q := p.Pair
	if ctx == nil || nutShift == nil || radii[0] == nil || radii[1] == nil || q.radialRange[0] == nil ||
		radii[0].Cmp(radii[1]) >= 0 || radii[0].Cmp(q.radialRange[0]) <= 0 || radii[1].Cmp(q.radialRange[1]) >= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxOperations < 1 {
		return empty, geom.ErrInvalid
	}
	cuts := []*big.Rat{new(big.Rat).Set(radii[0]), new(big.Rat).Set(radii[1])}
	walls := map[string]bool{}
	for _, spec := range []trimmed.HelicalReliefSpec{p.Reliefs.ShaftTip, p.Reliefs.NutLower, p.Reliefs.NutUpper} {
		if spec.Depth == 0 {
			continue
		}
		x := exact(spec.Radius)
		if x.Cmp(radii[0]) < 0 || x.Cmp(radii[1]) > 0 {
			continue
		}
		k := x.RatString()
		if !walls[k] && x.Cmp(radii[0]) > 0 && x.Cmp(radii[1]) < 0 {
			cuts = append(cuts, x)
		}
		walls[k] = true
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].Cmp(cuts[j]) < 0 })
	out := RelievedRunoutComparison{}
	check := func(rangeR [2]*big.Rat, bands [2][2]*big.Rat) (bool, error) {
		oo := o
		oo.MaxOperations -= out.ExactOperations
		if oo.MaxOperations < 1 {
			return false, geom.ErrConvergence
		}
		r, e := geom.CompareHelicalRetainedFullTurn(ctx, q.shaftStarts, q.nutStarts, nutShift, rangeR, bands[0], bands[1], oo)
		if e != nil {
			return false, e
		}
		out.ExactOperations += r.ExactOperations
		out.CheckedSlabs++
		if !r.NoPositiveOverlap {
			out.Complete = true
			out.WitnessOnReliefWall = walls[r.WitnessRadius.RatString()]
			out.Witness = r
			return true, nil
		}
		return false, nil
	}
	for i, r := range cuts {
		if walls[r.RatString()] {
			// At a step radius the larger closed axial band is present on
			// the wall. Comparisons use strict signs for removed bands.
			found, e := check([2]*big.Rat{r, r}, p.stockBandsAtRadius(r))
			if e != nil {
				return empty, e
			}
			if found {
				return out, nil
			}
		}
		if i+1 < len(cuts) {
			hi := cuts[i+1]
			mid := new(big.Rat).Quo(new(big.Rat).Add(r, hi), big.NewRat(2, 1))
			found, e := check([2]*big.Rat{r, hi}, p.stockBandsAtRadius(mid))
			if e != nil {
				return empty, e
			}
			if found {
				return out, nil
			}
		}
	}
	out.Complete, out.NoPositiveOverlap = true, true
	return out, nil
}
