package analytic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"strings"
)

// Descriptor is the format-neutral shared analytic metadata. Radius is unused
// for planes; MinorRadius is used only by tori (Radius is their major radius).
type Descriptor struct {
	Kind                                      string
	Origin, Axis, Reference                   geom.Vec3
	Radius, MinorRadius                       float64
	Name, OriginName, AxisName, ReferenceName string
}

// Model contains independent ordered support surfaces, never topology.
type Model struct {
	LengthUnit float64
	Supports   []Descriptor
}
type nativeEnvelope struct {
	Format  string
	Version int
	Model   Model
}

const MaxNativeBytes = 4 << 20

// FromSTEP admits only metadata that survives the strict IGES/STEP bridge.
func FromSTEP(m step.AnalyticSupportModel) (Model, error) {
	if _, e := ToIGES(m); e != nil {
		return Model{}, e
	}
	out := Model{LengthUnit: m.LengthUnit}
	for _, v := range m.Surfaces {
		var d Descriptor
		switch {
		case v.Plane != nil:
			x := v.Plane
			d = Descriptor{Kind: "plane", Origin: x.Location, Axis: x.Normal, Reference: x.Reference, Name: x.Name, OriginName: x.LocationName, AxisName: x.NormalName, ReferenceName: x.ReferenceName}
		case v.Cylinder != nil:
			x := v.Cylinder
			d = Descriptor{Kind: "cylinder", Origin: x.Location, Axis: x.Axis, Reference: x.Reference, Name: x.Name, OriginName: x.LocationName, AxisName: x.AxisName, ReferenceName: x.ReferenceName}
			d.Radius = x.Radius
		case v.Sphere != nil:
			x := v.Sphere
			d = Descriptor{Kind: "sphere", Origin: x.Location, Axis: x.Axis, Reference: x.Reference, Name: x.Name, OriginName: x.LocationName, AxisName: x.AxisName, ReferenceName: x.ReferenceName}
			d.Radius = x.Radius
		case v.Torus != nil:
			x := v.Torus
			d = Descriptor{Kind: "torus", Origin: x.Location, Axis: x.Axis, Reference: x.Reference, Name: x.Name, OriginName: x.LocationName, AxisName: x.AxisName, ReferenceName: x.ReferenceName}
			d.Radius = x.MajorRadius
			d.MinorRadius = x.MinorRadius
		}
		out.Supports = append(out.Supports, d)
	}
	return out, nil
}

// STEP validates the complete neutral model and returns owned typed descriptors.
func (m Model) STEP() (step.AnalyticSupportModel, error) {
	if len(m.Supports) < 1 || len(m.Supports) > 64 {
		return step.AnalyticSupportModel{}, fmt.Errorf("analytic: requires 1–64 supports")
	}
	out := step.AnalyticSupportModel{LengthUnit: m.LengthUnit}
	for _, d := range m.Supports {
		var v step.AnalyticSupport
		if (d.Kind == "plane" && d.Radius != 0) || (d.Kind != "torus" && d.MinorRadius != 0) {
			return step.AnalyticSupportModel{}, fmt.Errorf("analytic: unused radius")
		}
		switch d.Kind {
		case "plane":
			v.Plane = &step.PlaneSupport{Location: d.Origin, Normal: d.Axis, Reference: d.Reference, Name: d.Name, LocationName: d.OriginName, NormalName: d.AxisName, ReferenceName: d.ReferenceName}
		case "cylinder":
			v.Cylinder = &step.CylinderSupport{Location: d.Origin, Axis: d.Axis, Reference: d.Reference, Name: d.Name, LocationName: d.OriginName, AxisName: d.AxisName, ReferenceName: d.ReferenceName, Radius: d.Radius}
		case "sphere":
			v.Sphere = &step.SphereSupport{Location: d.Origin, Axis: d.Axis, Reference: d.Reference, Name: d.Name, LocationName: d.OriginName, AxisName: d.AxisName, ReferenceName: d.ReferenceName, Radius: d.Radius}
		case "torus":
			v.Torus = &step.TorusSupport{Location: d.Origin, Axis: d.Axis, Reference: d.Reference, Name: d.Name, LocationName: d.OriginName, AxisName: d.AxisName, ReferenceName: d.ReferenceName, MajorRadius: d.Radius, MinorRadius: d.MinorRadius}
		default:
			return step.AnalyticSupportModel{}, fmt.Errorf("analytic: unknown support kind")
		}
		out.Surfaces = append(out.Surfaces, v)
	}
	if _, e := ToIGES(out); e != nil {
		return step.AnalyticSupportModel{}, e
	}
	return out, nil
}

// WriteNative validates before writing the version-one neutral envelope.
func WriteNative(w io.Writer, m Model) error {
	if _, e := m.STEP(); e != nil {
		return e
	}
	b, e := json.Marshal(nativeEnvelope{Format: "plainsolid-analytic-supports", Version: 1, Model: m})
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if len(b) > MaxNativeBytes {
		return fmt.Errorf("analytic: byte limit")
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadNative rejects unknown/duplicate fields, excessive nesting, trailing data,
// unsupported versions and invalid geometry. Any failure returns an empty model.
func ReadNative(r io.Reader) (Model, error) {
	b, e := io.ReadAll(io.LimitReader(r, MaxNativeBytes+1))
	if e != nil {
		return Model{}, e
	}
	if len(b) > MaxNativeBytes {
		return Model{}, fmt.Errorf("analytic: byte limit")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if e = uniqueJSON(d, 0); e != nil {
		return Model{}, e
	}
	if _, e = d.Token(); e != io.EOF {
		return Model{}, fmt.Errorf("analytic: trailing data")
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var v nativeEnvelope
	if e = d.Decode(&v); e != nil {
		return Model{}, e
	}
	if v.Format != "plainsolid-analytic-supports" || v.Version != 1 {
		return Model{}, fmt.Errorf("analytic: unsupported native envelope")
	}
	if _, e = v.Model.STEP(); e != nil {
		return Model{}, e
	}
	return v.Model, nil
}
func uniqueJSON(d *json.Decoder, depth int) error {
	if depth > 8 {
		return fmt.Errorf("analytic: nesting limit")
	}
	t, e := d.Token()
	if e != nil {
		return e
	}
	if t == nil {
		return fmt.Errorf("analytic: null unsupported")
	}
	q, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	if q != '{' && q != '[' {
		return fmt.Errorf("analytic: invalid delimiter")
	}
	seen := map[string]bool{}
	for d.More() {
		if q == '{' {
			k, e := d.Token()
			if e != nil {
				return e
			}
			s, ok := k.(string)
			s = strings.ToLower(s)
			if !ok || seen[s] {
				return fmt.Errorf("analytic: duplicate field")
			}
			seen[s] = true
		}
		if e := uniqueJSON(d, depth+1); e != nil {
			return e
		}
	}
	_, e = d.Token()
	return e
}
