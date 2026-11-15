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
// A two-cut repair requires both owners to cross both lines in the same cell.
func mappedSplineSupportKnotCuts(ctx context.Context, chains [2]MappedSplineChain, patches [2]*geom.SurfacePatch, cuts []*big.Rat, count int, o edgecheck.Options) ([]*big.Rat, int, error) {
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	events := geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits, MaxSubdivisions: o.MaxDepth}
	type station struct {
		parameter *big.Rat
		owners    uint8
	}
	var stations []station
	var insert int
	for owner, patch := range patches {
		planes, err := mappedSplineInteriorSupportKnotPlanes(patch, count)
		if err != nil {
			return nil, 0, fmt.Errorf("roof %d: %w", owner, err)
		}
		if count == 2 && len(planes) != count {
			return nil, 0, fmt.Errorf("roof %d requires two original support knots: %w", owner, geom.ErrConvergence)
		}
		if len(planes) == 0 {
			continue
		}
		for i := 0; i+1 < len(cuts); i++ {
			trim, err := chains[owner].cell(cuts[i], cuts[i+1])
			if err != nil {
				return nil, 0, err
			}
			for axisKnot, plane := range planes {
				root, err := mappedSplineSupportKnotRoot(ctx, trim, plane, limits, events)
				if err != nil {
					return nil, 0, fmt.Errorf("roof %d chain cell %d support knot %d: %w", owner, i, axisKnot, err)
				}
				if root == nil {
					continue
				}
				if insert != 0 && insert != i+1 {
					return nil, 0, fmt.Errorf("original support crossings span common chain cells: %w", geom.ErrConvergence)
				}
				insert = i + 1
				width := new(big.Rat).Sub(cuts[i+1], cuts[i])
				t := new(big.Rat).Add(cuts[i], new(big.Rat).Mul(width, root))
				j := slices.IndexFunc(stations, func(s station) bool { return s.parameter.Cmp(t) == 0 })
				mask := uint8(1) << owner
				if j >= 0 {
					if stations[j].owners&mask != 0 {
						return nil, 0, fmt.Errorf("original support crossings must have distinct model stations: %w", geom.ErrConvergence)
					}
					stations[j].owners |= mask
					continue
				}
				if len(stations) == count {
					return nil, 0, fmt.Errorf("original support crossings disagree in model stations: %w", geom.ErrConvergence)
				}
				stations = append(stations, station{t, mask})
			}
		}
	}
	if len(stations) != count {
		return nil, 0, fmt.Errorf("required interior model crossings of original support knots were not proved: %w", geom.ErrConvergence)
	}
	added := make([]*big.Rat, len(stations))
	for i, s := range stations {
		if count == 2 && s.owners != 3 {
			return nil, 0, fmt.Errorf("both owners must cross at both original support knot stations: %w", geom.ErrConvergence)
		}
		added[i] = s.parameter
	}
	slices.SortFunc(added, func(a, b *big.Rat) int { return a.Cmp(b) })
	return slices.Insert(cuts, insert, added...), insert, nil
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
