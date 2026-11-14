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

// ThroughCircularCylinderDescriptor is a coaxial positive-Z cylinder with a
// complete coaxial circular through opening and two annular caps.
type ThroughCircularCylinderDescriptor struct {
	Origin                     geom.Vec3
	Radius, Height, BoreRadius float64
}

// ThroughCircularCylinderSolid retains the full source graph and the independently
// constructed native material. Source face order is outer wall, inner wall,
// upper annulus, lower annulus.
type ThroughCircularCylinderSolid struct {
	Descriptor                   ThroughCircularCylinderDescriptor
	LengthUnit                   float64
	Native                       parametric.FiniteCylinderPairDifferenceResult
	SourceSolidID, SourceShellID int
	SourceFaceIDs                [4]int
	Parameters                   map[int][]float64
	Metadata                     map[int]CylinderDatumMetadata
}

func throughCylinderFailure() error {
	return fmt.Errorf("iges: unsupported exact canonical through circular cylinder")
}
func throughCylinderAuthor(d ThroughCircularCylinderDescriptor) (topologyAuthor, error) {
	a := topologyAuthor{}
	if !d.Origin.IsFinite() || !finite(d.Radius) || !finite(d.Height) || !finite(d.BoreRadius) || d.Height <= 0 || d.BoreRadius <= 0 || d.Radius <= d.BoreRadius {
		return a, throughCylinderFailure()
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
	so := add(192, 1, float64(o), float64(axis), d.Radius, float64(ref))
	si := add(192, 1, float64(o), float64(axis), d.BoreRadius, float64(ref))
	pt := add(190, 1, float64(t), float64(axis), float64(ref))
	pb := add(190, 1, float64(o), float64(axis), float64(ref))
	vertices := add(502, 1, 4, outer, y, top, outer, y, z, inner, y, top, inner, y, floor)
	curves := []int{add(100, 0, top, x, y, outer, y, outer, y), add(110, 0, outer, y, z, outer, y, top), add(100, 0, z, x, y, outer, y, outer, y), add(100, 0, top, x, y, inner, y, inner, y), add(110, 0, inner, y, floor, inner, y, top), add(100, 0, floor, x, y, inner, y, inner, y)}
	ev := []float64{6}
	for i, pair := range [][2]int{{1, 1}, {2, 1}, {2, 2}, {3, 3}, {4, 3}, {4, 4}} {
		ev = append(ev, float64(curves[i]), float64(vertices), float64(pair[0]), float64(vertices), float64(pair[1]))
	}
	edges := add(504, 1, ev...)
	wall := func(h float64, offset int) int {
		pc := []int{add(110, 0, 360, h, 0, 0, h, 0), add(110, 0, 0, h, 0, 0, 0, 0), add(110, 0, 0, 0, 0, 360, 0, 0), add(110, 0, 360, 0, 0, 360, h, 0)}
		v := []float64{4}
		for i, edge := range []int{1, 2, 3, 2} {
			sense := 0.
			if i >= 2 {
				sense = 1
			}
			v = append(v, 0, float64(edges), float64(edge+offset), sense, 1, 0, float64(pc[i]))
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

// ReadThroughCircularCylinderSolid verifies every scalar, owner and orientation in
// a complete canonical analytic type186 graph. No source tolerance repair occurs.
func ReadThroughCircularCylinderSolid(ctx context.Context, r io.Reader, l Limits) (ThroughCircularCylinderSolid, error) {
	fail := func() (ThroughCircularCylinderSolid, error) {
		return ThroughCircularCylinderSolid{}, throughCylinderFailure()
	}
	if ctx == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return ThroughCircularCylinderSolid{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 36)
	l.MaxRecords = min(l.MaxRecords, 1024)
	l.MaxValues = min(l.MaxValues, 4096)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, true)
	if e != nil {
		return ThroughCircularCylinderSolid{}, e
	}
	if len(doc.entities) != 36 {
		return fail()
	}
	if e = mixedTopologyDirectory(doc.bytes); e != nil {
		return fail()
	}
	values := map[int][]float64{}
	meta := map[int]CylinderDatumMetadata{}
	for _, rec := range doc.entities {
		if e = compositeSourceNumbers(110, rec.values); e != nil {
			return fail()
		}
		v := make([]float64, len(rec.values))
		for i, p := range rec.values {
			if circularCylinderIntegerField(rec.kind, i) {
				if _, e = intValue(p); e != nil {
					return fail()
				}
			}
			v[i], e = realValue(p)
			if e != nil {
				return fail()
			}
		}
		values[rec.id] = v
		meta[rec.id] = CylinderDatumMetadata{Label: rec.label, Subscript: rec.subscript}
	}
	if len(values[1]) != 5 || len(values[3]) != 5 || len(values[9]) != 5 || len(values[11]) != 5 {
		return fail()
	}
	h, e := cylinderExactSum(values[3][3], -values[1][3])
	if e != nil {
		return fail()
	}
	d := ThroughCircularCylinderDescriptor{Origin: geom.Vec3{X: values[1][1], Y: values[1][2], Z: values[1][3]}, Radius: values[9][3], Height: h, BoreRadius: values[11][3]}
	a, e := throughCylinderAuthor(d)
	if e != nil {
		return fail()
	}
	for i, rec := range doc.entities {
		want := a.records[i]
		if rec.id != 2*i+1 || rec.kind != want.kind || rec.transform != 0 {
			return fail()
		}
		budget := 4096
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
	axis := geom.Vec3{Z: 1}
	native, e := parametric.FiniteCylinderPairDifference(ctx, geom.RawCylinder{Origin: d.Origin, Axis: axis, Radius: d.Radius, Max: d.Height}, geom.RawCylinder{Origin: d.Origin, Axis: axis, Radius: d.BoreRadius, Max: d.Height}, 1e-9)
	if e != nil {
		return ThroughCircularCylinderSolid{}, e
	}
	if e = ctx.Err(); e != nil {
		return ThroughCircularCylinderSolid{}, e
	}
	return ThroughCircularCylinderSolid{Descriptor: d, LengthUnit: doc.unit, Native: native, SourceSolidID: 71, SourceShellID: 69, SourceFaceIDs: [4]int{61, 63, 65, 67}, Parameters: values, Metadata: meta}, nil
}

// WriteThroughCircularCylinderSolid validates the staged source and native proof
// before publishing any bytes. It emits analytic circles, not approximate splines.
func WriteThroughCircularCylinderSolid(ctx context.Context, w io.Writer, d ThroughCircularCylinderDescriptor, unit float64) error {
	if ctx == nil || w == nil {
		return throughCylinderFailure()
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	a, e := throughCylinderAuthor(d)
	if e != nil {
		return e
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	if _, e = ReadThroughCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
