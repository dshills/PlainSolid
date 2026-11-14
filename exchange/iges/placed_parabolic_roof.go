package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
)

// PlacedParabolicRoofSolid retains the original local geometry and topology
// ancestry separately from its world material. Only one shared proper signed
// coordinate type124 placement on model curves and support surfaces is admitted.
// Source vertices are world coordinates; UV curves have no placement.
type PlacedParabolicRoofSolid struct {
	Local                   MappedParabolicRoofSolid
	Placement               geom.Affine
	SourceTransformID       int
	SourceTransformMetadata CylinderDatumMetadata
	Solid                   trimmed.Solid
	Embedding               trimmed.SplineRectangleEmbedding
}

func roofRigid(m geom.Affine) bool {
	if !m.IsFinite() || m.Determinant() != 1 {
		return false
	}
	var cols [3]int
	for i := 0; i < 3; i++ {
		n := 0
		for j := 0; j < 3; j++ {
			x := m[i][j]
			if x != 0 {
				if x != 1 && x != -1 {
					return false
				}
				n++
				cols[j]++
			}
		}
		if n != 1 {
			return false
		}
	}
	return cols == [3]int{1, 1, 1}
}
func roofMapPoint(m geom.Affine, p geom.Vec3, inverse bool) (geom.Vec3, error) {
	a := [3]float64{p.X, p.Y, p.Z}
	var out [3]float64
	for i := 0; i < 3; i++ {
		q := new(big.Rat)
		if !inverse {
			q.SetFloat64(m[i][3])
		}
		for j := 0; j < 3; j++ {
			c := m[i][j]
			if inverse {
				c = m[j][i]
			}
			if c == 0 {
				continue
			}
			v := new(big.Rat).SetFloat64(a[j])
			if inverse {
				v.Sub(v, new(big.Rat).SetFloat64(m[j][3]))
			}
			q.Add(q, v.Mul(v, new(big.Rat).SetFloat64(c)))
		}
		var exact bool
		out[i], exact = q.Float64()
		if !exact || !finite(out[i]) {
			return geom.Vec3{}, roofFailure()
		}
	}
	return geom.Vec3{X: out[0], Y: out[1], Z: out[2]}, nil
}
func roofPlaceMaterial(ctx context.Context, local MappedParabolicRoofSolid, m geom.Affine) (trimmed.Solid, trimmed.SplineRectangleEmbedding, error) {
	fail := func(e error) (trimmed.Solid, trimmed.SplineRectangleEmbedding, error) {
		return trimmed.Solid{}, trimmed.SplineRectangleEmbedding{}, e
	}
	surfaces := make([]*geom.BSplineSurface, 6)
	for i, id := range local.SourceSurfaceIDs {
		d := local.Geometry[id].Surface.Geometry.Data()
		for j, p := range d.Poles {
			q, e := roofMapPoint(m, p, false)
			if e != nil {
				return fail(e)
			}
			d.Poles[j] = q
		}
		s, e := geom.NewBSplineSurfaceWithExactWeights(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			return fail(e)
		}
		surfaces[i] = s
	}
	s, _, e := trimmed.SewSplineRectangles(ctx, surfaces, make([]geom.ParameterRectangle, 6), make([]bool, 6), trimmed.SewOptions{Tolerance: 1e-10, MaxFaces: 6, MaxUses: 24})
	if e != nil {
		return fail(e)
	}
	proof, e := s.ValidateSplineRectangleEmbedding(ctx, trimmed.SplineRectangleEmbeddingOptions{MaxFaces: 6})
	if e != nil {
		return fail(e)
	}
	if !proof.Complete {
		return fail(roofFailure())
	}
	return s, proof, nil
}

// ReadPlacedParabolicRoofSolid proves both local and world material boundaries.
// It rejects root/topology/UV transforms, independently placed leaves, reflections,
// noncardinal rotations, and any required rounding of source coefficients.
func ReadPlacedParabolicRoofSolid(ctx context.Context, r io.Reader, l Limits) (PlacedParabolicRoofSolid, error) {
	fail := func() (PlacedParabolicRoofSolid, error) {
		return PlacedParabolicRoofSolid{}, roofFailure()
	}
	if ctx == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return PlacedParabolicRoofSolid{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxRecords = min(l.MaxRecords, 1024)
	l.MaxEntities = min(l.MaxEntities, 59)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxControlPoints = min(l.MaxControlPoints, 104)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true)
	if e != nil {
		return PlacedParabolicRoofSolid{}, e
	}
	if len(doc.entities) != 59 || l.MaxControlPoints < 104 {
		return fail()
	}
	// Keep every otherwise unsupported directory property visible to the same
	// strict metadata gate before constructing a temporary unplaced graph.
	for _, line := range bytes.Split(doc.bytes, []byte("\n")) {
		if len(line) != 80 || line[72] != 'D' {
			continue
		}
		seq, e := field(string(line[73:]))
		if e != nil {
			return fail()
		}
		indexes := []int{1, 2}
		if seq%2 == 1 {
			indexes = []int{3, 4}
			kind, e := field(string(line[:8]))
			if e != nil {
				return fail()
			}
			status, e := field(string(line[64:72]))
			want := 10000
			if kind == 186 {
				want = 0
			}
			if kind == 504 {
				want = 10001
			}
			if e != nil || status != want {
				return fail()
			}
		}
		for _, i := range indexes {
			v, e := field(string(line[i*8 : i*8+8]))
			if e != nil || v != 0 {
				return fail()
			}
		}
	}
	tr := doc.entities[58]
	if tr.kind != 124 || tr.id != 117 || tr.transform != 0 || len(tr.values) != 13 {
		return fail()
	}
	if e = compositeSourceNumbers(110, tr.values); e != nil {
		return fail()
	}
	kind, err := intValue(tr.values[0])
	if err != nil || kind != 124 {
		return fail()
	}
	var m geom.Affine
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			m[i][j], e = realValue(tr.values[1+4*i+j])
			if e != nil {
				return fail()
			}
		}
	}
	if !roofRigid(m) {
		return fail()
	}
	a := topologyAuthor{}
	a.geometry.Entities = []Entity{{Point: &geom.Vec3{}}}
	for _, rec := range doc.entities[:58] {
		if rec.kind == 124 {
			return fail()
		}
		if e = compositeSourceNumbers(110, rec.values); e != nil {
			return fail()
		}
		vals := append([]parameter(nil), rec.values...)
		if rec.kind == 502 {
			if len(vals) != 26 {
				return fail()
			}
			n, e := intValue(rec.values[1])
			if e != nil || n != 8 || len(vals) != 26 {
				return fail()
			}
			for j := 0; j < 8; j++ {
				var p [3]float64
				for k := range p {
					p[k], e = realValue(vals[2+3*j+k])
					if e != nil {
						return fail()
					}
				}
				q, e := roofMapPoint(m, geom.Vec3{X: p[0], Y: p[1], Z: p[2]}, true)
				if e != nil {
					return fail()
				}
				for k, x := range []float64{q.X, q.Y, q.Z} {
					vals[2+3*j+k].text = number(x)
				}
			}
		}
		texts := make([]string, len(vals))
		for i, v := range vals {
			if v.stringValue {
				return fail()
			}
			texts[i] = v.text
		}
		form := 0
		if rec.kind >= 500 {
			form = 1
		}
		a.records = append(a.records, nativeTopologyRecord{kind: rec.kind, form: form, label: rec.label, subscript: rec.subscript, data: strings.Join(texts, ",") + ";"})
	}
	raw, e := a.file(doc.unit)
	if e != nil {
		return fail()
	}
	local, e := ReadMappedParabolicRoofSolid(ctx, bytes.NewReader(raw), l)
	if e != nil {
		return PlacedParabolicRoofSolid{}, e
	}
	placed := map[int]bool{}
	for _, id := range local.SourceSurfaceIDs {
		placed[id] = true
	}
	for _, id := range local.SourceModelCurveIDs {
		placed[id] = true
	}
	for _, rec := range doc.entities[:58] {
		want := 0
		if placed[rec.id] {
			want = 117
		}
		if rec.transform != want {
			return fail()
		}
	}
	s, proof, e := roofPlaceMaterial(ctx, local, m)
	if e != nil {
		return PlacedParabolicRoofSolid{}, e
	}
	return PlacedParabolicRoofSolid{Local: local, Placement: m, SourceTransformID: 117, SourceTransformMetadata: CylinderDatumMetadata{Label: tr.label, Subscript: tr.subscript}, Solid: s, Embedding: proof}, nil
}

// WritePlacedParabolicRoofSolid emits local original spline charts, world-space
// topology vertices and a shared leaf type124 placement. Validation is staged
// before writing. Only exact proper signed-coordinate poses are supported.
func WritePlacedParabolicRoofSolid(ctx context.Context, w io.Writer, d ParabolicRoofDescriptor, m geom.Affine, unit float64) error {
	if ctx == nil || w == nil || !roofRigid(m) {
		return roofFailure()
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	a, roles, e := roofAuthor(d)
	if e != nil {
		return e
	}
	// The vertex record is generated from the descriptor, not from rounded
	// transformed charts. Every addition is checked in rational arithmetic.
	rec := &a.records[(roles.vertices-1)/2]
	budget := 4096
	vals, e := parseParameters(rec.data, ',', ';', &budget)
	if e != nil {
		return e
	}
	for j := 0; j < 8; j++ {
		var p [3]float64
		for k := range p {
			p[k], e = realValue(vals[2+3*j+k])
			if e != nil {
				return e
			}
		}
		q, e := roofMapPoint(m, geom.Vec3{X: p[0], Y: p[1], Z: p[2]}, false)
		if e != nil {
			return e
		}
		for k, x := range []float64{q.X, q.Y, q.Z} {
			vals[2+3*j+k].text = number(x)
		}
	}
	texts := make([]string, len(vals))
	for i, v := range vals {
		texts[i] = v.text
	}
	rec.data = strings.Join(texts, ",") + ";"
	coeff := []string{"124"}
	for _, row := range m {
		for _, x := range row {
			coeff = append(coeff, number(x))
		}
	}
	tid := a.add(124, 0, strings.Join(coeff, ",")+";")
	if tid != 117 {
		return roofFailure()
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	placed := map[int]bool{}
	for _, id := range roles.surfaces {
		placed[id] = true
	}
	for _, id := range roles.models {
		placed[id] = true
	}
	lines := bytes.Split(raw, []byte("\n"))
	for _, line := range lines {
		if len(line) == 80 && line[72] == 'D' {
			id, e := field(string(line[73:]))
			if e != nil {
				return e
			}
			if placed[id] {
				copy(line[48:56], fmt.Sprintf("%8d", tid))
			}
		}
	}
	raw = bytes.Join(lines, []byte("\n"))
	if _, e = ReadPlacedParabolicRoofSolid(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
