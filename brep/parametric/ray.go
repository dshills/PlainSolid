package parametric

import (
	"fmt"
	"math"
	"sort"

	"plainsolid/geom"
)

// RayHit is an isolated ray contact on an analytic rectangular patch.
type RayHit struct {
	Point    geom.Vec3
	Distance float64
	Face     int
	U, V     float64
	Tangent  bool
}

// Ray intersects an outward ray with supported analytical patches. It handles
// plane, sphere, cylinder, cone, torus and polar disk faces. It validates
// the UV ranges and deduplicates shared-edge contacts within tolerance.
// Coincident generators and custom/spline or mapped patches
// return explicit errors. Distance is in world units.
func (s Solid) Ray(origin, direction geom.Vec3, maxDistance, tolerance float64) ([]RayHit, error) {
	if maxDistance <= 0 || math.IsNaN(maxDistance) || tolerance <= 0 || !finite(tolerance) {
		return nil, geom.ErrInvalid
	}
	l, e := geom.NewLine(origin, direction)
	if e != nil {
		return nil, e
	}
	var out []RayHit
	for i, f := range s.faces {
		points, frame, e := raySupport(f.Surface, l, maxDistance, tolerance)
		if e != nil {
			return nil, fmt.Errorf("parametric: ray face %d: %w", i, e)
		}
		for _, hit := range points {
			x := frame.ToLocal(hit.Point)
			u, v := math.Atan2(x.Y, x.X), x.Z
			periodic := true
			switch a := f.Surface.(type) {
			case geom.PlaneSurface:
				u, v = x.X, x.Y
				periodic = false
			case geom.SphereSurface:
				v = math.Atan2(x.Z, math.Hypot(x.X, x.Y))
			case geom.TorusSurface:
				_, r, _ := a.Parameters()
				v = math.Atan2(x.Z, math.Hypot(x.X, x.Y)-r)
			case geom.OuterTorusSurface:
				_, r, _ := a.Parameters()
				v = math.Atan2(x.Z, math.Hypot(x.X, x.Y)-r)
			case disk:
				v = math.Hypot(x.X, x.Y)
			}
			uTolerance, vTolerance := 1e-10, tolerance
			vPeriodic := false
			switch f.Surface.(type) {
			case geom.PlaneSurface:
				uTolerance = tolerance
			case geom.SphereSurface:
				vTolerance = 1e-10
			case geom.TorusSurface:
				vTolerance = 1e-10
				vPeriodic = true
			case geom.OuterTorusSurface:
				vTolerance = 1e-10
			}
			v, insideV := admissible(v, f.V, vTolerance, vPeriodic)
			if !insideV {
				continue
			}
			u, insideU := admissible(u, f.U, uTolerance, periodic)
			if !insideU {
				// Longitude at a pole or disk center is arbitrary.
				if periodic && math.Hypot(x.X, x.Y) <= tolerance {
					u = f.U.Min
				} else {
					continue
				}
			}
			t := hit.Parameter
			if t < 0 || t > maxDistance {
				continue
			}
			p, e := f.Surface.Eval(u, v)
			if e != nil {
				return nil, e
			}
			if p.Sub(hit.Point).Length() > tolerance {
				continue
			}
			out = append(out, RayHit{hit.Point, t, i, u, v, hit.Tangent})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Distance < out[j].Distance })
	k := 0
	for _, h := range out {
		if k > 0 && h.Face != out[k-1].Face && h.Point.Sub(out[k-1].Point).Length() <= tolerance {
			out[k-1].Tangent = out[k-1].Tangent && h.Tangent
			continue
		}
		out[k] = h
		k++
	}
	return out[:k], nil
}

func admissible(x float64, d Interval, tol float64, periodic bool) (float64, bool) {
	if periodic {
		n := math.Ceil((d.Min - x - tol) / (2 * math.Pi))
		x += n * (2 * math.Pi)
	}
	if !finite(x) || x < d.Min-tol || x > d.Max+tol {
		return x, false
	}
	return math.Max(d.Min, math.Min(d.Max, x)), true
}
func raySupport(s geom.Surface, l geom.Line, maxDistance, tolerance float64) ([]geom.LineSurfaceHit, geom.Frame, error) {
	o := geom.LineSurfaceOptions{UseParameterRange: true, MinParameter: 0, MaxParameter: maxDistance, DistanceTolerance: tolerance}
	switch a := s.(type) {
	case geom.SphereSurface:
		f, r := a.Parameters()
		x, e := geom.NewSphere(f.Origin(), r)
		if e != nil {
			return nil, f, e
		}
		p, e := geom.IntersectLineSphere(l, x, tolerance)
		if e != nil {
			return nil, f, e
		}
		out := make([]geom.LineSurfaceHit, len(p))
		for i, y := range p {
			t := y.Sub(l.Origin()).Dot(l.Direction())
			out[i] = geom.LineSurfaceHit{Point: y, Parameter: t, Tangent: len(p) == 1}
		}
		return out, f, nil
	case geom.CylinderSurface:
		f, _ := a.Parameters()
		r, e := geom.IntersectLineCylinder(l, a, o)
		if e == nil && len(r.Coincident) > 0 {
			e = fmt.Errorf("coincident cylinder generator")
		}
		return r.Hits, f, e
	case geom.ConeSurface:
		f, _ := a.Parameters()
		r, e := geom.IntersectLineCone(l, a, o)
		if e == nil && len(r.Coincident) > 0 {
			e = fmt.Errorf("coincident cone generator")
		}
		return r.Hits, f, e
	case geom.TorusSurface:
		f, _, _ := a.Parameters()
		r, e := geom.IntersectLineTorus(l, a, geom.LineTorusOptions{UseParameterRange: true, MinParameter: 0, MaxParameter: maxDistance, DistanceTolerance: tolerance})
		var out []geom.LineSurfaceHit
		for _, h := range r.Hits {
			out = append(out, h.LineSurfaceHit)
		}
		return out, f, e
	case geom.OuterTorusSurface:
		f, _, _ := a.Parameters()
		r, e := geom.IntersectLineOuterTorus(l, a, geom.LineTorusOptions{UseParameterRange: true, MinParameter: 0, MaxParameter: maxDistance, DistanceTolerance: tolerance})
		var out []geom.LineSurfaceHit
		for _, h := range r.Hits {
			out = append(out, h.LineSurfaceHit)
		}
		return out, f, e
	case geom.PlaneSurface:
		return rayPlane(a.Frame(), l, tolerance)
	case disk:
		return rayPlane(a.frame, l, tolerance)
	default:
		return nil, geom.Frame{}, fmt.Errorf("unsupported analytic ray surface %T", s)
	}
}
func rayPlane(f geom.Frame, l geom.Line, tolerance float64) ([]geom.LineSurfaceHit, geom.Frame, error) {
	p, e := geom.NewPlane(f.Origin(), f.ZAxis())
	if e != nil {
		return nil, f, e
	}
	r, e := geom.IntersectLinePlane(l, p, geom.IntersectionTolerance{Linear: tolerance, Angular: 1e-14})
	if e != nil {
		return nil, f, e
	}
	switch r.Kind {
	case geom.IntersectionCoincident:
		return nil, f, fmt.Errorf("coincident plane")
	case geom.IntersectionPoint:
		return []geom.LineSurfaceHit{{Point: r.Point, Parameter: r.Parameter}}, f, nil
	default:
		return nil, f, nil
	}
}
