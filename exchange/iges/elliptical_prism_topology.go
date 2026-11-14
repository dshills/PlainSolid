package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/geom"
	"strings"
)

// EllipticalPrismDescriptor defines a positive-Z centered ellipse prism. Raw
// conic coefficients must satisfy the exact104 atom gate; UpperZ>LowerZ.
type EllipticalPrismDescriptor struct {
	Coefficients   [6]float64
	LowerZ, UpperZ float64
}

// EllipticalPrismTopology retains authenticated original model-only IGES186
// topology and analytic supports. It does not construct a native trimmed.Solid.
// Three edges: upper complete rim, upward seam, lower complete rim. Side uses
// (upper−,seam−,lower+,seam+); upper cap+, lower cap reversed by shell ownership.
type EllipticalPrismTopology struct {
	source                                                             []byte
	Descriptor                                                         EllipticalPrismDescriptor
	LengthUnit                                                         float64
	SourceSolidID, SourceShellID, SourceVertexListID, SourceEdgeListID int
	SourceFaceIDs, SourceSurfaceIDs                                    [3]int
	Parameters                                                         map[int][]float64
	Metadata                                                           map[int]CylinderDatumMetadata
}

func ellipticalPrismAuthor(d EllipticalPrismDescriptor) (topologyAuthor, error) {
	a := topologyAuthor{}
	bad := func() (topologyAuthor, error) { return topologyAuthor{}, geom.ErrInvalid }
	c := EllipseConic{Coefficients: d.Coefficients, Z: d.LowerZ}
	major, _, e := ellipseConicRadii(c)
	if e != nil || !finite(d.UpperZ) || d.UpperZ <= d.LowerZ {
		return bad()
	}
	if _, e = cylinderExactSum(d.UpperZ, -d.LowerZ); e != nil {
		return bad()
	}
	add := func(k, f int, v ...float64) int {
		s := []string{fmt.Sprint(k)}
		for _, x := range v {
			s = append(s, strings.TrimSuffix(number(x), "."))
		}
		return a.add(k, f, strings.Join(s, ",")+";")
	}
	o := add(116, 0, 0, 0, d.LowerZ, 0)
	t := add(116, 0, 0, 0, d.UpperZ, 0)
	z := add(123, 0, 0, 0, 1)
	x := add(123, 0, 1, 0, 0)
	curve := func(height float64) int {
		v := append([]float64(nil), d.Coefficients[:]...)
		v = append(v, height, major, 0, major, 0)
		return add(104, 1, v...)
	}
	cb, ct := curve(d.LowerZ), curve(d.UpperZ)
	line := add(110, 0, major, 0, d.LowerZ, major, 0, d.UpperZ)
	side := add(122, 0, float64(cb), major, 0, d.UpperZ)
	pt := add(190, 1, float64(t), float64(z), float64(x))
	pb := add(190, 1, float64(o), float64(z), float64(x))
	vertices := add(502, 1, 2, major, 0, d.UpperZ, major, 0, d.LowerZ)
	edges := add(504, 1, 3, float64(ct), float64(vertices), 1, float64(vertices), 1, float64(line), float64(vertices), 2, float64(vertices), 1, float64(cb), float64(vertices), 2, float64(vertices), 2)
	loop := []float64{4}
	for i, edge := range []int{1, 2, 3, 2} {
		sense := 0.
		if i >= 2 {
			sense = 1
		}
		loop = append(loop, 0, float64(edges), float64(edge), sense, 0)
	}
	ls := add(508, 1, loop...)
	lt := add(508, 1, 1, 0, float64(edges), 1, 1, 0)
	lb := add(508, 1, 1, 0, float64(edges), 3, 1, 0)
	fs := add(510, 1, float64(side), 1, 1, float64(ls))
	ft := add(510, 1, float64(pt), 1, 1, float64(lt))
	fb := add(510, 1, float64(pb), 1, 1, float64(lb))
	shell := add(514, 1, 3, float64(fs), 1, float64(ft), 1, float64(fb), 0)
	add(186, 0, float64(shell), 1, 0)
	return a, nil
}

// ReadEllipticalPrismTopology checks one canonical20-record graph. All model
// edges are exact analytic104/110; absent pcurves remain absent. Reordering,
// approximating126 replacements and alternate trim graphs fail closed.
func ReadEllipticalPrismTopology(ctx context.Context, r io.Reader, l Limits) (EllipticalPrismTopology, error) {
	bad := func() (EllipticalPrismTopology, error) {
		return EllipticalPrismTopology{}, fmt.Errorf("iges: unsupported exact elliptical prism topology")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return EllipticalPrismTopology{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 20)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, false, false, true, true, true, true, true)
	if e != nil {
		return EllipticalPrismTopology{}, e
	}
	if len(doc.entities) != 20 || mixedTopologyDirectory(doc.bytes) != nil {
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
	if len(values[1]) != 5 || len(values[3]) != 5 || len(values[9]) != 12 {
		return bad()
	}
	d := EllipticalPrismDescriptor{LowerZ: values[1][3], UpperZ: values[3][3]}
	copy(d.Coefficients[:], values[9][1:7])
	a, e := ellipticalPrismAuthor(d)
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
		return EllipticalPrismTopology{}, e
	}
	return EllipticalPrismTopology{source: append([]byte(nil), doc.bytes...), Descriptor: d, LengthUnit: doc.unit, SourceSolidID: 39, SourceShellID: 37, SourceVertexListID: 21, SourceEdgeListID: 23, SourceFaceIDs: [3]int{31, 33, 35}, SourceSurfaceIDs: [3]int{15, 17, 19}, Parameters: values, Metadata: metadata}, nil
}
func WriteEllipticalPrismTopology(ctx context.Context, w io.Writer, d EllipticalPrismDescriptor, unit float64) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	a, e := ellipticalPrismAuthor(d)
	if e != nil {
		return e
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	if _, e = ReadEllipticalPrismTopology(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

// ExactLateralPatches exposes the original122 analytic support in four exact
// rational charts. This representation change does not invent topology/pcurves.
func (m EllipticalPrismTopology) ExactLateralPatches(ctx context.Context, o geom.SurfacePatchOptions) ([4]*geom.SurfacePatch, error) {
	if _, e := ellipticalPrismAuthor(m.Descriptor); e != nil {
		return [4]*geom.SurfacePatch{}, e
	}
	c := EllipseConic{Coefficients: m.Descriptor.Coefficients, Z: m.Descriptor.LowerZ}
	major, _, e := ellipseConicRadii(c)
	if e != nil {
		return [4]*geom.SurfacePatch{}, e
	}
	ring := EllipseConicRing{}
	for i := range ring.Members {
		ring.Members[i] = QuarterEllipseConic{EllipseConicModel: EllipseConicModel{Curve: c, LengthUnit: m.LengthUnit}, StartQuarter: i}
	}
	return (EllipseTabulated{Directrix: ring, End: geom.Vec3{X: major, Z: m.Descriptor.UpperZ}}).ExactPatches(ctx, o)
}
