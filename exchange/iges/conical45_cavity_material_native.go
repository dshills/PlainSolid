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

type conical45CavityMaterialEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Native    json.RawMessage
}

func WriteConical45CavityMaterialJSON(ctx context.Context, w io.Writer, m Conical45CavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadConical45CavityMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid.Tolerance())
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
	x, y := m, p
	x.Solid, y.Solid = trimmed.Solid{}, trimmed.Solid{}
	if !bytes.Equal(a, b) || !reflect.DeepEqual(x, y) {
		return geom.ErrInvalid
	}
	raw, e := json.Marshal(conical45CavityMaterialEnvelope{"plainsolid-iges-conical45-cavity-material", 1, string(p.source), p.Solid.Tolerance(), b})
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
func ReadConical45CavityMaterialJSON(ctx context.Context, r io.Reader, l Limits) (Conical45CavityMaterial, error) {
	bad := func() (Conical45CavityMaterial, error) { return Conical45CavityMaterial{}, geom.ErrInvalid }
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
	var q conical45CavityMaterialEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-conical45-cavity-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadConical45CavityMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	want, e := igesPrismBytes(p.Solid)
	if e != nil {
		return bad()
	}
	var a, b bytes.Buffer
	if json.Compact(&a, q.Native) != nil || json.Compact(&b, want) != nil || !bytes.Equal(a.Bytes(), b.Bytes()) {
		return bad()
	}
	return p, nil
}
func WriteConical45CavityMaterialSource(ctx context.Context, w io.Writer, m Conical45CavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WriteConical45CavityMaterialJSON(ctx, io.Discard, m); e != nil {
		return e
	}
	n, e := w.Write(m.source)
	if e == nil && n != len(m.source) {
		e = io.ErrShortWrite
	}
	return e
}
