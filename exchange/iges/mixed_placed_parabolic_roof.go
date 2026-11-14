package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/geom"
	"strings"
)

// MixedPlacedParabolicRoofSolid retains each actual source leaf and its authored
// placement separately from the normalized local proof and world material.
// OriginalGeometry contains model curves and supports, never reconstructed fits.
type MixedPlacedParabolicRoofSolid struct {
	PlacedParabolicRoofSolid
	OriginalGeometry     map[int]Entity
	SourceLeafTransforms map[int]int
}

// RoofLeafBake selects canonical geometric roles to emit directly in world
// coordinates. At least one leaf must be baked and one must retain type124.
type RoofLeafBake struct {
	Surfaces    [6]bool
	ModelCurves [12]bool
}

func mixedRoofSource(r io.Reader, l Limits) (topologySource, error) {
	l, e := l.defaults()
	if e != nil {
		return topologySource{}, e
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxRecords = min(l.MaxRecords, 1024)
	l.MaxEntities = min(l.MaxEntities, 59)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxControlPoints = min(l.MaxControlPoints, 104)
	if l.MaxControlPoints < 104 {
		return topologySource{}, roofFailure()
	}
	d, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true)
	if e != nil {
		return d, e
	}
	if len(d.entities) != 59 {
		return topologySource{}, roofFailure()
	}
	return d, nil
}
func mixedRoofMatrix(d topologySource) (geom.Affine, error) {
	r := d.entities[58]
	if r.id != 117 || r.kind != 124 || r.transform != 0 || len(r.values) != 13 {
		return geom.Affine{}, roofFailure()
	}
	if e := compositeSourceNumbers(110, r.values); e != nil {
		return geom.Affine{}, e
	}
	m, e := decodeMatrix(r.values)
	if e != nil || !roofRigid(m) {
		return geom.Affine{}, roofFailure()
	}
	return m, nil
}
func mixedRoofLeafIDs(d topologySource) (map[int]bool, error) {
	ids := map[int]bool{}
	lists := 0
	for _, r := range d.entities {
		if r.kind == 128 {
			ids[r.id] = true
		}
		if r.kind != 504 {
			continue
		}
		lists++
		v, e := topologyInts(r, 62)
		if e != nil || v[1] != 12 {
			return nil, roofFailure()
		}
		for j := 0; j < 12; j++ {
			if ids[v[2+5*j]] {
				return nil, roofFailure()
			}
			ids[v[2+5*j]] = true
		}
	}
	if lists != 1 || len(ids) != 18 {
		return nil, roofFailure()
	}
	return ids, nil
}
func mixedRoofMapRecord(r topologyRecord, m geom.Affine, inverse bool) (topologyRecord, Entity, error) {
	fail := func() (topologyRecord, Entity, error) { return topologyRecord{}, Entity{}, roofFailure() }
	if e := compositeSourceNumbers(110, r.values); e != nil {
		return fail()
	}
	en, e := decode(r.kind, r.values, 104)
	if e != nil || en.sourceWeightsRounded {
		return fail()
	}
	if e = en.validate(); e != nil {
		return fail()
	}
	count, start := 0, 0
	if r.kind == 128 && en.Surface != nil {
		d := en.Surface.Geometry.Data()
		count = len(d.Poles)
		start = 10 + len(d.KnotsU) + len(d.KnotsV) + count
	} else if r.kind == 126 && en.Spline != nil {
		c := en.Spline.Curve
		count = len(c.Poles())
		start = 7 + len(c.Knots()) + count
	} else {
		return fail()
	}
	if count > 104 || start+3*count > len(r.values) {
		return fail()
	}
	r.values = append([]parameter(nil), r.values...)
	for i := 0; i < count; i++ {
		var p [3]float64
		for j := range p {
			p[j], e = realValue(r.values[start+3*i+j])
			if e != nil {
				return fail()
			}
		}
		q, e := roofMapPoint(m, geom.Vec3{X: p[0], Y: p[1], Z: p[2]}, inverse)
		if e != nil {
			return fail()
		}
		for j, x := range []float64{q.X, q.Y, q.Z} {
			r.values[start+3*i+j].text = number(x)
		}
	}
	return r, en, nil
}

// Rebuild only parameter storage. Preserve every original directory field other
// than relocated P offsets/counts and the explicitly supplied transform pointer.
// In particular, appearance/status/form metadata cannot disappear in staging.
func mixedRoofFile(d topologySource) ([]byte, error) {
	a := topologyAuthor{}
	a.geometry.Entities = []Entity{{Point: &geom.Vec3{}}}
	for _, r := range d.entities {
		v := make([]string, len(r.values))
		for i, p := range r.values {
			if p.stringValue {
				return nil, roofFailure()
			}
			v[i] = p.text
		}
		form := 0
		if r.kind >= 500 {
			form = 1
		}
		a.add(r.kind, form, strings.Join(v, ",")+";")
	}
	b, e := a.file(d.unit)
	if e != nil {
		return nil, e
	}
	old := map[int][]byte{}
	for _, line := range bytes.Split(d.bytes, []byte("\n")) {
		if len(line) == 80 && line[72] == 'D' {
			id, e := field(string(line[73:]))
			if e != nil {
				return nil, e
			}
			old[id] = line
		}
	}
	lines := bytes.Split(b, []byte("\n"))
	for _, line := range lines {
		if len(line) != 80 || line[72] != 'D' {
			continue
		}
		id, e := field(string(line[73:]))
		if e != nil || len(old[id]) != 80 {
			return nil, roofFailure()
		}
		save := append([]byte(nil), line...)
		copy(line[:72], old[id][:72])
		if id%2 == 1 {
			copy(line[8:16], save[8:16])
			copy(line[48:56], fmt.Sprintf("%8d", d.entities[(id-1)/2].transform))
		} else {
			copy(line[24:32], save[24:32])
		}
	}
	return bytes.Join(lines, []byte("\n")), nil
}

// ReadMixedPlacedParabolicRoofSolid accepts the independently qualified mixture
// of exact baked world leaves and local leaves sharing one proper signed pose.
// It preserves source coefficients and does not widen the uniform placed reader.
func ReadMixedPlacedParabolicRoofSolid(ctx context.Context, r io.Reader, l Limits) (MixedPlacedParabolicRoofSolid, error) {
	fail := func() (MixedPlacedParabolicRoofSolid, error) { return MixedPlacedParabolicRoofSolid{}, roofFailure() }
	if ctx == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return MixedPlacedParabolicRoofSolid{}, e
	}
	d, e := mixedRoofSource(r, l)
	if e != nil {
		return MixedPlacedParabolicRoofSolid{}, e
	}
	m, e := mixedRoofMatrix(d)
	if e != nil {
		return fail()
	}
	ids, e := mixedRoofLeafIDs(d)
	if e != nil {
		return fail()
	}
	original := map[int]Entity{}
	pointers := map[int]int{}
	baked, placed := 0, 0
	for i, rec := range d.entities {
		if !ids[rec.id] {
			continue
		}
		if rec.transform != 0 && rec.transform != 117 {
			return fail()
		}
		mapped, en, e := mixedRoofMapRecord(rec, m, rec.transform == 0)
		if e != nil {
			return fail()
		}
		original[rec.id] = en
		pointers[rec.id] = rec.transform
		if rec.transform == 0 {
			baked++
			mapped.transform = 117
			d.entities[i] = mapped
		} else {
			placed++
		}
	}
	if baked == 0 || placed == 0 {
		return fail()
	}
	raw, e := mixedRoofFile(d)
	if e != nil {
		return fail()
	}
	proof, e := ReadPlacedParabolicRoofSolid(ctx, bytes.NewReader(raw), l)
	if e != nil {
		return MixedPlacedParabolicRoofSolid{}, e
	}
	return MixedPlacedParabolicRoofSolid{PlacedParabolicRoofSolid: proof, OriginalGeometry: original, SourceLeafTransforms: pointers}, nil
}

// WriteMixedPlacedParabolicRoofSolid bakes exactly the selected local leaf
// control coefficients into world coordinates; all topology/UV roles stay intact.
func WriteMixedPlacedParabolicRoofSolid(ctx context.Context, w io.Writer, d ParabolicRoofDescriptor, m geom.Affine, unit float64, bake RoofLeafBake) error {
	if w == nil {
		return roofFailure()
	}
	var b bytes.Buffer
	if e := WritePlacedParabolicRoofSolid(ctx, &b, d, m, unit); e != nil {
		return e
	}
	proof, e := ReadPlacedParabolicRoofSolid(ctx, bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		return e
	}
	doc, e := mixedRoofSource(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		return e
	}
	selected := map[int]bool{}
	for i, x := range bake.Surfaces {
		if x {
			selected[proof.Local.SourceSurfaceIDs[i]] = true
		}
	}
	for i, x := range bake.ModelCurves {
		if x {
			selected[proof.Local.SourceModelCurveIDs[i]] = true
		}
	}
	if len(selected) == 0 || len(selected) == 18 {
		return roofFailure()
	}
	for i, r := range doc.entities {
		if !selected[r.id] {
			continue
		}
		v, _, e := mixedRoofMapRecord(r, m, false)
		if e != nil {
			return e
		}
		v.transform = 0
		doc.entities[i] = v
	}
	raw, e := mixedRoofFile(doc)
	if e != nil {
		return e
	}
	if _, e = ReadMixedPlacedParabolicRoofSolid(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
