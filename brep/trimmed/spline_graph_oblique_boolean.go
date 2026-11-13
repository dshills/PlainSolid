package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// CombineObliqueLineSplineGraphs performs a shared-base graph Boolean when
// an exhaustive exact oblique line proof divides the unit square into two
// strict-side regions. Results retain their original supports and one exact
// contact edge; nonbinary UV terminals are retained in history on failure.
func CombineObliqueLineSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	op parametric.BooleanOp, tolerance float64, o intersect.SplinePlaneOptions, proof geom.GraphFactorAxisLineOptions) (
	Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference ||
		!finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return Solid{}, h, geom.ErrInvalid
	}
	p, e := prepareSharedBaseGraphContact(ctx, base, a, b, o)
	if e != nil {
		return Solid{}, h, e
	}
	return assembleObliqueLineGraphBoolean(ctx, base, a, b, p[0], p[1], op, tolerance, proof)
}

func assembleObliqueLineGraphBoolean(ctx context.Context, base, a, b *geom.BSplineSurface, ap, bp *geom.SurfacePatch,
	op parametric.BooleanOp, tolerance float64, proof geom.GraphFactorAxisLineOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	p, e := geom.DiscoverSplineGraphObliqueLine(ctx, ap, bp, proof)
	if e != nil {
		return Solid{}, h, e
	}
	if !p.Proven || p.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return Solid{}, h, geom.ErrConvergence
	}
	h.ObliqueLine = &p
	regions, e := obliqueGraphRegions(p.Regions[:], [][2][2]*big.Rat{p.Terminals})
	if e != nil {
		return Solid{}, h, e
	}
	out, report, e := combineOpenGraphRegions(ctx, base, a, b, regions, op, tolerance)
	if e != nil {
		return Solid{}, h, e
	}
	if e := validateExactGraphPartitionEdges(ctx, out, proof.GraphPointContactOptions); e != nil {
		return Solid{}, h, e
	}
	report.ObliqueLine = &p
	return out, report, nil
}

// CombineParallelObliqueLineSplineGraphs combines shared-base graphs divided
// into three signed regions by two exhaustive oblique parallel contacts.
func CombineParallelObliqueLineSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	op parametric.BooleanOp, tolerance float64, o intersect.SplinePlaneOptions, proof geom.GraphFactorAxisLineOptions) (
	Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference ||
		!finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return Solid{}, h, geom.ErrInvalid
	}
	p, e := prepareSharedBaseGraphContact(ctx, base, a, b, o)
	if e != nil {
		return Solid{}, h, e
	}
	return assembleParallelObliqueLineGraphBoolean(ctx, base, a, b, p[0], p[1], op, tolerance, proof)
}

func assembleParallelObliqueLineGraphBoolean(ctx context.Context, base, a, b *geom.BSplineSurface, ap, bp *geom.SurfacePatch,
	op parametric.BooleanOp, tolerance float64, proof geom.GraphFactorAxisLineOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	p, e := geom.DiscoverSplineGraphParallelObliqueLines(ctx, ap, bp, proof)
	if e != nil {
		return Solid{}, h, e
	}
	if !p.Proven || p.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return Solid{}, h, geom.ErrConvergence
	}
	h.ParallelOblique = &p
	regions, e := obliqueGraphRegions(p.Regions[:], p.Terminals[:])
	if e != nil {
		return Solid{}, h, e
	}
	out, report, e := combineOpenGraphRegions(ctx, base, a, b, regions, op, tolerance)
	if e != nil {
		return Solid{}, h, e
	}
	if e := validateExactGraphPartitionEdges(ctx, out, proof.GraphPointContactOptions); e != nil {
		return Solid{}, h, e
	}
	report.ParallelOblique = &p
	return out, report, nil
}

func obliqueGraphRegions(polygons []geom.GraphLineRegion, segments [][2][2]*big.Rat) ([]openGraphRegion, error) {
	var regions []openGraphRegion
	edges := make(map[[2]geom.Vec2]int, len(segments))
	for _, s := range segments {
		a, e := exactGraphPartitionUV(s[0])
		if e != nil {
			return nil, geom.ErrConvergence
		}
		b, e := exactGraphPartitionUV(s[1])
		if e != nil {
			return nil, geom.ErrConvergence
		}
		edges[[2]geom.Vec2{a, b}], edges[[2]geom.Vec2{b, a}] = 0, 0
	}
	for _, region := range polygons {
		var poly []geom.Vec2
		for _, x := range region.Vertices {
			v, e := exactGraphPartitionUV(x)
			if e != nil {
				return nil, geom.ErrConvergence
			}
			poly = append(poly, v)
		}
		r := openGraphRegion{sign: region.Side}
		contacts := 0
		for i, a := range poly {
			b := poly[(i+1)%len(poly)]
			side := -1
			switch {
			case a.Y == 0 && b.Y == 0:
				side = 0
			case a.X == 1 && b.X == 1:
				side = 1
			case a.Y == 1 && b.Y == 1:
				side = 2
			case a.X == 0 && b.X == 0:
				side = 3
			default:
				key := [2]geom.Vec2{a, b}
				if _, ok := edges[key]; !ok {
					return nil, geom.ErrConvergence
				}
				edges[key]++
				contacts++
			}
			r.loop = append(r.loop, openSectionPiece{a: a, b: b, side: side})
		}
		if contacts == 0 || len(poly) < 3 {
			return nil, geom.ErrConvergence
		}
		regions = append(regions, r)
	}
	for _, n := range edges {
		if n != 1 {
			return nil, geom.ErrConvergence
		}
	}
	return regions, nil
}

// CombineIntersectingObliqueLineSplineGraphs builds an exact four-sector
// union or intersection around two proved oblique contact lines.
func CombineIntersectingObliqueLineSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	op parametric.BooleanOp, tolerance float64, o intersect.SplinePlaneOptions, proof geom.GraphFactorAxisLineOptions) (
	Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference ||
		!finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return Solid{}, h, geom.ErrInvalid
	}
	p, e := prepareSharedBaseGraphContact(ctx, base, a, b, o)
	if e != nil {
		return Solid{}, h, e
	}
	return assembleIntersectingObliqueLineGraphBoolean(ctx, base, a, b, p[0], p[1], op, tolerance, proof)
}

func assembleIntersectingObliqueLineGraphBoolean(ctx context.Context, base, a, b *geom.BSplineSurface, ap, bp *geom.SurfacePatch,
	op parametric.BooleanOp, tolerance float64, proof geom.GraphFactorAxisLineOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	p, e := geom.DiscoverSplineGraphIntersectingObliqueLines(ctx, ap, bp, proof)
	if e != nil {
		return Solid{}, h, e
	}
	if !p.Proven || p.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return Solid{}, h, geom.ErrConvergence
	}
	h.IntersectingOblique = &p
	if op == parametric.Difference {
		return Solid{}, h, fmt.Errorf("trimmed: graph difference pinches at the oblique crossing: %w", geom.ErrDegenerate)
	}
	var lines [][2][2]*big.Rat
	for _, l := range p.Terminals {
		lines = append(lines, [2][2]*big.Rat{l[0], p.Crossing}, [2][2]*big.Rat{p.Crossing, l[1]})
	}
	regions, e := obliqueGraphRegions(p.Regions[:], lines)
	if e != nil {
		return Solid{}, h, e
	}
	out, report, e := combineOpenGraphRegions(ctx, base, a, b, regions, op, tolerance)
	if e != nil {
		return Solid{}, h, e
	}
	if e := validateExactGraphPartitionEdges(ctx, out, proof.GraphPointContactOptions); e != nil {
		return Solid{}, h, e
	}
	report.IntersectingOblique = &p
	return out, report, nil
}
