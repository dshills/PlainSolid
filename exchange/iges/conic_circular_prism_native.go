package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/geom"
	"reflect"
)

type conicCircularMaterialEnvelope struct {
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

func WriteConicCircularPrismMaterialJSON(ctx context.Context, w io.Writer, m ConicCircularPrismMaterial) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadConicCircularPrismMaterial(ctx, bytes.NewReader(m.Topology.source), Limits{}, m.Solid.Tolerance())
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
	raw, e := json.Marshal(conicCircularMaterialEnvelope{"plainsolid-iges-conic-circular-prism-material", 1, string(source), p.Solid.Tolerance(), want, p.Topology, p.NativeFaceSourceIDs, p.NativeRimEdges, p.SourceRimCurveIDs})
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

// ReadConicCircularPrismMaterialJSON regenerates from source and checks native
// geometry/ancestry byte-for-byte. No serialized display or proof is trusted.
func ReadConicCircularPrismMaterialJSON(ctx context.Context, r io.Reader, l Limits) (ConicCircularPrismMaterial, error) {
	bad := func() (ConicCircularPrismMaterial, error) {
		return ConicCircularPrismMaterial{}, fmt.Errorf("iges: invalid ellipse material envelope")
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
	var q conicCircularMaterialEnvelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&q) != nil || q.Format != "plainsolid-iges-conic-circular-prism-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadConicCircularPrismMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
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
