package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
)

type PlacedSphericalMaterial struct {
	Local                   SphericalMaterial
	Placement               geom.Affine
	SourceTransformID       int
	SourceTransformMetadata CylinderDatumMetadata
	SourceVertices          []geom.Vec3
	Solid                   trimmed.Solid
	NativeEdges             [3]int
	source                  []byte
}

func ReadPlacedSphericalMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (PlacedSphericalMaterial, error) {
	bad := func() (PlacedSphericalMaterial, error) { return PlacedSphericalMaterial{}, geom.ErrInvalid }
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return PlacedSphericalMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 17)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true, true, true)
	if e != nil || len(doc.entities) != 17 || mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	tr := doc.entities[16]
	if tr.id != 33 || tr.kind != 124 || tr.transform != 0 || compositeSourceNumbers(124, tr.values) != nil {
		return bad()
	}
	pose, e := decodeMatrix(tr.values)
	if e != nil || !roofRigid(pose) {
		return bad()
	}
	out := PlacedSphericalMaterial{Placement: pose, SourceTransformID: 33, SourceTransformMetadata: CylinderDatumMetadata{tr.label, tr.subscript}, source: append([]byte(nil), doc.bytes...)}
	expected, e := torusMinorPose(pose, 0)
	if e != nil {
		return bad()
	}
	minor := doc.entities[12]
	if minor.id != 25 || minor.kind != 124 || compositeSourceNumbers(124, minor.values) != nil {
		return bad()
	}
	actual, e := decodeMatrix(minor.values)
	if e != nil || actual != expected {
		return bad()
	}
	for i, rec := range doc.entities[:16] {
		want := 0
		if rec.id == 7 {
			want = 33
		}
		if rec.id == 23 {
			want = 25
		}
		if rec.transform != want {
			return bad()
		}
		if rec.id != 23 {
			doc.entities[i].transform = 0
		}
	}
	vertex := doc.entities[8]
	if vertex.kind != 502 || len(vertex.values) != 8 || compositeSourceNumbers(110, vertex.values) != nil {
		return bad()
	}
	for j := 0; j < 2; j++ {
		var xyz [3]float64
		for k := range xyz {
			xyz[k], e = realValue(vertex.values[2+3*j+k])
			if e != nil {
				return bad()
			}
		}
		world := geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
		local, e := roofMapPoint(pose, world, true)
		if e != nil {
			return bad()
		}
		out.SourceVertices = append(out.SourceVertices, world)
		for k, x := range []float64{local.X, local.Y, local.Z} {
			doc.entities[8].values[2+3*j+k].text = number(x)
		}
	}
	canonical, e := torusMinorPose(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}, 0)
	if e != nil {
		return bad()
	}
	budget := 2048
	doc.entities[12].values, e = parseParameters(torusMatrixText(canonical), ',', ';', &budget)
	if e != nil {
		return bad()
	}
	doc.entities = doc.entities[:16]
	raw, e := mixedRoofFile(doc)
	if e != nil {
		return bad()
	}
	out.Local, e = ReadSphericalMaterial(ctx, bytes.NewReader(raw), l, tol)
	if e != nil {
		return bad()
	}
	// Require all six cardinal radius witnesses to survive the native placement
	// losslessly, including equator points not listed as source vertices.
	radius := out.Local.Descriptor.Radius
	for _, p := range []geom.Vec3{{X: radius}, {X: -radius}, {Y: radius}, {Y: -radius}, {Z: radius}, {Z: -radius}} {
		if _, e = roofMapPoint(pose, p, false); e != nil {
			return bad()
		}
	}
	out.Solid, e = out.Local.Solid.Transformed(pose)
	if e != nil {
		return PlacedSphericalMaterial{}, e
	}
	out.Solid, e = trimmed.NewSolid(out.Solid.Vertices(), out.Solid.Edges(), out.Solid.Faces(), tol)
	if e != nil {
		return PlacedSphericalMaterial{}, e
	}
	out.NativeEdges = out.Local.NativeEdges
	// Source poles were inverted exactly above. Check the mapped native vertices
	// reproduce them; the native angular seam evaluation remains numerical.
	v := out.Solid.Vertices()
	if len(v) != 2 || v[0] != out.SourceVertices[1] || v[1] != out.SourceVertices[0] {
		return bad()
	}
	return out, nil
}
func WritePlacedSpherical(ctx context.Context, w io.Writer, d SphericalMaterialDescriptor, pose geom.Affine, unit, tol float64) error {
	if ctx == nil || w == nil || !roofRigid(pose) {
		return geom.ErrInvalid
	}
	a, e := sphericalMaterialAuthor(d)
	if e != nil {
		return e
	}
	minor, e := torusMinorPose(pose, 0)
	if e != nil {
		return e
	}
	a.records[12].data = torusMatrixText(minor)
	return writePlacedConicLeaves(ctx, w, a, pose, unit, tol, func(k int) bool { return k == 196 }, func(raw []byte) error {
		for _, line := range bytes.Split(raw, []byte("\n")) {
			if len(line) == 80 && line[72] == 'D' {
				id, _ := field(string(line[73:]))
				if id == 23 {
					copy(line[48:56], "      25")
				}
			}
		}
		_, e := ReadPlacedSphericalMaterial(ctx, bytes.NewReader(raw), Limits{}, tol)
		return e
	})
}

type placedSphericalEnvelope struct {
	Format            string
	Version           int
	Source            string
	Tolerance         float64
	Local, Native     json.RawMessage
	Placement         geom.Affine
	TransformID       int
	TransformMetadata CylinderDatumMetadata
	Vertices          []geom.Vec3
	Rims              [3]int
}

func placedSphericalSnapshot(ctx context.Context, m PlacedSphericalMaterial) (placedSphericalEnvelope, error) {
	var local bytes.Buffer
	if e := WriteSphericalMaterialJSON(ctx, &local, m.Local); e != nil {
		return placedSphericalEnvelope{}, e
	}
	native, e := igesPrismBytes(m.Solid)
	if e != nil {
		return placedSphericalEnvelope{}, e
	}
	var l, n bytes.Buffer
	if json.Compact(&l, local.Bytes()) != nil || json.Compact(&n, native) != nil {
		return placedSphericalEnvelope{}, geom.ErrInvalid
	}
	return placedSphericalEnvelope{"plainsolid-iges-placed-spherical-material", 1, string(m.source), m.Solid.Tolerance(), l.Bytes(), n.Bytes(), m.Placement, m.SourceTransformID, m.SourceTransformMetadata, m.SourceVertices, m.NativeEdges}, nil
}
func WritePlacedSphericalMaterialJSON(ctx context.Context, w io.Writer, m PlacedSphericalMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadPlacedSphericalMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid.Tolerance())
	if e != nil {
		return e
	}
	a, e := placedSphericalSnapshot(ctx, m)
	if e != nil {
		return e
	}
	b, e := placedSphericalSnapshot(ctx, p)
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
func ReadPlacedSphericalMaterialJSON(ctx context.Context, r io.Reader, l Limits) (PlacedSphericalMaterial, error) {
	bad := func() (PlacedSphericalMaterial, error) {
		return PlacedSphericalMaterial{}, geom.ErrInvalid
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
	var q placedSphericalEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-placed-spherical-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadPlacedSphericalMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	want, e := placedSphericalSnapshot(ctx, p)
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
func WritePlacedSphericalSource(ctx context.Context, w io.Writer, m PlacedSphericalMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WritePlacedSphericalMaterialJSON(ctx, io.Discard, m); e != nil {
		return e
	}
	n, e := w.Write(m.source)
	if e == nil && n != len(m.source) {
		e = io.ErrShortWrite
	}
	return e
}
