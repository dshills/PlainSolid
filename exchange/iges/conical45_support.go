package iges

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"plainsolid/geom"
)

// ConeDatumMetadata retains labels and subscripts of each authored dependency.
type ConeDatumMetadata struct {
	Label     string
	Subscript int
}

// Conical45Support retains a type194 form1 cone with exactly 45 degrees semi-angle.
// U is azimuth and V is signed generatrix distance from the reference circle.
// Both nappes are retained; the apex is a parameter singularity. No trim or solid
// is implied. Geometry uses the symbolic factor 1/sqrt(2), not a rounded angle.
type Conical45Support struct {
	Center, Axis, Reference                                   geom.Vec3
	Radius                                                    float64
	Metadata, CenterMetadata, AxisMetadata, ReferenceMetadata ConeDatumMetadata
}
type Conical45SupportModel struct {
	LengthUnit float64
	Surfaces   []Conical45Support
}

func coneCoordinate(v geom.Vec3) bool {
	n := 0
	for _, x := range []float64{v.X, v.Y, v.Z} {
		if x != 0 {
			if x != 1 && x != -1 {
				return false
			}
			n++
		}
	}
	return n == 1
}

// Geometry preserves the explicit source frame; arbitrary direction normalization
// is deliberately unsupported in this bounded codec.
func (s Conical45Support) Geometry() (geom.Surface, error) {
	if !s.Center.IsFinite() || !finite(s.Radius) || s.Radius <= 0 || !coneCoordinate(s.Axis) || !coneCoordinate(s.Reference) || s.Axis.Dot(s.Reference) != 0 {
		return nil, geom.ErrDomain
	}
	f, e := geom.NewFrame(s.Center, s.Reference, s.Axis)
	if e != nil {
		return nil, e
	}
	return cone45Geometry{frame: f, radius: s.Radius}, nil
}

type cone45Geometry struct {
	frame  geom.Frame
	radius float64
}

func (s cone45Geometry) Eval(u, v float64) (geom.Vec3, error) {
	if !finite(u) || !finite(v) {
		return geom.Vec3{}, geom.ErrDomain
	}
	z := v * math.Sqrt(.5)
	r := s.radius + z
	p := s.frame.ToWorld(geom.Vec3{X: r * math.Cos(u), Y: r * math.Sin(u), Z: z})
	if !p.IsFinite() {
		return geom.Vec3{}, geom.ErrInvalid
	}
	return p, nil
}
func (s cone45Geometry) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	if _, e := s.Eval(u, v); e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	q := math.Sqrt(.5)
	r := s.radius + v*q
	du := s.frame.XAxis().Scale(-r * math.Sin(u)).Add(s.frame.YAxis().Scale(r * math.Cos(u)))
	dv := s.frame.XAxis().Scale(q * math.Cos(u)).Add(s.frame.YAxis().Scale(q * math.Sin(u))).Add(s.frame.ZAxis().Scale(q))
	if !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.ErrInvalid
	}
	return du, dv, nil
}
func coneMetadata(m ConeDatumMetadata) error {
	if m.Subscript < 0 || m.Subscript > 99999999 || len(m.Label) > 8 {
		return geom.ErrDomain
	}
	for _, c := range []byte(m.Label) {
		if c < 32 || c > 126 {
			return geom.ErrDomain
		}
	}
	if len(m.Label) > 0 && (m.Label[0] == ' ' || m.Label[len(m.Label)-1] == ' ') {
		return geom.ErrDomain
	}
	return nil
}

// ReadConical45Supports accepts only type194 form1 roots, with unique point116
// and direction123 dependencies. All source numbers must be float64-exact.
// Transformations, implicit frames, topology, sharing, and unused entities fail.
func ReadConical45Supports(r io.Reader, l Limits) (Conical45SupportModel, error) {
	l, e := l.defaults()
	if e != nil {
		return Conical45SupportModel{}, e
	}
	l.MaxEntities = min(l.MaxEntities, 256)
	d, e := readConeSupportRecords(r, l, false, false, false, false, false, true)
	if e != nil {
		return Conical45SupportModel{}, e
	}
	fail := func(s string) (Conical45SupportModel, error) {
		return Conical45SupportModel{}, errors.New("iges cone45: " + s)
	}
	byID := map[int]topologyRecord{}
	used := map[int]bool{}
	for _, x := range d.entities {
		byID[x.id] = x
	}
	metadata := func(x topologyRecord) ConeDatumMetadata { return ConeDatumMetadata{x.label, x.subscript} }
	datum := func(id, kind int) (geom.Vec3, ConeDatumMetadata, error) {
		x, ok := byID[id]
		if !ok || x.kind != kind || used[id] {
			return geom.Vec3{}, ConeDatumMetadata{}, geom.ErrDomain
		}
		used[id] = true
		n := 4
		if kind == 116 {
			n = 5
		}
		if len(x.values) != n && len(x.values) != n+2 {
			return geom.Vec3{}, ConeDatumMetadata{}, geom.ErrDomain
		}
		if e := compositeSourceNumbers(110, x.values); e != nil {
			return geom.Vec3{}, ConeDatumMetadata{}, e
		}
		k, e := intValue(x.values[0])
		if e != nil || k != kind {
			return geom.Vec3{}, ConeDatumMetadata{}, geom.ErrDomain
		}
		if kind == 116 {
			v, e := intValue(x.values[4])
			if e != nil || v != 0 {
				return geom.Vec3{}, ConeDatumMetadata{}, geom.ErrDomain
			}
		}
		if e := noPointers(x.values[n:]); e != nil {
			return geom.Vec3{}, ConeDatumMetadata{}, e
		}
		var a [3]float64
		for i := range a {
			a[i], e = realValue(x.values[i+1])
			if e != nil {
				return geom.Vec3{}, ConeDatumMetadata{}, e
			}
		}
		return geom.Vec3{X: a[0], Y: a[1], Z: a[2]}, metadata(x), nil
	}
	out := Conical45SupportModel{LengthUnit: d.unit}
	for _, x := range d.entities {
		if x.kind != 194 {
			continue
		}
		if len(out.Surfaces) >= 64 {
			return fail("surface count")
		}
		used[x.id] = true
		v := x.values
		if len(v) != 6 && len(v) != 8 {
			return fail("parameter count")
		}
		if e := compositeSourceNumbers(110, v); e != nil {
			return Conical45SupportModel{}, e
		}
		k, e := intValue(v[0])
		if e != nil || k != 194 {
			return fail("parameter type")
		}
		if e = noPointers(v[6:]); e != nil {
			return Conical45SupportModel{}, e
		}
		var ids [3]int
		for j, i := range []int{1, 2, 5} {
			ids[j], e = intValue(v[i])
			if e != nil {
				return fail("reference")
			}
		}
		s := Conical45Support{Metadata: metadata(x)}
		s.Radius, e = realValue(v[3])
		if e != nil {
			return fail("reference radius")
		}
		angle, e := realValue(v[4])
		if e != nil {
			return fail("angle")
		}
		if angle != 45 {
			return fail("requires exactly 45 degrees")
		}
		s.Center, s.CenterMetadata, e = datum(ids[0], 116)
		if e != nil {
			return fail("center dependency")
		}
		s.Axis, s.AxisMetadata, e = datum(ids[1], 123)
		if e != nil {
			return fail("axis dependency")
		}
		s.Reference, s.ReferenceMetadata, e = datum(ids[2], 123)
		if e != nil {
			return fail("reference dependency")
		}
		if _, e = s.Geometry(); e != nil {
			return fail("unsupported exact frame/radius")
		}
		out.Surfaces = append(out.Surfaces, s)
	}
	if len(out.Surfaces) == 0 || len(used) != len(d.entities) {
		return fail("empty or unowned graph")
	}
	return out, nil
}

// WriteConical45Supports stages and revalidates the entire file before I/O.
func WriteConical45Supports(w io.Writer, m Conical45SupportModel) error {
	if w == nil || len(m.Surfaces) == 0 || len(m.Surfaces) > 64 {
		return geom.ErrDomain
	}
	a := topologyAuthor{}
	add := func(kind, form int, data string, meta ConeDatumMetadata) int {
		id := a.add(kind, form, data)
		a.records[len(a.records)-1].label = meta.Label
		a.records[len(a.records)-1].subscript = meta.Subscript
		return id
	}
	for _, s := range m.Surfaces {
		if _, e := s.Geometry(); e != nil {
			return e
		}
		for _, meta := range []ConeDatumMetadata{s.Metadata, s.CenterMetadata, s.AxisMetadata, s.ReferenceMetadata} {
			if e := coneMetadata(meta); e != nil {
				return e
			}
		}
		p := s.Center
		pid := add(116, 0, fmt.Sprintf("116,%s,%s,%s,0,0,0;", number(p.X), number(p.Y), number(p.Z)), s.CenterMetadata)
		direction := func(v geom.Vec3, meta ConeDatumMetadata) int {
			return add(123, 0, fmt.Sprintf("123,%s,%s,%s,0,0;", number(v.X), number(v.Y), number(v.Z)), meta)
		}
		axis, ref := direction(s.Axis, s.AxisMetadata), direction(s.Reference, s.ReferenceMetadata)
		add(194, 1, fmt.Sprintf("194,%d,%d,%s,%s,%d,0,0;", pid, axis, number(s.Radius), "45.", ref), s.Metadata)
		a.geometry.Entities = append(a.geometry.Entities, Entity{Point: &p})
	}
	b, e := a.file(m.LengthUnit)
	if e != nil {
		return e
	}
	if _, e = ReadConical45Supports(bytes.NewReader(b), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return e
}
