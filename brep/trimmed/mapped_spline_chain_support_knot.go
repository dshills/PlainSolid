package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// Each original knot line has one curve-plane proof per common chain cell.
func mappedSplineSupportKnotCuts(ctx context.Context, chains [2]MappedSplineChain, patches [2]*geom.SurfacePatch, cuts []*big.Rat, repair mappedChainKnotRepair, o edgecheck.Options) ([]*big.Rat, []bool, error) {
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	events := geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits, MaxSubdivisions: o.MaxDepth}
	maxOwnerKnots, count := 1, 1
	switch repair {
	case mappedChainSynchronizedKnotsRepair, mappedChainMixedKnotsRepair:
		maxOwnerKnots, count = 2, 2
	case mappedChainAsynchronousKnotsRepair:
		count = 2
	case mappedChainThreeKnotsRepair:
		maxOwnerKnots, count = 2, 3
	case mappedChainMultipleKnotsRepair:
		maxOwnerKnots, count = 4, 6
	}
	type station struct {
		parameter *big.Rat
		insert    int
		owners    uint8
	}
	var stations []station
	var originalKnots int
	for owner, patch := range patches {
		planes, err := mappedSplineInteriorSupportKnotPlanes(patch, maxOwnerKnots)
		if err != nil {
			return nil, nil, fmt.Errorf("roof %d: %w", owner, err)
		}
		if (repair == mappedChainSynchronizedKnotsRepair || repair == mappedChainThreeKnotsRepair) && len(planes) != 2 {
			return nil, nil, fmt.Errorf("roof %d requires two original support knots: %w", owner, geom.ErrConvergence)
		}
		if repair == mappedChainAsynchronousKnotsRepair && len(planes) != 1 {
			return nil, nil, fmt.Errorf("roof %d requires one original support knot: %w", owner, geom.ErrConvergence)
		}
		if repair == mappedChainMixedKnotsRepair && len(planes) == 0 {
			return nil, nil, fmt.Errorf("mixed repair requires one or two original support knots per roof: %w", geom.ErrConvergence)
		}
		if repair == mappedChainMultipleKnotsRepair && len(planes) < 2 {
			return nil, nil, fmt.Errorf("roof %d requires two through four original support knots: %w", owner, geom.ErrConvergence)
		}
		originalKnots += len(planes)
		if len(planes) == 0 {
			continue
		}
		ownerCrossed := false
		for i := 0; i+1 < len(cuts); i++ {
			trim, err := chains[owner].cell(cuts[i], cuts[i+1])
			if err != nil {
				return nil, nil, err
			}
			for axisKnot, plane := range planes {
				root, err := mappedSplineSupportKnotRoot(ctx, trim, plane, limits, events)
				if err != nil {
					return nil, nil, fmt.Errorf("roof %d chain cell %d support knot %d: %w", owner, i, axisKnot, err)
				}
				if root == nil {
					continue
				}
				insert := i + 1
				if repair == mappedChainAsynchronousKnotsRepair {
					if ownerCrossed || len(stations) > 0 && stations[0].insert == insert {
						return nil, nil, fmt.Errorf("asynchronous support crossings require one crossing per roof in distinct common chain cells: %w", geom.ErrConvergence)
					}
					ownerCrossed = true
				} else if len(stations) > 0 && stations[0].insert != insert {
					return nil, nil, fmt.Errorf("original support crossings span common chain cells: %w", geom.ErrConvergence)
				}
				width := new(big.Rat).Sub(cuts[i+1], cuts[i])
				t := new(big.Rat).Add(cuts[i], new(big.Rat).Mul(width, root))
				j := slices.IndexFunc(stations, func(s station) bool { return s.parameter.Cmp(t) == 0 })
				mask := uint8(1) << owner
				if j >= 0 {
					if stations[j].owners&mask != 0 {
						return nil, nil, fmt.Errorf("original support crossings must have distinct model stations: %w", geom.ErrConvergence)
					}
					stations[j].owners |= mask
					continue
				}
				if len(stations) == count {
					return nil, nil, fmt.Errorf("original support crossings disagree in model stations: %w", geom.ErrConvergence)
				}
				stations = append(stations, station{parameter: t, insert: insert, owners: mask})
			}
		}
	}
	if repair == mappedChainMultipleKnotsRepair {
		if originalKnots < 5 || originalKnots > 7 {
			return nil, nil, fmt.Errorf("multiple-cut repair requires five through seven original roof knots: %w", geom.ErrConvergence)
		}
		count = originalKnots - 1
	}
	if len(stations) != count {
		return nil, nil, fmt.Errorf("required interior model crossings of original support knots were not proved: %w", geom.ErrConvergence)
	}
	if repair == mappedChainMixedKnotsRepair &&
		(originalKnots != 3 || (stations[0].owners == 3) == (stations[1].owners == 3)) {
		return nil, nil, fmt.Errorf("mixed repair requires one shared crossing and one single-owner crossing across three original support knots: %w", geom.ErrConvergence)
	}
	if repair == mappedChainThreeKnotsRepair || repair == mappedChainMultipleKnotsRepair {
		var ownerChanges [4]int
		for _, s := range stations {
			ownerChanges[s.owners]++
		}
		if repair == mappedChainThreeKnotsRepair && ownerChanges != [4]int{0, 1, 1, 1} {
			return nil, nil, fmt.Errorf("three-cut repair requires one shared crossing and one separate crossing for each owner: %w", geom.ErrConvergence)
		}
		if repair == mappedChainMultipleKnotsRepair && (ownerChanges[3] != 1 || min(ownerChanges[1], ownerChanges[2]) < 1 ||
			ownerChanges[1]+ownerChanges[2]+2 != originalKnots) {
			return nil, nil, fmt.Errorf("multiple-cut repair requires one shared crossing and separate crossings on both roofs: %w", geom.ErrConvergence)
		}
	}
	slices.SortFunc(stations, func(a, b station) int { return a.parameter.Cmp(b.parameter) })
	repaired := make([]bool, len(cuts)+len(stations))
	for i, s := range stations {
		if repair == mappedChainSynchronizedKnotsRepair && s.owners != 3 {
			return nil, nil, fmt.Errorf("both owners must cross at both original support knot stations: %w", geom.ErrConvergence)
		}
		index := s.insert + i
		cuts = slices.Insert(cuts, index, s.parameter)
		repaired[index] = true
	}
	return cuts, repaired, nil
}

func mappedSplineInteriorSupportKnotPlanes(patch *geom.SurfacePatch, maxKnots int) ([][4]*big.Rat, error) {
	u, v := patch.KnotBreaks()
	if len(u)+len(v)-4 > maxKnots {
		return nil, fmt.Errorf("at most %d original interior support knots are supported: %w", maxKnots, geom.ErrInvalid)
	}
	var planes [][4]*big.Rat
	for axis, knots := range [2][]float64{u, v} {
		for _, knot := range knots[1 : len(knots)-1] {
			var plane [4]*big.Rat
			plane[axis], plane[3] = big.NewRat(1, 1), ratInset(-knot)
			planes = append(planes, plane)
		}
	}
	return planes, nil
}

func mappedSplineSupportKnotRoot(ctx context.Context, trim ExactBezierUV, plane [4]*big.Rat, limits geom.SurfacePatchOptions, events geom.ExactCurvePlaneEventOptions) (*big.Rat, error) {
	d, err := trim.descriptor(ctx, limits)
	if err != nil {
		return nil, err
	}
	cell, err := geom.NewExactComposedBezierCell(ctx, d, limits)
	if err != nil {
		return nil, err
	}
	hits, err := cell.IntersectRawPlane(ctx, plane, events)
	if err != nil {
		return nil, err
	}
	if !hits.Complete || hits.Coincident {
		return nil, fmt.Errorf("crossing must be isolated: %w", geom.ErrConvergence)
	}
	var root *big.Rat
	for _, hit := range hits.Events {
		if hit.EndpointMask() != 0 {
			continue
		}
		if hit.Multiplicity() != 1 || root != nil {
			return nil, fmt.Errorf("interior crossing must be unique and transverse: %w", geom.ErrConvergence)
		}
		// The search width bounds rational reconstruction work. A root
		// bracket is never used as a cut.
		exact, err := hit.Root().RecoverRational(ctx, big.NewRat(1, 1000000000000), events)
		if err != nil {
			return nil, err
		}
		if exact == nil || exact.Sign() <= 0 || exact.Cmp(big.NewRat(1, 1)) >= 0 {
			return nil, fmt.Errorf("exact rational crossing parameter not proved: %w", geom.ErrUnrepresentable)
		}
		root = exact
	}
	return root, nil
}
