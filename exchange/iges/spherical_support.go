package iges

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"plainsolid/geom"
)

// SphereDatumMetadata retains labels and subscripts of each authored dependency.
type SphereDatumMetadata struct {
	Label     string
	Subscript int
}

// SphericalSupport is a complete type196 form1 support. U is longitude around
// Axis from Reference; V is latitude in radians, in [-pi/2,pi/2].
// Longitude is periodic and the poles are parameter singularities. The
// frame must consist of perpendicular signed coordinate unit directions.
// No finite trim, face, topology, or material is implied.
type SphericalSupport struct {
	Center, Axis, Reference                                   geom.Vec3
	Radius                                                    float64
	Metadata, CenterMetadata, AxisMetadata, ReferenceMetadata SphereDatumMetadata
}
type SphericalSupportModel struct {
	LengthUnit float64
	Surfaces   []SphericalSupport
}

func sphereCoordinate(v geom.Vec3) bool {
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
func (s SphericalSupport) Geometry() (geom.SphereSurface, error) {
	if !s.Center.IsFinite() || !finite(s.Radius) || s.Radius <= 0 || !sphereCoordinate(s.Axis) || !sphereCoordinate(s.Reference) || s.Axis.Dot(s.Reference) != 0 {
		return geom.SphereSurface{}, geom.ErrDomain
	}
	f, e := geom.NewFrame(s.Center, s.Reference, s.Axis)
	if e != nil {
		return geom.SphereSurface{}, e
	}
	return geom.NewSphereSurface(f, s.Radius)
}
func sphereMetadata(m SphereDatumMetadata) error {
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

// ReadSphericalSupports accepts only type196 form1 roots, with unique point116
// and direction123 dependencies. All source numbers must be float64-exact.
// Transformations, implicit frames, topology, sharing, and unused entities fail.
func ReadSphericalSupports(r io.Reader, l Limits) (SphericalSupportModel, error) {
	l, e := l.defaults()
	if e != nil {
		return SphericalSupportModel{}, e
	}
	l.MaxEntities = min(l.MaxEntities, 256)
	d, e := readElementarySupportRecords(r, l, false, false, false, true)
	if e != nil {
		return SphericalSupportModel{}, e
	}
	fail := func(s string) (SphericalSupportModel, error) {
		return SphericalSupportModel{}, errors.New("iges sphere: " + s)
	}
	byID := map[int]topologyRecord{}
	used := map[int]bool{}
	for _, x := range d.entities {
		byID[x.id] = x
	}
	metadata := func(x topologyRecord) SphereDatumMetadata { return SphereDatumMetadata{x.label, x.subscript} }
	datum := func(id, kind int) (geom.Vec3, SphereDatumMetadata, error) {
		x, ok := byID[id]
		if !ok || x.kind != kind || used[id] {
			return geom.Vec3{}, SphereDatumMetadata{}, geom.ErrDomain
		}
		used[id] = true
		n := 4
		if kind == 116 {
			n = 5
		}
		if len(x.values) != n && len(x.values) != n+2 {
			return geom.Vec3{}, SphereDatumMetadata{}, geom.ErrDomain
		}
		if e := compositeSourceNumbers(110, x.values); e != nil {
			return geom.Vec3{}, SphereDatumMetadata{}, e
		}
		k, e := intValue(x.values[0])
		if e != nil || k != kind {
			return geom.Vec3{}, SphereDatumMetadata{}, geom.ErrDomain
		}
		if kind == 116 {
			v, e := intValue(x.values[4])
			if e != nil || v != 0 {
				return geom.Vec3{}, SphereDatumMetadata{}, geom.ErrDomain
			}
		}
		if e := noPointers(x.values[n:]); e != nil {
			return geom.Vec3{}, SphereDatumMetadata{}, e
		}
		var a [3]float64
		for i := range a {
			a[i], e = realValue(x.values[i+1])
			if e != nil {
				return geom.Vec3{}, SphereDatumMetadata{}, e
			}
		}
		return geom.Vec3{X: a[0], Y: a[1], Z: a[2]}, metadata(x), nil
	}
	out := SphericalSupportModel{LengthUnit: d.unit}
	for _, x := range d.entities {
		if x.kind != 196 {
			continue
		}
		if len(out.Surfaces) >= 64 {
			return fail("surface count")
		}
		used[x.id] = true
		v := x.values
		if len(v) != 5 && len(v) != 7 {
			return fail("parameter count")
		}
		if e := compositeSourceNumbers(110, v); e != nil {
			return SphericalSupportModel{}, e
		}
		k, e := intValue(v[0])
		if e != nil || k != 196 {
			return fail("parameter type")
		}
		if e = noPointers(v[5:]); e != nil {
			return SphericalSupportModel{}, e
		}
		var ids [3]int
		for j, i := range []int{1, 3, 4} {
			ids[j], e = intValue(v[i])
			if e != nil {
				return fail("reference")
			}
		}
		s := SphericalSupport{Metadata: metadata(x)}
		s.Radius, e = realValue(v[2])
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

// WriteSphericalSupports stages and revalidates the entire file before I/O.
func WriteSphericalSupports(w io.Writer, m SphericalSupportModel) error {
	if w == nil || len(m.Surfaces) == 0 || len(m.Surfaces) > 64 {
		return geom.ErrDomain
	}
	a := topologyAuthor{}
	add := func(kind, form int, data string, meta SphereDatumMetadata) int {
		id := a.add(kind, form, data)
		a.records[len(a.records)-1].label = meta.Label
		a.records[len(a.records)-1].subscript = meta.Subscript
		return id
	}
	for _, s := range m.Surfaces {
		if _, e := s.Geometry(); e != nil {
			return e
		}
		for _, meta := range []SphereDatumMetadata{s.Metadata, s.CenterMetadata, s.AxisMetadata, s.ReferenceMetadata} {
			if e := sphereMetadata(meta); e != nil {
				return e
			}
		}
		p := s.Center
		pid := add(116, 0, fmt.Sprintf("116,%s,%s,%s,0,0,0;", number(p.X), number(p.Y), number(p.Z)), s.CenterMetadata)
		direction := func(v geom.Vec3, meta SphereDatumMetadata) int {
			return add(123, 0, fmt.Sprintf("123,%s,%s,%s,0,0;", number(v.X), number(v.Y), number(v.Z)), meta)
		}
		axis, ref := direction(s.Axis, s.AxisMetadata), direction(s.Reference, s.ReferenceMetadata)
		add(196, 1, fmt.Sprintf("196,%d,%s,%d,%d,0,0;", pid, number(s.Radius), axis, ref), s.Metadata)
		a.geometry.Entities = append(a.geometry.Entities, Entity{Point: &p})
	}
	b, e := a.file(m.LengthUnit)
	if e != nil {
		return e
	}
	if _, e = ReadSphericalSupports(bytes.NewReader(b), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return e
}
