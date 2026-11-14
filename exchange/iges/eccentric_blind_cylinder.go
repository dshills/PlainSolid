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

// EccentricBlindCylinderDescriptor is a positive-Z cylinder with an eccentric
// upper blind circular opening. Floor is measured above Origin, not a depth.
type EccentricBlindCylinderDescriptor struct {
	Origin                            geom.Vec3
	BoreOffset                        geom.Vec2 // nonzero exact XY displacement from Origin
	Radius, Height, BoreRadius, Floor float64
}

// EccentricBlindCylinderSolid retains the full source graph and the independently
// constructed native material. Source face order is outer wall, inner wall,
// opening annulus, bottom disk, floor disk.
type EccentricBlindCylinderSolid struct {
	Descriptor                   EccentricBlindCylinderDescriptor
	LengthUnit                   float64
	Native                       parametric.FiniteCylinderPairDifferenceResult
	SourceSolidID, SourceShellID int
	SourceFaceIDs                [5]int
	Parameters                   map[int][]float64
	Metadata                     map[int]CylinderDatumMetadata
}

func eccentricBlindCylinderFailure() error {
	return fmt.Errorf("iges: unsupported exact canonical eccentric blind cylinder")
}
func eccentricBlindCylinderAuthor(d EccentricBlindCylinderDescriptor) (topologyAuthor, error) {
	a := topologyAuthor{}
	if !d.Origin.IsFinite() || !finite(d.Radius) || !finite(d.Height) || !finite(d.BoreRadius) || !finite(d.Floor) || d.BoreRadius <= 0 || d.Radius <= d.BoreRadius || d.Floor <= 0 || d.Floor >= d.Height {
		return a, eccentricBlindCylinderFailure()
	}
	if !finite(d.BoreOffset.X) || !finite(d.BoreOffset.Y) || (d.BoreOffset.X == 0 && d.BoreOffset.Y == 0) {
		return a, eccentricBlindCylinderFailure()
	}
	gap := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.Radius), new(big.Rat).SetFloat64(d.BoreRadius))
	dx, dy := new(big.Rat).SetFloat64(d.BoreOffset.X), new(big.Rat).SetFloat64(d.BoreOffset.Y)
	if new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy)).Cmp(new(big.Rat).Mul(gap, gap)) >= 0 {
		return a, eccentricBlindCylinderFailure()
	}
	bx, e := cylinderExactSum(d.Origin.X, d.BoreOffset.X)
	if e != nil {
		return a, e
	}
	by, e := cylinderExactSum(d.Origin.Y, d.BoreOffset.Y)
	if e != nil {
		return a, e
	}
	top, e := cylinderExactSum(d.Origin.Z, d.Height)
	if e != nil {
		return a, e
	}
	floor, e := cylinderExactSum(d.Origin.Z, d.Floor)
	if e != nil {
		return a, e
	}
	outer, e := cylinderExactSum(d.Origin.X, d.Radius)
	if e != nil {
		return a, e
	}
	inner, e := cylinderExactSum(bx, d.BoreRadius)
	if e != nil {
		return a, e
	}
	depth, e := cylinderExactSum(d.Height, -d.Floor)
	if e != nil {
		return a, e
	}
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
	f := add(116, 0, bx, by, floor, 0)
	axis := add(123, 0, 0, 0, 1)
	ref := add(123, 0, 1, 0, 0)
	so := add(192, 1, float64(o), float64(axis), d.Radius, float64(ref))
	si := add(192, 1, float64(f), float64(axis), d.BoreRadius, float64(ref))
	pt := add(190, 1, float64(t), float64(axis), float64(ref))
	pb := add(190, 1, float64(o), float64(axis), float64(ref))
	pf := add(190, 1, float64(f), float64(axis), float64(ref))
	vertices := add(502, 1, 4, outer, y, top, outer, y, z, inner, by, top, inner, by, floor)
	curves := []int{add(100, 0, top, x, y, outer, y, outer, y), add(110, 0, outer, y, z, outer, y, top), add(100, 0, z, x, y, outer, y, outer, y), add(100, 0, top, bx, by, inner, by, inner, by), add(110, 0, inner, by, floor, inner, by, top), add(100, 0, floor, bx, by, inner, by, inner, by)}
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
	lf := loop(6, 1)
	fo := add(510, 1, float64(so), 1, 1, float64(lo))
	fi := add(510, 1, float64(si), 1, 1, float64(li))
	ft := add(510, 1, float64(pt), 2, 1, float64(lt), float64(lh))
	fb := add(510, 1, float64(pb), 1, 1, float64(lb))
	ff := add(510, 1, float64(pf), 1, 1, float64(lf))
	sh := add(514, 1, 5, float64(fo), 1, float64(fi), 0, float64(ft), 1, float64(fb), 0, float64(ff), 1)
	add(186, 0, float64(sh), 1, 0)
	a.geometry.Entities = []Entity{{Point: &geom.Vec3{}}}
	return a, nil
}

// ReadEccentricBlindCylinderSolid verifies every scalar, owner and orientation in
// a complete canonical analytic type186 graph. No source tolerance repair occurs.
func ReadEccentricBlindCylinderSolid(ctx context.Context, r io.Reader, l Limits) (EccentricBlindCylinderSolid, error) {
	fail := func() (EccentricBlindCylinderSolid, error) {
		return EccentricBlindCylinderSolid{}, eccentricBlindCylinderFailure()
	}
	if ctx == nil || r == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return EccentricBlindCylinderSolid{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 39)
	l.MaxRecords = min(l.MaxRecords, 1024)
	l.MaxValues = min(l.MaxValues, 4096)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, true)
	if e != nil {
		return EccentricBlindCylinderSolid{}, e
	}
	if len(doc.entities) != 39 {
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
	if len(values[1]) != 5 || len(values[3]) != 5 || len(values[5]) != 5 || len(values[11]) != 5 || len(values[13]) != 5 {
		return fail()
	}
	h, e := cylinderExactSum(values[3][3], -values[1][3])
	if e != nil {
		return fail()
	}
	f, e := cylinderExactSum(values[5][3], -values[1][3])
	if e != nil {
		return fail()
	}
	dx, e := cylinderExactSum(values[5][1], -values[1][1])
	if e != nil {
		return fail()
	}
	dy, e := cylinderExactSum(values[5][2], -values[1][2])
	if e != nil {
		return fail()
	}
	d := EccentricBlindCylinderDescriptor{BoreOffset: geom.Vec2{X: dx, Y: dy}, Origin: geom.Vec3{X: values[1][1], Y: values[1][2], Z: values[1][3]}, Radius: values[11][3], Height: h, BoreRadius: values[13][3], Floor: f}
	a, e := eccentricBlindCylinderAuthor(d)
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
	native, e := parametric.FiniteCylinderPairDifference(ctx, geom.RawCylinder{Origin: d.Origin, Axis: axis, Radius: d.Radius, Max: d.Height}, geom.RawCylinder{Origin: geom.Vec3{X: values[5][1], Y: values[5][2], Z: d.Origin.Z}, Axis: axis, Radius: d.BoreRadius, Min: d.Floor, Max: d.Height}, 1e-9)
	if e != nil {
		return EccentricBlindCylinderSolid{}, e
	}
	if e = ctx.Err(); e != nil {
		return EccentricBlindCylinderSolid{}, e
	}
	return EccentricBlindCylinderSolid{Descriptor: d, LengthUnit: doc.unit, Native: native, SourceSolidID: 77, SourceShellID: 75, SourceFaceIDs: [5]int{65, 67, 69, 71, 73}, Parameters: values, Metadata: meta}, nil
}

// WriteEccentricBlindCylinderSolid validates the staged source and native proof
// before publishing any bytes. It emits analytic circles, not approximate splines.
func WriteEccentricBlindCylinderSolid(ctx context.Context, w io.Writer, d EccentricBlindCylinderDescriptor, unit float64) error {
	if ctx == nil || w == nil {
		return eccentricBlindCylinderFailure()
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	a, e := eccentricBlindCylinderAuthor(d)
	if e != nil {
		return e
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	if _, e = ReadEccentricBlindCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
