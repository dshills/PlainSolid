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

// ReindexedSphericalMaterial separates original source identities from
// one canonical unplaced or properly placed exact material proof.
type ReindexedSphericalMaterial struct {
	Local                                    *SphericalMaterial
	Placed                                   *PlacedSphericalMaterial
	SourceForCanonical, SourceVertexOrdinals map[int]int
	SourceEdgeOrdinals                       map[int]int
	SourceUseOrdinals                        map[int][]int
	NativeFaceSourceIDs                      [1]int
	SourceDegreeCurveIDs                     [4]int
	source                                   []byte
}

func (m ReindexedSphericalMaterial) Solid() trimmed.Solid {
	if m.Local != nil {
		return m.Local.Solid
	}
	if m.Placed != nil {
		return m.Placed.Solid
	}
	return trimmed.Solid{}
}
func ReadReindexedSphericalMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ReindexedSphericalMaterial, error) {
	g, e := readReindexedSphericalGraph(ctx, r, l, tol)
	if e != nil {
		return ReindexedSphericalMaterial{}, e
	}
	out := ReindexedSphericalMaterial{SourceForCanonical: g.Roles, SourceVertexOrdinals: g.Vertices, SourceEdgeOrdinals: g.Edges, SourceUseOrdinals: g.Uses, source: g.Source}
	if g.Placed {
		p, e := ReadPlacedSphericalMaterial(ctx, bytes.NewReader(g.Canonical), l, tol)
		if e != nil {
			return ReindexedSphericalMaterial{}, e
		}
		out.Placed = &p
	} else {
		p, e := ReadSphericalMaterial(ctx, bytes.NewReader(g.Canonical), l, tol)
		if e != nil {
			return ReindexedSphericalMaterial{}, e
		}
		out.Local = &p
	}
	for i := range out.NativeFaceSourceIDs {
		out.NativeFaceSourceIDs[i] = g.Roles[5]
	}
	out.SourceDegreeCurveIDs = [4]int{g.Roles[19], g.Roles[27], g.Roles[29], g.Roles[31]}
	return out, nil
}
func reindexedSphericalProof(ctx context.Context, m ReindexedSphericalMaterial) ([]byte, error) {
	var b bytes.Buffer
	var e error
	if m.Local != nil && m.Placed == nil {
		e = WriteSphericalMaterialJSON(ctx, &b, *m.Local)
	} else if m.Placed != nil && m.Local == nil {
		e = WritePlacedSphericalMaterialJSON(ctx, &b, *m.Placed)
	} else {
		return nil, geom.ErrInvalid
	}
	return b.Bytes(), e
}
func reindexedSphericalEqual(ctx context.Context, a, b ReindexedSphericalMaterial) bool {
	if !reflect.DeepEqual(a.SourceEdgeOrdinals, b.SourceEdgeOrdinals) || !reflect.DeepEqual(a.SourceForCanonical, b.SourceForCanonical) || !reflect.DeepEqual(a.SourceVertexOrdinals, b.SourceVertexOrdinals) || !reflect.DeepEqual(a.SourceUseOrdinals, b.SourceUseOrdinals) || a.NativeFaceSourceIDs != b.NativeFaceSourceIDs || a.SourceDegreeCurveIDs != b.SourceDegreeCurveIDs {
		return false
	}
	x, e := reindexedSphericalProof(ctx, a)
	if e != nil {
		return false
	}
	y, e := reindexedSphericalProof(ctx, b)
	return e == nil && bytes.Equal(x, y)
}
func WriteReindexedSphericalSource(ctx context.Context, w io.Writer, m ReindexedSphericalMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadReindexedSphericalMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid().Tolerance())
	if e != nil {
		return e
	}
	if !reindexedSphericalEqual(ctx, m, p) {
		return geom.ErrInvalid
	}
	n, e := w.Write(p.source)
	if e == nil && n != len(p.source) {
		e = io.ErrShortWrite
	}
	return e
}

type reindexedSphericalEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Proof     json.RawMessage
}

func WriteReindexedSphericalMaterialJSON(ctx context.Context, w io.Writer, m ReindexedSphericalMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WriteReindexedSphericalSource(ctx, io.Discard, m); e != nil {
		return e
	}
	proof, e := reindexedSphericalProof(ctx, m)
	if e != nil {
		return e
	}
	b, e := json.Marshal(reindexedSphericalEnvelope{"plainsolid-iges-reindexed-spherical-material", 1, string(m.source), m.Solid().Tolerance(), proof})
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
func ReadReindexedSphericalMaterialJSON(ctx context.Context, r io.Reader, l Limits) (ReindexedSphericalMaterial, error) {
	bad := func() (ReindexedSphericalMaterial, error) {
		return ReindexedSphericalMaterial{}, geom.ErrInvalid
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
	var q reindexedSphericalEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-reindexed-spherical-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadReindexedSphericalMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	proof, e := reindexedSphericalProof(ctx, p)
	if e != nil {
		return bad()
	}
	var a, c bytes.Buffer
	if json.Compact(&a, proof) != nil || json.Compact(&c, q.Proof) != nil || !bytes.Equal(a.Bytes(), c.Bytes()) {
		return bad()
	}
	return p, nil
}
