//go:build research

package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AffineTriangleFourFanShell is an embedded four-face disk. Adjacent source
// triangles share exactly their declared radial mate; each opposite pair meets
// only at the independently certified common center. No solid is inferred.
type AffineTriangleFourFanShell struct {
	fan      *AffineTriangleFanShell
	opposite [2]*geom.SurfaceAffineTriangleVertexContact
}

// NewAffineTriangleFourFanShell takes cyclic mates F0/F1,F1/F2,F2/F3,F3/F0
// and vertex-only proofs F0/F2,F1/F3, in those original owner orders.
func NewAffineTriangleFourFanShell(ctx context.Context, mates [4]*geom.SurfaceAffineTriangleMate, opposite [2]*geom.SurfaceAffineTriangleVertexContact, o AffineTriangleShellOptions) (*AffineTriangleFourFanShell, error) {
	fan, e := buildAffineTriangleFan(ctx, mates[:], opposite[:], o)
	if e != nil {
		return nil, e
	}
	return &AffineTriangleFourFanShell{fan: fan, opposite: opposite}, nil
}
func (s *AffineTriangleFourFanShell) Vertices() []geom.Vec3 {
	if s == nil {
		return nil
	}
	return s.fan.Vertices()
}
func (s *AffineTriangleFourFanShell) Edges() []parametric.Edge {
	if s == nil {
		return nil
	}
	return s.fan.Edges()
}
func (s *AffineTriangleFourFanShell) Faces() []Face {
	if s == nil {
		return nil
	}
	return s.fan.Faces()
}
func (s *AffineTriangleFourFanShell) Boundary() []WrappedOpenBoundaryUse {
	if s == nil {
		return nil
	}
	return s.fan.Boundary()
}
func (s *AffineTriangleFourFanShell) VertexSources(i int) ([]WrappedOpenVertexSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.fan.VertexSources(i)
}
func (s *AffineTriangleFourFanShell) EdgeSources(i int) ([]WrappedOpenEdgeSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.fan.EdgeSources(i)
}
func (s *AffineTriangleFourFanShell) CenterVertex() int {
	if s == nil {
		return -1
	}
	return s.fan.CenterVertex()
}
func (s *AffineTriangleFourFanShell) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.fan.ExactOperations()
}
func (s *AffineTriangleFourFanShell) Opposite(i int) (*geom.SurfaceAffineTriangleVertexContact, error) {
	if s == nil || i < 0 || i > 1 {
		return nil, geom.ErrInvalid
	}
	return s.opposite[i], nil
}
