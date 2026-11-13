// Package plane converts the exact standalone STEP/IGES plane support
// subset. It does not construct a bounded face or a plane face.
package plane

import (
	"bytes"
	"errors"
	"fmt"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"strings"
)

var ErrUnsupportedConversion = errors.New("plane: unsupported lossless conversion")

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
func FromIGES(m iges.PlanarSupportModel) (step.PlaneSupportModel, error) {
	var b bytes.Buffer
	if e := iges.WritePlanarSupports(&b, m); e != nil {
		return step.PlaneSupportModel{}, unsupported("iges.geometry", e.Error())
	}
	out := step.PlaneSupportModel{LengthUnit: m.LengthUnit}
	for _, s := range m.Surfaces {
		for i, meta := range []iges.PlaneDatumMetadata{s.Metadata, s.LocationMetadata, s.NormalMetadata, s.ReferenceMetadata} {
			field := []string{"surface", "location", "normal", "reference"}[i]
			if meta.Subscript != 0 {
				return step.PlaneSupportModel{}, unsupported(field+".subscript", "not represented in STEP")
			}
			if e := label(meta.Label, field+".name"); e != nil {
				return step.PlaneSupportModel{}, e
			}
		}
		out.Surfaces = append(out.Surfaces, step.PlaneSupport{Name: s.Metadata.Label, LocationName: s.LocationMetadata.Label, NormalName: s.NormalMetadata.Label, ReferenceName: s.ReferenceMetadata.Label, Location: s.Location, Normal: s.Normal, Reference: s.Reference})
	}
	b.Reset()
	if e := step.WritePlaneSupports(&b, out); e != nil {
		return step.PlaneSupportModel{}, unsupported("step.representation", e.Error())
	}
	return out, nil
}

// ToIGES requires empty STEP representation/set/placement names and labels
// representable by IGES. Geometry and original units are never rescaled.
func ToIGES(m step.PlaneSupportModel) (iges.PlanarSupportModel, error) {
	if m.Name != "" {
		return iges.PlanarSupportModel{}, unsupported("representation.name", "not represented in IGES")
	}
	if m.SetName != "" {
		return iges.PlanarSupportModel{}, unsupported("set.name", "not represented in IGES")
	}
	if len(m.Surfaces) < 1 || len(m.Surfaces) > 64 {
		return iges.PlanarSupportModel{}, unsupported("surface.count", "requires 1–64")
	}
	for _, s := range m.Surfaces {
		if s.PlacementName != "" {
			return iges.PlanarSupportModel{}, unsupported("placement.name", "not represented in IGES")
		}
		for i, name := range []string{s.Name, s.LocationName, s.NormalName, s.ReferenceName} {
			if e := label(name, []string{"surface.name", "location.name", "normal.name", "reference.name"}[i]); e != nil {
				return iges.PlanarSupportModel{}, e
			}
		}
	}
	var b bytes.Buffer
	if e := step.WritePlaneSupports(&b, m); e != nil {
		return iges.PlanarSupportModel{}, unsupported("step.geometry", e.Error())
	}
	out := iges.PlanarSupportModel{LengthUnit: m.LengthUnit}
	for _, s := range m.Surfaces {
		out.Surfaces = append(out.Surfaces, iges.PlanarSupport{Location: s.Location, Normal: s.Normal, Reference: s.Reference, Metadata: iges.PlaneDatumMetadata{Label: s.Name}, LocationMetadata: iges.PlaneDatumMetadata{Label: s.LocationName}, NormalMetadata: iges.PlaneDatumMetadata{Label: s.NormalName}, ReferenceMetadata: iges.PlaneDatumMetadata{Label: s.ReferenceName}})
	}
	b.Reset()
	if e := iges.WritePlanarSupports(&b, out); e != nil {
		return iges.PlanarSupportModel{}, unsupported("iges.representation", e.Error())
	}
	return out, nil
}
