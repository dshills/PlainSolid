package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/ellipticprism"
	"reflect"
)

// PlacedConical45CylinderThroughMaterial retains one proper signed-coordinate source placement.
// Original leaf records and world-space vertices are authenticated separately.
type PlacedConical45CylinderThroughMaterial struct {
	Local                   Conical45CylinderThroughSolid
	Placement               geom.Affine
	SourceTransformID       int
	SourceTransformMetadata CylinderDatumMetadata
	SourceVertices          []geom.Vec3
	Solid                   trimmed.Solid
	NativeRimEdges          [2][2][4]int
	SourceRimCurveIDs       [2][2]int // outer/inner, lower/upper original full circles
	source                  []byte
}

func ReadPlacedConical45CylinderThroughMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (PlacedConical45CylinderThroughMaterial, error) {
	bad := func() (PlacedConical45CylinderThroughMaterial, error) {
		return PlacedConical45CylinderThroughMaterial{}, geom.ErrInvalid
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return PlacedConical45CylinderThroughMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 29)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true)
	if e != nil || len(doc.entities) != 29 || mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	source := append([]byte(nil), doc.bytes...)
	tr := doc.entities[28]
	if tr.kind != 124 || tr.id != 57 || tr.transform != 0 || len(tr.values) != 13 || compositeSourceNumbers(124, tr.values) != nil {
		return bad()
	}
	pose, e := decodeMatrix(tr.values)
	if e != nil || !roofRigid(pose) {
		return bad()
	}
	out := PlacedConical45CylinderThroughMaterial{Placement: pose, SourceTransformID: 57, SourceTransformMetadata: CylinderDatumMetadata{tr.label, tr.subscript}, source: source}
	for i, rec := range doc.entities[:28] {
		want := 0
		if conicalThroughPlacedLeaf(rec.kind) {
			want = 57
		}
		if rec.transform != want {
			return bad()
		}
		doc.entities[i].transform = 0
		if rec.kind != 502 {
			continue
		}
		if len(rec.values) != 14 || compositeSourceNumbers(110, rec.values) != nil {
			return bad()
		}
		v := append([]parameter(nil), rec.values...)
		for j := 0; j < 4; j++ {
			p := [3]float64{}
			for k := 0; k < 3; k++ {
				p[k], e = realValue(v[2+3*j+k])
				if e != nil {
					return bad()
				}
			}
			world := geom.Vec3{X: p[0], Y: p[1], Z: p[2]}
			local, e := roofMapPoint(pose, world, true)
			if e != nil {
				return bad()
			}
			out.SourceVertices = append(out.SourceVertices, world)
			for k, x := range []float64{local.X, local.Y, local.Z} {
				v[2+3*j+k].text = number(x)
			}
		}
		doc.entities[i].values = v
	}
	doc.entities = doc.entities[:28]
	raw, e := mixedRoofFile(doc)
	if e != nil {
		return bad()
	}
	out.Local, e = ReadConical45CylinderThroughSolid(ctx, bytes.NewReader(raw), l)
	if e != nil {
		return bad()
	}
	d := out.Local.Descriptor
	origin, e := roofMapPoint(pose, d.Origin, false)
	if e != nil {
		return bad()
	}
	frame, e := geom.NewFrame(origin, pose.ApplyVector(geom.Vec3{X: 1}), pose.ApplyVector(geom.Vec3{Z: 1}))
	if e != nil {
		return bad()
	}
	top, e := cylinderExactSum(d.Radius, d.Height)
	if e != nil {
		return bad()
	}
	out.Solid, out.NativeRimEdges, e = ellipticprism.CircularFrustumThrough(ctx, frame, d.Radius, top, d.BoreRadius, d.Height, tol)
	if e != nil {
		return bad()
	}
	out.SourceRimCurveIDs = [2][2]int{{23, 19}, {29, 25}}
	return out, nil
}
func WritePlacedConical45CylinderThrough(ctx context.Context, w io.Writer, d Conical45CylinderThroughDescriptor, pose geom.Affine, unit, tol float64) error {
	a, e := conical45CylinderThroughAuthor(d)
	if e != nil {
		return e
	}
	return writePlacedConicLeaves(ctx, w, a, pose, unit, tol, conicalThroughPlacedLeaf, func(raw []byte) error {
		_, e := ReadPlacedConical45CylinderThroughMaterial(ctx, bytes.NewReader(raw), Limits{}, tol)
		return e
	})
}

type placedConical45CylinderThroughEnvelope struct {
	Format            string
	Version           int
	Source            string
	Tolerance         float64
	Local, Native     json.RawMessage
	Placement         geom.Affine
	TransformID       int
	TransformMetadata CylinderDatumMetadata
	Vertices          []geom.Vec3
	Rims              [2][2][4]int
	SourceRims        [2][2]int
}

func placedConical45CylinderThroughSnapshot(ctx context.Context, m PlacedConical45CylinderThroughMaterial) (placedConical45CylinderThroughEnvelope, error) {
	var local bytes.Buffer
	if e := WriteConical45CylinderThroughJSON(ctx, &local, m.Local); e != nil {
		return placedConical45CylinderThroughEnvelope{}, e
	}
	native, e := igesPrismBytes(m.Solid)
	if e != nil {
		return placedConical45CylinderThroughEnvelope{}, e
	}
	var l, n bytes.Buffer
	if json.Compact(&l, local.Bytes()) != nil || json.Compact(&n, native) != nil {
		return placedConical45CylinderThroughEnvelope{}, geom.ErrInvalid
	}
	return placedConical45CylinderThroughEnvelope{"plainsolid-iges-placed-conical45-cylinder-through-material", 1, string(m.source), m.Solid.Tolerance(), l.Bytes(), n.Bytes(), m.Placement, m.SourceTransformID, m.SourceTransformMetadata, m.SourceVertices, m.NativeRimEdges, m.SourceRimCurveIDs}, nil
}
func WritePlacedConical45CylinderThroughMaterialJSON(ctx context.Context, w io.Writer, m PlacedConical45CylinderThroughMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadPlacedConical45CylinderThroughMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid.Tolerance())
	if e != nil {
		return e
	}
	a, e := placedConical45CylinderThroughSnapshot(ctx, m)
	if e != nil {
		return e
	}
	b, e := placedConical45CylinderThroughSnapshot(ctx, p)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(a, b) {
		return geom.ErrInvalid
	}
	raw, e := json.Marshal(b)
	if e != nil {
		return e
	}
	if len(raw) > 1<<20 {
		return geom.ErrConvergence
	}
	raw = append(raw, '\n')
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
func ReadPlacedConical45CylinderThroughMaterialJSON(ctx context.Context, r io.Reader, l Limits) (PlacedConical45CylinderThroughMaterial, error) {
	bad := func() (PlacedConical45CylinderThroughMaterial, error) {
		return PlacedConical45CylinderThroughMaterial{}, geom.ErrInvalid
	}
	if ctx == nil || r == nil {
		return bad()
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	limit := min(l.MaxBytes, int64(1<<20))
	raw, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil || int64(len(raw)) > limit {
		return bad()
	}
	var q placedConical45CylinderThroughEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-placed-conical45-cylinder-through-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadPlacedConical45CylinderThroughMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	want, e := placedConical45CylinderThroughSnapshot(ctx, p)
	if e != nil {
		return bad()
	}
	var a, b bytes.Buffer
	if json.Compact(&a, q.Local) != nil || json.Compact(&b, q.Native) != nil {
		return bad()
	}
	q.Local, q.Native = a.Bytes(), b.Bytes()
	if !reflect.DeepEqual(q, want) {
		return bad()
	}
	return p, nil
}
func WritePlacedConical45CylinderThroughSource(ctx context.Context, w io.Writer, m PlacedConical45CylinderThroughMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WritePlacedConical45CylinderThroughMaterialJSON(ctx, io.Discard, m); e != nil {
		return e
	}
	n, e := w.Write(m.source)
	if e == nil && n != len(m.source) {
		e = io.ErrShortWrite
	}
	return e
}

func conicalThroughPlacedLeaf(kind int) bool {
	return kind == 100 || kind == 110 || kind == 190 || kind == 192 || kind == 194
}
