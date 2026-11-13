// Package sphere converts the exact standalone STEP/IGES sphere support
// subset. It does not construct a bounded face or a sphere solid.
package sphere

import (
	"bytes"
	"errors"
	"fmt"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"strings"
)

var ErrUnsupportedConversion = errors.New("sphere: unsupported lossless conversion")

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
func FromIGES(m iges.SphericalSupportModel) (step.SphereSupportModel, error) {
	var b bytes.Buffer
	if e := iges.WriteSphericalSupports(&b, m); e != nil {
		return step.SphereSupportModel{}, unsupported("iges.geometry", e.Error())
	}
	out := step.SphereSupportModel{LengthUnit: m.LengthUnit}
	for _, s := range m.Surfaces {
		for i, meta := range []iges.SphereDatumMetadata{s.Metadata, s.CenterMetadata, s.AxisMetadata, s.ReferenceMetadata} {
			field := []string{"surface", "location", "axis", "reference"}[i]
			if meta.Subscript != 0 {
				return step.SphereSupportModel{}, unsupported(field+".subscript", "not represented in STEP")
			}
			if e := label(meta.Label, field+".name"); e != nil {
				return step.SphereSupportModel{}, e
			}
		}
		out.Surfaces = append(out.Surfaces, step.SphereSupport{Name: s.Metadata.Label, LocationName: s.CenterMetadata.Label, AxisName: s.AxisMetadata.Label, ReferenceName: s.ReferenceMetadata.Label, Location: s.Center, Axis: s.Axis, Reference: s.Reference, Radius: s.Radius})
	}
	b.Reset()
	if e := step.WriteSphereSupports(&b, out); e != nil {
		return step.SphereSupportModel{}, unsupported("step.representation", e.Error())
	}
	return out, nil
}

// ToIGES requires empty STEP representation/set/placement names and labels
// representable by IGES. Geometry and original units are never rescaled.
func ToIGES(m step.SphereSupportModel) (iges.SphericalSupportModel, error) {
	if m.Name != "" {
		return iges.SphericalSupportModel{}, unsupported("representation.name", "not represented in IGES")
	}
	if m.SetName != "" {
		return iges.SphericalSupportModel{}, unsupported("set.name", "not represented in IGES")
	}
	if len(m.Surfaces) < 1 || len(m.Surfaces) > 64 {
		return iges.SphericalSupportModel{}, unsupported("surface.count", "requires 1–64")
	}
	for _, s := range m.Surfaces {
		if s.PlacementName != "" {
			return iges.SphericalSupportModel{}, unsupported("placement.name", "not represented in IGES")
		}
		for i, name := range []string{s.Name, s.LocationName, s.AxisName, s.ReferenceName} {
			if e := label(name, []string{"surface.name", "location.name", "axis.name", "reference.name"}[i]); e != nil {
				return iges.SphericalSupportModel{}, e
			}
		}
	}
	var b bytes.Buffer
	if e := step.WriteSphereSupports(&b, m); e != nil {
		return iges.SphericalSupportModel{}, unsupported("step.geometry", e.Error())
	}
	out := iges.SphericalSupportModel{LengthUnit: m.LengthUnit}
	for _, s := range m.Surfaces {
		out.Surfaces = append(out.Surfaces, iges.SphericalSupport{Center: s.Location, Axis: s.Axis, Reference: s.Reference, Radius: s.Radius, Metadata: iges.SphereDatumMetadata{Label: s.Name}, CenterMetadata: iges.SphereDatumMetadata{Label: s.LocationName}, AxisMetadata: iges.SphereDatumMetadata{Label: s.AxisName}, ReferenceMetadata: iges.SphereDatumMetadata{Label: s.ReferenceName}})
	}
	b.Reset()
	if e := iges.WriteSphericalSupports(&b, out); e != nil {
		return iges.SphericalSupportModel{}, unsupported("iges.representation", e.Error())
	}
	return out, nil
}
