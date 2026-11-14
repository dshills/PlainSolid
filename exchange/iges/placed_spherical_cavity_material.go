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

type PlacedSphericalCavityMaterial struct {
	Local                   SphericalCavityMaterial
	Placement               geom.Affine
	SourceTransformID       int
	SourceTransformMetadata CylinderDatumMetadata
	SourceVertices          []geom.Vec3
	Solid                   trimmed.Solid
	NativeEdges             [2][3]int
	source                  []byte
}

func ReadPlacedSphericalCavityMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (PlacedSphericalCavityMaterial, error) {
	bad := func() (PlacedSphericalCavityMaterial, error) { return PlacedSphericalCavityMaterial{}, geom.ErrInvalid }
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return PlacedSphericalCavityMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 32)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true, true, true)
	if e != nil || len(doc.entities) != 32 || mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	tr := doc.entities[31]
	if tr.id != 63 || tr.kind != 124 || tr.transform != 0 || compositeSourceNumbers(124, tr.values) != nil {
		return bad()
	}
	pose, e := decodeMatrix(tr.values)
	if e != nil || !roofRigid(pose) {
		return bad()
	}
	out := PlacedSphericalCavityMaterial{Placement: pose, SourceTransformID: 63, SourceTransformMetadata: CylinderDatumMetadata{tr.label, tr.subscript}, source: append([]byte(nil), doc.bytes...)}
	expected, e := torusMinorPose(pose, 0)
	if e != nil {
		return bad()
	}
	for _, index := range []int{12, 27} {
		minor := doc.entities[index]
		if minor.id != 2*index+1 || minor.kind != 124 || compositeSourceNumbers(124, minor.values) != nil {
			return bad()
		}
		actual, e := decodeMatrix(minor.values)
		if e != nil || actual != expected {
			return bad()
		}
	}
	for i, rec := range doc.entities[:31] {
		want := 0
		if rec.id == 7 || rec.id == 37 {
			want = 63
		}
		if rec.id == 23 {
			want = 25
		}
		if rec.id == 53 {
			want = 55
		}
		if rec.transform != want {
			return bad()
		}
		if rec.id != 23 && rec.id != 53 {
			doc.entities[i].transform = 0
		}
	}
	for _, index := range []int{8, 23} {
		vertex := doc.entities[index]
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
				doc.entities[index].values[2+3*j+k].text = number(x)
			}
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
	budget = 2048
	doc.entities[27].values, e = parseParameters(torusMatrixText(canonical), ',', ';', &budget)
	if e != nil {
		return bad()
	}
	doc.entities = doc.entities[:31]
	raw, e := mixedRoofFile(doc)
	if e != nil {
		return bad()
	}
	out.Local, e = ReadSphericalCavityMaterial(ctx, bytes.NewReader(raw), l, tol)
	if e != nil {
		return bad()
	}
	// Require all six cardinal radius witnesses to survive the native placement
	// losslessly, including equator points not listed as source vertices.
	for _, radius := range []float64{out.Local.Descriptor.OuterRadius, out.Local.Descriptor.InnerRadius} {
		for _, p := range []geom.Vec3{{X: radius}, {X: -radius}, {Y: radius}, {Y: -radius}, {Z: radius}, {Z: -radius}} {
			if _, e = roofMapPoint(pose, p, false); e != nil {
				return bad()
			}
		}
	}
	out.Solid, e = out.Local.Solid.Transformed(pose)
	if e != nil {
		return PlacedSphericalCavityMaterial{}, e
	}
	out.Solid, e = trimmed.NewSolid(out.Solid.Vertices(), out.Solid.Edges(), out.Solid.Faces(), tol)
	if e != nil {
		return PlacedSphericalCavityMaterial{}, e
	}
	out.NativeEdges = out.Local.NativeEdges
	// Source poles were inverted exactly above. Check the mapped native vertices
	// reproduce them; the native angular seam evaluation remains numerical.
	v := out.Solid.Vertices()
	if len(v) != 4 || v[0] != out.SourceVertices[1] || v[1] != out.SourceVertices[0] || v[2] != out.SourceVertices[3] || v[3] != out.SourceVertices[2] {
		return bad()
	}
	return out, nil
}
func WritePlacedSphericalCavity(ctx context.Context, w io.Writer, d SphericalCavityMaterialDescriptor, pose geom.Affine, unit, tol float64) error {
	if ctx == nil || w == nil || !roofRigid(pose) {
		return geom.ErrInvalid
	}
	a, e := sphericalCavityMaterialAuthor(d)
	if e != nil {
		return e
	}
	minor, e := torusMinorPose(pose, 0)
	if e != nil {
		return e
	}
	a.records[12].data = torusMatrixText(minor)
	a.records[27].data = torusMatrixText(minor)
	return writePlacedConicLeaves(ctx, w, a, pose, unit, tol, func(k int) bool { return k == 196 }, func(raw []byte) error {
		for _, line := range bytes.Split(raw, []byte("\n")) {
			if len(line) == 80 && line[72] == 'D' {
				id, _ := field(string(line[73:]))
				if id == 23 {
					copy(line[48:56], "      25")
				}
				if id == 53 {
					copy(line[48:56], "      55")
				}
			}
		}
		_, e := ReadPlacedSphericalCavityMaterial(ctx, bytes.NewReader(raw), Limits{}, tol)
		return e
	})
}

type placedSphericalCavityEnvelope struct {
	Format            string
	Version           int
	Source            string
	Tolerance         float64
	Local, Native     json.RawMessage
	Placement         geom.Affine
	TransformID       int
	TransformMetadata CylinderDatumMetadata
	Vertices          []geom.Vec3
	Rims              [2][3]int
}

func placedSphericalCavitySnapshot(ctx context.Context, m PlacedSphericalCavityMaterial) (placedSphericalCavityEnvelope, error) {
	var local bytes.Buffer
	if e := WriteSphericalCavityMaterialJSON(ctx, &local, m.Local); e != nil {
		return placedSphericalCavityEnvelope{}, e
	}
	native, e := igesPrismBytes(m.Solid)
	if e != nil {
		return placedSphericalCavityEnvelope{}, e
	}
	var l, n bytes.Buffer
	if json.Compact(&l, local.Bytes()) != nil || json.Compact(&n, native) != nil {
		return placedSphericalCavityEnvelope{}, geom.ErrInvalid
	}
	return placedSphericalCavityEnvelope{"plainsolid-iges-placed-spherical-cavity-material", 1, string(m.source), m.Solid.Tolerance(), l.Bytes(), n.Bytes(), m.Placement, m.SourceTransformID, m.SourceTransformMetadata, m.SourceVertices, m.NativeEdges}, nil
}
func WritePlacedSphericalCavityMaterialJSON(ctx context.Context, w io.Writer, m PlacedSphericalCavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadPlacedSphericalCavityMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid.Tolerance())
	if e != nil {
		return e
	}
	a, e := placedSphericalCavitySnapshot(ctx, m)
	if e != nil {
		return e
	}
	b, e := placedSphericalCavitySnapshot(ctx, p)
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
func ReadPlacedSphericalCavityMaterialJSON(ctx context.Context, r io.Reader, l Limits) (PlacedSphericalCavityMaterial, error) {
	bad := func() (PlacedSphericalCavityMaterial, error) {
		return PlacedSphericalCavityMaterial{}, geom.ErrInvalid
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
	var q placedSphericalCavityEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-placed-spherical-cavity-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadPlacedSphericalCavityMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	want, e := placedSphericalCavitySnapshot(ctx, p)
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
func WritePlacedSphericalCavitySource(ctx context.Context, w io.Writer, m PlacedSphericalCavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WritePlacedSphericalCavityMaterialJSON(ctx, io.Discard, m); e != nil {
		return e
	}
	n, e := w.Write(m.source)
	if e == nil && n != len(m.source) {
		e = io.ErrShortWrite
	}
	return e
}
