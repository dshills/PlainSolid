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

// ReindexedConical45CylinderThroughMaterial separates original source identities from
// one canonical unplaced or properly placed exact material proof.
type ReindexedConical45CylinderThroughMaterial struct {
	Local                                                        *Conical45CylinderThroughSolid
	Placed                                                       *PlacedConical45CylinderThroughMaterial
	SourceForCanonical                                           map[int]int
	SourceVertexOrdinals, SourceEdgeOrdinals, SourceFaceOrdinals map[int]map[int]int
	SourceUseOrdinals                                            map[int][]int
	NativeFaceSourceIDs                                          [10]int
	SourceModelCurveIDs                                          [4]int
	source                                                       []byte
}

func (m ReindexedConical45CylinderThroughMaterial) Solid() trimmed.Solid {
	if m.Local != nil {
		return m.Local.Solid
	}
	if m.Placed != nil {
		return m.Placed.Solid
	}
	return trimmed.Solid{}
}
func ReadReindexedConical45CylinderThroughMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ReindexedConical45CylinderThroughMaterial, error) {
	if !finite(tol) || tol <= 0 {
		return ReindexedConical45CylinderThroughMaterial{}, geom.ErrInvalid
	}
	g, e := readReindexedConical45CylinderThroughGraph(ctx, r, l, tol)
	if e != nil {
		return ReindexedConical45CylinderThroughMaterial{}, e
	}
	out := ReindexedConical45CylinderThroughMaterial{SourceForCanonical: g.Roles, SourceVertexOrdinals: g.Vertices, SourceEdgeOrdinals: g.Edges, SourceFaceOrdinals: g.Faces, SourceUseOrdinals: g.Uses, source: g.Source}
	if g.Placed {
		p, e := ReadPlacedConical45CylinderThroughMaterial(ctx, bytes.NewReader(g.Canonical), l, tol)
		if e != nil {
			return ReindexedConical45CylinderThroughMaterial{}, e
		}
		out.Placed = &p
	} else {
		p, e := ReadConical45CylinderThroughSolid(ctx, bytes.NewReader(g.Canonical), l)
		if e != nil {
			return ReindexedConical45CylinderThroughMaterial{}, e
		}
		out.Local = &p
	}
	for i, id := range [10]int{51, 49, 45, 45, 45, 45, 47, 47, 47, 47} {
		out.NativeFaceSourceIDs[i] = g.Roles[id]
	}
	out.SourceModelCurveIDs = [4]int{g.Roles[23], g.Roles[19], g.Roles[29], g.Roles[25]}
	return out, nil
}
func reindexedConical45CylinderThroughProof(ctx context.Context, m ReindexedConical45CylinderThroughMaterial) ([]byte, error) {
	var b bytes.Buffer
	var e error
	if m.Local != nil && m.Placed == nil {
		e = WriteConical45CylinderThroughJSON(ctx, &b, *m.Local)
	} else if m.Placed != nil && m.Local == nil {
		e = WritePlacedConical45CylinderThroughMaterialJSON(ctx, &b, *m.Placed)
	} else {
		return nil, geom.ErrInvalid
	}
	return b.Bytes(), e
}
func reindexedConical45CylinderThroughEqual(ctx context.Context, a, b ReindexedConical45CylinderThroughMaterial) bool {
	if !reflect.DeepEqual(a.SourceEdgeOrdinals, b.SourceEdgeOrdinals) || !reflect.DeepEqual(a.SourceFaceOrdinals, b.SourceFaceOrdinals) || !reflect.DeepEqual(a.SourceForCanonical, b.SourceForCanonical) || !reflect.DeepEqual(a.SourceVertexOrdinals, b.SourceVertexOrdinals) || !reflect.DeepEqual(a.SourceUseOrdinals, b.SourceUseOrdinals) || a.NativeFaceSourceIDs != b.NativeFaceSourceIDs || a.SourceModelCurveIDs != b.SourceModelCurveIDs {
		return false
	}
	x, e := reindexedConical45CylinderThroughProof(ctx, a)
	if e != nil {
		return false
	}
	y, e := reindexedConical45CylinderThroughProof(ctx, b)
	return e == nil && bytes.Equal(x, y)
}
func WriteReindexedConical45CylinderThroughSource(ctx context.Context, w io.Writer, m ReindexedConical45CylinderThroughMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadReindexedConical45CylinderThroughMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid().Tolerance())
	if e != nil {
		return e
	}
	if !reindexedConical45CylinderThroughEqual(ctx, m, p) {
		return geom.ErrInvalid
	}
	n, e := w.Write(p.source)
	if e == nil && n != len(p.source) {
		e = io.ErrShortWrite
	}
	return e
}

type reindexedConical45CylinderThroughEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Proof     json.RawMessage
}

func WriteReindexedConical45CylinderThroughMaterialJSON(ctx context.Context, w io.Writer, m ReindexedConical45CylinderThroughMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WriteReindexedConical45CylinderThroughSource(ctx, io.Discard, m); e != nil {
		return e
	}
	proof, e := reindexedConical45CylinderThroughProof(ctx, m)
	if e != nil {
		return e
	}
	b, e := json.Marshal(reindexedConical45CylinderThroughEnvelope{"plainsolid-iges-reindexed-conical45-cylinder-through-material", 1, string(m.source), m.Solid().Tolerance(), proof})
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
func ReadReindexedConical45CylinderThroughMaterialJSON(ctx context.Context, r io.Reader, l Limits) (ReindexedConical45CylinderThroughMaterial, error) {
	bad := func() (ReindexedConical45CylinderThroughMaterial, error) {
		return ReindexedConical45CylinderThroughMaterial{}, geom.ErrInvalid
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
	var q reindexedConical45CylinderThroughEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-reindexed-conical45-cylinder-through-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadReindexedConical45CylinderThroughMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	proof, e := reindexedConical45CylinderThroughProof(ctx, p)
	if e != nil {
		return bad()
	}
	var a, c bytes.Buffer
	if json.Compact(&a, proof) != nil || json.Compact(&c, q.Proof) != nil || !bytes.Equal(a.Bytes(), c.Bytes()) {
		return bad()
	}
	return p, nil
}
