// Package analytic converts independently owned mixed analytic supports between
// the strict IGES and STEP subsets. It does not convert assemblies or BReps.
package analytic

import (
	"bytes"
	"errors"
	"fmt"
	"plainsolid/exchange/cylinder"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/plane"
	"plainsolid/exchange/sphere"
	"plainsolid/exchange/step"
	"plainsolid/exchange/torus"
)

var ErrUnsupportedConversion = errors.New("analytic: unsupported lossless conversion")

// ConversionError identifies a zero-based support, or -1 for global metadata.
// Its cause retains the specialized converter's typed diagnostic.
type ConversionError struct {
	Support int
	Cause   error
}

func (e *ConversionError) Error() string {
	return fmt.Sprintf("%v: support %d: %v", ErrUnsupportedConversion, e.Support, e.Cause)
}
func (e *ConversionError) Unwrap() []error {
	if e.Cause == nil {
		return []error{ErrUnsupportedConversion}
	}
	return []error{ErrUnsupportedConversion, e.Cause}
}
func failure(i int, e error) error { return &ConversionError{Support: i, Cause: e} }
func count(a, b, c, d bool) int {
	n := 0
	for _, v := range []bool{a, b, c, d} {
		if v {
			n++
		}
	}
	return n
}

// FromIGES preserves root order and all exactly transferable data. Individual
// leaf converters retain their existing label/subscript/frame/unit restrictions.
// Both complete models are validated; no partial converted result is returned.
func FromIGES(m iges.AnalyticSupportModel) (step.AnalyticSupportModel, error) {
	if len(m.Surfaces) < 1 || len(m.Surfaces) > 64 {
		return step.AnalyticSupportModel{}, failure(-1, errors.New("requires 1–64 supports"))
	}
	out := step.AnalyticSupportModel{LengthUnit: m.LengthUnit}
	for i, s := range m.Surfaces {
		if count(s.Plane != nil, s.Cylinder != nil, s.Sphere != nil, s.Torus != nil) != 1 {
			return step.AnalyticSupportModel{}, failure(i, errors.New("exactly one variant required"))
		}
		v := step.AnalyticSupport{}
		switch {
		case s.Plane != nil:
			x, e := plane.FromIGES(iges.PlanarSupportModel{LengthUnit: m.LengthUnit, Surfaces: []iges.PlanarSupport{*s.Plane}})
			if e != nil {
				return step.AnalyticSupportModel{}, failure(i, e)
			}
			v.Plane = &x.Surfaces[0]
		case s.Cylinder != nil:
			x, e := cylinder.FromIGES(iges.CylindricalSurfaceModel{LengthUnit: m.LengthUnit, Surfaces: []iges.CylindricalSurface{*s.Cylinder}})
			if e != nil {
				return step.AnalyticSupportModel{}, failure(i, e)
			}
			v.Cylinder = &x.Surfaces[0]
		case s.Sphere != nil:
			x, e := sphere.FromIGES(iges.SphericalSupportModel{LengthUnit: m.LengthUnit, Surfaces: []iges.SphericalSupport{*s.Sphere}})
			if e != nil {
				return step.AnalyticSupportModel{}, failure(i, e)
			}
			v.Sphere = &x.Surfaces[0]
		case s.Torus != nil:
			x, e := torus.FromIGES(iges.ToroidalSupportModel{LengthUnit: m.LengthUnit, Surfaces: []iges.ToroidalSupport{*s.Torus}})
			if e != nil {
				return step.AnalyticSupportModel{}, failure(i, e)
			}
			v.Torus = &x.Surfaces[0]
		}
		out.Surfaces = append(out.Surfaces, v)
	}
	var b bytes.Buffer
	if e := iges.WriteAnalyticSupports(&b, m); e != nil {
		return step.AnalyticSupportModel{}, failure(-1, e)
	}
	b.Reset()
	if e := step.WriteAnalyticSupports(&b, out); e != nil {
		return step.AnalyticSupportModel{}, failure(-1, e)
	}
	return out, nil
}

// ToIGES additionally rejects nonempty STEP representation/set names. It keeps
// all leaf metadata restrictions and does not infer shared instances or trims.
func ToIGES(m step.AnalyticSupportModel) (iges.AnalyticSupportModel, error) {
	if m.Name != "" || m.SetName != "" {
		return iges.AnalyticSupportModel{}, failure(-1, errors.New("representation/set names have no IGES counterpart"))
	}
	if len(m.Surfaces) < 1 || len(m.Surfaces) > 64 {
		return iges.AnalyticSupportModel{}, failure(-1, errors.New("requires 1–64 supports"))
	}
	out := iges.AnalyticSupportModel{LengthUnit: m.LengthUnit}
	for i, s := range m.Surfaces {
		if count(s.Plane != nil, s.Cylinder != nil, s.Sphere != nil, s.Torus != nil) != 1 {
			return iges.AnalyticSupportModel{}, failure(i, errors.New("exactly one variant required"))
		}
		v := iges.AnalyticSupport{}
		switch {
		case s.Plane != nil:
			x, e := plane.ToIGES(step.PlaneSupportModel{LengthUnit: m.LengthUnit, Surfaces: []step.PlaneSupport{*s.Plane}})
			if e != nil {
				return iges.AnalyticSupportModel{}, failure(i, e)
			}
			v.Plane = &x.Surfaces[0]
		case s.Cylinder != nil:
			x, e := cylinder.ToIGES(step.CylinderSupportModel{LengthUnit: m.LengthUnit, Surfaces: []step.CylinderSupport{*s.Cylinder}})
			if e != nil {
				return iges.AnalyticSupportModel{}, failure(i, e)
			}
			v.Cylinder = &x.Surfaces[0]
		case s.Sphere != nil:
			x, e := sphere.ToIGES(step.SphereSupportModel{LengthUnit: m.LengthUnit, Surfaces: []step.SphereSupport{*s.Sphere}})
			if e != nil {
				return iges.AnalyticSupportModel{}, failure(i, e)
			}
			v.Sphere = &x.Surfaces[0]
		case s.Torus != nil:
			x, e := torus.ToIGES(step.TorusSupportModel{LengthUnit: m.LengthUnit, Surfaces: []step.TorusSupport{*s.Torus}})
			if e != nil {
				return iges.AnalyticSupportModel{}, failure(i, e)
			}
			v.Torus = &x.Surfaces[0]
		}
		out.Surfaces = append(out.Surfaces, v)
	}
	var b bytes.Buffer
	if e := step.WriteAnalyticSupports(&b, m); e != nil {
		return iges.AnalyticSupportModel{}, failure(-1, e)
	}
	b.Reset()
	if e := iges.WriteAnalyticSupports(&b, out); e != nil {
		return iges.AnalyticSupportModel{}, failure(-1, e)
	}
	return out, nil
}
