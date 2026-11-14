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

// ReindexedConical45CavityMaterial separates original source identities from
// one canonical unplaced or properly placed exact material proof.
type ReindexedConical45CavityMaterial struct {
	Local                                                        *Conical45CavityMaterial
	Placed                                                       *PlacedConical45CavityMaterial
	SourceForCanonical                                           map[int]int
	SourceVertexOrdinals, SourceEdgeOrdinals, SourceFaceOrdinals map[int]map[int]int
	SourceUseOrdinals                                            map[int][]int
	NativeFaceSourceIDs                                          [12]int
	SourceRimCurveIDs                                            [2][2]int
	source                                                       []byte
}

func (m ReindexedConical45CavityMaterial) Solid() trimmed.Solid {
	if m.Local != nil {
		return m.Local.Solid
	}
	if m.Placed != nil {
		return m.Placed.Solid
	}
	return trimmed.Solid{}
}
func ReadReindexedConical45CavityMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ReindexedConical45CavityMaterial, error) {
	g, e := readReindexedConical45CavityGraph(ctx, r, l, tol)
	if e != nil {
		return ReindexedConical45CavityMaterial{}, e
	}
	out := ReindexedConical45CavityMaterial{SourceForCanonical: g.Roles, SourceVertexOrdinals: g.Vertices, SourceEdgeOrdinals: g.Edges, SourceFaceOrdinals: g.Faces, SourceUseOrdinals: g.Uses, source: g.Source}
	if g.Placed {
		p, e := ReadPlacedConical45CavityMaterial(ctx, bytes.NewReader(g.Canonical), l, tol)
		if e != nil {
			return ReindexedConical45CavityMaterial{}, e
		}
		out.Placed = &p
	} else {
		p, e := ReadConical45CavityMaterial(ctx, bytes.NewReader(g.Canonical), l, tol)
		if e != nil {
			return ReindexedConical45CavityMaterial{}, e
		}
		out.Local = &p
	}
	for i, id := range [12]int{35, 33, 31, 31, 31, 31, 75, 73, 71, 71, 71, 71} {
		out.NativeFaceSourceIDs[i] = g.Roles[id]
	}
	out.SourceRimCurveIDs = [2][2]int{{g.Roles[19], g.Roles[17]}, {g.Roles[59], g.Roles[57]}}
	return out, nil
}
func reindexedConeCavityProof(ctx context.Context, m ReindexedConical45CavityMaterial) ([]byte, error) {
	var b bytes.Buffer
	var e error
	if m.Local != nil && m.Placed == nil {
		e = WriteConical45CavityMaterialJSON(ctx, &b, *m.Local)
	} else if m.Placed != nil && m.Local == nil {
		e = WritePlacedConical45CavityMaterialJSON(ctx, &b, *m.Placed)
	} else {
		return nil, geom.ErrInvalid
	}
	return b.Bytes(), e
}
func reindexedConeCavityEqual(ctx context.Context, a, b ReindexedConical45CavityMaterial) bool {
	if !reflect.DeepEqual(a.SourceEdgeOrdinals, b.SourceEdgeOrdinals) || !reflect.DeepEqual(a.SourceFaceOrdinals, b.SourceFaceOrdinals) || !reflect.DeepEqual(a.SourceForCanonical, b.SourceForCanonical) || !reflect.DeepEqual(a.SourceVertexOrdinals, b.SourceVertexOrdinals) || !reflect.DeepEqual(a.SourceUseOrdinals, b.SourceUseOrdinals) || a.NativeFaceSourceIDs != b.NativeFaceSourceIDs || a.SourceRimCurveIDs != b.SourceRimCurveIDs {
		return false
	}
	x, e := reindexedConeCavityProof(ctx, a)
	if e != nil {
		return false
	}
	y, e := reindexedConeCavityProof(ctx, b)
	return e == nil && bytes.Equal(x, y)
}
func WriteReindexedConical45CavitySource(ctx context.Context, w io.Writer, m ReindexedConical45CavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadReindexedConical45CavityMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid().Tolerance())
	if e != nil {
		return e
	}
	if !reindexedConeCavityEqual(ctx, m, p) {
		return geom.ErrInvalid
	}
	n, e := w.Write(p.source)
	if e == nil && n != len(p.source) {
		e = io.ErrShortWrite
	}
	return e
}

type reindexedConical45CavityEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Proof     json.RawMessage
}

func WriteReindexedConical45CavityMaterialJSON(ctx context.Context, w io.Writer, m ReindexedConical45CavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WriteReindexedConical45CavitySource(ctx, io.Discard, m); e != nil {
		return e
	}
	proof, e := reindexedConeCavityProof(ctx, m)
	if e != nil {
		return e
	}
	b, e := json.Marshal(reindexedConical45CavityEnvelope{"plainsolid-iges-reindexed-conical45-cavity-material", 1, string(m.source), m.Solid().Tolerance(), proof})
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
func ReadReindexedConical45CavityMaterialJSON(ctx context.Context, r io.Reader, l Limits) (ReindexedConical45CavityMaterial, error) {
	bad := func() (ReindexedConical45CavityMaterial, error) {
		return ReindexedConical45CavityMaterial{}, geom.ErrInvalid
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
	var q reindexedConical45CavityEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-reindexed-conical45-cavity-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadReindexedConical45CavityMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	proof, e := reindexedConeCavityProof(ctx, p)
	if e != nil {
		return bad()
	}
	var a, c bytes.Buffer
	if json.Compact(&a, proof) != nil || json.Compact(&c, q.Proof) != nil || !bytes.Equal(a.Bytes(), c.Bytes()) {
		return bad()
	}
	return p, nil
}
