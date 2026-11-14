package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

type conicalThroughEnvelope struct {
	Format  string
	Version int
	Source  string
	Native  json.RawMessage
}

func WriteConical45CylinderThroughSource(ctx context.Context, w io.Writer, m Conical45CylinderThroughSolid) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadConical45CylinderThroughSolid(ctx, bytes.NewReader(m.source), Limits{})
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
	if !bytes.Equal(a, b) || m.Descriptor != p.Descriptor || m.LengthUnit != p.LengthUnit || m.SourceSolidID != p.SourceSolidID || m.SourceShellID != p.SourceShellID || m.SourceFaceIDs != p.SourceFaceIDs || m.NativeFaceSourceIDs != p.NativeFaceSourceIDs || m.NativeRimEdges != p.NativeRimEdges || !reflect.DeepEqual(m.Parameters, p.Parameters) || !reflect.DeepEqual(m.Metadata, p.Metadata) {
		return geom.ErrInvalid
	}
	n, e := w.Write(m.source)
	if e == nil && n != len(m.source) {
		e = io.ErrShortWrite
	}
	return e
}
func WriteConical45CylinderThroughJSON(ctx context.Context, w io.Writer, m Conical45CylinderThroughSolid) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WriteConical45CylinderThroughSource(ctx, io.Discard, m); e != nil {
		return e
	}
	native, e := igesPrismBytes(m.Solid)
	if e != nil {
		return e
	}
	b, e := json.Marshal(conicalThroughEnvelope{"plainsolid-iges-conical45-cylinder-through", 1, string(m.source), native})
	if e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}
func ReadConical45CylinderThroughJSON(ctx context.Context, r io.Reader, l Limits) (Conical45CylinderThroughSolid, error) {
	bad := func() (Conical45CylinderThroughSolid, error) { return Conical45CylinderThroughSolid{}, geom.ErrInvalid }
	if ctx == nil || r == nil {
		return bad()
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	limit := min(l.MaxBytes, int64(1<<20))
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil || int64(len(b)) > limit {
		return bad()
	}
	var q conicalThroughEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-conical45-cylinder-through" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadConical45CylinderThroughSolid(ctx, bytes.NewReader([]byte(q.Source)), l)
	if e != nil {
		return bad()
	}
	native, e := igesPrismBytes(p.Solid)
	if e != nil {
		return bad()
	}
	var compact, expected bytes.Buffer
	if json.Compact(&compact, q.Native) != nil || json.Compact(&expected, native) != nil || !bytes.Equal(expected.Bytes(), compact.Bytes()) {
		return bad()
	}
	return p, nil
}
