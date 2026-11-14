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

// PlacedCircularCylinderSolid retains original local analytic material, its
// single authored proper signed-coordinate type124 placement, and the world
// cylinder. UV degree coordinates remain unchanged by placement.
type PlacedCircularCylinderSolid struct {
	Local                   CircularCylinderSolid
	Placement               geom.Affine
	WorldFrame              geom.Frame
	Solid                   parametric.Solid
	SourceTransformID       int
	SourceVertices          [2]geom.Vec3
	SourceTransformMetadata CylinderDatumMetadata
}

func circularCylinderLeaf(id int) bool {
	return id == 9 || id == 11 || id == 13 || id == 17 || id == 19 || id == 21
}
func placedCircularSource(r io.Reader, l Limits) (topologySource, error) {
	l, e := l.defaults()
	if e != nil {
		return topologySource{}, e
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 25)
	l.MaxValues = min(l.MaxValues, 2048)
	return readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true)
}

// ReadPlacedCircularCylinderSolid admits exactly one type124 shared by all six
// analytic/model leaves. Source vertices are world coordinates, and topology,
// support datums and owner pcurves cannot carry independent placements.
func ReadPlacedCircularCylinderSolid(ctx context.Context, r io.Reader, l Limits) (PlacedCircularCylinderSolid, error) {
	fail := func() (PlacedCircularCylinderSolid, error) {
		return PlacedCircularCylinderSolid{}, circularCylinderFailure()
	}
	if ctx == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return PlacedCircularCylinderSolid{}, e
	}
	doc, e := placedCircularSource(r, l)
	if e != nil {
		return PlacedCircularCylinderSolid{}, e
	}
	if len(doc.entities) != 25 {
		return fail()
	}
	if e = mixedTopologyDirectory(doc.bytes); e != nil {
		return fail()
	}
	tr := doc.entities[24]
	if tr.kind != 124 || tr.id != 49 || tr.transform != 0 || len(tr.values) != 13 {
		return fail()
	}
	kind, e := intValue(tr.values[0])
	if e != nil || kind != 124 {
		return fail()
	}
	if e = compositeSourceNumbers(110, tr.values); e != nil {
		return fail()
	}
	m, e := decodeMatrix(tr.values)
	if e != nil || !roofRigid(m) {
		return fail()
	}
	var sourceVertices [2]geom.Vec3
	for i, rec := range doc.entities[:24] {
		want := 0
		if circularCylinderLeaf(rec.id) {
			want = 49
		}
		if rec.transform != want {
			return fail()
		}
		doc.entities[i].transform = 0
		if rec.id == 15 {
			if rec.kind != 502 || len(rec.values) != 8 {
				return fail()
			}
			if e = compositeSourceNumbers(110, rec.values); e != nil {
				return fail()
			}
			v := append([]parameter(nil), rec.values...)
			for j := 0; j < 2; j++ {
				var p [3]float64
				for k := range p {
					p[k], e = realValue(v[2+3*j+k])
					if e != nil {
						return fail()
					}
				}
				sourceVertices[j] = geom.Vec3{X: p[0], Y: p[1], Z: p[2]}
				q, e := roofMapPoint(m, sourceVertices[j], true)
				if e != nil {
					return fail()
				}
				for k, x := range []float64{q.X, q.Y, q.Z} {
					v[2+3*j+k].text = number(x)
				}
			}
			doc.entities[i].values = v
		}
	}
	doc.entities = doc.entities[:24]
	raw, e := mixedRoofFile(doc)
	if e != nil {
		return fail()
	}
	local, e := ReadCircularCylinderSolid(ctx, bytes.NewReader(raw), l)
	if e != nil {
		return PlacedCircularCylinderSolid{}, e
	}
	origin, e := roofMapPoint(m, local.Descriptor.Origin, false)
	if e != nil {
		return fail()
	}
	frame, e := geom.NewFrame(origin, m.ApplyVector(geom.Vec3{X: 1}), m.ApplyVector(geom.Vec3{Z: 1}))
	if e != nil {
		return fail()
	}
	solid, e := parametric.Cylinder(frame, local.Descriptor.Radius, local.Descriptor.Height)
	if e != nil {
		return fail()
	}
	if e = ctx.Err(); e != nil {
		return PlacedCircularCylinderSolid{}, e
	}
	return PlacedCircularCylinderSolid{Local: local, Placement: m, WorldFrame: frame, Solid: solid, SourceTransformID: 49, SourceVertices: sourceVertices, SourceTransformMetadata: CylinderDatumMetadata{Label: tr.label, Subscript: tr.subscript}}, nil
}

// WritePlacedCircularCylinderSolid preserves all local analytic supports and
// uses one proper signed-coordinate leaf placement. World vertex coordinates
// must be exactly representable. Typed validation completes before output.
func WritePlacedCircularCylinderSolid(ctx context.Context, w io.Writer, d CircularCylinderDescriptor, m geom.Affine, unit float64) error {
	if ctx == nil || w == nil || !roofRigid(m) {
		return circularCylinderFailure()
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	a, e := circularCylinderAuthor(d)
	if e != nil {
		return e
	}
	budget := 2048
	v, e := parseParameters(a.records[7].data, ',', ';', &budget)
	if e != nil {
		return e
	}
	for j := 0; j < 2; j++ {
		var p [3]float64
		for k := range p {
			p[k], e = realValue(v[2+3*j+k])
			if e != nil {
				return e
			}
		}
		q, e := roofMapPoint(m, geom.Vec3{X: p[0], Y: p[1], Z: p[2]}, false)
		if e != nil {
			return e
		}
		for k, x := range []float64{q.X, q.Y, q.Z} {
			v[2+3*j+k].text = number(x)
		}
	}
	values := make([]string, len(v))
	for i, x := range v {
		values[i] = x.text
	}
	a.records[7].data = strings.Join(values, ",") + ";"
	coeff := []string{"124"}
	for _, row := range m {
		for _, x := range row {
			coeff = append(coeff, number(x))
		}
	}
	if a.add(124, 0, strings.Join(coeff, ",")+";") != 49 {
		return circularCylinderFailure()
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	lines := bytes.Split(raw, []byte("\n"))
	for _, line := range lines {
		if len(line) != 80 || line[72] != 'D' {
			continue
		}
		id, e := field(string(line[73:]))
		if e != nil {
			return e
		}
		if circularCylinderLeaf(id) {
			copy(line[48:56], fmt.Sprintf("%8d", 49))
		}
	}
	raw = bytes.Join(lines, []byte("\n"))
	if _, e = ReadPlacedCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
