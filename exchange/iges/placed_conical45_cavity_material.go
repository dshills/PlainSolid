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

type PlacedConical45CavityMaterial struct {
	Local                   Conical45CavityMaterial
	Placement               geom.Affine
	SourceTransformID       int
	SourceTransformMetadata CylinderDatumMetadata
	SourceVertices          []geom.Vec3
	Solid                   trimmed.Solid
	NativeRimEdges          [2][2][4]int
	source                  []byte
}

func ReadPlacedConical45CavityMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (PlacedConical45CavityMaterial, error) {
	bad := func() (PlacedConical45CavityMaterial, error) {
		return PlacedConical45CavityMaterial{}, geom.ErrInvalid
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return PlacedConical45CavityMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 40)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true)
	if e != nil || len(doc.entities) != 40 || mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	source := append([]byte(nil), doc.bytes...)
	tr := doc.entities[39]
	if tr.kind != 124 || tr.id != 79 || tr.transform != 0 || len(tr.values) != 13 || compositeSourceNumbers(124, tr.values) != nil {
		return bad()
	}
	pose, e := decodeMatrix(tr.values)
	if e != nil || !roofRigid(pose) {
		return bad()
	}
	out := PlacedConical45CavityMaterial{Placement: pose, SourceTransformID: 79, SourceTransformMetadata: CylinderDatumMetadata{tr.label, tr.subscript}, source: source}
	for i, rec := range doc.entities[:39] {
		want := 0
		if conical45PlacedLeaf(rec.kind) {
			want = 79
		}
		if rec.transform != want {
			return bad()
		}
		doc.entities[i].transform = 0
		if rec.kind != 502 {
			continue
		}
		if len(rec.values) != 8 || compositeSourceNumbers(110, rec.values) != nil {
			return bad()
		}
		v := append([]parameter(nil), rec.values...)
		for j := 0; j < 2; j++ {
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
	doc.entities = doc.entities[:39]
	raw, e := mixedRoofFile(doc)
	if e != nil {
		return bad()
	}
	out.Local, e = ReadConical45CavityMaterial(ctx, bytes.NewReader(raw), l, tol)
	if e != nil {
		return bad()
	}
	var solids [2]trimmed.Solid
	var rims [2][2][4]int
	for i, d := range []Conical45FrustumDescriptor{out.Local.Descriptor.Outer, out.Local.Descriptor.Inner} {
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
		solids[i], rims[i], e = ellipticprism.CircularFrustumFrame(ctx, frame, d.Radius, top, d.Height, tol)
		if e != nil {
			return PlacedConical45CavityMaterial{}, e
		}
	}
	out.Solid, out.NativeRimEdges, e = joinConical45Cavity(solids, rims, tol)
	if e != nil {
		return PlacedConical45CavityMaterial{}, e
	}
	return out, nil
}
func WritePlacedConical45Cavity(ctx context.Context, w io.Writer, d Conical45CavityMaterialDescriptor, pose geom.Affine, unit, tol float64) error {
	a, e := conical45CavityMaterialAuthor(d)
	if e != nil {
		return e
	}
	return writePlacedConicLeaves(ctx, w, a, pose, unit, tol, conical45PlacedLeaf, func(raw []byte) error {
		_, e := ReadPlacedConical45CavityMaterial(ctx, bytes.NewReader(raw), Limits{}, tol)
		return e
	})
}

type placedConical45CavityEnvelope struct {
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
}

func placedConical45CavitySnapshot(ctx context.Context, m PlacedConical45CavityMaterial) (placedConical45CavityEnvelope, error) {
	var local bytes.Buffer
	if e := WriteConical45CavityMaterialJSON(ctx, &local, m.Local); e != nil {
		return placedConical45CavityEnvelope{}, e
	}
	native, e := igesPrismBytes(m.Solid)
	if e != nil {
		return placedConical45CavityEnvelope{}, e
	}
	var l, n bytes.Buffer
	if json.Compact(&l, local.Bytes()) != nil || json.Compact(&n, native) != nil {
		return placedConical45CavityEnvelope{}, geom.ErrInvalid
	}
	return placedConical45CavityEnvelope{"plainsolid-iges-placed-conical45-cavity-material", 1, string(m.source), m.Solid.Tolerance(), l.Bytes(), n.Bytes(), m.Placement, m.SourceTransformID, m.SourceTransformMetadata, m.SourceVertices, m.NativeRimEdges}, nil
}
func WritePlacedConical45CavityMaterialJSON(ctx context.Context, w io.Writer, m PlacedConical45CavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadPlacedConical45CavityMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid.Tolerance())
	if e != nil {
		return e
	}
	a, e := placedConical45CavitySnapshot(ctx, m)
	if e != nil {
		return e
	}
	b, e := placedConical45CavitySnapshot(ctx, p)
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
func ReadPlacedConical45CavityMaterialJSON(ctx context.Context, r io.Reader, l Limits) (PlacedConical45CavityMaterial, error) {
	bad := func() (PlacedConical45CavityMaterial, error) {
		return PlacedConical45CavityMaterial{}, geom.ErrInvalid
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
	var q placedConical45CavityEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-placed-conical45-cavity-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadPlacedConical45CavityMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	want, e := placedConical45CavitySnapshot(ctx, p)
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
func WritePlacedConical45CavitySource(ctx context.Context, w io.Writer, m PlacedConical45CavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WritePlacedConical45CavityMaterialJSON(ctx, io.Discard, m); e != nil {
		return e
	}
	n, e := w.Write(m.source)
	if e == nil && n != len(m.source) {
		e = io.ErrShortWrite
	}
	return e
}
