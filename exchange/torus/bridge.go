// Package torus converts the exact standalone STEP/IGES torus support
// subset. It does not construct a bounded face or a torus solid.
package torus

import (
	"bytes"
	"errors"
	"fmt"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"strings"
)

var ErrUnsupportedConversion = errors.New("torus: unsupported lossless conversion")

// UnsupportedConversionError identifies metadata or representation that cannot
// survive both strict codecs without being discarded or rounded.
type UnsupportedConversionError struct{ Field, Reason string }

func (e *UnsupportedConversionError) Error() string {
	return fmt.Sprintf("%v: %s: %s", ErrUnsupportedConversion, e.Field, e.Reason)
}
func (e *UnsupportedConversionError) Unwrap() error { return ErrUnsupportedConversion }
func unsupported(field, reason string) error        { return &UnsupportedConversionError{field, reason} }
func label(s, field string) error {
	if len(s) > 8 || strings.TrimSpace(s) != s {
		return unsupported(field, "requires at most eight ASCII bytes without boundary spaces")
	}
	for _, c := range []byte(s) {
		if c < 32 || c > 126 {
			return unsupported(field, "requires printable ASCII")
		}
	}
	return nil
}

// FromIGES preserves names, units and the explicit signed-coordinate frame.
// Nonzero directory subscripts are unsupported because STEP has no matching
// field. No representation/set/placement names are invented.
func FromIGES(m iges.ToroidalSupportModel) (step.TorusSupportModel, error) {
	var b bytes.Buffer
	if e := iges.WriteToroidalSupports(&b, m); e != nil {
		return step.TorusSupportModel{}, unsupported("iges.geometry", e.Error())
	}
	out := step.TorusSupportModel{LengthUnit: m.LengthUnit}
	for _, s := range m.Surfaces {
		for i, meta := range []iges.TorusDatumMetadata{s.Metadata, s.CenterMetadata, s.AxisMetadata, s.ReferenceMetadata} {
			field := []string{"surface", "location", "axis", "reference"}[i]
			if meta.Subscript != 0 {
				return step.TorusSupportModel{}, unsupported(field+".subscript", "not represented in STEP")
			}
			if e := label(meta.Label, field+".name"); e != nil {
				return step.TorusSupportModel{}, e
			}
		}
		out.Surfaces = append(out.Surfaces, step.TorusSupport{Name: s.Metadata.Label, LocationName: s.CenterMetadata.Label, AxisName: s.AxisMetadata.Label, ReferenceName: s.ReferenceMetadata.Label, Location: s.Center, Axis: s.Axis, Reference: s.Reference, MajorRadius: s.MajorRadius, MinorRadius: s.MinorRadius})
	}
	b.Reset()
	if e := step.WriteTorusSupports(&b, out); e != nil {
		return step.TorusSupportModel{}, unsupported("step.representation", e.Error())
	}
	return out, nil
}

// ToIGES requires empty STEP representation/set/placement names and labels
// representable by IGES. Geometry and original units are never rescaled.
func ToIGES(m step.TorusSupportModel) (iges.ToroidalSupportModel, error) {
	if m.Name != "" {
		return iges.ToroidalSupportModel{}, unsupported("representation.name", "not represented in IGES")
	}
	if m.SetName != "" {
		return iges.ToroidalSupportModel{}, unsupported("set.name", "not represented in IGES")
	}
	if len(m.Surfaces) < 1 || len(m.Surfaces) > 64 {
		return iges.ToroidalSupportModel{}, unsupported("surface.count", "requires 1–64")
	}
	for _, s := range m.Surfaces {
		if s.PlacementName != "" {
			return iges.ToroidalSupportModel{}, unsupported("placement.name", "not represented in IGES")
		}
		for i, name := range []string{s.Name, s.LocationName, s.AxisName, s.ReferenceName} {
			if e := label(name, []string{"surface.name", "location.name", "axis.name", "reference.name"}[i]); e != nil {
				return iges.ToroidalSupportModel{}, e
			}
		}
	}
	var b bytes.Buffer
	if e := step.WriteTorusSupports(&b, m); e != nil {
		return iges.ToroidalSupportModel{}, unsupported("step.geometry", e.Error())
	}
	out := iges.ToroidalSupportModel{LengthUnit: m.LengthUnit}
	for _, s := range m.Surfaces {
		out.Surfaces = append(out.Surfaces, iges.ToroidalSupport{Center: s.Location, Axis: s.Axis, Reference: s.Reference, MajorRadius: s.MajorRadius, MinorRadius: s.MinorRadius, Metadata: iges.TorusDatumMetadata{Label: s.Name}, CenterMetadata: iges.TorusDatumMetadata{Label: s.LocationName}, AxisMetadata: iges.TorusDatumMetadata{Label: s.AxisName}, ReferenceMetadata: iges.TorusDatumMetadata{Label: s.ReferenceName}})
	}
	b.Reset()
	if e := iges.WriteToroidalSupports(&b, out); e != nil {
		return iges.ToroidalSupportModel{}, unsupported("iges.representation", e.Error())
	}
	return out, nil
}
