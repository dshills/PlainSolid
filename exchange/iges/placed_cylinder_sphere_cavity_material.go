package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/ellipticprism"
	"reflect"
)

type PlacedCylinderSphereCavityMaterial struct {
	Local                   CylinderSphereCavityMaterial
	Placement               geom.Affine
	SourceTransformID       int
	SourceTransformMetadata CylinderDatumMetadata
	SourceVertices          []geom.Vec3
	Solid                   trimmed.Solid
	NativeSphereEdges       [3]int
	NativeRimEdges          [2][4]int
	source                  []byte
}

func ReadPlacedCylinderSphereCavityMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (PlacedCylinderSphereCavityMaterial, error) {
	bad := func() (PlacedCylinderSphereCavityMaterial, error) {
		return PlacedCylinderSphereCavityMaterial{}, geom.ErrInvalid
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return PlacedCylinderSphereCavityMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 40)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true, true, true)
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
	out := PlacedCylinderSphereCavityMaterial{Placement: pose, SourceTransformID: 79, SourceTransformMetadata: CylinderDatumMetadata{tr.label, tr.subscript}, source: source}
	for i, rec := range doc.entities[:39] {
		want := 0
		switch rec.id {
		case 9, 11, 13, 17, 19, 21, 53:
			want = 79
		case 69:
			want = 71
		}
		if rec.transform != want {
			return bad()
		}
		if rec.id != 69 {
			doc.entities[i].transform = 0
		}
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
	expected, e := torusMinorPose(pose, 0)
	if e != nil {
		return bad()
	}
	minor := doc.entities[35]
	if minor.kind != 124 || compositeSourceNumbers(124, minor.values) != nil {
		return bad()
	}
	actual, e := decodeMatrix(minor.values)
	if e != nil || actual != expected {
		return bad()
	}
	canonical, e := torusMinorPose(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}, 0)
	if e != nil {
		return bad()
	}
	budget := 2048
	doc.entities[35].values, e = parseParameters(torusMatrixText(canonical), ',', ';', &budget)
	if e != nil {
		return bad()
	}
	doc.entities = doc.entities[:39]
	raw, e := mixedRoofFile(doc)
	if e != nil {
		return bad()
	}
	out.Local, e = ReadCylinderSphereCavityMaterial(ctx, bytes.NewReader(raw), l, tol)
	if e != nil {
		return bad()
	}
	d := out.Local.Descriptor
	origin, e := roofMapPoint(pose, d.Stock.Origin, false)
	if e != nil {
		return bad()
	}
	frame, e := geom.NewFrame(origin, pose.ApplyVector(geom.Vec3{X: 1}), pose.ApplyVector(geom.Vec3{Z: 1}))
	if e != nil {
		return bad()
	}
	outer, rims, e := ellipticprism.CircularFrustumFrame(ctx, frame, d.Stock.Radius, d.Stock.Radius, d.Stock.Height, tol)
	if e != nil {
		return PlacedCylinderSphereCavityMaterial{}, e
	}
	radius := d.SphereRadius
	for _, p := range []geom.Vec3{{X: radius}, {X: -radius}, {Y: radius}, {Y: -radius}, {Z: radius}, {Z: -radius}} {
		if _, e = roofMapPoint(pose, p, false); e != nil {
			return bad()
		}
	}
	ball, e := parametric.Sphere(geom.Vec3{}, radius)
	if e != nil {
		return bad()
	}
	inner, e := trimmed.FromParametric(ball)
	if e != nil {
		return bad()
	}
	inner, e = inner.Transformed(pose)
	if e != nil {
		return bad()
	}
	out.Solid, out.NativeSphereEdges, e = joinCylinderSphereCavity(outer, inner, tol)
	if e != nil {
		return PlacedCylinderSphereCavityMaterial{}, e
	}
	out.NativeRimEdges = rims
	return out, nil
}
func WritePlacedCylinderSphereCavity(ctx context.Context, w io.Writer, d CylinderSphereCavityMaterialDescriptor, pose geom.Affine, unit, tol float64) error {
	a, e := cylinderSphereCavityMaterialAuthor(d)
	if e != nil {
		return e
	}
	minor, e := torusMinorPose(pose, 0)
	if e != nil {
		return e
	}
	a.records[35].data = torusMatrixText(minor)
	return writePlacedConicLeaves(ctx, w, a, pose, unit, tol, func(k int) bool { return k == 192 || k == 190 || k == 196 || k == 100 }, func(raw []byte) error {
		for _, line := range bytes.Split(raw, []byte("\n")) {
			if len(line) == 80 && line[72] == 'D' {
				id, _ := field(string(line[73:]))
				if id == 21 {
					copy(line[48:56], "      79")
				}
				if id == 69 {
					copy(line[48:56], "      71")
				}
			}
		}
		_, e := ReadPlacedCylinderSphereCavityMaterial(ctx, bytes.NewReader(raw), Limits{}, tol)
		return e
	})
}

type placedCylinderSphereCavityEnvelope struct {
	Format            string
	Version           int
	Source            string
	Tolerance         float64
	Local, Native     json.RawMessage
	Placement         geom.Affine
	TransformID       int
	TransformMetadata CylinderDatumMetadata
	Vertices          []geom.Vec3
	Rims              [2][4]int
	SphereEdges       [3]int
}

func placedCylinderSphereCavitySnapshot(ctx context.Context, m PlacedCylinderSphereCavityMaterial) (placedCylinderSphereCavityEnvelope, error) {
	var local bytes.Buffer
	if e := WriteCylinderSphereCavityMaterialJSON(ctx, &local, m.Local); e != nil {
		return placedCylinderSphereCavityEnvelope{}, e
	}
	native, e := igesPrismBytes(m.Solid)
	if e != nil {
		return placedCylinderSphereCavityEnvelope{}, e
	}
	var l, n bytes.Buffer
	if json.Compact(&l, local.Bytes()) != nil || json.Compact(&n, native) != nil {
		return placedCylinderSphereCavityEnvelope{}, geom.ErrInvalid
	}
	return placedCylinderSphereCavityEnvelope{"plainsolid-iges-placed-cylinder-sphere-cavity-material", 1, string(m.source), m.Solid.Tolerance(), l.Bytes(), n.Bytes(), m.Placement, m.SourceTransformID, m.SourceTransformMetadata, m.SourceVertices, m.NativeRimEdges, m.NativeSphereEdges}, nil
}
func WritePlacedCylinderSphereCavityMaterialJSON(ctx context.Context, w io.Writer, m PlacedCylinderSphereCavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadPlacedCylinderSphereCavityMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid.Tolerance())
	if e != nil {
		return e
	}
	a, e := placedCylinderSphereCavitySnapshot(ctx, m)
	if e != nil {
		return e
	}
	b, e := placedCylinderSphereCavitySnapshot(ctx, p)
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
func ReadPlacedCylinderSphereCavityMaterialJSON(ctx context.Context, r io.Reader, l Limits) (PlacedCylinderSphereCavityMaterial, error) {
	bad := func() (PlacedCylinderSphereCavityMaterial, error) {
		return PlacedCylinderSphereCavityMaterial{}, geom.ErrInvalid
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
	var q placedCylinderSphereCavityEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-placed-cylinder-sphere-cavity-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadPlacedCylinderSphereCavityMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	want, e := placedCylinderSphereCavitySnapshot(ctx, p)
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
func WritePlacedCylinderSphereCavitySource(ctx context.Context, w io.Writer, m PlacedCylinderSphereCavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WritePlacedCylinderSphereCavityMaterialJSON(ctx, io.Discard, m); e != nil {
		return e
	}
	n, e := w.Write(m.source)
	if e == nil && n != len(m.source) {
		e = io.ErrShortWrite
	}
	return e
}
