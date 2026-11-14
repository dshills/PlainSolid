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

// ReindexedSphericalCavityMaterial separates original source identities from
// one canonical unplaced or properly placed exact material proof.
type ReindexedSphericalCavityMaterial struct {
	Local                *SphericalCavityMaterial
	Placed               *PlacedSphericalCavityMaterial
	SourceForCanonical   map[int]int
	SourceVertexOrdinals map[int]map[int]int
	SourceEdgeOrdinals   map[int]int
	SourceUseOrdinals    map[int][]int
	NativeFaceSourceIDs  [2]int
	SourceDegreeCurveIDs [2][4]int
	source               []byte
}

func (m ReindexedSphericalCavityMaterial) Solid() trimmed.Solid {
	if m.Local != nil {
		return m.Local.Solid
	}
	if m.Placed != nil {
		return m.Placed.Solid
	}
	return trimmed.Solid{}
}
func ReadReindexedSphericalCavityMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ReindexedSphericalCavityMaterial, error) {
	g, e := readReindexedSphericalCavityGraph(ctx, r, l, tol)
	if e != nil {
		return ReindexedSphericalCavityMaterial{}, e
	}
	out := ReindexedSphericalCavityMaterial{SourceForCanonical: g.Roles, SourceVertexOrdinals: g.Vertices, SourceEdgeOrdinals: g.Edges, SourceUseOrdinals: g.Uses, source: g.Source}
	if g.Placed {
		p, e := ReadPlacedSphericalCavityMaterial(ctx, bytes.NewReader(g.Canonical), l, tol)
		if e != nil {
			return ReindexedSphericalCavityMaterial{}, e
		}
		out.Placed = &p
	} else {
		p, e := ReadSphericalCavityMaterial(ctx, bytes.NewReader(g.Canonical), l, tol)
		if e != nil {
			return ReindexedSphericalCavityMaterial{}, e
		}
		out.Local = &p
	}
	out.NativeFaceSourceIDs = [2]int{g.Roles[5], g.Roles[35]}
	out.SourceDegreeCurveIDs = [2][4]int{{g.Roles[19], g.Roles[27], g.Roles[29], g.Roles[31]}, {g.Roles[49], g.Roles[57], g.Roles[59], g.Roles[61]}}
	return out, nil
}
func reindexedSphericalCavityProof(ctx context.Context, m ReindexedSphericalCavityMaterial) ([]byte, error) {
	var b bytes.Buffer
	var e error
	if m.Local != nil && m.Placed == nil {
		e = WriteSphericalCavityMaterialJSON(ctx, &b, *m.Local)
	} else if m.Placed != nil && m.Local == nil {
		e = WritePlacedSphericalCavityMaterialJSON(ctx, &b, *m.Placed)
	} else {
		return nil, geom.ErrInvalid
	}
	return b.Bytes(), e
}
func reindexedSphericalCavityEqual(ctx context.Context, a, b ReindexedSphericalCavityMaterial) bool {
	if !reflect.DeepEqual(a.SourceEdgeOrdinals, b.SourceEdgeOrdinals) || !reflect.DeepEqual(a.SourceForCanonical, b.SourceForCanonical) || !reflect.DeepEqual(a.SourceVertexOrdinals, b.SourceVertexOrdinals) || !reflect.DeepEqual(a.SourceUseOrdinals, b.SourceUseOrdinals) || a.NativeFaceSourceIDs != b.NativeFaceSourceIDs || a.SourceDegreeCurveIDs != b.SourceDegreeCurveIDs {
		return false
	}
	x, e := reindexedSphericalCavityProof(ctx, a)
	if e != nil {
		return false
	}
	y, e := reindexedSphericalCavityProof(ctx, b)
	return e == nil && bytes.Equal(x, y)
}
func WriteReindexedSphericalCavitySource(ctx context.Context, w io.Writer, m ReindexedSphericalCavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadReindexedSphericalCavityMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid().Tolerance())
	if e != nil {
		return e
	}
	if !reindexedSphericalCavityEqual(ctx, m, p) {
		return geom.ErrInvalid
	}
	n, e := w.Write(p.source)
	if e == nil && n != len(p.source) {
		e = io.ErrShortWrite
	}
	return e
}

type reindexedSphericalCavityEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Proof     json.RawMessage
}

func WriteReindexedSphericalCavityMaterialJSON(ctx context.Context, w io.Writer, m ReindexedSphericalCavityMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := WriteReindexedSphericalCavitySource(ctx, io.Discard, m); e != nil {
		return e
	}
	proof, e := reindexedSphericalCavityProof(ctx, m)
	if e != nil {
		return e
	}
	b, e := json.Marshal(reindexedSphericalCavityEnvelope{"plainsolid-iges-reindexed-spherical-cavity-material", 1, string(m.source), m.Solid().Tolerance(), proof})
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
func ReadReindexedSphericalCavityMaterialJSON(ctx context.Context, r io.Reader, l Limits) (ReindexedSphericalCavityMaterial, error) {
	bad := func() (ReindexedSphericalCavityMaterial, error) {
		return ReindexedSphericalCavityMaterial{}, geom.ErrInvalid
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
	var q reindexedSphericalCavityEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-reindexed-spherical-cavity-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadReindexedSphericalCavityMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	proof, e := reindexedSphericalCavityProof(ctx, p)
	if e != nil {
		return bad()
	}
	var a, c bytes.Buffer
	if json.Compact(&a, proof) != nil || json.Compact(&c, q.Proof) != nil || !bytes.Equal(a.Bytes(), c.Bytes()) {
		return bad()
	}
	return p, nil
}
