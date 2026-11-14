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

// ReindexedToroidalMaterial separates original source identities from
// one canonical unplaced or properly placed exact material proof.
type ReindexedToroidalMaterial struct {
	Local                                    *ToroidalMaterial
	Placed                                   *PlacedToroidalMaterial
	SourceForCanonical, SourceVertexOrdinals map[int]int
	SourceEdgeOrdinals                       map[int]int
	SourceUseOrdinals                        map[int][]int
	NativeFaceSourceIDs                      [16]int
	SourceSeamCurveIDs                       [2]int
	source                                   []byte
}

func (m ReindexedToroidalMaterial) Solid() trimmed.Solid {
	if m.Local != nil {
		return m.Local.Solid
	}
	if m.Placed != nil {
		return m.Placed.Solid
	}
	return trimmed.Solid{}
}
func ReadReindexedToroidalMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ReindexedToroidalMaterial, error) {
	g, e := readReindexedToroidalGraph(ctx, r, l, tol)
	if e != nil {
		return ReindexedToroidalMaterial{}, e
	}
	out := ReindexedToroidalMaterial{SourceForCanonical: g.Roles, SourceVertexOrdinals: g.Vertices, SourceEdgeOrdinals: g.Edges, SourceUseOrdinals: g.Uses, source: g.Source}
	if g.Placed {
		p, e := ReadPlacedToroidalMaterial(ctx, bytes.NewReader(g.Canonical), l, tol)
		if e != nil {
			return ReindexedToroidalMaterial{}, e
		}
		out.Placed = &p
	} else {
		p, e := ReadToroidalMaterial(ctx, bytes.NewReader(g.Canonical), l, tol)
		if e != nil {
			return ReindexedToroidalMaterial{}, e
		}
		out.Local = &p
	}
	for i := range out.NativeFaceSourceIDs {
		out.NativeFaceSourceIDs[i] = g.Roles[21]
	}
	out.SourceSeamCurveIDs = [2]int{g.Roles[11], g.Roles[13]}
	return out, nil
}
func reindexedToroidalProof(ctx context.Context, m ReindexedToroidalMaterial) ([]byte, error) {
	var b bytes.Buffer
	var e error
	if m.Local != nil && m.Placed == nil {
		e = WriteToroidalMaterialJSON(ctx, &b, *m.Local)
	} else if m.Placed != nil && m.Local == nil {
		e = WritePlacedToroidalMaterialJSON(ctx, &b, *m.Placed)
	} else {
		return nil, geom.ErrInvalid
	}
	return b.Bytes(), e
}
func reindexedToroidalEqual(ctx context.Context, a, b ReindexedToroidalMaterial) bool {
	if !reflect.DeepEqual(a.SourceEdgeOrdinals, b.SourceEdgeOrdinals) || !reflect.DeepEqual(a.SourceForCanonical, b.SourceForCanonical) || !reflect.DeepEqual(a.SourceVertexOrdinals, b.SourceVertexOrdinals) || !reflect.DeepEqual(a.SourceUseOrdinals, b.SourceUseOrdinals) || a.NativeFaceSourceIDs != b.NativeFaceSourceIDs || a.SourceSeamCurveIDs != b.SourceSeamCurveIDs {
		return false
	}
	x, e := reindexedToroidalProof(ctx, a)
	if e != nil {
		return false
	}
	y, e := reindexedToroidalProof(ctx, b)
	return e == nil && bytes.Equal(x, y)
}
func WriteReindexedToroidalSource(ctx context.Context, w io.Writer, m ReindexedToroidalMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadReindexedToroidalMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid().Tolerance())
	if e != nil {
		return e
	}
	if !reindexedToroidalEqual(ctx, m, p) {
		return geom.ErrInvalid
	}
	n, e := w.Write(p.source)
	if e == nil && n != len(p.source) {
		e = io.ErrShortWrite
	}
	return e
}

type reindexedToroidalEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Proof     json.RawMessage
}

func WriteReindexedToroidalMaterialJSON(ctx context.Context, w io.Writer, m ReindexedToroidalMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WriteReindexedToroidalSource(ctx, io.Discard, m); e != nil {
		return e
	}
	proof, e := reindexedToroidalProof(ctx, m)
	if e != nil {
		return e
	}
	b, e := json.Marshal(reindexedToroidalEnvelope{"plainsolid-iges-reindexed-toroidal-material", 1, string(m.source), m.Solid().Tolerance(), proof})
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
func ReadReindexedToroidalMaterialJSON(ctx context.Context, r io.Reader, l Limits) (ReindexedToroidalMaterial, error) {
	bad := func() (ReindexedToroidalMaterial, error) {
		return ReindexedToroidalMaterial{}, geom.ErrInvalid
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
	var q reindexedToroidalEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-reindexed-toroidal-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadReindexedToroidalMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	proof, e := reindexedToroidalProof(ctx, p)
	if e != nil {
		return bad()
	}
	var a, c bytes.Buffer
	if json.Compact(&a, proof) != nil || json.Compact(&c, q.Proof) != nil || !bytes.Equal(a.Bytes(), c.Bytes()) {
		return bad()
	}
	return p, nil
}
