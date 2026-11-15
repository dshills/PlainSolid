package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// Work is bounded by one curve-plane proof per original common cell and
// owner. Any interior events must all identify one common model station.
func mappedSplineSupportKnotCut(ctx context.Context, chains [2]MappedSplineChain, patches [2]*geom.SurfacePatch, cuts []*big.Rat, o edgecheck.Options) ([]*big.Rat, int, error) {
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	events := geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits, MaxSubdivisions: o.MaxDepth}
	var station *big.Rat
	var insert int
	for owner, patch := range patches {
		plane, err := mappedSplineInteriorSupportKnotPlane(patch)
		if err != nil {
			return nil, 0, fmt.Errorf("roof %d: %w", owner, err)
		}
		if plane[3] == nil {
			continue
		}
		for i := 0; i+1 < len(cuts); i++ {
			trim, err := chains[owner].cell(cuts[i], cuts[i+1])
			if err != nil {
				return nil, 0, err
			}
			root, err := mappedSplineSupportKnotRoot(ctx, trim, plane, limits, events)
			if err != nil {
				return nil, 0, fmt.Errorf("roof %d chain cell %d support knot: %w", owner, i, err)
			}
			if root == nil {
				continue
			}
			width := new(big.Rat).Sub(cuts[i+1], cuts[i])
			t := new(big.Rat).Add(cuts[i], new(big.Rat).Mul(width, root))
			if station != nil && t.Cmp(station) != 0 {
				return nil, 0, fmt.Errorf("multiple model crossings of original support knots: %w", geom.ErrConvergence)
			}
			station, insert = t, i+1
		}
	}
	if station == nil {
		return nil, 0, fmt.Errorf("no interior model crossing of an original support knot: %w", geom.ErrConvergence)
	}
	return slices.Insert(cuts, insert, station), insert, nil
}

func mappedSplineInteriorSupportKnotPlane(patch *geom.SurfacePatch) ([4]*big.Rat, error) {
	var plane [4]*big.Rat
	u, v := patch.KnotBreaks()
	if len(u)+len(v) > 5 {
		return plane, fmt.Errorf("at most one original interior support knot is supported: %w", geom.ErrInvalid)
	}
	for axis, knots := range [2][]float64{u, v} {
		if len(knots) == 3 {
			plane[axis], plane[3] = big.NewRat(1, 1), ratInset(-knots[1])
		}
	}
	return plane, nil
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
