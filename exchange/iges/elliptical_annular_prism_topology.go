package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/geom"
	"strings"
)

// EllipticalAnnularPrismDescriptor defines a positive-Z centered ellipse prism. Raw
// conic coefficients must satisfy the exact104 atom gate; UpperZ>LowerZ.
type EllipticalAnnularPrismDescriptor struct {
	OuterCoefficients, InnerCoefficients [6]float64
	LowerZ, UpperZ                       float64
}

// EllipticalAnnularPrismTopology retains authenticated original model-only IGES186
// topology and analytic supports. It does not construct a native trimmed.Solid.
// Six edges retain upper rim, upward seam and lower rim for each wall.
// The inner wall is reversed; both caps have explicit negative inner loops.
type EllipticalAnnularPrismTopology struct {
	source                                                             []byte
	Descriptor                                                         EllipticalAnnularPrismDescriptor
	LengthUnit                                                         float64
	SourceSolidID, SourceShellID, SourceVertexListID, SourceEdgeListID int
	SourceFaceIDs, SourceSurfaceIDs                                    [4]int
	Parameters                                                         map[int][]float64
	Metadata                                                           map[int]CylinderDatumMetadata
}

func ellipticalAnnularPrismAuthor(d EllipticalAnnularPrismDescriptor) (topologyAuthor, error) {
	a := topologyAuthor{}
	bad := func() (topologyAuthor, error) { return topologyAuthor{}, geom.ErrInvalid }
	outer := EllipseConic{Coefficients: d.OuterCoefficients, Z: d.LowerZ}
	inner := EllipseConic{Coefficients: d.InnerCoefficients, Z: d.LowerZ}
	oa, ob, e := ellipseConicRadii(outer)
	if e != nil {
		return bad()
	}
	ia, ib, e := ellipseConicRadii(inner)
	if e != nil || ia >= oa || ib >= ob || !finite(d.UpperZ) || d.UpperZ <= d.LowerZ {
		return bad()
	}
	if _, e = cylinderExactSum(d.UpperZ, -d.LowerZ); e != nil {
		return bad()
	}
	add := func(k, f int, v ...float64) int {
		p := []string{fmt.Sprint(k)}
		for _, n := range v {
			p = append(p, strings.TrimSuffix(number(n), "."))
		}
		return a.add(k, f, strings.Join(p, ",")+";")
	}
	add(116, 0, 0, 0, d.LowerZ, 0)
	add(116, 0, 0, 0, d.UpperZ, 0)
	add(123, 0, 0, 0, 1)
	add(123, 0, 1, 0, 0)
	add(122, 0, 23, oa, 0, d.UpperZ)
	add(122, 0, 29, ia, 0, d.UpperZ)
	add(190, 1, 3, 5, 7)
	add(190, 1, 1, 5, 7)
	add(502, 1, 4, oa, 0, d.UpperZ, oa, 0, d.LowerZ, ia, 0, d.UpperZ, ia, 0, d.LowerZ)
	for k, c := range [][6]float64{d.OuterCoefficients, d.InnerCoefficients} {
		major := oa
		if k == 1 {
			major = ia
		}
		curve := func(z float64) {
			v := append([]float64(nil), c[:]...)
			v = append(v, z, major, 0, major, 0)
			add(104, 1, v...)
		}
		curve(d.UpperZ)
		add(110, 0, major, 0, d.LowerZ, major, 0, d.UpperZ)
		curve(d.LowerZ)
	}
	add(504, 1, 6, 19, 17, 1, 17, 1, 21, 17, 2, 17, 1, 23, 17, 2, 17, 2, 25, 17, 3, 17, 3, 27, 17, 4, 17, 3, 29, 17, 4, 17, 4)
	for _, offset := range []int{0, 3} {
		loop := []float64{4}
		for i, edge := range []int{1, 2, 3, 2} {
			sense := 0.
			if i >= 2 {
				sense = 1
			}
			loop = append(loop, 0, 31, float64(edge+offset), sense, 0)
		}
		add(508, 1, loop...)
	}
	for _, p := range [][2]float64{{1, 1}, {4, 0}, {3, 1}, {6, 0}} {
		add(508, 1, 1, 0, 31, p[0], p[1], 0)
	}
	add(510, 1, 9, 1, 1, 33)
	add(510, 1, 11, 1, 1, 35)
	add(510, 1, 13, 2, 1, 37, 39)
	add(510, 1, 15, 2, 1, 41, 43)
	add(514, 1, 4, 45, 1, 47, 0, 49, 1, 51, 0)
	add(186, 0, 53, 1, 0)
	return a, nil
}

// ReadEllipticalAnnularPrismTopology checks one canonical28-record graph. All model
// edges are exact analytic104/110; absent pcurves remain absent. Reordering,
// approximating126 replacements and alternate trim graphs fail closed.
func ReadEllipticalAnnularPrismTopology(ctx context.Context, r io.Reader, l Limits) (EllipticalAnnularPrismTopology, error) {
	bad := func() (EllipticalAnnularPrismTopology, error) {
		return EllipticalAnnularPrismTopology{}, fmt.Errorf("iges: unsupported exact elliptical prism topology")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return EllipticalAnnularPrismTopology{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 28)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, false, false, true, true, true, true, true)
	if e != nil {
		return EllipticalAnnularPrismTopology{}, e
	}
	if len(doc.entities) != 28 || mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	values := map[int][]float64{}
	metadata := map[int]CylinderDatumMetadata{}
	for _, rec := range doc.entities {
		if compositeSourceNumbers(110, rec.values) != nil {
			return bad()
		}
		v := make([]float64, len(rec.values))
		for i, p := range rec.values {
			v[i], e = realValue(p)
			if e != nil {
				return bad()
			}
			if circularCylinderIntegerField(rec.kind, i) || rec.kind == 122 && i == 1 {
				if _, e = intValue(p); e != nil {
					return bad()
				}
			}
		}
		values[rec.id] = v
		metadata[rec.id] = CylinderDatumMetadata{rec.label, rec.subscript}
	}
	if len(values[1]) != 5 || len(values[3]) != 5 || len(values[19]) != 12 || len(values[25]) != 12 {
		return bad()
	}
	d := EllipticalAnnularPrismDescriptor{LowerZ: values[1][3], UpperZ: values[3][3]}
	copy(d.OuterCoefficients[:], values[19][1:7])
	copy(d.InnerCoefficients[:], values[25][1:7])
	a, e := ellipticalAnnularPrismAuthor(d)
	if e != nil {
		return bad()
	}
	for i, rec := range doc.entities {
		want := a.records[i]
		if rec.id != 2*i+1 || rec.kind != want.kind || rec.transform != 0 {
			return bad()
		}
		budget := 2048
		v, er := parseParameters(want.data, ',', ';', &budget)
		if er != nil || len(v) != len(rec.values) {
			return bad()
		}
		for j, p := range v {
			n, er := realValue(p)
			if er != nil || n != values[rec.id][j] {
				return bad()
			}
		}
	}
	if e = ctx.Err(); e != nil {
		return EllipticalAnnularPrismTopology{}, e
	}
	return EllipticalAnnularPrismTopology{source: append([]byte(nil), doc.bytes...), Descriptor: d, LengthUnit: doc.unit, SourceSolidID: 55, SourceShellID: 53, SourceVertexListID: 17, SourceEdgeListID: 31, SourceFaceIDs: [4]int{45, 47, 49, 51}, SourceSurfaceIDs: [4]int{9, 11, 13, 15}, Parameters: values, Metadata: metadata}, nil
}
func WriteEllipticalAnnularPrismTopology(ctx context.Context, w io.Writer, d EllipticalAnnularPrismDescriptor, unit float64) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	a, e := ellipticalAnnularPrismAuthor(d)
	if e != nil {
		return e
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	if _, e = ReadEllipticalAnnularPrismTopology(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
