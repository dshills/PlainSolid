package geom

import "math"

// SurfaceXYInverseOptions bounds the independent injectivity and line-contact
// proofs. Injectivity.ProjectionAxes must be default or [0,1]. Intersection's
// line range is derived from the exact patch's outward Z bounds and overridden.
type SurfaceXYInverseOptions struct {
	Injectivity  SurfaceInjectivityOptions
	Intersection LineSplineSurfaceOptions
}

// SurfaceXYInverse certifies inversion of the horizontal projection when Found.
// UV and Height enclose the unique point above (x,y). Excluded and Complete
// prove no such point. Pending contact/injectivity returns an error rather than
// assigning an arbitrary projected point. Height is an outward scalar range.
type SurfaceXYInverse struct {
	UV                        UVBox
	Height                    CertifiedScalarRange
	Found, Excluded, Complete bool
	Injectivity               SurfaceInjectivity
	Intersection              LineSplineSurfaceResult
}

// InvertSurfaceXY first proves global injectivity of the horizontal projection,
// then isolates contact with the exact vertical line through (x,y). This shares
// the line query's explicit unresolved boundary/tangent limitations. Extreme Z
// bounds whose outward neighboring float is infinite return ErrConvergence.
func InvertSurfaceXY(p *SurfacePatch, x, y float64, o SurfaceXYInverseOptions) (result SurfaceXYInverse, err error) {
	if p == nil || len(p.net) == 0 || !finite(x) || !finite(y) || (o.Injectivity.ProjectionAxes != ([2]int{}) && o.Injectivity.ProjectionAxes != ([2]int{0, 1})) {
		return result, ErrInvalid
	}
	result.Injectivity, err = CertifySurfaceInjectivity(p, o.Injectivity)
	if err != nil {
		return result, err
	}
	if !result.Injectivity.ProvenInjective {
		return result, ErrConvergence
	}
	if x < p.bounds.Min.X || x > p.bounds.Max.X || y < p.bounds.Min.Y || y > p.bounds.Max.Y {
		result.Excluded, result.Complete = true, true
		return result, nil
	}
	lo, hi := math.Nextafter(p.bounds.Min.Z, math.Inf(-1)), math.Nextafter(p.bounds.Max.Z, math.Inf(1))
	if !finite(lo) || !finite(hi) || !finite(hi-lo) {
		return result, ErrConvergence
	}
	line, e := NewLine(Vec3{x, y, 0}, Vec3{Z: 1})
	if e != nil {
		return result, e
	}
	options := o.Intersection
	options.MinParameter, options.MaxParameter = lo, hi
	result.Intersection, err = IntersectLineSplineSurface(line, p, options)
	if err != nil {
		return result, err
	}
	if len(result.Intersection.Hits) == 0 {
		result.Excluded, result.Complete = true, true
		return result, nil
	}
	if len(result.Intersection.Hits) != 1 {
		return result, ErrConvergence
	}
	hit := result.Intersection.Hits[0]
	result.UV = hit.UV
	sign := 0
	if hit.ParameterMin > 0 {
		sign = 1
	} else if hit.ParameterMax < 0 {
		sign = -1
	}
	result.Height = CertifiedScalarRange{hit.ParameterMin, hit.ParameterMax, sign}
	result.Found, result.Complete = true, true
	return result, nil
}
