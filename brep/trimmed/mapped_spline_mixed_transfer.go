package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// ProveMappedSplineMixedSupportKnotsTransfer proves one original raw owner
// against two stored model Béziers. Each owner crosses the stored model
// knot. An owner with a second support crossing must retain the same
// positive raw map on both sides of that additional cut. The returned
// normalized station is nil for the owner that crosses only the model knot.
func ProveMappedSplineMixedSupportKnotsTransfer(ctx context.Context, support geom.Surface, source SplineUV, pieces [2]*geom.ExactSplineCurve, join *big.Rat, o geom.SurfacePatchOptions) (MappedSplineUV, *big.Rat, error) {
	mapped, extra, err := ProveMappedSplineSingleStoredKnotTransfer(ctx, support, source, pieces, join, 2, o)
	if len(extra) == 0 {
		return mapped, nil, err
	}
	return mapped, extra[0], err
}

// ProveMappedSplineSingleStoredKnotTransfer recovers one positive raw map
// from a pair of stored model Béziers and an original owner coedge. One
// original support crossing must match the stored knot. Up to three other
// support crossings are proved in the same map and returned in traversal order.
// maxSupportKnots is between two and four.
func ProveMappedSplineSingleStoredKnotTransfer(ctx context.Context, support geom.Surface, source SplineUV, pieces [2]*geom.ExactSplineCurve, join *big.Rat, maxSupportKnots int, o geom.SurfacePatchOptions) (MappedSplineUV, []*big.Rat, error) {
	return proveMappedSplineSingleStoredKnotTransfer(ctx, support, source, pieces, join, maxSupportKnots, false, o)
}

// ProveMappedSplineCellLocalStoredKnotTransfer proves an original owner coedge
// against two stored pieces when the support also has knots outside this
// coedge. The shared stored knot and at most one other active support crossing
// must fit one exact raw map. Missing or nonisolated active events fail.
func ProveMappedSplineCellLocalStoredKnotTransfer(ctx context.Context, support geom.Surface, source SplineUV, pieces [2]*geom.ExactSplineCurve, join *big.Rat, o geom.SurfacePatchOptions) (MappedSplineUV, []*big.Rat, error) {
	return proveMappedSplineSingleStoredKnotTransfer(ctx, support, source, pieces, join, 4, true, o)
}

func proveMappedSplineSingleStoredKnotTransfer(ctx context.Context, support geom.Surface, source SplineUV, pieces [2]*geom.ExactSplineCurve, join *big.Rat, maxSupportKnots int, cellLocal bool, o geom.SurfacePatchOptions) (MappedSplineUV, []*big.Rat, error) {
	var zero MappedSplineUV
	one := big.NewRat(1, 1)
	if ctx == nil || support == nil || pieces[0] == nil || pieces[1] == nil || join == nil ||
		join.Sign() <= 0 || join.Cmp(one) >= 0 || maxSupportKnots < 2 || maxSupportKnots > 4 {
		return zero, nil, geom.ErrInvalid
	}
	patch, err := exactFaceSupportPatch(Face{Surface: support}, o)
	if err != nil {
		return zero, nil, err
	}
	planes, err := mappedSplineInteriorSupportKnotPlanes(patch, maxSupportKnots)
	if err != nil {
		return zero, nil, err
	}
	if len(planes) < 2 {
		q, err := ProveMappedSplineSupportKnotTransfer(ctx, support, source, pieces, join, o)
		return q, nil, err
	}
	raw, err := NewMappedSplineUV(ctx, source, one, o)
	if err != nil {
		return zero, nil, err
	}
	roots, err := mappedSplineSupportRoots(ctx, raw, planes, !cellLocal, o)
	if err != nil {
		return zero, nil, err
	}
	if cellLocal && (len(roots) == 0 || len(roots) > 2) {
		return zero, nil, fmt.Errorf("the original model cell requires one or two active support knots: %w", geom.ErrConvergence)
	}
	firstModel, err := pieces[0].Reparameterize(0, 1, false, o)
	if err != nil {
		return zero, nil, err
	}
	weight, err := mappedSplineFirstBranchWeight(ctx, support, raw, firstModel, roots[0], join, o)
	if err != nil {
		return zero, nil, err
	}
	mapped, err := NewMappedSplineUV(ctx, source, weight, o)
	if err != nil {
		return zero, nil, err
	}
	var extra []*big.Rat
	last := new(big.Rat)
	for _, root := range roots {
		station := mappedSplineModelStation(root, weight)
		if station.Cmp(last) <= 0 {
			return zero, nil, fmt.Errorf("original support crossings must have distinct model stations: %w", geom.ErrConvergence)
		}
		last = station
		if station.Cmp(join) != 0 {
			extra = append(extra, station)
		}
	}
	if len(extra) != len(roots)-1 {
		return zero, nil, fmt.Errorf("one exact support crossing must match the stored model knot: %w", geom.ErrConvergence)
	}
	if err := ProveMappedSplineStoredKnotPartition(ctx, support, mapped, pieces, join, extra, o); err != nil {
		return zero, nil, err
	}
	return mapped, extra, nil
}

// ProveMappedSplineMixedSplitTransfer compares three exact raw restrictions
// against two stored model Béziers, one of which is further restricted at
// the additional cut. Both input cuts are in whole normalized traversal.
func ProveMappedSplineMixedSplitTransfer(ctx context.Context, support geom.Surface, mapped MappedSplineUV, pieces [2]*geom.ExactSplineCurve, join, extra *big.Rat, o geom.SurfacePatchOptions) error {
	return ProveMappedSplineStoredKnotPartition(ctx, support, mapped, pieces, join, []*big.Rat{extra}, o)
}

// ProveMappedSplineThreeKnotSplitTransfer compares the four exact raw
// restrictions of one already mapped owner against two stored model
// Béziers. The three distinct cuts are in whole normalized traversal.
func ProveMappedSplineThreeKnotSplitTransfer(ctx context.Context, support geom.Surface, mapped MappedSplineUV, pieces [2]*geom.ExactSplineCurve, join *big.Rat, extra [2]*big.Rat, o geom.SurfacePatchOptions) error {
	return ProveMappedSplineStoredKnotPartition(ctx, support, mapped, pieces, join, extra[:], o)
}

// ProveMappedSplineStoredKnotPartition compares up to seven original owner
// restrictions against two stored model Béziers. join and up to five extra
// distinct cuts are in whole normalized model traversal.
func ProveMappedSplineStoredKnotPartition(ctx context.Context, support geom.Surface, mapped MappedSplineUV, pieces [2]*geom.ExactSplineCurve, join *big.Rat, extra []*big.Rat, o geom.SurfacePatchOptions) error {
	one := big.NewRat(1, 1)
	if ctx == nil || support == nil || mapped.weight == nil || pieces[0] == nil || pieces[1] == nil || len(extra) > 5 {
		return geom.ErrInvalid
	}
	stations := append([]*big.Rat{join}, extra...)
	for _, t := range stations {
		if t == nil || t.Sign() <= 0 || t.Cmp(one) >= 0 {
			return geom.ErrInvalid
		}
	}
	slices.SortFunc(stations, func(a, b *big.Rat) int { return a.Cmp(b) })
	for i := 1; i < len(stations); i++ {
		if stations[i-1].Cmp(stations[i]) == 0 {
			return geom.ErrInvalid
		}
	}
	for i, piece := range pieces {
		var err error
		pieces[i], err = piece.Reparameterize(0, 1, false, o)
		if err != nil {
			return err
		}
	}
	cuts := append(append([]*big.Rat{new(big.Rat)}, stations...), one)
	for i := 0; i < len(cuts)-1; i++ {
		a, b := cuts[i], cuts[i+1]
		part, err := restrictMappedSource(ctx, mapped, a, b, o)
		if err != nil {
			return err
		}
		span, origin, width := 0, cuts[0], join
		if a.Cmp(join) >= 0 {
			span, origin, width = 1, join, new(big.Rat).Sub(one, join)
		}
		lo := new(big.Rat).Quo(new(big.Rat).Sub(a, origin), width)
		hi := new(big.Rat).Quo(new(big.Rat).Sub(b, origin), width)
		model, err := pieces[span].RationalSegment(ctx, lo, hi, geom.TubeAffineLineOptions{
			MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
		})
		if err != nil {
			return err
		}
		if err := proveMappedSplineLift(ctx, support, part, model, o); err != nil {
			return err
		}
	}
	return nil
}
