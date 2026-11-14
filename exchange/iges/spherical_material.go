package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
)

// SphericalMaterialDescriptor is an origin-centered sphere with positive-Z axis.
type SphericalMaterialDescriptor struct{ Radius float64 }

// SphericalMaterial authenticates a196 source face, two pole uses and a meridian
// seam. Native geometry uses the existing analytic angular sphere convention.
type SphericalMaterial struct {
	Descriptor SphericalMaterialDescriptor
	LengthUnit float64
	Solid      trimmed.Solid
	Parameters map[int][]float64
	Metadata   map[int]CylinderDatumMetadata
	// NativeEdges are south pole, meridian, north pole; source pole ordinals 1/2 map to native vertices 1/0.
	NativeEdges                                [3]int
	SourceSolidID, SourceShellID, SourceFaceID int
	SourceMeridianCurveID                      int
	source                                     []byte
}

func sphericalMaterialAuthor(d SphericalMaterialDescriptor) (topologyAuthor, error) {
	a := topologyAuthor{}
	r := d.Radius
	if !finite(r) || r <= 0 {
		return a, geom.ErrInvalid
	}
	add := func(k, f int, v ...float64) {
		s := []string{fmt.Sprint(k)}
		for _, x := range v {
			s = append(s, strings.TrimSuffix(number(x), "."))
		}
		a.add(k, f, strings.Join(s, ",")+";")
	}
	add(186, 0, 3, 1, 0)
	add(514, 1, 1, 5, 1)
	add(510, 1, 7, 1, 1, 15)
	add(196, 1, 9, r, 11, 13)
	add(116, 0, 0, 0, 0, 0)
	add(123, 0, 0, 0, 1)
	add(123, 0, 1, 0, 0)
	add(508, 1, 4, 1, 17, 1, 0, 1, 0, 19, 0, 21, 1, 0, 1, 0, 27, 1, 17, 2, 1, 1, 0, 29, 0, 21, 1, 1, 1, 0, 31)
	add(502, 1, 2, 0, 0, r, 0, 0, -r)
	add(110, 0, 360, 90, 0, 0, 90, 0)
	add(504, 1, 1, 23, 17, 2, 17, 1)
	add(100, 0, 0, 0, 0, 0, -r, 0, r)
	add(124, 0, 1, 0, 0, 0, 0, 0, -1, 0, 0, 1, 0, 0)
	add(110, 0, 0, 90, 0, 0, -90, 0)
	add(110, 0, 0, -90, 0, 360, -90, 0)
	add(110, 0, 360, -90, 0, 360, 90, 0)
	return a, nil
}
func ReadSphericalMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (SphericalMaterial, error) {
	bad := func() (SphericalMaterial, error) {
		return SphericalMaterial{}, fmt.Errorf("iges: unsupported exact spherical material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return SphericalMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 16)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true, true, true)
	if e != nil || len(doc.entities) != 16 || mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	values := map[int][]float64{}
	metadata := map[int]CylinderDatumMetadata{}
	for _, rec := range doc.entities {
		if compositeSourceNumbers(110, rec.values) != nil {
			return bad()
		}
		v := make([]float64, len(rec.values))
		for j, p := range rec.values {
			if circularCylinderIntegerField(rec.kind, j) || (rec.kind == 196 && (j == 1 || j == 3 || j == 4)) {
				if _, e := intValue(p); e != nil {
					return bad()
				}
			}
			v[j], e = realValue(p)
			if e != nil {
				return bad()
			}
		}
		values[rec.id] = v
		metadata[rec.id] = CylinderDatumMetadata{rec.label, rec.subscript}
	}
	if len(values[7]) != 5 {
		return bad()
	}
	d := SphericalMaterialDescriptor{values[7][2]}
	a, e := sphericalMaterialAuthor(d)
	if e != nil {
		return bad()
	}
	for i, rec := range doc.entities {
		w := a.records[i]
		tr := 0
		if rec.id == 23 {
			tr = 25
		}
		if rec.id != 2*i+1 || rec.kind != w.kind || rec.transform != tr {
			return bad()
		}
		budget := 2048
		v, e := parseParameters(w.data, ',', ';', &budget)
		if e != nil || len(v) != len(rec.values) {
			return bad()
		}
		for j, p := range v {
			x, e := realValue(p)
			if e != nil || x != values[rec.id][j] {
				return bad()
			}
		}
	}
	p, e := parametric.Sphere(geom.Vec3{}, d.Radius)
	if e != nil {
		return SphericalMaterial{}, e
	}
	s, e := trimmed.FromParametric(p)
	if e != nil {
		return SphericalMaterial{}, e
	}
	s, e = trimmed.NewSolid(s.Vertices(), s.Edges(), s.Faces(), tol)
	if e != nil {
		return SphericalMaterial{}, e
	}
	return SphericalMaterial{d, doc.unit, s, values, metadata, [3]int{0, 1, 2}, 1, 3, 5, 23, append([]byte(nil), doc.bytes...)}, nil
}
func WriteSphericalMaterial(ctx context.Context, w io.Writer, d SphericalMaterialDescriptor, unit, tol float64) error {
	if w == nil {
		return geom.ErrInvalid
	}
	a, e := sphericalMaterialAuthor(d)
	if e != nil {
		return e
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(line) == 80 && line[72] == 'D' {
			id, _ := field(string(line[73:]))
			if id == 23 {
				copy(line[48:56], "      25")
			}
		}
	}
	if _, e = ReadSphericalMaterial(ctx, bytes.NewReader(raw), Limits{}, tol); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
