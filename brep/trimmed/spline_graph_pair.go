package trimmed

import (
	"context"
	"errors"
	"fmt"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineGraphRegion describes one closed region and its shared section edges.
type SplineGraphRegion struct {
	FaceA, FaceB  int
	OuterContour  int
	InnerContours []int
	OuterEdges    []int
	InnerEdges    [][]int
}

// SealSplineGraphPair forms all bounded solids between two rational spline
// graphs with identical XY parameterization. The degrees, knot breaks and
// positive rational weight fields may differ. The
// chart must be provably injective in XY and the exact height difference must
// have a strict boundary sign. Closed transverse contours can form holes;
// coincident areas, tangencies and unresolved arrangements return errors.
//
// Both faces retain the original rational surfaces and use the same section
// p-curve in their shared chart.
func SealSplineGraphPair(ctx context.Context, a, b *geom.BSplineSurface, tolerance float64, opts intersect.SplinePlaneOptions, proof geom.SurfaceInjectivityOptions) (Solid, []SplineGraphRegion, error) {
	if ctx == nil || a == nil || b == nil || !finite(tolerance) || tolerance <= 0 {
		return Solid{}, nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, nil, e
	}
	delta, _, _, e := exactGraphPatchesContext(ctx, a, b, opts.Patch, opts.Domain)
	if e != nil {
		return Solid{}, nil, e
	}
	orientation, e := projectedPatchProof(delta, proof)
	if e != nil {
		return Solid{}, nil, e
	}
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	pl, _ := geom.NewPlane(frame.Origin(), frame.ZAxis())
	sign, e := geom.ClassifyPlaneBoundarySigns(delta, pl, geom.PlaneBoundarySignOptions{
		MaxOperations: proof.MaxOperations, MaxCoefficientBits: proof.MaxCoefficientBits,
	})
	if e != nil {
		return Solid{}, nil, e
	}
	if !sign.Complete || sign.StrictSign == 0 {
		return Solid{}, nil, fmt.Errorf("trimmed: spline graph pair requires a strict boundary height order")
	}
	if opts.ResidualTolerance == 0 {
		opts.ResidualTolerance = tolerance
	}
	loops, trace, e := intersect.TraceSplineHeightDifferenceContext(ctx, a, b, opts)
	if e != nil {
		return Solid{}, nil, e
	}
	if len(trace.Isolation.Tangencies) != 0 {
		return Solid{}, nil, geom.ErrConvergence
	}
	out, history, e := assembleNestedSplineSections(ctx, a, b, frame, loops, delta.Domain(), sign.StrictSign, orientation, tolerance)
	if e != nil {
		return Solid{}, nil, e
	}
	regions := make([]SplineGraphRegion, 0, len(history))
	for _, region := range history {
		regions = append(regions, SplineGraphRegion{region.SplineFace, region.PlaneFace, region.OuterContour,
			region.InnerContours, region.OuterEdges, region.InnerEdges})
	}
	return out, regions, nil
}

func exactGraphPatches(a, b *geom.BSplineSurface, limits geom.SurfacePatchOptions, d geom.ParameterRectangle) (difference, pa, pb *geom.SurfacePatch, err error) {
	return exactGraphPatchesContext(context.Background(), a, b, limits, d)
}

func exactGraphPatchesContext(ctx context.Context, a, b *geom.BSplineSurface, limits geom.SurfacePatchOptions, d geom.ParameterRectangle) (difference, pa, pb *geom.SurfacePatch, err error) {
	if err = ctx.Err(); err != nil {
		return
	}
	pa, err = geom.NewSurfacePatch(a, limits)
	if err != nil {
		return
	}
	pb, err = geom.NewSurfacePatch(b, limits)
	if err != nil {
		return
	}
	difference, err = geom.NewSurfaceHeightDifferencePatch(pa, pb, geom.SurfaceHeightPatchOptions{
		MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits,
	})
	if errors.Is(err, geom.ErrInvalid) {
		difference, err = geom.NewGeneralSurfaceHeightDifferencePatch(ctx, pa, pb, geom.GeneralSurfaceHeightOptions{
			MaxPoles: limits.MaxPoles, MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits,
		})
	}
	if err != nil || d == (geom.ParameterRectangle{}) {
		return
	}
	difference, err = difference.Trim(d)
	if err == nil {
		pa, err = pa.Trim(d)
	}
	if err == nil {
		pb, err = pb.Trim(d)
	}
	return
}
