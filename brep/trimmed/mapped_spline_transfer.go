package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// ExactTrimModelBezier composes an original bounded pcurve in one original
// support knot cell. The output uses normalized trim traversal or its exact
// reverse and retains the rational model coefficients. A multi-span support
// keeps its original UV coordinates. Its trim control hull must fit in one
// closed original knot rectangle. A represented affine placement is allowed.
func ExactTrimModelBezier(ctx context.Context, support geom.Surface, trim PCurve, reverse bool, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	if ctx == nil || support == nil || trim == nil {
		return nil, geom.ErrInvalid
	}
	p, err := exactFaceSupportPatch(Face{Surface: support}, o)
	if err != nil {
		return nil, err
	}
	c, err := polynomialTrimDescriptor(ctx, trim, reverse, o)
	if err != nil {
		return nil, err
	}
	p, err = trimModelSupportCell(ctx, p, c, o)
	if err != nil {
		return nil, err
	}
	return p.ComposeParameterCurve(ctx, c, o)
}

// Exact containment of the positive-weight UV control hull proves the entire
// original source trim lies in the selected knot rectangle. Shared knot lines
// have the same C0 lift on either side of a clamped continuous support.
func trimModelSupportCell(ctx context.Context, p *geom.SurfacePatch, uv *geom.ExactSplineCurve, o geom.SurfacePatchOptions) (*geom.SurfacePatch, error) {
	u, v := p.KnotBreaks()
	if len(u) == 2 && len(v) == 2 {
		return p, nil
	}
	net, _, err := uv.BezierControlNet(ctx, o)
	if err != nil {
		return nil, err
	}
	var box [2][2]*big.Rat
	for i, h := range net.Control {
		for axis := range box {
			x := new(big.Rat).Quo(h[axis], h[3])
			if i == 0 || x.Cmp(box[axis][0]) < 0 {
				box[axis][0] = x
			}
			if i == 0 || x.Cmp(box[axis][1]) > 0 {
				box[axis][1] = x
			}
		}
	}
	var ranges [2][2]float64
	for axis, knots := range [2][]float64{u, v} {
		found := false
		for i := 0; i+1 < len(knots); i++ {
			if box[axis][0].Cmp(ratInset(knots[i])) >= 0 && box[axis][1].Cmp(ratInset(knots[i+1])) <= 0 {
				ranges[axis] = [2]float64{knots[i], knots[i+1]}
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("original trim control hull spans support knot cells: %w", geom.ErrConvergence)
		}
	}
	return p.Trim(geom.ParameterRectangle{
		UMin: ranges[0][0], UMax: ranges[0][1], VMin: ranges[1][0], VMax: ranges[1][1],
	})
}

// ProveMappedSplineTransfer recovers a positive owner speed for an original
// directed cubic or quartic spline restriction and a standard model Bézier
// edge. The endpoint tangent proposes the map. Whole-curve homogeneous
// equality certifies it over the complete original edge. The model basis
// may have its own represented parameter interval.
func ProveMappedSplineTransfer(ctx context.Context, support geom.Surface, source SplineUV, model *geom.ExactSplineCurve, o geom.SurfacePatchOptions) (MappedSplineUV, error) {
	var zero MappedSplineUV
	if ctx == nil || model == nil {
		return zero, geom.ErrInvalid
	}
	raw, err := ExactTrimModelBezier(ctx, support, source, false, o)
	if err != nil {
		return zero, err
	}
	normalized, err := model.Reparameterize(0, 1, false, o)
	if err != nil {
		return zero, err
	}
	weight, err := mappedSplineEndpointWeight(ctx, raw, normalized, o)
	if err != nil {
		return zero, err
	}
	q, err := NewMappedSplineUV(ctx, source, weight, o)
	if err != nil {
		return zero, err
	}
	if err := proveMappedSplineLift(ctx, support, q, normalized, o); err != nil {
		return zero, err
	}
	return q, nil
}

func mappedSplineEndpointWeight(ctx context.Context, raw, model *geom.ExactSplineCurve, o geom.SurfacePatchOptions) (*big.Rat, error) {
	a, _, err := raw.BezierControlNet(ctx, o)
	if err != nil {
		return nil, err
	}
	b, _, err := model.BezierControlNet(ctx, o)
	if err != nil {
		return nil, err
	}
	if len(a.Control) < 2 || len(b.Control) < 2 {
		return nil, geom.ErrDegenerate
	}
	tangent := func(span geom.ExactBezierSpan, axis int) *big.Rat {
		p, q := span.Control[0], span.Control[1]
		num := new(big.Rat).Sub(new(big.Rat).Mul(q[axis], p[3]), new(big.Rat).Mul(p[axis], q[3]))
		num.Mul(num, big.NewRat(int64(len(span.Control)-1), 1))
		return num.Quo(num, new(big.Rat).Mul(p[3], p[3]))
	}
	var weight *big.Rat
	for axis := range 3 {
		x := tangent(a, axis)
		if x.Sign() != 0 {
			weight = new(big.Rat).Quo(tangent(b, axis), x)
			break
		}
	}
	if weight == nil || weight.Sign() <= 0 {
		return nil, geom.ErrConvergence
	}
	return weight, nil
}

func proveMappedSplineLift(ctx context.Context, support geom.Surface, curve MappedSplineUV, model *geom.ExactSplineCurve, o geom.SurfacePatchOptions) error {
	lift, err := ExactTrimModelBezier(ctx, support, curve, false, o)
	if err != nil {
		return err
	}
	comparison, err := geom.CompareExactCurvesContext(ctx, lift, model, geom.ExactCurveComparisonOptions{
		MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	if err != nil {
		return err
	}
	if !comparison.Complete || !comparison.Equal {
		return geom.ErrConvergence
	}
	return nil
}

// ProveMappedSplineAsynchronousTransfer proves an original trim against one
// stored model Bézier, retaining an isolated exact model station if the trim
// crosses one original support knot. A nil station means the complete trim
// fits one original support cell. The model Bézier need not have a knot at a
// crossing: its exact rational restrictions are compared on each side.
func ProveMappedSplineAsynchronousTransfer(ctx context.Context, support geom.Surface, source SplineUV, model *geom.ExactSplineCurve, o geom.SurfacePatchOptions) (MappedSplineUV, *big.Rat, error) {
	return proveMappedSplineAsynchronousTransfer(ctx, support, source, model, 1, o)
}

// ProveMappedSplineCellLocalAsynchronousTransfer proves one original model
// Bézier on a chart with up to two authored knots. Only zero or one interior
// event may occur on this coedge. The full raw map and every model restriction
// must agree exactly, including when no support event occurs.
func ProveMappedSplineCellLocalAsynchronousTransfer(ctx context.Context, support geom.Surface, source SplineUV, model *geom.ExactSplineCurve, o geom.SurfacePatchOptions) (MappedSplineUV, *big.Rat, error) {
	return proveMappedSplineAsynchronousTransfer(ctx, support, source, model, 2, o)
}

func proveMappedSplineAsynchronousTransfer(ctx context.Context, support geom.Surface, source SplineUV, model *geom.ExactSplineCurve, maxSupportKnots int, o geom.SurfacePatchOptions) (MappedSplineUV, *big.Rat, error) {
	mapped, stations, err := proveMappedSplineInferredSupportTransfer(ctx, support, source, model, maxSupportKnots, 1, o)
	if len(stations) == 0 {
		return mapped, nil, err
	}
	return mapped, stations[0], err
}

// ProveMappedSplineInferredSupportTransfer proves an original owner map on a
// single model Bézier across up to two transverse original support events.
// It preserves every exact raw restriction. An empty stream certifies the
// original edge stays within one support cell.
func ProveMappedSplineInferredSupportTransfer(ctx context.Context, support geom.Surface, source SplineUV, model *geom.ExactSplineCurve, o geom.SurfacePatchOptions) (MappedSplineUV, []*big.Rat, error) {
	return proveMappedSplineInferredSupportTransfer(ctx, support, source, model, 2, 2, o)
}

func proveMappedSplineInferredSupportTransfer(ctx context.Context, support geom.Surface, source SplineUV, model *geom.ExactSplineCurve, maxSupportKnots, maxActiveKnots int, o geom.SurfacePatchOptions) (MappedSplineUV, []*big.Rat, error) {
	var zero MappedSplineUV
	if ctx == nil || support == nil || model == nil {
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
	if len(planes) == 0 {
		q, err := ProveMappedSplineTransfer(ctx, support, source, model, o)
		return q, nil, err
	}
	one := big.NewRat(1, 1)
	raw, err := NewMappedSplineUV(ctx, source, one, o)
	if err != nil {
		return zero, nil, err
	}
	crossings, err := mappedSplineSupportRoots(ctx, raw, planes, false, o)
	if err != nil {
		return zero, nil, err
	}
	if len(crossings) == 0 {
		q, err := ProveMappedSplineTransfer(ctx, support, source, model, o)
		return q, nil, err
	}
	if len(crossings) > maxActiveKnots {
		return zero, nil, fmt.Errorf("the original model coedge permits at most %d active support knots: %w", maxActiveKnots, geom.ErrConvergence)
	}
	normalized, err := model.Reparameterize(0, 1, false, o)
	if err != nil {
		return zero, nil, err
	}
	weight, err := mappedSplineFirstBranchWeight(ctx, support, raw, normalized, crossings[0], one, o)
	if err != nil {
		return zero, nil, err
	}
	mapped, err := NewMappedSplineUV(ctx, source, weight, o)
	if err != nil {
		return zero, nil, err
	}
	stations := make([]*big.Rat, len(crossings))
	for i, crossing := range crossings {
		stations[i] = mappedSplineModelStation(crossing, weight)
	}
	if err := proveMappedSplineBezierPartition(ctx, support, mapped, normalized, stations, o); err != nil {
		return zero, nil, err
	}
	return mapped, stations, nil
}

func mappedSplineFirstBranchWeight(ctx context.Context, support geom.Surface, raw MappedSplineUV, firstModel *geom.ExactSplineCurve, rawEnd, modelEnd *big.Rat, o geom.SurfacePatchOptions) (*big.Rat, error) {
	first, err := restrictMappedSource(ctx, raw, new(big.Rat), rawEnd, o)
	if err != nil {
		return nil, err
	}
	lift, err := ExactTrimModelBezier(ctx, support, first, false, o)
	if err != nil {
		return nil, err
	}
	weight, err := mappedSplineEndpointWeight(ctx, lift, firstModel, o)
	if err != nil {
		return nil, err
	}
	// Each branch uses its own normalized interval. Convert the tangent
	// ratio to the full original raw and model intervals.
	return weight.Mul(weight, new(big.Rat).Quo(rawEnd, modelEnd)), nil
}

func mappedSplineModelStation(raw, weight *big.Rat) *big.Rat {
	den := new(big.Rat).Add(raw, new(big.Rat).Mul(weight, new(big.Rat).Sub(big.NewRat(1, 1), raw)))
	return new(big.Rat).Quo(raw, den)
}

// ProveMappedSplineSplitTransfer compares both exact raw restrictions of one
// already mapped owner with the corresponding parts of a single model Bézier.
// The station is in normalized model traversal. Either support side may use
// the same original knot cell when its complete control hull fits there.
func ProveMappedSplineSplitTransfer(ctx context.Context, support geom.Surface, mapped MappedSplineUV, model *geom.ExactSplineCurve, station *big.Rat, o geom.SurfacePatchOptions) error {
	return proveMappedSplineBezierPartition(ctx, support, mapped, model, []*big.Rat{station}, o)
}

// ProveMappedSplineBezierSupportPartition proves an original owner
// restriction at two strictly ordered normalized stations on one model
// Bézier. Its three exact branches use the original model basis and
// only restrict the existing raw owner map.
func ProveMappedSplineBezierSupportPartition(ctx context.Context, support geom.Surface, mapped MappedSplineUV, model *geom.ExactSplineCurve, stations [2]*big.Rat, o geom.SurfacePatchOptions) error {
	return proveMappedSplineBezierPartition(ctx, support, mapped, model, stations[:], o)
}

// ProveMappedSplineBezierThreeSupportPartition proves all four exact
// restrictions of one mapped owner against a single original model Bézier.
// The three strictly ordered model stations may be the union of both owners'
// support events. The original raw map stays fixed across every restriction.
func ProveMappedSplineBezierThreeSupportPartition(ctx context.Context, support geom.Surface, mapped MappedSplineUV, model *geom.ExactSplineCurve, stations [3]*big.Rat, o geom.SurfacePatchOptions) error {
	return proveMappedSplineBezierPartition(ctx, support, mapped, model, stations[:], o)
}

func proveMappedSplineBezierPartition(ctx context.Context, support geom.Surface, mapped MappedSplineUV, model *geom.ExactSplineCurve, stations []*big.Rat, o geom.SurfacePatchOptions) error {
	if ctx == nil || support == nil || mapped.weight == nil || model == nil {
		return geom.ErrInvalid
	}
	one := big.NewRat(1, 1)
	cuts := []*big.Rat{new(big.Rat)}
	for _, station := range stations {
		if station == nil || station.Cmp(cuts[len(cuts)-1]) <= 0 || station.Cmp(one) >= 0 {
			return geom.ErrInvalid
		}
		cuts = append(cuts, station)
	}
	cuts = append(cuts, one)
	normalized, err := model.Reparameterize(0, 1, false, o)
	if err != nil {
		return err
	}
	for i := range len(cuts) - 1 {
		part, err := restrictMappedSource(ctx, mapped, cuts[i], cuts[i+1], o)
		if err != nil {
			return err
		}
		original, err := normalized.RationalSegment(ctx, cuts[i], cuts[i+1], geom.TubeAffineLineOptions{
			MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
		})
		if err != nil {
			return err
		}
		if err := proveMappedSplineLift(ctx, support, part, original, o); err != nil {
			return err
		}
	}
	return nil
}

// ProveMappedSplineSupportKnotTransfer checks one unsplit original UV
// restriction against a two-piece model curve with the given normalized join.
// A unique transverse rational UV event proposes a positive owner speed.
// Exact homogeneous equality on both original support cells proves it.
func ProveMappedSplineSupportKnotTransfer(ctx context.Context, support geom.Surface, source SplineUV, pieces [2]*geom.ExactSplineCurve, join *big.Rat, o geom.SurfacePatchOptions) (MappedSplineUV, error) {
	return proveMappedSplineSupportKnotsTransfer(ctx, support, source, pieces[:], []*big.Rat{join}, o)
}

// ProveMappedSplineTwoSupportKnotsTransfer proves a single original UV
// restriction against three stored model pieces with two normalized joins.
// One positive owner parameter map must explain both isolated transverse
// support crossings and all three exact homogeneous model lifts.
func ProveMappedSplineTwoSupportKnotsTransfer(ctx context.Context, support geom.Surface, source SplineUV, pieces [3]*geom.ExactSplineCurve, joins [2]*big.Rat, o geom.SurfacePatchOptions) (MappedSplineUV, error) {
	return proveMappedSplineSupportKnotsTransfer(ctx, support, source, pieces[:], joins[:], o)
}

func proveMappedSplineSupportKnotsTransfer(ctx context.Context, support geom.Surface, source SplineUV, pieces []*geom.ExactSplineCurve, joins []*big.Rat, o geom.SurfacePatchOptions) (MappedSplineUV, error) {
	var zero MappedSplineUV
	one := big.NewRat(1, 1)
	if ctx == nil || support == nil {
		return zero, geom.ErrInvalid
	}
	cuts := []*big.Rat{new(big.Rat)}
	for _, join := range joins {
		if join == nil || join.Cmp(cuts[len(cuts)-1]) <= 0 || join.Cmp(one) >= 0 {
			return zero, geom.ErrInvalid
		}
		cuts = append(cuts, join)
	}
	cuts = append(cuts, one)
	for _, piece := range pieces {
		if piece == nil {
			return zero, geom.ErrInvalid
		}
	}
	p, err := exactFaceSupportPatch(Face{Surface: support}, o)
	if err != nil {
		return zero, err
	}
	planes, err := mappedSplineInteriorSupportKnotPlanes(p, 2)
	if err != nil {
		return zero, err
	}
	if len(planes) != len(joins) {
		return zero, fmt.Errorf("stored model junctions must match original support knots: %w", geom.ErrConvergence)
	}
	raw, err := NewMappedSplineUV(ctx, source, one, o)
	if err != nil {
		return zero, err
	}
	roots, err := mappedSplineRequiredSupportRoots(ctx, raw, planes, o)
	if err != nil {
		return zero, err
	}
	num := new(big.Rat).Mul(roots[0], new(big.Rat).Sub(one, joins[0]))
	den := new(big.Rat).Mul(joins[0], new(big.Rat).Sub(one, roots[0]))
	mapped, err := NewMappedSplineUV(ctx, source, num.Quo(num, den), o)
	if err != nil {
		return zero, err
	}
	for i := 1; i < len(roots); i++ {
		proposed, err := mapped.AuthoredParameter(joins[i])
		if err != nil {
			return zero, err
		}
		exact, err := raw.AuthoredParameter(roots[i])
		if err != nil {
			return zero, err
		}
		if proposed.Cmp(exact) != 0 {
			return zero, fmt.Errorf("one owner parameter map must explain all support crossings: %w", geom.ErrConvergence)
		}
	}
	for i, model := range pieces {
		part, err := restrictMappedSource(ctx, mapped, cuts[i], cuts[i+1], o)
		if err != nil {
			return zero, err
		}
		normalized, err := model.Reparameterize(0, 1, false, o)
		if err != nil {
			return zero, err
		}
		if err := proveMappedSplineLift(ctx, support, part, normalized, o); err != nil {
			return zero, err
		}
	}
	return mapped, nil
}

func mappedSplineRequiredSupportRoots(ctx context.Context, raw MappedSplineUV, planes [][4]*big.Rat, o geom.SurfacePatchOptions) ([]*big.Rat, error) {
	return mappedSplineSupportRoots(ctx, raw, planes, true, o)
}

func mappedSplineSupportRoots(ctx context.Context, raw MappedSplineUV, planes [][4]*big.Rat, requireEveryKnot bool, o geom.SurfacePatchOptions) ([]*big.Rat, error) {
	events := geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	var roots []*big.Rat
	for _, plane := range planes {
		root, err := mappedSplineSupportKnotRoot(ctx, raw.curve, plane, o, events)
		if err != nil {
			return nil, err
		}
		if root == nil {
			if requireEveryKnot {
				return nil, fmt.Errorf("original support crossing missing from unsplit coedge: %w", geom.ErrConvergence)
			}
			continue
		}
		roots = append(roots, root)
	}
	slices.SortFunc(roots, func(a, b *big.Rat) int { return a.Cmp(b) })
	return roots, nil
}
