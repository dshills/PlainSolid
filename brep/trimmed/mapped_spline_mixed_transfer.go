package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// ProveMappedSplineMixedSupportKnotsTransfer proves one original raw owner
// against two stored model Béziers. Each owner crosses the stored model
// knot. An owner with a second support crossing must retain the same
// positive raw map on both sides of that additional cut. The returned
// normalized station is nil for the owner that crosses only the model knot.
func ProveMappedSplineMixedSupportKnotsTransfer(ctx context.Context, support geom.Surface, source SplineUV, pieces [2]*geom.ExactSplineCurve, join *big.Rat, o geom.SurfacePatchOptions) (MappedSplineUV, *big.Rat, error) {
	var zero MappedSplineUV
	one := big.NewRat(1, 1)
	if ctx == nil || support == nil || pieces[0] == nil || pieces[1] == nil || join == nil ||
		join.Sign() <= 0 || join.Cmp(one) >= 0 {
		return zero, nil, geom.ErrInvalid
	}
	patch, err := exactFaceSupportPatch(Face{Surface: support}, o)
	if err != nil {
		return zero, nil, err
	}
	planes, err := mappedSplineInteriorSupportKnotPlanes(patch, 2)
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
	roots, err := mappedSplineRequiredSupportRoots(ctx, raw, planes, o)
	if err != nil {
		return zero, nil, err
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
	a, b := mappedSplineModelStation(roots[0], weight), mappedSplineModelStation(roots[1], weight)
	firstIsJoin, secondIsJoin := a.Cmp(join) == 0, b.Cmp(join) == 0
	if a.Cmp(b) >= 0 || !firstIsJoin && !secondIsJoin {
		return zero, nil, fmt.Errorf("one exact support crossing must match the stored model knot: %w", geom.ErrConvergence)
	}
	extra := a
	if firstIsJoin {
		extra = b
	}
	if err := ProveMappedSplineMixedSplitTransfer(ctx, support, mapped, pieces, join, extra, o); err != nil {
		return zero, nil, err
	}
	return mapped, extra, nil
}

// ProveMappedSplineMixedSplitTransfer compares three exact raw restrictions
// against two stored model Béziers, one of which is further restricted at
// the additional cut. Both input cuts are in whole normalized traversal.
func ProveMappedSplineMixedSplitTransfer(ctx context.Context, support geom.Surface, mapped MappedSplineUV, pieces [2]*geom.ExactSplineCurve, join, extra *big.Rat, o geom.SurfacePatchOptions) error {
	one := big.NewRat(1, 1)
	if ctx == nil || support == nil || mapped.weight == nil || pieces[0] == nil || pieces[1] == nil ||
		join == nil || extra == nil || join.Sign() <= 0 || extra.Sign() <= 0 ||
		join.Cmp(one) >= 0 || extra.Cmp(one) >= 0 || join.Cmp(extra) == 0 {
		return geom.ErrInvalid
	}
	for i, piece := range pieces {
		var err error
		pieces[i], err = piece.Reparameterize(0, 1, false, o)
		if err != nil {
			return err
		}
	}
	middle := [2]*big.Rat{join, extra}
	if join.Cmp(extra) > 0 {
		middle = [2]*big.Rat{extra, join}
	}
	cuts := [4]*big.Rat{new(big.Rat), middle[0], middle[1], one}
	for i := range 3 {
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
