package trimmed

import (
	"context"
	"fmt"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// HelicalSurfaceArea integrates the retained straight-chart faces of a
// cylindrical helical body. It uses Green's theorem on each oriented trim.
// Helical flank density depends only on the meridian coordinate; all other
// accepted chart densities are constant or linear in radius. Volume, moments,
// and a tessellation are unnecessary.
func (s Solid) HelicalSurfaceArea(ctx context.Context) (float64, error) {
	if ctx == nil || len(s.faces) == 0 {
		return 0, geom.ErrInvalid
	}
	area := 0.
	for id, f := range s.faces {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		base, m := parametric.SurfaceMapping(f.Surface)
		if m != geom.Identity() {
			return 0, geom.ErrDomain
		}
		var density func(float64) float64
		linear := true
		switch surf := base.(type) {
		case geom.PlaneSurface:
			faceArea, err := s.PlanarFaceArea(id)
			if err != nil {
				return 0, err
			}
			area += faceArea
			continue
		case geom.CylinderSurface:
			_, r := surf.Parameters()
			density = func(float64) float64 { return r }
		case geom.TwistedCylinderSurface:
			_, r, _, _ := surf.Parameters()
			density = func(float64) float64 { return 2 * math.Pi * r }
		case geom.TwistedAnnularSurface:
			density = func(v float64) float64 { return 2 * math.Pi * v }
		case geom.HelicalProfileSurface:
			linear = false
			_, a, b, lead, _ := surf.Parameters()
			dr, dz := b.X-a.X, b.Y-a.Y
			density = func(v float64) float64 {
				r := a.X + v*dr
				return math.Hypot(2*math.Pi*r*math.Hypot(dr, dz), lead*dr)
			}
		default:
			if _, ok := parametric.PolarPlaneFrame(base); !ok {
				return 0, fmt.Errorf("helical area face %d unsupported surface %T: %w", id, base, geom.ErrDomain)
			}
			density = func(v float64) float64 { return v }
		}
		anchor := f.Outer[0].UVStart.X
		faceArea := 0.
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, use := range loop {
				if use.Curve != nil {
					return 0, fmt.Errorf("helical area face %d unsupported trim %T: %w", id, use.Curve, geom.ErrDomain)
				}
				a, b := use.UVStart, use.UVEnd
				dv := b.Y - a.Y
				if dv == 0 {
					continue
				}
				integrand := func(t float64) float64 {
					return ((1-t)*(a.X-anchor) + t*(b.X-anchor)) * density((1-t)*a.Y+t*b.Y)
				}
				if linear {
					faceArea += dv * (integrand(0) + 4*integrand(.5) + integrand(1)) / 6
				} else {
					value, ok := helicalAreaIntegral(integrand, 0, 1, 0)
					if !ok {
						return 0, geom.ErrConvergence
					}
					faceArea += dv * value
				}
			}
		}
		if faceArea <= 0 || !finite(faceArea) {
			return 0, geom.ErrInvalid
		}
		area += faceArea
	}
	return area, nil
}

func helicalAreaIntegral(f func(float64) float64, a, b float64, depth int) (float64, bool) {
	mid := (a + b) / 2
	x, y, z := f(a), f(mid), f(b)
	coarse := (x + 4*y + z) / 6
	fine := (x + 4*f((a+mid)/2) + 2*y + 4*f((mid+b)/2) + z) / 12
	if math.Abs(coarse-fine) <= 1e-11*max(1, math.Abs(fine)) {
		return (fine + (fine-coarse)/15) * (b - a), true
	}
	if depth >= 12 {
		return 0, false
	}
	left, l := helicalAreaIntegral(f, a, mid, depth+1)
	right, r := helicalAreaIntegral(f, mid, b, depth+1)
	return left + right, l && r
}
