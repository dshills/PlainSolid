//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// SplineAxisSectionResult encloses the one possible intersection of an axis
// directed finite line with a projection-injective spline face.
// ParameterLower and ParameterUpper are in original line units. Parameter
// is present only for a recovered rational root, and algebraic UV remains in
// Inverse. A face or segment exclusion is complete when Complete is true.
type SplineAxisSectionResult struct {
	Found, OnTrim, Complete                   bool
	ParameterLower, ParameterUpper, Parameter *big.Rat
	Inverse                                   geom.TriangularSurfaceInverse
	Projection                                *geom.SurfaceProjectionIntervalInverse
}

// IntersectTriangularSplineFaceAxisLine proves a finite axis-directed line
// contact on a clamped spline face. Projection axes must be the other two
// model coordinates. If an algebraic bracket crosses a line endpoint or an
// unproved trim contact, the result remains unresolved.
func (s Solid) IntersectTriangularSplineFaceAxisLine(ctx context.Context, face, modelAxis int, origin geom.Vec3, speed, minT, maxT float64,
	o geom.TriangularSurfaceInverseOptions, trim trimvalidate.Options) (SplineAxisSectionResult, error) {
	var r SplineAxisSectionResult
	if ctx == nil || face < 0 || face >= len(s.faces) || modelAxis < 0 || modelAxis > 2 || !origin.IsFinite() ||
		!finite(speed) || speed == 0 || !finite(minT) || !finite(maxT) || minT > maxT {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	axes := o.ProjectionAxes
	if axes == ([2]int{}) {
		axes = [2]int{(modelAxis + 1) % 3, (modelAxis + 2) % 3}
		o.ProjectionAxes = axes
	}
	if axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] ||
		axes[0] == modelAxis || axes[1] == modelAxis {
		return r, geom.ErrInvalid
	}
	f := s.faces[face]
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	patch, e := exactFaceSupportPatch(f, limits)
	if e != nil {
		return r, e
	}
	xyz := [3]float64{origin.X, origin.Y, origin.Z}
	q := [2]*big.Rat{ratInset(xyz[axes[0]]), ratInset(xyz[axes[1]])}
	r.Inverse, e = geom.InvertTriangularSurfaceProjectionRational(ctx, patch, q, o)
	if e != nil {
		return r, e
	}
	return finishSplineFaceAxisLine(ctx, f, patch, modelAxis, xyz[modelAxis], speed, minT, maxT, limits, trim, r)
}

// IntersectSplineFaceAxisLine uses the general exact interval inverse on
// a projection-injective spline patch. Interior coupled projections can
// return an enclosed root. Boundary or trim cases can remain unresolved.
func (s Solid) IntersectSplineFaceAxisLine(ctx context.Context, face, modelAxis int, origin geom.Vec3, speed, minT, maxT float64,
	o geom.RationalSurfaceInverseOptions, trim trimvalidate.Options) (SplineAxisSectionResult, error) {
	var r SplineAxisSectionResult
	if ctx == nil || face < 0 || face >= len(s.faces) || modelAxis < 0 || modelAxis > 2 || !origin.IsFinite() ||
		!finite(speed) || speed == 0 || !finite(minT) || !finite(maxT) || minT > maxT {
		return r, geom.ErrInvalid
	}
	axes := o.ProjectionAxes
	if axes == ([2]int{}) {
		axes = [2]int{(modelAxis + 1) % 3, (modelAxis + 2) % 3}
		o.ProjectionAxes = axes
	}
	if axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] ||
		axes[0] == modelAxis || axes[1] == modelAxis {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	f := s.faces[face]
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	patch, e := exactFaceSupportPatch(f, limits)
	if e != nil {
		return r, e
	}
	xyz := [3]float64{origin.X, origin.Y, origin.Z}
	q := [2]*big.Rat{ratInset(xyz[axes[0]]), ratInset(xyz[axes[1]])}
	proof, e := geom.InvertSurfaceProjectionInterval(ctx, patch, q, o)
	r.Projection = &proof
	if e != nil {
		return r, e
	}
	r.Inverse = geom.TriangularSurfaceInverse{Found: proof.Found, Excluded: proof.Excluded, Complete: proof.Complete,
		Lower: proof.Lower, Upper: proof.Upper, ModelBounds: proof.ModelBounds}
	return finishSplineFaceAxisLine(ctx, f, patch, modelAxis, xyz[modelAxis], speed, minT, maxT, limits, trim, r)
}

func finishSplineFaceAxisLine(ctx context.Context, f Face, patch *geom.SurfacePatch, modelAxis int,
	axisOrigin, speed, minT, maxT float64, o geom.SurfacePatchOptions, trim trimvalidate.Options,
	r SplineAxisSectionResult) (SplineAxisSectionResult, error) {
	if r.Inverse.Excluded {
		r.Complete = true
		return r, nil
	}
	if !r.Inverse.Found {
		return r, geom.ErrConvergence
	}
	t := func(p *big.Rat) *big.Rat {
		return new(big.Rat).Quo(new(big.Rat).Sub(p, ratInset(axisOrigin)), ratInset(speed))
	}
	var location SplineFacePointLocation
	if r.Inverse.Parameters[0] != nil && r.Inverse.Parameters[1] != nil {
		p, e := patch.EvaluateRational(ctx, r.Inverse.Parameters[0], r.Inverse.Parameters[1],
			geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return r, e
		}
		r.Parameter = t(p.Point[modelAxis])
		r.ParameterLower, r.ParameterUpper = new(big.Rat).Set(r.Parameter), new(big.Rat).Set(r.Parameter)
		loc, _, e := LocateRationalInFaceTrim(ctx, f, r.Inverse.Parameters, trim)
		if e != nil {
			return r, e
		}
		switch loc {
		case trimvalidate.Outside:
			location = SplineFacePointOutside
		case trimvalidate.Inside:
			location = SplineFacePointInterior
		case trimvalidate.Boundary:
			location = SplineFacePointOnTrim
		}
	} else {
		b := r.Inverse.ModelBounds
		minimum, maximum := [3]float64{b.Min.X, b.Min.Y, b.Min.Z}, [3]float64{b.Max.X, b.Max.Y, b.Max.Z}
		if !finite(minimum[modelAxis]) || !finite(maximum[modelAxis]) {
			return r, geom.ErrConvergence
		}
		r.ParameterLower, r.ParameterUpper = t(ratInset(minimum[modelAxis])), t(ratInset(maximum[modelAxis]))
		if speed < 0 {
			r.ParameterLower, r.ParameterUpper = r.ParameterUpper, r.ParameterLower
		}
		if r.ParameterUpper.Cmp(ratInset(minT)) < 0 || r.ParameterLower.Cmp(ratInset(maxT)) > 0 {
			r.Complete = true
			return r, nil
		}
		loc, e := classifyAlgebraicFacePoint(ctx, f, r.Inverse, SplineFacePointOptions{Trim: trim, MaxTrimSubdivisions: 1024}, SplineFacePointResult{})
		if e != nil {
			return r, e
		}
		location = loc.Location
	}
	if location == SplineFacePointOutside || r.ParameterUpper.Cmp(ratInset(minT)) < 0 || r.ParameterLower.Cmp(ratInset(maxT)) > 0 {
		r.Complete = true
		return r, nil
	}
	if location == SplineFacePointUnknown || r.ParameterLower.Cmp(ratInset(minT)) < 0 || r.ParameterUpper.Cmp(ratInset(maxT)) > 0 {
		return r, geom.ErrConvergence
	}
	r.Found, r.OnTrim, r.Complete = true, location == SplineFacePointOnTrim, true
	return r, nil
}
