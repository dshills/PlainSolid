package iges

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"plainsolid/geom"
)

// CylinderDatumMetadata retains labels and subscripts of each authored dependency.
type CylinderDatumMetadata struct {
	Label     string
	Subscript int
}

// CylindricalSurface is an unbounded type192 form1 support. U is radians around
// Axis starting at Reference; V is signed physical distance along Axis. The
// frame must consist of perpendicular signed coordinate unit directions.
// No finite trim, face, topology, or material is implied.
type CylindricalSurface struct {
	Location, Axis, Reference                                   geom.Vec3
	Radius                                                      float64
	Metadata, LocationMetadata, AxisMetadata, ReferenceMetadata CylinderDatumMetadata
}
type CylindricalSurfaceModel struct {
	LengthUnit float64
	Surfaces   []CylindricalSurface
}

func cylinderCoordinate(v geom.Vec3) bool {
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
func (s CylindricalSurface) Geometry() (geom.CylinderSurface, error) {
	if !s.Location.IsFinite() || !finite(s.Radius) || s.Radius <= 0 || !cylinderCoordinate(s.Axis) || !cylinderCoordinate(s.Reference) || s.Axis.Dot(s.Reference) != 0 {
		return geom.CylinderSurface{}, geom.ErrDomain
	}
	f, e := geom.NewFrame(s.Location, s.Reference, s.Axis)
	if e != nil {
		return geom.CylinderSurface{}, e
	}
	return geom.NewCylinderSurface(f, s.Radius)
}
func cylinderMetadata(m CylinderDatumMetadata) error {
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

// ReadCylindricalSurfaces accepts only type192 form1 roots, with unique point116
// and direction123 dependencies. All source numbers must be float64-exact.
// Transformations, implicit frames, topology, sharing, and unused entities fail.
func ReadCylindricalSurfaces(r io.Reader, l Limits) (CylindricalSurfaceModel, error) {
	l, e := l.defaults()
	if e != nil {
		return CylindricalSurfaceModel{}, e
	}
	l.MaxEntities = min(l.MaxEntities, 256)
	d, e := readSupportRecords(r, l, false, true)
	if e != nil {
		return CylindricalSurfaceModel{}, e
	}
	fail := func(s string) (CylindricalSurfaceModel, error) {
		return CylindricalSurfaceModel{}, errors.New("iges cylinder: " + s)
	}
	byID := map[int]topologyRecord{}
	used := map[int]bool{}
	for _, x := range d.entities {
		byID[x.id] = x
	}
	metadata := func(x topologyRecord) CylinderDatumMetadata { return CylinderDatumMetadata{x.label, x.subscript} }
	datum := func(id, kind int) (geom.Vec3, CylinderDatumMetadata, error) {
		x, ok := byID[id]
		if !ok || x.kind != kind || used[id] {
			return geom.Vec3{}, CylinderDatumMetadata{}, geom.ErrDomain
		}
		used[id] = true
		n := 4
		if kind == 116 {
			n = 5
		}
		if len(x.values) != n && len(x.values) != n+2 {
			return geom.Vec3{}, CylinderDatumMetadata{}, geom.ErrDomain
		}
		if e := compositeSourceNumbers(110, x.values); e != nil {
			return geom.Vec3{}, CylinderDatumMetadata{}, e
		}
		k, e := intValue(x.values[0])
		if e != nil || k != kind {
			return geom.Vec3{}, CylinderDatumMetadata{}, geom.ErrDomain
		}
		if kind == 116 {
			v, e := intValue(x.values[4])
			if e != nil || v != 0 {
				return geom.Vec3{}, CylinderDatumMetadata{}, geom.ErrDomain
			}
		}
		if e := noPointers(x.values[n:]); e != nil {
			return geom.Vec3{}, CylinderDatumMetadata{}, e
		}
		var a [3]float64
		for i := range a {
			a[i], e = realValue(x.values[i+1])
			if e != nil {
				return geom.Vec3{}, CylinderDatumMetadata{}, e
			}
		}
		return geom.Vec3{X: a[0], Y: a[1], Z: a[2]}, metadata(x), nil
	}
	out := CylindricalSurfaceModel{LengthUnit: d.unit}
	for _, x := range d.entities {
		if x.kind != 192 {
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
			return CylindricalSurfaceModel{}, e
		}
		k, e := intValue(v[0])
		if e != nil || k != 192 {
			return fail("parameter type")
		}
		if e = noPointers(v[5:]); e != nil {
			return CylindricalSurfaceModel{}, e
		}
		var ids [3]int
		for j, i := range []int{1, 2, 4} {
			ids[j], e = intValue(v[i])
			if e != nil {
				return fail("reference")
			}
		}
		s := CylindricalSurface{Metadata: metadata(x)}
		s.Radius, e = realValue(v[3])
		if e != nil {
			return fail("radius")
		}
		s.Location, s.LocationMetadata, e = datum(ids[0], 116)
		if e != nil {
			return fail("location dependency")
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

// WriteCylindricalSurfaces stages and revalidates the entire file before I/O.
func WriteCylindricalSurfaces(w io.Writer, m CylindricalSurfaceModel) error {
	if w == nil || len(m.Surfaces) == 0 || len(m.Surfaces) > 64 {
		return geom.ErrDomain
	}
	a := topologyAuthor{}
	add := func(kind, form int, data string, meta CylinderDatumMetadata) int {
		id := a.add(kind, form, data)
		a.records[len(a.records)-1].label = meta.Label
		a.records[len(a.records)-1].subscript = meta.Subscript
		return id
	}
	for _, s := range m.Surfaces {
		if _, e := s.Geometry(); e != nil {
			return e
		}
		for _, meta := range []CylinderDatumMetadata{s.Metadata, s.LocationMetadata, s.AxisMetadata, s.ReferenceMetadata} {
			if e := cylinderMetadata(meta); e != nil {
				return e
			}
		}
		p := s.Location
		pid := add(116, 0, fmt.Sprintf("116,%s,%s,%s,0,0,0;", number(p.X), number(p.Y), number(p.Z)), s.LocationMetadata)
		direction := func(v geom.Vec3, meta CylinderDatumMetadata) int {
			return add(123, 0, fmt.Sprintf("123,%s,%s,%s,0,0;", number(v.X), number(v.Y), number(v.Z)), meta)
		}
		axis, ref := direction(s.Axis, s.AxisMetadata), direction(s.Reference, s.ReferenceMetadata)
		add(192, 1, fmt.Sprintf("192,%d,%d,%s,%d,0,0;", pid, axis, number(s.Radius), ref), s.Metadata)
		a.geometry.Entities = append(a.geometry.Entities, Entity{Point: &p})
	}
	b, e := a.file(m.LengthUnit)
	if e != nil {
		return e
	}
	if _, e = ReadCylindricalSurfaces(bytes.NewReader(b), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return e
}
