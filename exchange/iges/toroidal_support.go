package iges

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"plainsolid/geom"
)

// TorusDatumMetadata retains labels and subscripts of each authored dependency.
type TorusDatumMetadata struct {
	Label     string
	Subscript int
}

// ToroidalSupport is a complete type198 form1 support. U is longitude around
// Axis from Reference; V is the tube angle, both in radians and periodic.
// Only regular ring tori with MajorRadius > MinorRadius > 0 are accepted. The
// frame must consist of perpendicular signed coordinate unit directions.
// No finite trim, face, topology, or material is implied.
type ToroidalSupport struct {
	Center, Axis, Reference                                   geom.Vec3
	MajorRadius, MinorRadius                                  float64
	Metadata, CenterMetadata, AxisMetadata, ReferenceMetadata TorusDatumMetadata
}
type ToroidalSupportModel struct {
	LengthUnit float64
	Surfaces   []ToroidalSupport
}

func torusCoordinate(v geom.Vec3) bool {
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
func (s ToroidalSupport) Geometry() (geom.TorusSurface, error) {
	if !s.Center.IsFinite() || !finite(s.MajorRadius) || !finite(s.MinorRadius) || s.MinorRadius <= 0 || s.MajorRadius <= s.MinorRadius || !torusCoordinate(s.Axis) || !torusCoordinate(s.Reference) || s.Axis.Dot(s.Reference) != 0 {
		return geom.TorusSurface{}, geom.ErrDomain
	}
	f, e := geom.NewFrame(s.Center, s.Reference, s.Axis)
	if e != nil {
		return geom.TorusSurface{}, e
	}
	return geom.NewTorusSurface(f, s.MajorRadius, s.MinorRadius)
}
func torusMetadata(m TorusDatumMetadata) error {
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

// ReadToroidalSupports accepts only type198 form1 roots, with unique point116
// and direction123 dependencies. All source numbers must be float64-exact.
// Transformations, implicit frames, topology, sharing, and unused entities fail.
func ReadToroidalSupports(r io.Reader, l Limits) (ToroidalSupportModel, error) {
	l, e := l.defaults()
	if e != nil {
		return ToroidalSupportModel{}, e
	}
	l.MaxEntities = min(l.MaxEntities, 256)
	d, e := readRingSupportRecords(r, l, false, false, false, false, true)
	if e != nil {
		return ToroidalSupportModel{}, e
	}
	fail := func(s string) (ToroidalSupportModel, error) {
		return ToroidalSupportModel{}, errors.New("iges torus: " + s)
	}
	byID := map[int]topologyRecord{}
	used := map[int]bool{}
	for _, x := range d.entities {
		byID[x.id] = x
	}
	metadata := func(x topologyRecord) TorusDatumMetadata { return TorusDatumMetadata{x.label, x.subscript} }
	datum := func(id, kind int) (geom.Vec3, TorusDatumMetadata, error) {
		x, ok := byID[id]
		if !ok || x.kind != kind || used[id] {
			return geom.Vec3{}, TorusDatumMetadata{}, geom.ErrDomain
		}
		used[id] = true
		n := 4
		if kind == 116 {
			n = 5
		}
		if len(x.values) != n && len(x.values) != n+2 {
			return geom.Vec3{}, TorusDatumMetadata{}, geom.ErrDomain
		}
		if e := compositeSourceNumbers(110, x.values); e != nil {
			return geom.Vec3{}, TorusDatumMetadata{}, e
		}
		k, e := intValue(x.values[0])
		if e != nil || k != kind {
			return geom.Vec3{}, TorusDatumMetadata{}, geom.ErrDomain
		}
		if kind == 116 {
			v, e := intValue(x.values[4])
			if e != nil || v != 0 {
				return geom.Vec3{}, TorusDatumMetadata{}, geom.ErrDomain
			}
		}
		if e := noPointers(x.values[n:]); e != nil {
			return geom.Vec3{}, TorusDatumMetadata{}, e
		}
		var a [3]float64
		for i := range a {
			a[i], e = realValue(x.values[i+1])
			if e != nil {
				return geom.Vec3{}, TorusDatumMetadata{}, e
			}
		}
		return geom.Vec3{X: a[0], Y: a[1], Z: a[2]}, metadata(x), nil
	}
	out := ToroidalSupportModel{LengthUnit: d.unit}
	for _, x := range d.entities {
		if x.kind != 198 {
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
			return ToroidalSupportModel{}, e
		}
		k, e := intValue(v[0])
		if e != nil || k != 198 {
			return fail("parameter type")
		}
		if e = noPointers(v[6:]); e != nil {
			return ToroidalSupportModel{}, e
		}
		var ids [3]int
		for j, i := range []int{1, 2, 5} {
			ids[j], e = intValue(v[i])
			if e != nil {
				return fail("reference")
			}
		}
		s := ToroidalSupport{Metadata: metadata(x)}
		s.MajorRadius, e = realValue(v[3])
		if e != nil {
			return fail("major radius")
		}
		s.MinorRadius, e = realValue(v[4])
		if e != nil {
			return fail("radius")
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

// WriteToroidalSupports stages and revalidates the entire file before I/O.
func WriteToroidalSupports(w io.Writer, m ToroidalSupportModel) error {
	if w == nil || len(m.Surfaces) == 0 || len(m.Surfaces) > 64 {
		return geom.ErrDomain
	}
	a := topologyAuthor{}
	add := func(kind, form int, data string, meta TorusDatumMetadata) int {
		id := a.add(kind, form, data)
		a.records[len(a.records)-1].label = meta.Label
		a.records[len(a.records)-1].subscript = meta.Subscript
		return id
	}
	for _, s := range m.Surfaces {
		if _, e := s.Geometry(); e != nil {
			return e
		}
		for _, meta := range []TorusDatumMetadata{s.Metadata, s.CenterMetadata, s.AxisMetadata, s.ReferenceMetadata} {
			if e := torusMetadata(meta); e != nil {
				return e
			}
		}
		p := s.Center
		pid := add(116, 0, fmt.Sprintf("116,%s,%s,%s,0,0,0;", number(p.X), number(p.Y), number(p.Z)), s.CenterMetadata)
		direction := func(v geom.Vec3, meta TorusDatumMetadata) int {
			return add(123, 0, fmt.Sprintf("123,%s,%s,%s,0,0;", number(v.X), number(v.Y), number(v.Z)), meta)
		}
		axis, ref := direction(s.Axis, s.AxisMetadata), direction(s.Reference, s.ReferenceMetadata)
		add(198, 1, fmt.Sprintf("198,%d,%d,%s,%s,%d,0,0;", pid, axis, number(s.MajorRadius), number(s.MinorRadius), ref), s.Metadata)
		a.geometry.Entities = append(a.geometry.Entities, Entity{Point: &p})
	}
	b, e := a.file(m.LengthUnit)
	if e != nil {
		return e
	}
	if _, e = ReadToroidalSupports(bytes.NewReader(b), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return e
}
