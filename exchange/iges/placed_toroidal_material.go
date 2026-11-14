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
	"strings"
)

type PlacedToroidalMaterial struct {
	Local                   ToroidalMaterial
	Placement               geom.Affine
	SourceTransformID       int
	SourceTransformMetadata CylinderDatumMetadata
	SourceVertices          []geom.Vec3
	Solid                   trimmed.Solid
	NativeSeamEdges         [2][4]int
	source                  []byte
}

// torusMinorPose composes the body pose with the original local XZ circle frame.
func torusMinorPose(p geom.Affine, R float64) (geom.Affine, error) {
	c, e := roofMapPoint(p, geom.Vec3{X: R}, false)
	if e != nil {
		return geom.Affine{}, e
	}
	q := geom.Affine{}
	for i := 0; i < 3; i++ {
		q[i][0] = p[i][0]
		q[i][1] = p[i][2]
		q[i][2] = -p[i][1]
	}
	q[0][3] = c.X
	q[1][3] = c.Y
	q[2][3] = c.Z
	return q, nil
}
func torusMatrixText(p geom.Affine) string {
	v := []string{"124"}
	for _, row := range p {
		for _, x := range row {
			v = append(v, number(x))
		}
	}
	return strings.Join(v, ",") + ";"
}
func ReadPlacedToroidalMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (PlacedToroidalMaterial, error) {
	bad := func() (PlacedToroidalMaterial, error) { return PlacedToroidalMaterial{}, geom.ErrInvalid }
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return PlacedToroidalMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 14)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true, true)
	if e != nil || len(doc.entities) != 14 || mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	tr := doc.entities[13]
	if tr.id != 27 || tr.kind != 124 || tr.transform != 0 || compositeSourceNumbers(124, tr.values) != nil {
		return bad()
	}
	pose, e := decodeMatrix(tr.values)
	if e != nil || !roofRigid(pose) {
		return bad()
	}
	out := PlacedToroidalMaterial{Placement: pose, SourceTransformID: 27, SourceTransformMetadata: CylinderDatumMetadata{tr.label, tr.subscript}, source: append([]byte(nil), doc.bytes...)}
	if len(doc.entities[3].values) != 8 {
		return bad()
	}
	R, e := realValue(doc.entities[3].values[3])
	if e != nil {
		return bad()
	}
	expected, e := torusMinorPose(pose, R)
	if e != nil {
		return bad()
	}
	minor := doc.entities[7]
	if minor.id != 15 || minor.kind != 124 || compositeSourceNumbers(124, minor.values) != nil {
		return bad()
	}
	actual, e := decodeMatrix(minor.values)
	if e != nil || actual != expected {
		return bad()
	}
	for i, rec := range doc.entities[:13] {
		want := 0
		if rec.id == 7 || rec.id == 11 {
			want = 27
		}
		if rec.id == 13 {
			want = 15
		}
		if rec.transform != want {
			return bad()
		}
		if rec.id != 13 {
			doc.entities[i].transform = 0
		}
	}
	vertex := doc.entities[4]
	if vertex.kind != 502 || len(vertex.values) != 5 || compositeSourceNumbers(110, vertex.values) != nil {
		return bad()
	}
	var xyz [3]float64
	for k := range xyz {
		xyz[k], e = realValue(vertex.values[2+k])
		if e != nil {
			return bad()
		}
	}
	world := geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
	local, e := roofMapPoint(pose, world, true)
	if e != nil {
		return bad()
	}
	out.SourceVertices = []geom.Vec3{world}
	for k, x := range []float64{local.X, local.Y, local.Z} {
		doc.entities[4].values[2+k].text = number(x)
	}
	canonical, e := torusMinorPose(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}, R)
	if e != nil {
		return bad()
	}
	budget := 2048
	doc.entities[7].values, e = parseParameters(torusMatrixText(canonical), ',', ';', &budget)
	if e != nil {
		return bad()
	}
	doc.entities = doc.entities[:13]
	raw, e := mixedRoofFile(doc)
	if e != nil {
		return bad()
	}
	out.Local, e = ReadToroidalMaterial(ctx, bytes.NewReader(raw), l, tol)
	if e != nil {
		return bad()
	}
	out.Solid, out.NativeSeamEdges, e = ellipticprism.RingTorusPlaced(ctx, out.Local.Descriptor.MajorRadius, out.Local.Descriptor.MinorRadius, tol, pose)
	if e != nil {
		return PlacedToroidalMaterial{}, e
	}
	return out, nil
}
func WritePlacedToroidal(ctx context.Context, w io.Writer, d ToroidalMaterialDescriptor, pose geom.Affine, unit, tol float64) error {
	if ctx == nil || w == nil || !roofRigid(pose) {
		return geom.ErrInvalid
	}
	a, e := toroidalMaterialAuthor(d)
	if e != nil {
		return e
	}
	minor, e := torusMinorPose(pose, d.MajorRadius)
	if e != nil {
		return e
	}
	a.records[7].data = torusMatrixText(minor)
	return writePlacedConicLeaves(ctx, w, a, pose, unit, tol, func(k int) bool { return k == 198 }, func(raw []byte) error {
		for _, line := range bytes.Split(raw, []byte("\n")) {
			if len(line) == 80 && line[72] == 'D' {
				id, _ := field(string(line[73:]))
				if id == 11 {
					copy(line[48:56], "      27")
				}
				if id == 13 {
					copy(line[48:56], "      15")
				}
			}
		}
		_, e := ReadPlacedToroidalMaterial(ctx, bytes.NewReader(raw), Limits{}, tol)
		return e
	})
}

type placedToroidalEnvelope struct {
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
}

func placedToroidalSnapshot(ctx context.Context, m PlacedToroidalMaterial) (placedToroidalEnvelope, error) {
	var local bytes.Buffer
	if e := WriteToroidalMaterialJSON(ctx, &local, m.Local); e != nil {
		return placedToroidalEnvelope{}, e
	}
	native, e := igesPrismBytes(m.Solid)
	if e != nil {
		return placedToroidalEnvelope{}, e
	}
	var l, n bytes.Buffer
	if json.Compact(&l, local.Bytes()) != nil || json.Compact(&n, native) != nil {
		return placedToroidalEnvelope{}, geom.ErrInvalid
	}
	return placedToroidalEnvelope{"plainsolid-iges-placed-toroidal-material", 1, string(m.source), m.Solid.Tolerance(), l.Bytes(), n.Bytes(), m.Placement, m.SourceTransformID, m.SourceTransformMetadata, m.SourceVertices, m.NativeSeamEdges}, nil
}
func WritePlacedToroidalMaterialJSON(ctx context.Context, w io.Writer, m PlacedToroidalMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadPlacedToroidalMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid.Tolerance())
	if e != nil {
		return e
	}
	a, e := placedToroidalSnapshot(ctx, m)
	if e != nil {
		return e
	}
	b, e := placedToroidalSnapshot(ctx, p)
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
func ReadPlacedToroidalMaterialJSON(ctx context.Context, r io.Reader, l Limits) (PlacedToroidalMaterial, error) {
	bad := func() (PlacedToroidalMaterial, error) {
		return PlacedToroidalMaterial{}, geom.ErrInvalid
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
	var q placedToroidalEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-placed-toroidal-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadPlacedToroidalMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	want, e := placedToroidalSnapshot(ctx, p)
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
func WritePlacedToroidalSource(ctx context.Context, w io.Writer, m PlacedToroidalMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WritePlacedToroidalMaterialJSON(ctx, io.Discard, m); e != nil {
		return e
	}
	n, e := w.Write(m.source)
	if e == nil && n != len(m.source) {
		e = io.ErrShortWrite
	}
	return e
}
