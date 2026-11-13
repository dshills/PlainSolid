package parametric

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"

	"plainsolid/geom"
)

// SplineRayOptions supplies a finite line-distance interval and bounds exact
// work. Line arithmetic limits apply per face; MaxFaces bounds total faces.
type SplineRayOptions struct {
	Patch    geom.SurfacePatchOptions
	Line     geom.LineSplineSurfaceOptions
	MaxFaces int
	Crossing geom.SurfaceTransversalityOptions
}

type SplineRayHit struct {
	Face int
	geom.LineSplineSurfaceHit
	// Direction is the proved sign relative to the oriented face normal.
	// Zero means the direction was not certified.
	Direction       int
	OnPatchBoundary bool
}
type SplineRayPending struct {
	Face int
	geom.LineSplineSurfacePending
}

// SplineRayResult includes exact root enclosures and face ancestry. Only
// Complete results can exclude additional contacts. Seams are not merged.
type SplineRayResult struct {
	Hits     []SplineRayHit
	Pending  []SplineRayPending
	Complete bool
}

// SplineRay intersects a line segment with original rectangular spline
// faces. It avoids tessellation and retains unresolved tangent, coincident
// and boundary contacts. Affine maps are applied to the exact patch
// coefficients before the line test. Other surface families are outside
// this method's domain.
func (s Solid) SplineRay(ctx context.Context, origin, direction geom.Vec3, o SplineRayOptions) (SplineRayResult, error) {
	return IntersectSplineFaces(ctx, s.faces, origin, direction, o)
}

// IntersectSplineFaces applies SplineRay to independent rectangular faces.
func IntersectSplineFaces(ctx context.Context, faces []Face, origin, direction geom.Vec3, o SplineRayOptions) (SplineRayResult, error) {
	var r SplineRayResult
	if o.MaxFaces == 0 {
		o.MaxFaces = 128
	}
	if ctx == nil || o.MaxFaces < 1 || len(faces) > o.MaxFaces || !finite(o.Line.MinParameter) || !finite(o.Line.MaxParameter) ||
		o.Line.MaxParameter <= o.Line.MinParameter {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	l, e := geom.NewLine(origin, direction)
	if e != nil {
		return r, e
	}
	for i, f := range faces {
		base, _ := SurfaceMapping(f.Surface)
		if _, ok := base.(*geom.BSplineSurface); !ok {
			return r, fmt.Errorf("parametric: spline ray face %d has unsupported surface %T", i, f.Surface)
		}
		if !f.U.valid() || !f.V.valid() {
			return r, geom.ErrDomain
		}
	}
	r.Complete = true
	for i, f := range faces {
		if e := ctx.Err(); e != nil {
			r.Complete = false
			return r, e
		}
		base, m := SurfaceMapping(f.Surface)
		a, e := geom.NewSurfacePatch(base.(*geom.BSplineSurface), o.Patch)
		if e != nil {
			r.Complete = false
			return r, e
		}
		a, e = a.Trim(geom.ParameterRectangle{UMin: f.U.Min, UMax: f.U.Max, VMin: f.V.Min, VMax: f.V.Max})
		if e != nil {
			r.Complete = false
			return r, e
		}
		a, e = a.Transformed(m)
		if e != nil {
			r.Complete = false
			return r, e
		}
		x, e := geom.IntersectLineSplineSurfaceContext(ctx, l, a, o.Line)
		if e != nil && !errors.Is(e, geom.ErrConvergence) {
			r.Complete = false
			return r, e
		}
		r.Complete = r.Complete && x.Complete
		for _, h := range x.Hits {
			c, e := h.Crossing(l, a, o.Crossing)
			if e != nil && !errors.Is(e, geom.ErrDomain) && !errors.Is(e, geom.ErrConvergence) {
				r.Complete = false
				return r, e
			}
			sign := c.Sign
			if f.Reversed {
				sign = -sign
			}
			on := h.UV.Min.X <= f.U.Min || h.UV.Max.X >= f.U.Max || h.UV.Min.Y <= f.V.Min || h.UV.Max.Y >= f.V.Max
			r.Hits = append(r.Hits, SplineRayHit{Face: i, LineSplineSurfaceHit: h, Direction: sign, OnPatchBoundary: on})
		}
		for _, h := range x.Pending {
			r.Pending = append(r.Pending, SplineRayPending{i, h})
		}
	}
	sort.SliceStable(r.Hits, func(i, j int) bool { return r.Hits[i].ParameterMin < r.Hits[j].ParameterMin })
	if !r.Complete {
		return r, geom.ErrConvergence
	}
	return r, nil
}

// ClassifySpline determines winding for a closed, outward, embedded spline
// boundary using exact transverse crossings. A face seam or unproved crossing
// causes a different ray direction to be tried. It does not infer Boundary
// from an approximate origin hit; points within originTolerance of a contact
// return an unresolved error unless exactly separated by the hit enclosure.
func (s Solid) ClassifySpline(ctx context.Context, p geom.Vec3, originTolerance float64, o SplineRayOptions) (Location, error) {
	if ctx == nil || !p.IsFinite() || !finite(originTolerance) || originTolerance <= 0 {
		return Outside, geom.ErrInvalid
	}
	if len(s.faces) == 0 {
		return Outside, nil
	}
	box, e := s.Bounds()
	if e != nil {
		return Outside, e
	}
	far := 0.
	for _, q := range []geom.Vec3{box.Min, box.Max} {
		far = max(far, p.Distance(q))
	}
	far = 2*far + originTolerance
	if !finite(far) || far <= originTolerance {
		return Outside, geom.ErrInvalid
	}
	o.Line.MinParameter, o.Line.MaxParameter = -originTolerance, math.Nextafter(far, math.Inf(1))
	dirs := []geom.Vec3{{X: 1, Y: 2, Z: 3}, {X: -2, Y: 5, Z: 1}, {X: 7, Y: -3, Z: 2}, {X: 2, Y: 1, Z: -5}}
	for _, d := range dirs {
		r, e := s.SplineRay(ctx, p, d, o)
		if e != nil && !errors.Is(e, geom.ErrConvergence) {
			return Outside, e
		}
		if !r.Complete {
			continue
		}
		w, ok := 0, true
		for _, h := range r.Hits {
			if h.ParameterMin <= originTolerance && h.ParameterMax >= -originTolerance || h.OnPatchBoundary || h.Direction == 0 {
				ok = false
				break
			}
			if h.ParameterMin > 0 {
				w += h.Direction
			}
		}
		if ok {
			switch w {
			case 0:
				return Outside, nil
			case 1:
				return Inside, nil
			default:
				return Outside, fmt.Errorf("parametric: inconsistent spline solid winding %d", w)
			}
		}
	}
	return Outside, fmt.Errorf("%w: no certified spline classification ray", geom.ErrConvergence)
}
