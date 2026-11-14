package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
)

// Conical45FrustumDescriptor defines an exact expanding 45 degree frustum along positive
// global Z. The fixed meridian is positive X; translation is preserved literally.
type Conical45FrustumDescriptor struct {
	Origin         geom.Vec3
	Radius, Height float64 // positive base radius and axial height; top radius is Radius+Height
}

// Conical45FrustumSolid retains the complete canonical source graph's numeric
// parameters and metadata, alongside its constructively equivalent material.
// The source wall has model-only trims; no slant-distance pcurves are invented.
type Conical45FrustumSolid struct {
	Descriptor                                                         Conical45FrustumDescriptor
	LengthUnit                                                         float64
	Solid                                                              parametric.Solid
	SourceSolidID, SourceShellID, SourceVertexListID, SourceEdgeListID int
	SourceFaceIDs, SourceSurfaceIDs                                    [3]int
	NativeFaceSourceIDs                                                [3]int // native lower cap, wall, upper cap
	Parameters                                                         map[int][]float64
	Metadata                                                           map[int]CylinderDatumMetadata
}

func conical45FrustumFailure() error {
	return fmt.Errorf("iges: unsupported exact canonical 45 degree frustum material")
}
func conical45FrustumAuthor(d Conical45FrustumDescriptor) (topologyAuthor, error) {
	a := topologyAuthor{}
	if !d.Origin.IsFinite() || !finite(d.Radius) || !finite(d.Height) || d.Radius <= 0 || d.Height <= 0 {
		return a, conical45FrustumFailure()
	}
	top, e := cylinderExactSum(d.Origin.Z, d.Height)
	if e != nil {
		return a, e
	}
	rim, e := cylinderExactSum(d.Origin.X, d.Radius)
	if e != nil {
		return a, e
	}
	topRadius, e := cylinderExactSum(d.Radius, d.Height)
	if e != nil {
		return a, e
	}
	topRim, e := cylinderExactSum(d.Origin.X, topRadius)
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
	side := add(194, 1, float64(o), float64(axis), d.Radius, 45, float64(ref), 0, 0)
	pt := add(190, 1, float64(t), float64(axis), float64(ref))
	pb := add(190, 1, float64(o), float64(axis), float64(ref))
	vertices := add(502, 1, 2, topRim, y, top, rim, y, z)
	ct := add(100, 0, top, x, y, topRim, y, topRim, y)
	cb := add(100, 0, z, x, y, rim, y, rim, y)
	line := add(110, 0, rim, y, z, topRim, y, top)
	edges := add(504, 1, 3, float64(ct), float64(vertices), 1, float64(vertices), 1, float64(line), float64(vertices), 2, float64(vertices), 1, float64(cb), float64(vertices), 2, float64(vertices), 2)
	loop := []float64{4}
	for i, edge := range []int{1, 2, 3, 2} {
		sense := 0.0
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
	a.geometry.Entities = []Entity{{Point: &geom.Vec3{}}}
	return a, nil
}

// ReadConical45FrustumSolid proves the exact canonical 186/514/510/508/504/502
// graph with analytic194/190 supports and complete100 circular rims. It checks
// every source scalar and reference against independently recovered dimensions.
// Rounded seam vertices, angular gaps, alternate graph encodings and transforms
// are rejected; no repair or generic IGES solid reconstruction is attempted.
func ReadConical45FrustumSolid(ctx context.Context, r io.Reader, l Limits) (Conical45FrustumSolid, error) {
	fail := func() (Conical45FrustumSolid, error) { return Conical45FrustumSolid{}, conical45FrustumFailure() }
	if ctx == nil || r == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return Conical45FrustumSolid{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 20)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, true, true)
	if e != nil {
		return Conical45FrustumSolid{}, e
	}
	if len(doc.entities) != 20 {
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
			if circularCylinderIntegerField(rec.kind, i) || rec.kind == 194 && (i == 1 || i == 2 || i == 5 || i == 6 || i == 7) {
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
	if len(values[1]) != 5 || len(values[3]) != 5 || len(values[9]) != 8 {
		return fail()
	}
	height, e := cylinderExactSum(values[3][3], -values[1][3])
	if e != nil {
		return fail()
	}
	d := Conical45FrustumDescriptor{Origin: geom.Vec3{X: values[1][1], Y: values[1][2], Z: values[1][3]}, Radius: values[9][3], Height: height}
	a, e := conical45FrustumAuthor(d)
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
	topRadius, e := cylinderExactSum(d.Radius, d.Height)
	if e != nil {
		return fail()
	}
	solid, e := parametric.Revolve(frame, []geom.Vec2{{}, {X: d.Radius}, {X: topRadius, Y: d.Height}, {Y: d.Height}})
	if e != nil {
		return fail()
	}
	if e = ctx.Err(); e != nil {
		return Conical45FrustumSolid{}, e
	}
	return Conical45FrustumSolid{Descriptor: d, LengthUnit: doc.unit, Solid: solid, SourceSolidID: 39, SourceShellID: 37, SourceVertexListID: 15, SourceEdgeListID: 23, SourceFaceIDs: [3]int{31, 33, 35}, NativeFaceSourceIDs: [3]int{35, 31, 33}, SourceSurfaceIDs: [3]int{9, 11, 13}, Parameters: values, Metadata: metadata}, nil
}

// WriteConical45FrustumSolid emits exact 45 degree support, model-only trims and full
// circular rims only after staged typed re-import. No approximate spline is used.
func WriteConical45FrustumSolid(ctx context.Context, w io.Writer, d Conical45FrustumDescriptor, unit float64) error {
	if ctx == nil || w == nil {
		return conical45FrustumFailure()
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	a, e := conical45FrustumAuthor(d)
	if e != nil {
		return e
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	if _, e = ReadConical45FrustumSolid(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
