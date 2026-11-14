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

// ReindexedCylinderSphereCavityMaterial separates original source identities from
// one canonical unplaced or properly placed exact material proof.
type ReindexedCylinderSphereCavityMaterial struct {
	Local                                                        *CylinderSphereCavityMaterial
	Placed                                                       *PlacedCylinderSphereCavityMaterial
	SourceForCanonical                                           map[int]int
	SourceVertexOrdinals, SourceEdgeOrdinals, SourceFaceOrdinals map[int]map[int]int
	SourceUseOrdinals                                            map[int][]int
	NativeFaceSourceIDs                                          [7]int
	SourceModelCurveIDs                                          [3]int
	SourceDegreeCurveIDs                                         [2][4]int
	source                                                       []byte
}

func (m ReindexedCylinderSphereCavityMaterial) Solid() trimmed.Solid {
	if m.Local != nil {
		return m.Local.Solid
	}
	if m.Placed != nil {
		return m.Placed.Solid
	}
	return trimmed.Solid{}
}
func ReadReindexedCylinderSphereCavityMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ReindexedCylinderSphereCavityMaterial, error) {
	g, e := readReindexedCylinderSphereCavityGraph(ctx, r, l, tol)
	if e != nil {
		return ReindexedCylinderSphereCavityMaterial{}, e
	}
	out := ReindexedCylinderSphereCavityMaterial{SourceForCanonical: g.Roles, SourceVertexOrdinals: g.Vertices, SourceEdgeOrdinals: g.Edges, SourceFaceOrdinals: g.Faces, SourceUseOrdinals: g.Uses, source: g.Source}
	if g.Placed {
		p, e := ReadPlacedCylinderSphereCavityMaterial(ctx, bytes.NewReader(g.Canonical), l, tol)
		if e != nil {
			return ReindexedCylinderSphereCavityMaterial{}, e
		}
		out.Placed = &p
	} else {
		p, e := ReadCylinderSphereCavityMaterial(ctx, bytes.NewReader(g.Canonical), l, tol)
		if e != nil {
			return ReindexedCylinderSphereCavityMaterial{}, e
		}
		out.Local = &p
	}
	for i, id := range [7]int{43, 41, 39, 39, 39, 39, 51} {
		out.NativeFaceSourceIDs[i] = g.Roles[id]
	}
	out.SourceModelCurveIDs = [3]int{g.Roles[17], g.Roles[19], g.Roles[69]}
	for i, ids := range [2][4]int{{25, 27, 29, 31}, {65, 73, 75, 77}} {
		for j, id := range ids {
			out.SourceDegreeCurveIDs[i][j] = g.Roles[id]
		}
	}
	return out, nil
}
func reindexedCylinderSphereCavityProof(ctx context.Context, m ReindexedCylinderSphereCavityMaterial) ([]byte, error) {
	var b bytes.Buffer
	var e error
	if m.Local != nil && m.Placed == nil {
		e = WriteCylinderSphereCavityMaterialJSON(ctx, &b, *m.Local)
	} else if m.Placed != nil && m.Local == nil {
		e = WritePlacedCylinderSphereCavityMaterialJSON(ctx, &b, *m.Placed)
	} else {
		return nil, geom.ErrInvalid
	}
	return b.Bytes(), e
}
func reindexedCylinderSphereCavityEqual(ctx context.Context, a, b ReindexedCylinderSphereCavityMaterial) bool {
	if !reflect.DeepEqual(a.SourceEdgeOrdinals, b.SourceEdgeOrdinals) || !reflect.DeepEqual(a.SourceFaceOrdinals, b.SourceFaceOrdinals) || !reflect.DeepEqual(a.SourceForCanonical, b.SourceForCanonical) || !reflect.DeepEqual(a.SourceVertexOrdinals, b.SourceVertexOrdinals) || !reflect.DeepEqual(a.SourceUseOrdinals, b.SourceUseOrdinals) || a.NativeFaceSourceIDs != b.NativeFaceSourceIDs || a.SourceModelCurveIDs != b.SourceModelCurveIDs || a.SourceDegreeCurveIDs != b.SourceDegreeCurveIDs {
		return false
	}
	x, e := reindexedCylinderSphereCavityProof(ctx, a)
	if e != nil {
		return false
	}
	y, e := reindexedCylinderSphereCavityProof(ctx, b)
	return e == nil && bytes.Equal(x, y)
}
func WriteReindexedCylinderSphereCavitySource(ctx context.Context, w io.Writer, m ReindexedCylinderSphereCavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadReindexedCylinderSphereCavityMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid().Tolerance())
	if e != nil {
		return e
	}
	if !reindexedCylinderSphereCavityEqual(ctx, m, p) {
		return geom.ErrInvalid
	}
	n, e := w.Write(p.source)
	if e == nil && n != len(p.source) {
		e = io.ErrShortWrite
	}
	return e
}

type reindexedCylinderSphereCavityEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Proof     json.RawMessage
}

func WriteReindexedCylinderSphereCavityMaterialJSON(ctx context.Context, w io.Writer, m ReindexedCylinderSphereCavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WriteReindexedCylinderSphereCavitySource(ctx, io.Discard, m); e != nil {
		return e
	}
	proof, e := reindexedCylinderSphereCavityProof(ctx, m)
	if e != nil {
		return e
	}
	b, e := json.Marshal(reindexedCylinderSphereCavityEnvelope{"plainsolid-iges-reindexed-cylinder-sphere-cavity-material", 1, string(m.source), m.Solid().Tolerance(), proof})
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
func ReadReindexedCylinderSphereCavityMaterialJSON(ctx context.Context, r io.Reader, l Limits) (ReindexedCylinderSphereCavityMaterial, error) {
	bad := func() (ReindexedCylinderSphereCavityMaterial, error) {
		return ReindexedCylinderSphereCavityMaterial{}, geom.ErrInvalid
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
	var q reindexedCylinderSphereCavityEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-reindexed-cylinder-sphere-cavity-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadReindexedCylinderSphereCavityMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	proof, e := reindexedCylinderSphereCavityProof(ctx, p)
	if e != nil {
		return bad()
	}
	var a, c bytes.Buffer
	if json.Compact(&a, proof) != nil || json.Compact(&c, q.Proof) != nil || !bytes.Equal(a.Bytes(), c.Bytes()) {
		return bad()
	}
	return p, nil
}
