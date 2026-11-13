package trimmed

import (
	"context"
	"errors"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// CombineAxisLineSplineGraphs handles a single certified straight contact of
// two strictly positive identity-chart shared-base graphs. Original spline
// supports and exact contact coedges are retained for crossing operations.
// An even-order contact gives an ordered envelope; pinched subtraction
// is rejected. If a crossing's exact rational coordinate is not representable in the native trim
// endpoint type, AxisLine is still returned in history with ErrConvergence.
func CombineAxisLineSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	op parametric.BooleanOp, axis int, tolerance float64, o intersect.SplinePlaneOptions, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference ||
		axis < 0 || axis > 1 || !finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return Solid{}, h, geom.ErrInvalid
	}
	p, e := prepareSharedBaseGraphContact(ctx, base, a, b, o)
	if e != nil {
		return Solid{}, h, e
	}
	return assembleAxisLineGraphBoolean(ctx, base, a, b, p[0], p[1], op, axis, tolerance, o, proof)
}

func assembleAxisLineGraphBoolean(ctx context.Context, base, a, b *geom.BSplineSurface, ap, bp *geom.SurfacePatch,
	op parametric.BooleanOp, axis int, tolerance float64, o intersect.SplinePlaneOptions, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	section, e := geom.CertifySplineGraphAxisLine(ctx, ap, bp, axis, proof)
	multiplicity := 1
	if errors.Is(e, geom.ErrConvergence) {
		factor, err := geom.DiscoverSplineGraphAxisContact(ctx, ap, bp, axis, geom.GraphFactorAxisLineOptions{GraphPointContactOptions: proof})
		section, multiplicity, e = factor.Line, factor.Multiplicity, err
	}
	if e != nil || !section.Proven {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, h, e
	}
	h.AxisLine, h.AxisMultiplicity = &section, multiplicity
	if section.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return Solid{}, h, geom.ErrConvergence
	}
	if section.LowerSide == section.UpperSide {
		if op == parametric.Difference {
			if section.LowerSide < 0 {
				return Solid{}, h, nil
			}
			return Solid{}, h, fmt.Errorf("trimmed: graph difference pinches along its certified contact line: %w", geom.ErrDegenerate)
		}
		return combineOrderedSplineGraphSolids(base, a, b, section.LowerSide, op, tolerance, o, h)
	}
	f, e := geom.RepresentedExactScalar(section.Coordinate)
	if e != nil {
		return Solid{}, h, geom.ErrConvergence
	}
	lower, upper := section.Domain, section.Domain
	if axis == 0 {
		lower.UMax, upper.UMin = f, f
	} else {
		lower.VMax, upper.VMin = f, f
	}
	regions := []openGraphRegion{
		{loop: partitionGraphRectangleLoop(lower), sign: section.LowerSide},
		{loop: partitionGraphRectangleLoop(upper), sign: section.UpperSide},
	}
	out, report, e := combineOpenGraphRegions(ctx, base, a, b, regions, op, tolerance)
	if e != nil {
		return Solid{}, h, e
	}
	if e := validateExactGraphPartitionEdges(ctx, out, proof); e != nil {
		return Solid{}, h, e
	}
	report.AxisLine, report.AxisMultiplicity = &section, multiplicity
	return out, report, nil
}

// CombineParallelLineSplineGraphs performs a Boolean of two strictly positive
// shared-base graphs whose entire contact consists of multiple certified
// parallel axis lines. It retains the original supports and may return
// multiple disjoint material components for subtraction.
func CombineParallelLineSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	op parametric.BooleanOp, axis int, tolerance float64, o intersect.SplinePlaneOptions, proof geom.GraphFactorAxisLineOptions) (
	Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference ||
		axis < 0 || axis > 1 || !finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return Solid{}, h, geom.ErrInvalid
	}
	p, e := prepareSharedBaseGraphContact(ctx, base, a, b, o)
	if e != nil {
		return Solid{}, h, e
	}
	return assembleParallelLineGraphBoolean(ctx, base, a, b, p[0], p[1], op, axis, tolerance, proof)
}

func assembleParallelLineGraphBoolean(ctx context.Context, base, a, b *geom.BSplineSurface, ap, bp *geom.SurfacePatch,
	op parametric.BooleanOp, axis int, tolerance float64, proof geom.GraphFactorAxisLineOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	p, e := geom.DiscoverSplineGraphParallelLines(ctx, ap, bp, axis, proof)
	h.ParallelLines = &p
	if e != nil {
		return Solid{}, h, e
	}
	if !p.Proven || len(p.Lines) < 2 || len(p.Lines) > 12 ||
		len(p.Bands) != len(p.Lines)+1 || p.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return Solid{}, h, geom.ErrConvergence
	}
	regions := make([]openGraphRegion, len(p.Bands))
	for i, b := range p.Bands {
		a, e := geom.RepresentedExactScalar(b.Start)
		if e != nil {
			return Solid{}, h, geom.ErrConvergence
		}
		z, e := geom.RepresentedExactScalar(b.End)
		if e != nil {
			return Solid{}, h, geom.ErrConvergence
		}
		d := p.Domain
		if axis == 0 {
			d.UMin, d.UMax = a, z
		} else {
			d.VMin, d.VMax = a, z
		}
		regions[i] = openGraphRegion{loop: partitionGraphRectangleLoop(d), sign: b.Side}
	}
	out, report, e := combineOpenGraphRegions(ctx, base, a, b, regions, op, tolerance)
	if e != nil {
		return Solid{}, h, e
	}
	if e := validateExactGraphPartitionEdges(ctx, out, proof.GraphPointContactOptions); e != nil {
		return Solid{}, h, e
	}
	report.ParallelLines = &p
	return out, report, nil
}

// CombineCrossingLineSplineGraphs performs a union or intersection when two
// exact axis-aligned contact lines cross within strictly positive shared-base
// graphs. The four sectors retain their original supports. Subtraction
// would retain diagonally touching material and returns ErrDegenerate.
func CombineCrossingLineSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	op parametric.BooleanOp, tolerance float64, o intersect.SplinePlaneOptions, proof geom.GraphCrossingLinesOptions) (
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
	return assembleCrossingLineGraphBoolean(ctx, base, a, b, p[0], p[1], op, tolerance, proof)
}

func assembleCrossingLineGraphBoolean(ctx context.Context, base, a, b *geom.BSplineSurface, ap, bp *geom.SurfacePatch,
	op parametric.BooleanOp, tolerance float64, proof geom.GraphCrossingLinesOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	p, e := geom.DiscoverSplineGraphCrossingLines(ctx, ap, bp, proof)
	if e != nil {
		return Solid{}, h, e
	}
	if !p.Proven || p.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return Solid{}, h, geom.ErrConvergence
	}
	h.CrossingLines = &p
	if op == parametric.Difference {
		return Solid{}, h, fmt.Errorf("trimmed: graph difference pinches at crossing contact lines: %w", geom.ErrDegenerate)
	}
	point, e := exactGraphPartitionUV(p.Crossing)
	if e != nil {
		return Solid{}, h, geom.ErrConvergence
	}
	regions := make([]openGraphRegion, 0, len(p.Sectors))
	for _, s := range p.Sectors {
		d := p.Domain
		if s.Direction[0] > 0 {
			d.UMin = point.X
		} else {
			d.UMax = point.X
		}
		if s.Direction[1] > 0 {
			d.VMin = point.Y
		} else {
			d.VMax = point.Y
		}
		regions = append(regions, openGraphRegion{loop: partitionGraphRectangleLoop(d), sign: s.Side})
	}
	out, report, e := combineOpenGraphRegions(ctx, base, a, b, regions, op, tolerance)
	if e != nil {
		return Solid{}, h, e
	}
	if e := validateExactGraphPartitionEdges(ctx, out, proof.GraphPointContactOptions); e != nil {
		return Solid{}, h, e
	}
	report.CrossingLines = &p
	return out, report, nil
}

// prepareSharedBaseGraphContact validates the model semantics before any
// contact specialization is allowed to classify or assemble material.
func prepareSharedBaseGraphContact(ctx context.Context, base, a, b *geom.BSplineSurface,
	o intersect.SplinePlaneOptions) ([2]*geom.SurfacePatch, error) {
	var p [2]*geom.SurfacePatch
	if e := requireIdentitySplineChart(ctx, a, o.Patch); e != nil {
		return p, e
	}
	pl, _ := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	for i, s := range [2]*geom.BSplineSurface{a, b} {
		d, top, _, e := exactGraphPatchesContext(ctx, s, base, o.Patch, o.Domain)
		if e == nil {
			e = requireGraphAbove(ctx, d, pl, o)
		}
		if e != nil {
			return [2]*geom.SurfacePatch{}, e
		}
		p[i] = top
	}
	return p, nil
}

// CombineContactGridSplineGraphs combines strictly positive shared-base
// graphs with an exhaustive rectilinear contact grid. Original supports
// are kept on every cell. Subtraction is allowed for separated parallel
// bands; crossing grids have diagonally touching material and are rejected.
func CombineContactGridSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	op parametric.BooleanOp, tolerance float64, o intersect.SplinePlaneOptions, proof geom.GraphCrossingLinesOptions) (
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
	return assembleContactGridGraphBoolean(ctx, base, a, b, p[0], p[1], op, tolerance, proof)
}

func assembleContactGridGraphBoolean(ctx context.Context, base, a, b *geom.BSplineSurface, ap, bp *geom.SurfacePatch,
	op parametric.BooleanOp, tolerance float64, proof geom.GraphCrossingLinesOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	p, e := geom.DiscoverSplineGraphContactGrid(ctx, ap, bp, proof)
	if e != nil {
		return Solid{}, h, e
	}
	if !p.Proven || p.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || len(p.Cells) < 2 || len(p.Cells) > 64 {
		return Solid{}, h, geom.ErrConvergence
	}
	h.ContactGrid = &p
	if op == parametric.Difference && len(p.Lines[0]) != 0 && len(p.Lines[1]) != 0 {
		return Solid{}, h, fmt.Errorf("trimmed: graph difference pinches at contact grid crossings: %w", geom.ErrDegenerate)
	}
	regions := make([]openGraphRegion, len(p.Cells))
	for i, c := range p.Cells {
		lo, e := exactGraphPartitionUV(c.Lower)
		if e != nil {
			return Solid{}, h, geom.ErrConvergence
		}
		hi, e := exactGraphPartitionUV(c.Upper)
		if e != nil {
			return Solid{}, h, geom.ErrConvergence
		}
		d := geom.ParameterRectangle{UMin: lo.X, UMax: hi.X, VMin: lo.Y, VMax: hi.Y}
		regions[i] = openGraphRegion{loop: partitionGraphRectangleLoop(d), sign: c.Side}
	}
	out, report, e := combineOpenGraphRegions(ctx, base, a, b, regions, op, tolerance)
	if e != nil {
		return Solid{}, h, e
	}
	if e := validateExactGraphPartitionEdges(ctx, out, proof.GraphPointContactOptions); e != nil {
		return Solid{}, h, e
	}
	report.ContactGrid = &p
	return out, report, nil
}
