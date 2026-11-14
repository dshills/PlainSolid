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

// SphericalCavityMaterialDescriptor is a strictly nested concentric sphere cavity.
type SphericalCavityMaterialDescriptor struct{ OuterRadius, InnerRadius float64 }

// SphericalCavityMaterial authenticates separately owned original shells.
// Native geometry uses the established analytic angular sphere convention.
type SphericalCavityMaterial struct {
	Descriptor                                                               SphericalCavityMaterialDescriptor
	LengthUnit                                                               float64
	Solid                                                                    trimmed.Solid
	Parameters                                                               map[int][]float64
	Metadata                                                                 map[int]CylinderDatumMetadata
	NativeEdges                                                              [2][3]int
	SourceSolidID                                                            int
	SourceShellIDs, SourceFaceIDs, SourcePoleListIDs, SourceMeridianCurveIDs [2]int
	SourceDegreeCurveIDs                                                     [2][4]int
	source                                                                   []byte
}

func sphericalCavityMaterialAuthor(d SphericalCavityMaterialDescriptor) (topologyAuthor, error) {
	if !finite(d.OuterRadius) || !finite(d.InnerRadius) || d.InnerRadius <= 0 || d.OuterRadius <= d.InnerRadius {
		return topologyAuthor{}, geom.ErrInvalid
	}
	a, e := sphericalMaterialAuthor(SphericalMaterialDescriptor{Radius: d.OuterRadius})
	if e != nil {
		return a, e
	}
	inner, e := sphericalMaterialAuthor(SphericalMaterialDescriptor{Radius: d.InnerRadius})
	if e != nil {
		return a, e
	}
	for _, rec := range inner.records[1:] {
		budget := 2048
		v, e := parseParameters(rec.data, ',', ';', &budget)
		if e != nil {
			return topologyAuthor{}, e
		}
		slots, e := sphericalReferenceSlots(topologyRecord{kind: rec.kind, values: v})
		if e != nil {
			return topologyAuthor{}, e
		}
		for _, j := range slots {
			id, e := intValue(v[j])
			if e != nil {
				return topologyAuthor{}, e
			}
			v[j].text = fmt.Sprint(id + 30)
		}
		ss := []string{}
		for _, p := range v {
			ss = append(ss, p.text)
		}
		rec.data = strings.Join(ss, ",") + ";"
		a.records = append(a.records, rec)
	}
	a.records[0].data = "186,3,1,1,33,0;"
	return a, nil
}
func ReadSphericalCavityMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (SphericalCavityMaterial, error) {
	bad := func() (SphericalCavityMaterial, error) {
		return SphericalCavityMaterial{}, fmt.Errorf("iges: unsupported exact spherical cavity material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return SphericalCavityMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 31)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true, true, true)
	if e != nil || len(doc.entities) != 31 || mixedTopologyDirectory(doc.bytes) != nil {
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
	if len(values[7]) != 5 || len(values[37]) != 5 {
		return bad()
	}
	d := SphericalCavityMaterialDescriptor{values[7][2], values[37][2]}
	a, e := sphericalCavityMaterialAuthor(d)
	if e != nil {
		return bad()
	}
	for i, rec := range doc.entities {
		w := a.records[i]
		tr := 0
		if rec.id == 23 {
			tr = 25
		}
		if rec.id == 53 {
			tr = 55
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
	p, e := parametric.CombineSpheres(parametric.Ball{Center: geom.Vec3{}, Radius: d.OuterRadius}, parametric.Ball{Center: geom.Vec3{}, Radius: d.InnerRadius}, parametric.Difference, tol)
	if e != nil {
		return SphericalCavityMaterial{}, e
	}
	s, e := trimmed.FromParametric(p)
	if e != nil {
		return SphericalCavityMaterial{}, e
	}
	s, e = trimmed.NewSolid(s.Vertices(), s.Edges(), s.Faces(), tol)
	if e != nil {
		return SphericalCavityMaterial{}, e
	}
	if len(s.Faces()) != 2 || !s.Faces()[1].Reversed || s.Faces()[0].Reversed {
		return bad()
	}
	return SphericalCavityMaterial{d, doc.unit, s, values, metadata, [2][3]int{{0, 1, 2}, {3, 4, 5}}, 1, [2]int{3, 33}, [2]int{5, 35}, [2]int{17, 47}, [2]int{23, 53}, [2][4]int{{19, 27, 29, 31}, {49, 57, 59, 61}}, append([]byte(nil), doc.bytes...)}, nil
}
func WriteSphericalCavityMaterial(ctx context.Context, w io.Writer, d SphericalCavityMaterialDescriptor, unit, tol float64) error {
	if w == nil {
		return geom.ErrInvalid
	}
	a, e := sphericalCavityMaterialAuthor(d)
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
			if id == 53 {
				copy(line[48:56], "      55")
			}
		}
	}
	if _, e = ReadSphericalCavityMaterial(ctx, bytes.NewReader(raw), Limits{}, tol); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
