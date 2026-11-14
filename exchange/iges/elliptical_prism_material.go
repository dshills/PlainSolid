package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/ellipticprism"
	"reflect"
)

// EllipticalPrismMaterial keeps original source role IDs distinct from native
// exact quarter partitions. Native faces are lower/upper caps then four sides.
type EllipticalPrismMaterial struct {
	Topology            EllipticalPrismTopology
	Solid               trimmed.Solid
	NativeFaceSourceIDs [6]int
	NativeRimEdges      [2][4]int
	SourceRimCurveIDs   [2]int
}

// ReadEllipticalPrismMaterial promotes only the authenticated exact20-record
// graph. Tolerance controls native validation; it never snaps source geometry.
func ReadEllipticalPrismMaterial(ctx context.Context, r io.Reader, l Limits, tolerance float64) (EllipticalPrismMaterial, error) {
	fail := func(e error) (EllipticalPrismMaterial, error) { return EllipticalPrismMaterial{}, e }
	t, e := ReadEllipticalPrismTopology(ctx, r, l)
	if e != nil {
		return fail(e)
	}
	d := t.Descriptor
	a, b, e := ellipseConicRadii(EllipseConic{Coefficients: d.Coefficients, Z: d.LowerZ})
	if e != nil {
		return fail(e)
	}
	h, e := cylinderExactSum(d.UpperZ, -d.LowerZ)
	if e != nil {
		return fail(e)
	}
	frame, e := geom.NewFrame(geom.Vec3{Z: d.LowerZ}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return fail(e)
	}
	solid, rim, e := ellipticprism.Prism(ctx, frame, a, b, h, tolerance)
	if e != nil {
		return fail(e)
	}
	return EllipticalPrismMaterial{Topology: t, Solid: solid, NativeRimEdges: rim, NativeFaceSourceIDs: [6]int{35, 33, 31, 31, 31, 31}, SourceRimCurveIDs: [2]int{9, 11}}, nil
}

type ellipticalMaterialEnvelope struct {
	Format     string
	Version    int
	Source     string
	Tolerance  float64
	Native     json.RawMessage
	Topology   EllipticalPrismTopology
	Faces      [6]int
	Rims       [2][4]int
	SourceRims [2]int
}

func igesPrismBytes(s trimmed.Solid) ([]byte, error) {
	var b bytes.Buffer
	e := trimmed.WriteJSON(&b, s)
	return b.Bytes(), e
}
func WriteEllipticalPrismMaterialJSON(ctx context.Context, w io.Writer, m EllipticalPrismMaterial) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadEllipticalPrismMaterial(ctx, bytes.NewReader(m.Topology.source), Limits{}, m.Solid.Tolerance())
	if e != nil {
		return e
	}
	actual, e := igesPrismBytes(m.Solid)
	if e != nil {
		return e
	}
	want, e := igesPrismBytes(p.Solid)
	if e != nil {
		return e
	}
	if !bytes.Equal(actual, want) || !reflect.DeepEqual(m.Topology, p.Topology) || m.NativeFaceSourceIDs != p.NativeFaceSourceIDs || m.NativeRimEdges != p.NativeRimEdges || m.SourceRimCurveIDs != p.SourceRimCurveIDs {
		return fmt.Errorf("iges: stale ellipse material binding")
	}
	source := p.Topology.source
	p.Topology.source = nil
	raw, e := json.Marshal(ellipticalMaterialEnvelope{"plainsolid-iges-elliptical-prism-material", 1, string(source), p.Solid.Tolerance(), want, p.Topology, p.NativeFaceSourceIDs, p.NativeRimEdges, p.SourceRimCurveIDs})
	if e != nil {
		return e
	}
	if len(raw) > 512<<10 {
		return fmt.Errorf("iges: ellipse material envelope limit")
	}
	raw = append(raw, '\n')
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadEllipticalPrismMaterialJSON regenerates from source and checks native
// geometry/ancestry byte-for-byte. No serialized display or proof is trusted.
func ReadEllipticalPrismMaterialJSON(ctx context.Context, r io.Reader, l Limits) (EllipticalPrismMaterial, error) {
	bad := func() (EllipticalPrismMaterial, error) {
		return EllipticalPrismMaterial{}, fmt.Errorf("iges: invalid ellipse material envelope")
	}
	if ctx == nil || r == nil {
		return bad()
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	limit := min(l.MaxBytes, int64(512<<10))
	raw, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil || int64(len(raw)) > limit {
		return bad()
	}
	var q ellipticalMaterialEnvelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&q) != nil || q.Format != "plainsolid-iges-elliptical-prism-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadEllipticalPrismMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	native, e := igesPrismBytes(p.Solid)
	if e != nil {
		return bad()
	}
	var compact bytes.Buffer
	if json.Compact(&compact, q.Native) != nil {
		return bad()
	}
	var expected bytes.Buffer
	if json.Compact(&expected, native) != nil {
		return bad()
	}
	source := p.Topology.source
	p.Topology.source = nil
	if !bytes.Equal(compact.Bytes(), expected.Bytes()) || !reflect.DeepEqual(p.Topology, q.Topology) || p.NativeFaceSourceIDs != q.Faces || p.NativeRimEdges != q.Rims || p.SourceRimCurveIDs != q.SourceRims {
		return bad()
	}
	p.Topology.source = source
	return p, nil
}
