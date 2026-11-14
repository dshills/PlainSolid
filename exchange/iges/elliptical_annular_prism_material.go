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

// EllipticalAnnularPrismMaterial keeps original source role IDs distinct from native
// exact quarter partitions. Native faces are lower/upper caps, four outer sides then four inner sides.
type EllipticalAnnularPrismMaterial struct {
	Topology            EllipticalAnnularPrismTopology
	Solid               trimmed.Solid
	NativeFaceSourceIDs [10]int
	NativeRimEdges      [2][2][4]int
	SourceRimCurveIDs   [2][2]int
}

// ReadEllipticalAnnularPrismMaterial promotes only the authenticated exact28-record
// graph. Tolerance controls native validation; it never snaps source geometry.
func ReadEllipticalAnnularPrismMaterial(ctx context.Context, r io.Reader, l Limits, tolerance float64) (EllipticalAnnularPrismMaterial, error) {
	fail := func(e error) (EllipticalAnnularPrismMaterial, error) { return EllipticalAnnularPrismMaterial{}, e }
	t, e := ReadEllipticalAnnularPrismTopology(ctx, r, l)
	if e != nil {
		return fail(e)
	}
	d := t.Descriptor
	a, b, e := ellipseConicRadii(EllipseConic{Coefficients: d.OuterCoefficients, Z: d.LowerZ})
	if e != nil {
		return fail(e)
	}
	ia, ib, e := ellipseConicRadii(EllipseConic{Coefficients: d.InnerCoefficients, Z: d.LowerZ})
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
	solid, rim, e := ellipticprism.Annulus(ctx, frame, a, b, ia, ib, h, tolerance)
	if e != nil {
		return fail(e)
	}
	return EllipticalAnnularPrismMaterial{Topology: t, Solid: solid, NativeRimEdges: rim, NativeFaceSourceIDs: [10]int{51, 49, 45, 45, 45, 45, 47, 47, 47, 47}, SourceRimCurveIDs: [2][2]int{{23, 19}, {29, 25}}}, nil
}

type ellipticalAnnularMaterialEnvelope struct {
	Format     string
	Version    int
	Source     string
	Tolerance  float64
	Native     json.RawMessage
	Topology   EllipticalAnnularPrismTopology
	Faces      [10]int
	Rims       [2][2][4]int
	SourceRims [2][2]int
}

func WriteEllipticalAnnularPrismMaterialJSON(ctx context.Context, w io.Writer, m EllipticalAnnularPrismMaterial) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadEllipticalAnnularPrismMaterial(ctx, bytes.NewReader(m.Topology.source), Limits{}, m.Solid.Tolerance())
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
	raw, e := json.Marshal(ellipticalAnnularMaterialEnvelope{"plainsolid-iges-elliptical-annular-prism-material", 1, string(source), p.Solid.Tolerance(), want, p.Topology, p.NativeFaceSourceIDs, p.NativeRimEdges, p.SourceRimCurveIDs})
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

// ReadEllipticalAnnularPrismMaterialJSON regenerates from source and checks native
// geometry/ancestry byte-for-byte. No serialized display or proof is trusted.
func ReadEllipticalAnnularPrismMaterialJSON(ctx context.Context, r io.Reader, l Limits) (EllipticalAnnularPrismMaterial, error) {
	bad := func() (EllipticalAnnularPrismMaterial, error) {
		return EllipticalAnnularPrismMaterial{}, fmt.Errorf("iges: invalid ellipse material envelope")
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
	var q ellipticalAnnularMaterialEnvelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&q) != nil || q.Format != "plainsolid-iges-elliptical-annular-prism-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadEllipticalAnnularPrismMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
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
