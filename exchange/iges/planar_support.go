package iges

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"plainsolid/geom"
)

// PlaneDatumMetadata retains labels and subscripts of each authored dependency.
type PlaneDatumMetadata struct {
	Label     string
	Subscript int
}

// PlanarSupport is an unbounded type190 form1 support. U and V are signed
// physical distances along Reference and Normal×Reference. The frame must consist of perpendicular signed coordinate unit directions.
// No finite trim, face, topology, or material is implied.
type PlanarSupport struct {
	Location, Normal, Reference                                   geom.Vec3
	Metadata, LocationMetadata, NormalMetadata, ReferenceMetadata PlaneDatumMetadata
}
type PlanarSupportModel struct {
	LengthUnit float64
	Surfaces   []PlanarSupport
}

func planeCoordinate(v geom.Vec3) bool {
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
func (s PlanarSupport) Geometry() (geom.PlaneSurface, error) {
	if !s.Location.IsFinite() || !planeCoordinate(s.Normal) || !planeCoordinate(s.Reference) || s.Normal.Dot(s.Reference) != 0 {
		return geom.PlaneSurface{}, geom.ErrDomain
	}
	f, e := geom.NewFrame(s.Location, s.Reference, s.Normal)
	if e != nil {
		return geom.PlaneSurface{}, e
	}
	return geom.NewPlaneSurface(f)
}
func planeMetadata(m PlaneDatumMetadata) error {
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

// ReadPlanarSupports accepts only type190 form1 roots, with unique point116
// and direction123 dependencies. All source numbers must be float64-exact.
// Transformations, implicit frames, topology, sharing, and unused entities fail.
func ReadPlanarSupports(r io.Reader, l Limits) (PlanarSupportModel, error) {
	l, e := l.defaults()
	if e != nil {
		return PlanarSupportModel{}, e
	}
	l.MaxEntities = min(l.MaxEntities, 256)
	d, e := readAnalyticSupportRecords(r, l, false, false, true)
	if e != nil {
		return PlanarSupportModel{}, e
	}
	fail := func(s string) (PlanarSupportModel, error) {
		return PlanarSupportModel{}, errors.New("iges plane: " + s)
	}
	byID := map[int]topologyRecord{}
	used := map[int]bool{}
	for _, x := range d.entities {
		byID[x.id] = x
	}
	metadata := func(x topologyRecord) PlaneDatumMetadata { return PlaneDatumMetadata{x.label, x.subscript} }
	datum := func(id, kind int) (geom.Vec3, PlaneDatumMetadata, error) {
		x, ok := byID[id]
		if !ok || x.kind != kind || used[id] {
			return geom.Vec3{}, PlaneDatumMetadata{}, geom.ErrDomain
		}
		used[id] = true
		n := 4
		if kind == 116 {
			n = 5
		}
		if len(x.values) != n && len(x.values) != n+2 {
			return geom.Vec3{}, PlaneDatumMetadata{}, geom.ErrDomain
		}
		if e := compositeSourceNumbers(110, x.values); e != nil {
			return geom.Vec3{}, PlaneDatumMetadata{}, e
		}
		k, e := intValue(x.values[0])
		if e != nil || k != kind {
			return geom.Vec3{}, PlaneDatumMetadata{}, geom.ErrDomain
		}
		if kind == 116 {
			v, e := intValue(x.values[4])
			if e != nil || v != 0 {
				return geom.Vec3{}, PlaneDatumMetadata{}, geom.ErrDomain
			}
		}
		if e := noPointers(x.values[n:]); e != nil {
			return geom.Vec3{}, PlaneDatumMetadata{}, e
		}
		var a [3]float64
		for i := range a {
			a[i], e = realValue(x.values[i+1])
			if e != nil {
				return geom.Vec3{}, PlaneDatumMetadata{}, e
			}
		}
		return geom.Vec3{X: a[0], Y: a[1], Z: a[2]}, metadata(x), nil
	}
	out := PlanarSupportModel{LengthUnit: d.unit}
	for _, x := range d.entities {
		if x.kind != 190 {
			continue
		}
		if len(out.Surfaces) >= 64 {
			return fail("surface count")
		}
		used[x.id] = true
		v := x.values
		if len(v) != 4 && len(v) != 6 {
			return fail("parameter count")
		}
		if e := compositeSourceNumbers(110, v); e != nil {
			return PlanarSupportModel{}, e
		}
		k, e := intValue(v[0])
		if e != nil || k != 190 {
			return fail("parameter type")
		}
		if e = noPointers(v[4:]); e != nil {
			return PlanarSupportModel{}, e
		}
		var ids [3]int
		for j, i := range []int{1, 2, 3} {
			ids[j], e = intValue(v[i])
			if e != nil {
				return fail("reference")
			}
		}
		s := PlanarSupport{Metadata: metadata(x)}
		s.Location, s.LocationMetadata, e = datum(ids[0], 116)
		if e != nil {
			return fail("location dependency")
		}
		s.Normal, s.NormalMetadata, e = datum(ids[1], 123)
		if e != nil {
			return fail("normal dependency")
		}
		s.Reference, s.ReferenceMetadata, e = datum(ids[2], 123)
		if e != nil {
			return fail("reference dependency")
		}
		if _, e = s.Geometry(); e != nil {
			return fail("unsupported exact frame")
		}
		out.Surfaces = append(out.Surfaces, s)
	}
	if len(out.Surfaces) == 0 || len(used) != len(d.entities) {
		return fail("empty or unowned graph")
	}
	return out, nil
}

// WritePlanarSupports stages and revalidates the entire file before I/O.
func WritePlanarSupports(w io.Writer, m PlanarSupportModel) error {
	if w == nil || len(m.Surfaces) == 0 || len(m.Surfaces) > 64 {
		return geom.ErrDomain
	}
	a := topologyAuthor{}
	add := func(kind, form int, data string, meta PlaneDatumMetadata) int {
		id := a.add(kind, form, data)
		a.records[len(a.records)-1].label = meta.Label
		a.records[len(a.records)-1].subscript = meta.Subscript
		return id
	}
	for _, s := range m.Surfaces {
		if _, e := s.Geometry(); e != nil {
			return e
		}
		for _, meta := range []PlaneDatumMetadata{s.Metadata, s.LocationMetadata, s.NormalMetadata, s.ReferenceMetadata} {
			if e := planeMetadata(meta); e != nil {
				return e
			}
		}
		p := s.Location
		pid := add(116, 0, fmt.Sprintf("116,%s,%s,%s,0,0,0;", number(p.X), number(p.Y), number(p.Z)), s.LocationMetadata)
		direction := func(v geom.Vec3, meta PlaneDatumMetadata) int {
			return add(123, 0, fmt.Sprintf("123,%s,%s,%s,0,0;", number(v.X), number(v.Y), number(v.Z)), meta)
		}
		normal, ref := direction(s.Normal, s.NormalMetadata), direction(s.Reference, s.ReferenceMetadata)
		add(190, 1, fmt.Sprintf("190,%d,%d,%d,0,0;", pid, normal, ref), s.Metadata)
		a.geometry.Entities = append(a.geometry.Entities, Entity{Point: &p})
	}
	b, e := a.file(m.LengthUnit)
	if e != nil {
		return e
	}
	if _, e = ReadPlanarSupports(bytes.NewReader(b), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return e
}
