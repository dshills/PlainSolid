package exchange

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"unicode/utf8"
)

// RevolutionSurfaceJSONLimits bounds standalone analytic support persistence.
// Zero fields default to 4 MiB, 64 surfaces and 4096 cumulative profile poles.
// Limits cannot exceed 64 MiB, 64 surfaces or 4096 poles.
type RevolutionSurfaceJSONLimits struct {
	MaxBytes              int64
	MaxSurfaces, MaxPoles int
}

func (l RevolutionSurfaceJSONLimits) defaults() (RevolutionSurfaceJSONLimits, error) {
	if l.MaxBytes == 0 {
		l.MaxBytes = 4 << 20
	}
	if l.MaxSurfaces == 0 {
		l.MaxSurfaces = 64
	}
	if l.MaxPoles == 0 {
		l.MaxPoles = 4096
	}
	if l.MaxBytes < 1 || l.MaxBytes > 64<<20 || l.MaxSurfaces < 1 || l.MaxSurfaces > 64 || l.MaxPoles < 2 || l.MaxPoles > 4096 {
		return l, geom.ErrDomain
	}
	return l, nil
}

type revolutionProfileJSON struct {
	Name                  string
	Degree                int
	Poles                 []geom.Vec3
	Knots, Weights        []float64
	Closed, SelfIntersect string
}
type revolutionSupportJSON struct {
	Name                 string
	Profile              revolutionProfileJSON
	Origin, Axis         geom.Vec3
	AngleStart, AngleEnd float64
}
type revolutionSurfacesJSON struct {
	Version           int
	Kind, AngularUnit string
	LengthUnit        float64
	Surfaces          []revolutionSupportJSON
}

func revolutionSurfacePayload(m step.RevolutionSurfaceModel, l RevolutionSurfaceJSONLimits) (revolutionSurfacesJSON, error) {
	d := revolutionSurfacesJSON{Version: 1, Kind: "standalone-analytic-revolution-surfaces", AngularUnit: "radian", LengthUnit: m.LengthUnit}
	if m.LengthUnit <= 0 || math.IsNaN(m.LengthUnit) || math.IsInf(m.LengthUnit, 0) || len(m.Surfaces) < 1 || len(m.Surfaces) > l.MaxSurfaces {
		return d, geom.ErrDomain
	}
	poles := 0
	for _, s := range m.Surfaces {
		if !utf8.ValidString(s.Name) || !utf8.ValidString(s.Profile.Name) || len(s.Name) > 4096 || len(s.Profile.Name) > 4096 {
			return d, geom.ErrDomain
		}
		for _, v := range []string{s.Profile.Closed, s.Profile.SelfIntersect} {
			if v != "" && v != "F" && v != "U" {
				return d, geom.ErrDomain
			}
		}
		c := s.Profile.Geometry
		if c == nil {
			return d, geom.ErrDomain
		}
		poles += len(c.Poles())
		if poles > l.MaxPoles {
			return d, geom.ErrDomain
		}
		for _, w := range c.Weights() {
			if w != 1 {
				return d, geom.ErrDomain
			}
		} // Version1 remains polynomial-only.
		if _, e := s.Surface(); e != nil {
			return d, e
		}
		d.Surfaces = append(d.Surfaces, revolutionSupportJSON{Name: s.Name, Profile: revolutionProfileJSON{Name: s.Profile.Name, Degree: c.Degree(), Poles: c.Poles(), Knots: c.Knots(), Weights: c.Weights(), Closed: s.Profile.Closed, SelfIntersect: s.Profile.SelfIntersect}, Origin: s.Origin, Axis: s.Axis, AngleStart: s.AngleStart, AngleEnd: s.AngleEnd})
	}
	return d, nil
}

// WriteRevolutionSurfacesJSON writes version 1 standalone analytic supports.
// It preserves represented spline coefficients, names, logical metadata, original
// V parameters, radian U bounds and meters-per-model-unit metadata. It creates no
// faces, meshes or document assets. Validation and encoding complete before I/O.
func WriteRevolutionSurfacesJSON(w io.Writer, m step.RevolutionSurfaceModel) error {
	if w == nil {
		return geom.ErrDomain
	}
	l, _ := (RevolutionSurfaceJSONLimits{}).defaults()
	d, e := revolutionSurfacePayload(m, l)
	if e != nil {
		return e
	}
	b, e := json.Marshal(d)
	if e != nil {
		return e
	}
	if int64(len(b)) > l.MaxBytes {
		return fmt.Errorf("exchange: revolution JSON byte limit")
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return e
}

// ReadRevolutionSurfacesJSON accepts only the canonical version 1 native envelope
// (insignificant JSON whitespace is allowed). Duplicate/unknown fields, other
// kinds/versions and noncanonical numeric or profile representations fail.
// Every support is rebuilt through geom.NewRevolutionSurface; any failure returns
// an empty model. This validates geometry, not authenticity of arbitrary edits.
func ReadRevolutionSurfacesJSON(r io.Reader, l RevolutionSurfaceJSONLimits) (step.RevolutionSurfaceModel, error) {
	fail := func() (step.RevolutionSurfaceModel, error) {
		return step.RevolutionSurfaceModel{}, fmt.Errorf("exchange: invalid revolution surface JSON")
	}
	l, e := l.defaults()
	if e != nil || r == nil {
		return fail()
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return step.RevolutionSurfaceModel{}, e
	}
	if int64(len(b)) > l.MaxBytes {
		return fail()
	}
	var d revolutionSurfacesJSON
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&d); e != nil {
		return fail()
	}
	if d.Version != 1 || d.Kind != "standalone-analytic-revolution-surfaces" || d.AngularUnit != "radian" || len(d.Surfaces) < 1 || len(d.Surfaces) > l.MaxSurfaces {
		return fail()
	}
	m := step.RevolutionSurfaceModel{LengthUnit: d.LengthUnit}
	poles := 0
	for _, s := range d.Surfaces {
		p := s.Profile
		poles += len(p.Poles)
		if poles > l.MaxPoles || len(p.Knots) > l.MaxPoles+65 || len(p.Weights) != len(p.Poles) {
			return fail()
		}
		c, e := geom.NewBSpline(p.Degree, p.Poles, p.Knots, p.Weights)
		if e != nil {
			return fail()
		}
		m.Surfaces = append(m.Surfaces, step.RevolutionSurface{Name: s.Name, Profile: step.SplineCurve{Name: p.Name, Geometry: c, Closed: p.Closed, SelfIntersect: p.SelfIntersect}, Origin: s.Origin, Axis: s.Axis, AngleStart: s.AngleStart, AngleEnd: s.AngleEnd})
	}
	expected, e := revolutionSurfacePayload(m, l)
	if e != nil {
		return fail()
	}
	raw, e := json.Marshal(expected)
	if e != nil {
		return fail()
	}
	var compact bytes.Buffer
	if e = json.Compact(&compact, b); e != nil || !bytes.Equal(compact.Bytes(), raw) {
		return fail()
	}
	return m, nil
}
