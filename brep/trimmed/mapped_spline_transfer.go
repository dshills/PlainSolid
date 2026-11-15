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
	a, _, err := raw.BezierControlNet(ctx, o)
	if err != nil {
		return zero, err
	}
	b, _, err := normalized.BezierControlNet(ctx, o)
	if err != nil {
		return zero, err
	}
	if len(a.Control) < 2 || len(b.Control) < 2 {
		return zero, geom.ErrDegenerate
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
		return zero, geom.ErrConvergence
	}
	q, err := NewMappedSplineUV(ctx, source, weight, o)
	if err != nil {
		return zero, err
	}
	lift, err := ExactTrimModelBezier(ctx, support, q, false, o)
	if err != nil {
		return zero, err
	}
	comparison, err := geom.CompareExactCurvesContext(ctx, lift, normalized, geom.ExactCurveComparisonOptions{
		MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	if err != nil {
		return zero, err
	}
	if !comparison.Complete || !comparison.Equal {
		return zero, geom.ErrConvergence
	}
	return q, nil
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
	events := geom.ExactCurvePlaneEventOptions{
		MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	}
	var roots []*big.Rat
	for _, plane := range planes {
		root, err := mappedSplineSupportKnotRoot(ctx, raw.curve, plane, o, events)
		if err != nil {
			return zero, err
		}
		if root == nil {
			return zero, fmt.Errorf("original support crossing missing from unsplit coedge: %w", geom.ErrConvergence)
		}
		roots = append(roots, root)
	}
	slices.SortFunc(roots, func(a, b *big.Rat) int { return a.Cmp(b) })
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
		lift, err := ExactTrimModelBezier(ctx, support, part, false, o)
		if err != nil {
			return zero, err
		}
		normalized, err := model.Reparameterize(0, 1, false, o)
		if err != nil {
			return zero, err
		}
		proof, err := geom.CompareExactCurvesContext(ctx, lift, normalized, geom.ExactCurveComparisonOptions{
			MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
		})
		if err != nil {
			return zero, err
		}
		if !proof.Complete || !proof.Equal {
			return zero, geom.ErrConvergence
		}
	}
	return mapped, nil
}
