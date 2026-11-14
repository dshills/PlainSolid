package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
)

// CircularCylinderDescriptor defines an exact complete cylinder along positive
// global Z. The fixed meridian is positive X; translation is preserved literally.
type CircularCylinderDescriptor struct {
	Origin         geom.Vec3
	Radius, Height float64
}

// CircularCylinderSolid retains the complete canonical source graph's numeric
// parameters and metadata, alongside its constructively equivalent material.
// Side pcurve U is in IGES degrees, not the native surface's radian coordinate.
type CircularCylinderSolid struct {
	Descriptor                                                         CircularCylinderDescriptor
	LengthUnit                                                         float64
	Solid                                                              parametric.Solid
	SourceSolidID, SourceShellID, SourceVertexListID, SourceEdgeListID int
	SourceFaceIDs, SourceSurfaceIDs                                    [3]int
	Parameters                                                         map[int][]float64
	Metadata                                                           map[int]CylinderDatumMetadata
}

func circularCylinderFailure() error {
	return fmt.Errorf("iges: unsupported exact canonical circular cylinder material")
}
func cylinderExactSum(a, b float64) (float64, error) {
	if !finite(a) || !finite(b) {
		return 0, circularCylinderFailure()
	}
	r := new(big.Rat).Add(tetraRat(a), tetraRat(b))
	v, exact := r.Float64()
	if !exact || !finite(v) {
		return 0, circularCylinderFailure()
	}
	return v, nil
}
func circularCylinderAuthor(d CircularCylinderDescriptor) (topologyAuthor, error) {
	a := topologyAuthor{}
	if !d.Origin.IsFinite() || !finite(d.Radius) || !finite(d.Height) || d.Radius <= 0 || d.Height <= 0 {
		return a, circularCylinderFailure()
	}
	top, e := cylinderExactSum(d.Origin.Z, d.Height)
	if e != nil {
		return a, e
	}
	rim, e := cylinderExactSum(d.Origin.X, d.Radius)
	if e != nil {
		return a, e
	}
	x, y, z := d.Origin.X, d.Origin.Y, d.Origin.Z
	add := func(k, f int, v ...float64) int {
		s := []string{fmt.Sprint(k)}
		for _, n := range v {
			s = append(s, strings.TrimSuffix(number(n), "."))
		}
		return a.add(k, f, strings.Join(s, ",")+";")
	}
	o := add(116, 0, x, y, z, 0)
	t := add(116, 0, x, y, top, 0)
	axis := add(123, 0, 0, 0, 1)
	ref := add(123, 0, 1, 0, 0)
	side := add(192, 1, float64(o), float64(axis), d.Radius, float64(ref))
	pt := add(190, 1, float64(t), float64(axis), float64(ref))
	pb := add(190, 1, float64(o), float64(axis), float64(ref))
	vertices := add(502, 1, 2, rim, y, top, rim, y, z)
	ct := add(100, 0, top, x, y, rim, y, rim, y)
	cb := add(100, 0, z, x, y, rim, y, rim, y)
	line := add(110, 0, rim, y, z, rim, y, top)
	edges := add(504, 1, 3, float64(ct), float64(vertices), 1, float64(vertices), 1, float64(line), float64(vertices), 2, float64(vertices), 1, float64(cb), float64(vertices), 2, float64(vertices), 2)
	pc := [4]int{add(110, 0, 360, d.Height, 0, 0, d.Height, 0), add(110, 0, 0, d.Height, 0, 0, 0, 0), add(110, 0, 0, 0, 0, 360, 0, 0), add(110, 0, 360, 0, 0, 360, d.Height, 0)}
	loop := []float64{4}
	for i, edge := range []int{1, 2, 3, 2} {
		sense := 0.0
		if i >= 2 {
			sense = 1
		}
		loop = append(loop, 0, float64(edges), float64(edge), sense, 1, 0, float64(pc[i]))
	}
	ls := add(508, 1, loop...)
	lt := add(508, 1, 1, 0, float64(edges), 1, 1, 0)
	lb := add(508, 1, 1, 0, float64(edges), 3, 1, 0)
	fs := add(510, 1, float64(side), 1, 1, float64(ls))
	ft := add(510, 1, float64(pt), 1, 1, float64(lt))
	fb := add(510, 1, float64(pb), 1, 1, float64(lb))
	shell := add(514, 1, 3, float64(fs), 1, float64(ft), 1, float64(fb), 0)
	add(186, 0, float64(shell), 1, 0)
	a.geometry.Entities = []Entity{{Point: &geom.Vec3{}}}
	return a, nil
}

// ReadCircularCylinderSolid proves the exact canonical 186/514/510/508/504/502
// graph with analytic192/190 supports and complete100 circular rims. It checks
// every source scalar and reference against independently recovered dimensions.
// Rounded seam vertices, angular gaps, alternate graph encodings and transforms
// are rejected; no repair or generic IGES solid reconstruction is attempted.
func ReadCircularCylinderSolid(ctx context.Context, r io.Reader, l Limits) (CircularCylinderSolid, error) {
	fail := func() (CircularCylinderSolid, error) { return CircularCylinderSolid{}, circularCylinderFailure() }
	if ctx == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return CircularCylinderSolid{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 24)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, true)
	if e != nil {
		return CircularCylinderSolid{}, e
	}
	if len(doc.entities) != 24 {
		return fail()
	}
	if e = mixedTopologyDirectory(doc.bytes); e != nil {
		return fail()
	}
	values := map[int][]float64{}
	metadata := map[int]CylinderDatumMetadata{}
	for _, rec := range doc.entities {
		if e = compositeSourceNumbers(110, rec.values); e != nil {
			return fail()
		}
		v := make([]float64, len(rec.values))
		for i, p := range rec.values {
			if circularCylinderIntegerField(rec.kind, i) {
				if _, err := intValue(p); err != nil {
					return fail()
				}
			}
			v[i], e = realValue(p)
			if e != nil {
				return fail()
			}
		}
		values[rec.id] = v
		metadata[rec.id] = CylinderDatumMetadata{Label: rec.label, Subscript: rec.subscript}
	}
	if len(values[1]) != 5 || len(values[3]) != 5 || len(values[9]) != 5 {
		return fail()
	}
	height, e := cylinderExactSum(values[3][3], -values[1][3])
	if e != nil {
		return fail()
	}
	d := CircularCylinderDescriptor{Origin: geom.Vec3{X: values[1][1], Y: values[1][2], Z: values[1][3]}, Radius: values[9][3], Height: height}
	a, e := circularCylinderAuthor(d)
	if e != nil {
		return fail()
	}
	for i, rec := range doc.entities {
		want := a.records[i]
		if rec.id != 2*i+1 || rec.kind != want.kind || rec.transform != 0 {
			return fail()
		}
		budget := 2048
		v, e := parseParameters(want.data, ',', ';', &budget)
		if e != nil || len(v) != len(rec.values) {
			return fail()
		}
		for j, p := range v {
			x, e := realValue(p)
			if e != nil || x != values[rec.id][j] {
				return fail()
			}
		}
	}
	frame, e := geom.NewFrame(d.Origin, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return fail()
	}
	solid, e := parametric.Cylinder(frame, d.Radius, d.Height)
	if e != nil {
		return fail()
	}
	if e = ctx.Err(); e != nil {
		return CircularCylinderSolid{}, e
	}
	return CircularCylinderSolid{Descriptor: d, LengthUnit: doc.unit, Solid: solid, SourceSolidID: 47, SourceShellID: 45, SourceVertexListID: 15, SourceEdgeListID: 23, SourceFaceIDs: [3]int{39, 41, 43}, SourceSurfaceIDs: [3]int{9, 11, 13}, Parameters: values, Metadata: metadata}, nil
}

// WriteCircularCylinderSolid emits the exact degree-domain side pcurves and full
// circular rims only after staged typed re-import. No approximate spline is used.
func WriteCircularCylinderSolid(ctx context.Context, w io.Writer, d CircularCylinderDescriptor, unit float64) error {
	if ctx == nil || w == nil {
		return circularCylinderFailure()
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	a, e := circularCylinderAuthor(d)
	if e != nil {
		return e
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	if _, e = ReadCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

func circularCylinderIntegerField(kind, index int) bool {
	if index == 0 {
		return true
	}
	switch kind {
	case 186, 190, 504, 508, 510, 514:
		return true
	case 192:
		return index == 1 || index == 2 || index == 4
	case 502:
		return index == 1
	case 116:
		return index == 4
	}
	return false
}
