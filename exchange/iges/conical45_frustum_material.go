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

// Conical45FrustumMaterial retains the authenticated analytic194 source and
// maps its full circular rims to exact sewn native quarter partitions.
type Conical45FrustumMaterial struct {
	Descriptor          Conical45FrustumDescriptor
	LengthUnit          float64
	Solid               trimmed.Solid
	Parameters          map[int][]float64
	Metadata            map[int]CylinderDatumMetadata
	NativeFaceSourceIDs [6]int
	NativeRimEdges      [2][4]int
	SourceRimCurveIDs   [2]int
	source              []byte
}

func ReadConical45FrustumMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (Conical45FrustumMaterial, error) {
	bad := func() (Conical45FrustumMaterial, error) { return Conical45FrustumMaterial{}, geom.ErrInvalid }
	if ctx == nil || r == nil {
		return bad()
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	limit := min(l.MaxBytes, int64(32768))
	raw, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil || int64(len(raw)) > limit {
		return bad()
	}
	p, e := ReadConical45FrustumSolid(ctx, bytes.NewReader(raw), l)
	if e != nil {
		return Conical45FrustumMaterial{}, e
	}
	top, e := cylinderExactSum(p.Descriptor.Radius, p.Descriptor.Height)
	if e != nil {
		return bad()
	}
	s, rims, e := ellipticprism.CircularFrustum(ctx, p.Descriptor.Origin, p.Descriptor.Radius, top, p.Descriptor.Height, tol)
	if e != nil {
		return Conical45FrustumMaterial{}, e
	}
	return Conical45FrustumMaterial{p.Descriptor, p.LengthUnit, s, p.Parameters, p.Metadata, [6]int{35, 33, 31, 31, 31, 31}, rims, [2]int{19, 17}, raw}, nil
}

type conical45MaterialEnvelope struct {
	Format     string
	Version    int
	Source     string
	Tolerance  float64
	Native     json.RawMessage
	Descriptor Conical45FrustumDescriptor
	Unit       float64
	Parameters map[int][]float64
	Metadata   map[int]CylinderDatumMetadata
	Faces      [6]int
	Rims       [2][4]int
	SourceRims [2]int
}

func conical45MaterialSnapshot(m Conical45FrustumMaterial, native []byte) conical45MaterialEnvelope {
	return conical45MaterialEnvelope{"plainsolid-iges-conical45-frustum-material", 1, string(m.source), m.Solid.Tolerance(), native, m.Descriptor, m.LengthUnit, m.Parameters, m.Metadata, m.NativeFaceSourceIDs, m.NativeRimEdges, m.SourceRimCurveIDs}
}
func WriteConical45FrustumMaterialJSON(ctx context.Context, w io.Writer, m Conical45FrustumMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadConical45FrustumMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid.Tolerance())
	if e != nil {
		return e
	}
	a, e := igesPrismBytes(m.Solid)
	if e != nil {
		return e
	}
	b, e := igesPrismBytes(p.Solid)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(conical45MaterialSnapshot(m, a), conical45MaterialSnapshot(p, b)) {
		return fmt.Errorf("iges: stale conical material")
	}
	raw, e := json.Marshal(conical45MaterialSnapshot(p, b))
	if e != nil {
		return e
	}
	if len(raw) > 512<<10 {
		return geom.ErrConvergence
	}
	raw = append(raw, '\n')
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
func ReadConical45FrustumMaterialJSON(ctx context.Context, r io.Reader, l Limits) (Conical45FrustumMaterial, error) {
	bad := func() (Conical45FrustumMaterial, error) { return Conical45FrustumMaterial{}, geom.ErrInvalid }
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
	var q conical45MaterialEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-conical45-frustum-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadConical45FrustumMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	n, e := igesPrismBytes(p.Solid)
	if e != nil {
		return bad()
	}
	var c bytes.Buffer
	if json.Compact(&c, q.Native) != nil {
		return bad()
	}
	q.Native = append([]byte(nil), c.Bytes()...)
	want := conical45MaterialSnapshot(p, n)
	c.Reset()
	if json.Compact(&c, n) != nil {
		return bad()
	}
	want.Native = append([]byte(nil), c.Bytes()...)
	if !reflect.DeepEqual(q, want) {
		return bad()
	}
	return p, nil
}

// WriteConical45FrustumMaterialSource re-proves the native binding and emits the
// unchanged authenticated analytic source, preserving metadata and source IDs.
func WriteConical45FrustumMaterialSource(ctx context.Context, w io.Writer, m Conical45FrustumMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WriteConical45FrustumMaterialJSON(ctx, io.Discard, m); e != nil {
		return e
	}
	n, e := w.Write(m.source)
	if e == nil && n != len(m.source) {
		e = io.ErrShortWrite
	}
	return e
}
