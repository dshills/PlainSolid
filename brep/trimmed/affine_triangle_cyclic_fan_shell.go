package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"sort"
)

// AffineTriangleVertexPair names an original nonadjacent face pair in ascending
// order. Proof must bind those complete triangles, in exactly that owner order.
type AffineTriangleVertexPair struct {
	Faces [2]int
	Proof *geom.SurfaceAffineTriangleVertexContact
}

// AffineTriangleCyclicFanShell is a globally embedded 5..8-face tensor triangle
// disk. Every unordered original face pair is certified: adjacent pairs share
// one radial edge, and nonadjacent pairs meet only at the common center.
type AffineTriangleCyclicFanShell struct {
	fan   *AffineTriangleFanShell
	pairs []AffineTriangleVertexPair
}

// NewAffineTriangleCyclicFanShell requires cyclic Fi/F(i+1) mates and exactly
// n*(n-3)/2 distinct nonadjacent certificates. Missing/duplicate/adjacent entries
// reject atomically. Default MaxWork is 128M; existing 3/4 entry points retain
// their prior domains and defaults. Persistence is deliberately not widened.
func NewAffineTriangleCyclicFanShell(ctx context.Context, mates []*geom.SurfaceAffineTriangleMate, nonadjacent []AffineTriangleVertexPair, o AffineTriangleShellOptions) (*AffineTriangleCyclicFanShell, error) {
	n := len(mates)
	if n < 5 || n > 8 || len(nonadjacent) != n*(n-3)/2 {
		return nil, geom.ErrInvalid
	}
	if o.MaxWork == 0 {
		o.MaxWork = 128000000
	}
	pairs := append([]AffineTriangleVertexPair(nil), nonadjacent...)
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Faces[0] != pairs[j].Faces[0] {
			return pairs[i].Faces[0] < pairs[j].Faces[0]
		}
		return pairs[i].Faces[1] < pairs[j].Faces[1]
	})
	fan, e := buildAffineTriangleFanChecked(ctx, mates, pairs, o)
	if e != nil {
		return nil, e
	}
	return &AffineTriangleCyclicFanShell{fan: fan, pairs: pairs}, nil
}
func (s *AffineTriangleCyclicFanShell) Vertices() []geom.Vec3 {
	if s == nil {
		return nil
	}
	return s.fan.Vertices()
}
func (s *AffineTriangleCyclicFanShell) Edges() []parametric.Edge {
	if s == nil {
		return nil
	}
	return s.fan.Edges()
}
func (s *AffineTriangleCyclicFanShell) Faces() []Face {
	if s == nil {
		return nil
	}
	return s.fan.Faces()
}
func (s *AffineTriangleCyclicFanShell) Boundary() []WrappedOpenBoundaryUse {
	if s == nil {
		return nil
	}
	return s.fan.Boundary()
}
func (s *AffineTriangleCyclicFanShell) VertexSources(i int) ([]WrappedOpenVertexSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.fan.VertexSources(i)
}
func (s *AffineTriangleCyclicFanShell) EdgeSources(i int) ([]WrappedOpenEdgeSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.fan.EdgeSources(i)
}
func (s *AffineTriangleCyclicFanShell) CenterVertex() int {
	if s == nil {
		return -1
	}
	return s.fan.CenterVertex()
}
func (s *AffineTriangleCyclicFanShell) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.fan.ExactOperations()
}
func (s *AffineTriangleCyclicFanShell) Nonadjacent() []AffineTriangleVertexPair {
	if s == nil {
		return nil
	}
	return append([]AffineTriangleVertexPair(nil), s.pairs...)
}
