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

// Conical45CylinderThroughDescriptor defines a positive-Z expanding 45-degree stock and a concentric constant-radius through opening.
type Conical45CylinderThroughDescriptor struct {
	Origin                     geom.Vec3
	Radius, Height, BoreRadius float64
}

// Conical45CylinderThroughSolid authenticates one four-face source shell and ten exactly sewn native faces.
type Conical45CylinderThroughSolid struct {
	Descriptor                   Conical45CylinderThroughDescriptor
	LengthUnit                   float64
	Solid                        trimmed.Solid
	NativeRimEdges               [2][2][4]int
	SourceSolidID, SourceShellID int
	SourceFaceIDs                [4]int
	NativeFaceSourceIDs          [10]int
	Parameters                   map[int][]float64
	Metadata                     map[int]CylinderDatumMetadata
	source                       []byte
}

func conical45CylinderThroughFailure() error {
	return fmt.Errorf("iges: unsupported exact conical stock cylindrical through opening")
}
func conical45CylinderThroughAuthor(d Conical45CylinderThroughDescriptor) (topologyAuthor, error) {
	a := topologyAuthor{}
	if !d.Origin.IsFinite() || !finite(d.Radius) || !finite(d.Height) || !finite(d.BoreRadius) || d.Height <= 0 || d.BoreRadius <= 0 || d.Radius <= d.BoreRadius {
		return a, conical45CylinderThroughFailure()
	}
	top, e := cylinderExactSum(d.Origin.Z, d.Height)
	if e != nil {
		return a, e
	}
	floor := d.Origin.Z
	outer, e := cylinderExactSum(d.Origin.X, d.Radius)
	if e != nil {
		return a, e
	}
	inner, e := cylinderExactSum(d.Origin.X, d.BoreRadius)
	if e != nil {
		return a, e
	}
	topRadius, e := cylinderExactSum(d.Radius, d.Height)
	if e != nil {
		return a, e
	}
	topOuter, e := cylinderExactSum(d.Origin.X, topRadius)
	if e != nil {
		return a, e
	}
	depth := d.Height
	add := func(k, f int, v ...float64) int {
		s := []string{fmt.Sprint(k)}
		for _, n := range v {
			s = append(s, strings.TrimSuffix(number(n), "."))
		}
		return a.add(k, f, strings.Join(s, ",")+";")
	}
	x, y, z := d.Origin.X, d.Origin.Y, d.Origin.Z
	o := add(116, 0, x, y, z, 0)
	t := add(116, 0, x, y, top, 0)
	axis := add(123, 0, 0, 0, 1)
	ref := add(123, 0, 1, 0, 0)
	so := add(194, 1, float64(o), float64(axis), d.Radius, 45, float64(ref), 0, 0)
	si := add(192, 1, float64(o), float64(axis), d.BoreRadius, float64(ref))
	pt := add(190, 1, float64(t), float64(axis), float64(ref))
	pb := add(190, 1, float64(o), float64(axis), float64(ref))
	vertices := add(502, 1, 4, topOuter, y, top, outer, y, z, inner, y, top, inner, y, floor)
	curves := []int{add(100, 0, top, x, y, topOuter, y, topOuter, y), add(110, 0, outer, y, z, topOuter, y, top), add(100, 0, z, x, y, outer, y, outer, y), add(100, 0, top, x, y, inner, y, inner, y), add(110, 0, inner, y, floor, inner, y, top), add(100, 0, floor, x, y, inner, y, inner, y)}
	ev := []float64{6}
	for i, pair := range [][2]int{{1, 1}, {2, 1}, {2, 2}, {3, 3}, {4, 3}, {4, 4}} {
		ev = append(ev, float64(curves[i]), float64(vertices), float64(pair[0]), float64(vertices), float64(pair[1]))
	}
	edges := add(504, 1, ev...)
	wall := func(h float64, offset int) int {
		v := []float64{4}
		for i, edge := range []int{1, 2, 3, 2} {
			sense := 0.
			if i >= 2 {
				sense = 1
			}
			v = append(v, 0, float64(edges), float64(edge+offset), sense, 0)
		}
		return add(508, 1, v...)
	}
	lo := wall(d.Height, 0)
	li := wall(depth, 3)
	loop := func(edge, sense int) int { return add(508, 1, 1, 0, float64(edges), float64(edge), float64(sense), 0) }
	lt := loop(1, 1)
	lh := loop(4, 0)
	lb := loop(3, 1)
	lf := loop(6, 0)
	fo := add(510, 1, float64(so), 1, 1, float64(lo))
	fi := add(510, 1, float64(si), 1, 1, float64(li))
	ft := add(510, 1, float64(pt), 2, 1, float64(lt), float64(lh))
	fb := add(510, 1, float64(pb), 2, 1, float64(lb), float64(lf))
	sh := add(514, 1, 4, float64(fo), 1, float64(fi), 0, float64(ft), 1, float64(fb), 0)
	add(186, 0, float64(sh), 1, 0)
	a.geometry.Entities = []Entity{{Point: &geom.Vec3{}}}
	return a, nil
}

// ReadConical45CylinderThroughSolid verifies every scalar, owner and orientation in
// a complete canonical analytic type186 graph. No source tolerance repair occurs.
func ReadConical45CylinderThroughSolid(ctx context.Context, r io.Reader, l Limits) (Conical45CylinderThroughSolid, error) {
	fail := func() (Conical45CylinderThroughSolid, error) {
		return Conical45CylinderThroughSolid{}, conical45CylinderThroughFailure()
	}
	if ctx == nil || r == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return Conical45CylinderThroughSolid{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 28)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, true, true)
	if e != nil {
		return Conical45CylinderThroughSolid{}, e
	}
	if len(doc.entities) != 28 {
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
	if len(values[1]) != 5 || len(values[3]) != 5 || len(values[9]) != 8 || len(values[11]) != 5 {
		return fail()
	}
	height, e := cylinderExactSum(values[3][3], -values[1][3])
	if e != nil {
		return fail()
	}
	d := Conical45CylinderThroughDescriptor{Origin: geom.Vec3{X: values[1][1], Y: values[1][2], Z: values[1][3]}, Radius: values[9][3], Height: height, BoreRadius: values[11][3]}
	a, e := conical45CylinderThroughAuthor(d)
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
	solid, rims, e := ellipticprism.CircularFrustumThrough(ctx, frame, d.Radius, topRadius, d.BoreRadius, d.Height, 1e-9)
	if e != nil {
		return fail()
	}
	return Conical45CylinderThroughSolid{Descriptor: d, LengthUnit: doc.unit, Solid: solid, NativeRimEdges: rims, SourceSolidID: 55, SourceShellID: 53, SourceFaceIDs: [4]int{45, 47, 49, 51}, NativeFaceSourceIDs: [10]int{51, 49, 45, 45, 45, 45, 47, 47, 47, 47}, Parameters: values, Metadata: metadata, source: append([]byte(nil), doc.bytes...)}, nil

}

// WriteConical45CylinderThroughSolid emits exact 45 degree support, model-only trims and full
// circular rims only after staged typed re-import. No approximate spline is used.
func WriteConical45CylinderThroughSolid(ctx context.Context, w io.Writer, d Conical45CylinderThroughDescriptor, unit float64) error {
	if ctx == nil || w == nil {
		return conical45CylinderThroughFailure()
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	a, e := conical45CylinderThroughAuthor(d)
	if e != nil {
		return e
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	if _, e = ReadConical45CylinderThroughSolid(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
