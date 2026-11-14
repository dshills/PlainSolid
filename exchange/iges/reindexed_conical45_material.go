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

// ReindexedConical45FrustumMaterial separates original source identities from
// one canonical unplaced or properly placed exact material proof.
type ReindexedConical45FrustumMaterial struct {
	Local                                    *Conical45FrustumMaterial
	Placed                                   *PlacedConical45FrustumMaterial
	SourceForCanonical, SourceVertexOrdinals map[int]int
	SourceUseOrdinals                        map[int][]int
	NativeFaceSourceIDs                      [6]int
	SourceRimCurveIDs                        [2]int
	source                                   []byte
}

func (m ReindexedConical45FrustumMaterial) Solid() trimmed.Solid {
	if m.Local != nil {
		return m.Local.Solid
	}
	if m.Placed != nil {
		return m.Placed.Solid
	}
	return trimmed.Solid{}
}
func ReadReindexedConical45FrustumMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ReindexedConical45FrustumMaterial, error) {
	g, e := readReindexedConical45Graph(ctx, r, l, tol)
	if e != nil {
		return ReindexedConical45FrustumMaterial{}, e
	}
	out := ReindexedConical45FrustumMaterial{SourceForCanonical: g.Roles, SourceVertexOrdinals: g.Vertices, SourceUseOrdinals: g.Uses, source: g.Source}
	if g.Placed {
		p, e := ReadPlacedConical45FrustumMaterial(ctx, bytes.NewReader(g.Canonical), l, tol)
		if e != nil {
			return ReindexedConical45FrustumMaterial{}, e
		}
		out.Placed = &p
	} else {
		p, e := ReadConical45FrustumMaterial(ctx, bytes.NewReader(g.Canonical), l, tol)
		if e != nil {
			return ReindexedConical45FrustumMaterial{}, e
		}
		out.Local = &p
	}
	for i, id := range [6]int{35, 33, 31, 31, 31, 31} {
		out.NativeFaceSourceIDs[i] = g.Roles[id]
	}
	out.SourceRimCurveIDs = [2]int{g.Roles[19], g.Roles[17]}
	return out, nil
}
func reindexedConeProof(ctx context.Context, m ReindexedConical45FrustumMaterial) ([]byte, error) {
	var b bytes.Buffer
	var e error
	if m.Local != nil && m.Placed == nil {
		e = WriteConical45FrustumMaterialJSON(ctx, &b, *m.Local)
	} else if m.Placed != nil && m.Local == nil {
		e = WritePlacedConical45FrustumMaterialJSON(ctx, &b, *m.Placed)
	} else {
		return nil, geom.ErrInvalid
	}
	return b.Bytes(), e
}
func reindexedConeEqual(ctx context.Context, a, b ReindexedConical45FrustumMaterial) bool {
	if !reflect.DeepEqual(a.SourceForCanonical, b.SourceForCanonical) || !reflect.DeepEqual(a.SourceVertexOrdinals, b.SourceVertexOrdinals) || !reflect.DeepEqual(a.SourceUseOrdinals, b.SourceUseOrdinals) || a.NativeFaceSourceIDs != b.NativeFaceSourceIDs || a.SourceRimCurveIDs != b.SourceRimCurveIDs {
		return false
	}
	x, e := reindexedConeProof(ctx, a)
	if e != nil {
		return false
	}
	y, e := reindexedConeProof(ctx, b)
	return e == nil && bytes.Equal(x, y)
}
func WriteReindexedConical45FrustumSource(ctx context.Context, w io.Writer, m ReindexedConical45FrustumMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadReindexedConical45FrustumMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid().Tolerance())
	if e != nil {
		return e
	}
	if !reindexedConeEqual(ctx, m, p) {
		return geom.ErrInvalid
	}
	n, e := w.Write(p.source)
	if e == nil && n != len(p.source) {
		e = io.ErrShortWrite
	}
	return e
}

type reindexedConical45Envelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Proof     json.RawMessage
}

func WriteReindexedConical45FrustumMaterialJSON(ctx context.Context, w io.Writer, m ReindexedConical45FrustumMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WriteReindexedConical45FrustumSource(ctx, io.Discard, m); e != nil {
		return e
	}
	proof, e := reindexedConeProof(ctx, m)
	if e != nil {
		return e
	}
	b, e := json.Marshal(reindexedConical45Envelope{"plainsolid-iges-reindexed-conical45-material", 1, string(m.source), m.Solid().Tolerance(), proof})
	if e != nil {
		return e
	}
	if len(b) > 1<<20 {
		return geom.ErrConvergence
	}
	b = append(b, '\n')
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}
func ReadReindexedConical45FrustumMaterialJSON(ctx context.Context, r io.Reader, l Limits) (ReindexedConical45FrustumMaterial, error) {
	bad := func() (ReindexedConical45FrustumMaterial, error) {
		return ReindexedConical45FrustumMaterial{}, geom.ErrInvalid
	}
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
	var q reindexedConical45Envelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-reindexed-conical45-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadReindexedConical45FrustumMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	proof, e := reindexedConeProof(ctx, p)
	if e != nil {
		return bad()
	}
	var a, c bytes.Buffer
	if json.Compact(&a, proof) != nil || json.Compact(&c, q.Proof) != nil || !bytes.Equal(a.Bytes(), c.Bytes()) {
		return bad()
	}
	return p, nil
}
