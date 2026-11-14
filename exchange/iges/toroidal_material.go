package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/ellipticprism"
	"strings"
)

// ToroidalMaterialDescriptor is an origin-centered positive-Z ring torus.
type ToroidalMaterialDescriptor struct{ MajorRadius, MinorRadius float64 }

// ToroidalMaterial authenticates one periodic198 source face and two circular
// seams, separately mapped to sixteen exact sewn native patches.
type ToroidalMaterial struct {
	Descriptor                                 ToroidalMaterialDescriptor
	LengthUnit                                 float64
	Solid                                      trimmed.Solid
	Parameters                                 map[int][]float64
	Metadata                                   map[int]CylinderDatumMetadata
	NativeSeamEdges                            [2][4]int
	SourceSolidID, SourceShellID, SourceFaceID int
	SourceSeamCurveIDs                         [2]int
	source                                     []byte
}

func toroidalMaterialAuthor(d ToroidalMaterialDescriptor) (topologyAuthor, error) {
	a := topologyAuthor{}
	R, r := d.MajorRadius, d.MinorRadius
	if !finite(R) || !finite(r) || r <= 0 || R <= r {
		return a, geom.ErrInvalid
	}
	outer, e := cylinderExactSum(R, r)
	if e != nil {
		return a, e
	}
	if _, e = cylinderExactSum(R, -r); e != nil {
		return a, e
	}
	add := func(k, f int, v ...float64) {
		s := []string{fmt.Sprint(k)}
		for _, x := range v {
			s = append(s, strings.TrimSuffix(number(x), "."))
		}
		a.add(k, f, strings.Join(s, ",")+";")
	}
	add(116, 0, 0, 0, 0, 0)
	add(123, 0, 0, 0, 1)
	add(123, 0, 1, 0, 0)
	add(198, 1, 1, 3, R, r, 5, 0, 0)
	add(502, 1, 1, outer, 0, 0)
	add(100, 0, 0, 0, 0, outer, 0, outer, 0)
	add(100, 0, 0, 0, 0, r, 0, r, 0)
	add(124, 0, 1, 0, 0, R, 0, 0, -1, 0, 0, 1, 0, 0)
	add(504, 1, 2, 11, 9, 1, 9, 1, 13, 9, 1, 9, 1)
	add(508, 1, 4, 0, 17, 1, 1, 0, 0, 17, 2, 1, 0, 0, 17, 1, 0, 0, 0, 17, 2, 0, 0)
	add(510, 1, 7, 1, 1, 19)
	add(514, 1, 1, 21, 1)
	add(186, 0, 23, 1, 0)
	return a, nil
}
func ReadToroidalMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ToroidalMaterial, error) {
	bad := func() (ToroidalMaterial, error) {
		return ToroidalMaterial{}, fmt.Errorf("iges: unsupported exact toroidal material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return ToroidalMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 13)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true, true)
	if e != nil || len(doc.entities) != 13 || mixedTopologyDirectory(doc.bytes) != nil {
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
			if circularCylinderIntegerField(rec.kind, j) || (rec.kind == 198 && (j == 1 || j == 2 || j == 5 || j == 6 || j == 7)) {
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
	if len(values[7]) != 8 {
		return bad()
	}
	d := ToroidalMaterialDescriptor{values[7][3], values[7][4]}
	a, e := toroidalMaterialAuthor(d)
	if e != nil {
		return bad()
	}
	for i, rec := range doc.entities {
		w := a.records[i]
		tr := 0
		if rec.id == 13 {
			tr = 15
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
	s, seams, e := ellipticprism.RingTorus(ctx, d.MajorRadius, d.MinorRadius, tol)
	if e != nil {
		return ToroidalMaterial{}, e
	}
	return ToroidalMaterial{d, doc.unit, s, values, metadata, seams, 25, 23, 21, [2]int{11, 13}, append([]byte(nil), doc.bytes...)}, nil
}
func WriteToroidalMaterial(ctx context.Context, w io.Writer, d ToroidalMaterialDescriptor, unit, tol float64) error {
	if w == nil {
		return geom.ErrInvalid
	}
	a, e := toroidalMaterialAuthor(d)
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
			if id == 13 {
				copy(line[48:56], "      15")
			}
		}
	}
	if _, e = ReadToroidalMaterial(ctx, bytes.NewReader(raw), Limits{}, tol); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
