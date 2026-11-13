// Package cylinder converts the exact standalone STEP/IGES cylinder support
// subset. It does not construct a bounded face or a cylinder solid.
package cylinder

import (
	"bytes"
	"errors"
	"fmt"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"strings"
)

var ErrUnsupportedConversion = errors.New("cylinder: unsupported lossless conversion")

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
func FromIGES(m iges.CylindricalSurfaceModel) (step.CylinderSupportModel, error) {
	var b bytes.Buffer
	if e := iges.WriteCylindricalSurfaces(&b, m); e != nil {
		return step.CylinderSupportModel{}, unsupported("iges.geometry", e.Error())
	}
	out := step.CylinderSupportModel{LengthUnit: m.LengthUnit}
	for _, s := range m.Surfaces {
		for i, meta := range []iges.CylinderDatumMetadata{s.Metadata, s.LocationMetadata, s.AxisMetadata, s.ReferenceMetadata} {
			field := []string{"surface", "location", "axis", "reference"}[i]
			if meta.Subscript != 0 {
				return step.CylinderSupportModel{}, unsupported(field+".subscript", "not represented in STEP")
			}
			if e := label(meta.Label, field+".name"); e != nil {
				return step.CylinderSupportModel{}, e
			}
		}
		out.Surfaces = append(out.Surfaces, step.CylinderSupport{Name: s.Metadata.Label, LocationName: s.LocationMetadata.Label, AxisName: s.AxisMetadata.Label, ReferenceName: s.ReferenceMetadata.Label, Location: s.Location, Axis: s.Axis, Reference: s.Reference, Radius: s.Radius})
	}
	b.Reset()
	if e := step.WriteCylinderSupports(&b, out); e != nil {
		return step.CylinderSupportModel{}, unsupported("step.representation", e.Error())
	}
	return out, nil
}

// ToIGES requires empty STEP representation/set/placement names and labels
// representable by IGES. Geometry and original units are never rescaled.
func ToIGES(m step.CylinderSupportModel) (iges.CylindricalSurfaceModel, error) {
	if m.Name != "" {
		return iges.CylindricalSurfaceModel{}, unsupported("representation.name", "not represented in IGES")
	}
	if m.SetName != "" {
		return iges.CylindricalSurfaceModel{}, unsupported("set.name", "not represented in IGES")
	}
	if len(m.Surfaces) < 1 || len(m.Surfaces) > 64 {
		return iges.CylindricalSurfaceModel{}, unsupported("surface.count", "requires 1–64")
	}
	for _, s := range m.Surfaces {
		if s.PlacementName != "" {
			return iges.CylindricalSurfaceModel{}, unsupported("placement.name", "not represented in IGES")
		}
		for i, name := range []string{s.Name, s.LocationName, s.AxisName, s.ReferenceName} {
			if e := label(name, []string{"surface.name", "location.name", "axis.name", "reference.name"}[i]); e != nil {
				return iges.CylindricalSurfaceModel{}, e
			}
		}
	}
	var b bytes.Buffer
	if e := step.WriteCylinderSupports(&b, m); e != nil {
		return iges.CylindricalSurfaceModel{}, unsupported("step.geometry", e.Error())
	}
	out := iges.CylindricalSurfaceModel{LengthUnit: m.LengthUnit}
	for _, s := range m.Surfaces {
		out.Surfaces = append(out.Surfaces, iges.CylindricalSurface{Location: s.Location, Axis: s.Axis, Reference: s.Reference, Radius: s.Radius, Metadata: iges.CylinderDatumMetadata{Label: s.Name}, LocationMetadata: iges.CylinderDatumMetadata{Label: s.LocationName}, AxisMetadata: iges.CylinderDatumMetadata{Label: s.AxisName}, ReferenceMetadata: iges.CylinderDatumMetadata{Label: s.ReferenceName}})
	}
	b.Reset()
	if e := iges.WriteCylindricalSurfaces(&b, out); e != nil {
		return iges.CylindricalSurfaceModel{}, unsupported("iges.representation", e.Error())
	}
	return out, nil
}
