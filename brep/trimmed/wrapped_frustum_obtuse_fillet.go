package trimmed

import (
	"context"
	"plainsolid/geom"
)

// FilletWrappedFrustumObtuseRim rounds the upper rim of an ascending cone or
// lower rim of a descending cone, selected by upper. These wider circular arcs
// cross the torus's original outer meridian seam. An exact cyclic permutation
// of its four rational meridian quadrants moves that seam to the inner side;
// no point is rounded or welded and the new wrapped seam is independently proved.
//
// Circle centers and cone normals follow the existing end-specific formulas.
// The retained right-half-circle arc is monotone in height, with decreasing
// radial slope matching the original cone and cap; positive major>minor radius
// and strict axial runout prove an embedded convex meridian. The whole circular
// contact identity and zero native edge-agreement bounds are required. Exact
// rational normal length, represented chart endpoints/native rims and canonical
// U quadrants remain mandatory. Old acute factories keep their original gates.
func FilletWrappedFrustumObtuseRim(ctx context.Context, s Solid, radius float64, upper bool, o geom.SurfacePatchOptions) (Solid, WrappedFrustumRimFilletHistory, error) {
	return filletWrappedFrustumEndRim(ctx, s, radius, upper, true, o)
}

// Only called on the fresh canonical torus authored by the fillet engine.
// The four original degree-two Bezier spans are copied exactly in cyclic order.
// At oldV=4/0 the endpoint poles/weights coincide; at the new seam the oldV=2
// one-sided derivatives coincide. NewWrappedSurfacePatch rechecks this identity.
func halfTurnFilletMeridian(ctx context.Context, w *geom.WrappedSurfacePatch, o geom.SurfacePatchOptions) (*geom.WrappedSurfacePatch, error) {
	recipe, e := w.ExportRecipe()
	if e != nil {
		return nil, e
	}
	d := recipe.Surface.Data()
	if d.DegreeU != 2 || d.DegreeV != 2 || d.CountU != 9 || d.CountV != 9 || len(d.Weights) != 81 {
		return nil, geom.ErrConvergence
	}
	poles := make([]geom.Vec3, 81)
	weights := make([]float64, 81)
	index := [9]int{4, 5, 6, 7, 8, 1, 2, 3, 4}
	for u := 0; u < 9; u++ {
		for v, j := range index {
			poles[u*9+v] = d.Poles[u*9+j]
			weights[u*9+v] = d.Weights[u*9+j]
		}
	}
	s, e := geom.NewBSplineSurfaceWithExactWeights(2, 2, 9, 9, poles, d.KnotsU, d.KnotsV, weights)
	if e != nil {
		return nil, e
	}
	p, e := geom.NewSurfacePatch(s, o)
	if e != nil {
		return nil, e
	}
	out, _, e := geom.NewWrappedSurfacePatch(ctx, p, geom.WrappedSurfaceOptions{WrapU: true, WrapV: true, ContinuityOrder: 1, MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	return out, e
}
