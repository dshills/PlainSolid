package trimmed

import (
	"context"
	"plainsolid/geom"
)

type MixedBoundaryCurveKind uint8

const (
	MixedBoundarySourceCurve MixedBoundaryCurveKind = iota + 1
	MixedBoundaryTubeCurve
	MixedBoundaryRestrictedCurve
)

// MixedBoundaryEdgeCertificate summarizes an immutable underlying chord proof.
// Restricted curves retain the original source parameter interval, separately
// from their possibly different rational free coordinate.
type MixedBoundaryEdgeCertificate struct {
	Kind         MixedBoundaryCurveKind
	Segments     int
	SourceDomain geom.ExactParameterInterval
	ErrorUpper   float64
}

// CertifiedMixedAlgebraicShellBoundary is an immutable one-dimensional display
// network with exact source ancestry. It is NOT a face fill or a closed mesh.
// Distinct exact vertices are never welded by display-coordinate equality.
type CertifiedMixedAlgebraicShellBoundary struct {
	source     *MixedAlgebraicShellBoolean
	network    CertifiedAlgebraicShellBoundary
	restricted map[int]geom.CertifiedComposedCollectionPolyline
	deflection float64
	embedded   bool
	pairs      int
}

func (s *MixedAlgebraicShellBoolean) TessellateBoundaryCertified(ctx context.Context, o AlgebraicShellBoundaryOptions) (*CertifiedMixedAlgebraicShellBoundary, error) {
	if s == nil || s.section == nil {
		return nil, geom.ErrInvalid
	}
	certificates := map[int]geom.CertifiedComposedCollectionPolyline{}
	network, err := tessellateAlgebraicShellBoundary(ctx, s.boundary, o, certificates)
	if err != nil {
		return nil, err
	}
	return &CertifiedMixedAlgebraicShellBoundary{source: s, network: network, restricted: certificates, deflection: o.Deflection}, nil
}

// TessellateEmbeddedBoundaryCertified additionally rejects nonincident contact
// among all represented chords. This certifies only display-network embedding;
// it proves neither curved-face coverage nor any filled surface intersections.
func (s *MixedAlgebraicShellBoolean) TessellateEmbeddedBoundaryCertified(ctx context.Context, o AlgebraicShellBoundaryEmbeddingOptions) (*CertifiedMixedAlgebraicShellBoundary, error) {
	if ctx == nil || o.MaxPairs < 0 || o.MaxPairs > 16000000 {
		return nil, geom.ErrInvalid
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 2000000
	}
	r, err := s.TessellateBoundaryCertified(ctx, o.Boundary)
	if err != nil {
		return nil, err
	}
	checks, err := checkBoundaryPolylineEmbedding(ctx, r.network.Vertices, r.network.Edges, o.MaxPairs)
	if err != nil {
		return nil, err
	}
	r.embedded = true
	r.pairs = checks
	return r, nil
}
func (r *CertifiedMixedAlgebraicShellBoundary) MarshalJSON() ([]byte, error) {
	return nil, geom.ErrUnrepresentable
}
func (r *CertifiedMixedAlgebraicShellBoundary) Vertices() []geom.Vec3 {
	if r == nil || r.source == nil {
		return nil
	}
	return append([]geom.Vec3(nil), r.network.Vertices...)
}
func (r *CertifiedMixedAlgebraicShellBoundary) Edges() [][]int {
	if r == nil || r.source == nil {
		return nil
	}
	out := make([][]int, len(r.network.Edges))
	for i, p := range r.network.Edges {
		out[i] = append([]int(nil), p...)
	}
	return out
}
func (r *CertifiedMixedAlgebraicShellBoundary) TopologyVertices() []int {
	if r == nil || r.source == nil {
		return nil
	}
	return append([]int(nil), r.network.TopologyVertices...)
}
func (r *CertifiedMixedAlgebraicShellBoundary) Ancestry() []AlgebraicShellBooleanEdge {
	if r == nil || r.source == nil {
		return nil
	}
	return append([]AlgebraicShellBooleanEdge(nil), r.network.Ancestry...)
}
func (r *CertifiedMixedAlgebraicShellBoundary) Coedges() []AlgebraicShellBoundaryCoedge {
	if r == nil || r.source == nil {
		return nil
	}
	out := make([]AlgebraicShellBoundaryCoedge, len(r.network.Coedges))
	for i, p := range r.network.Coedges {
		out[i] = p
		out[i].Vertices = append([]int(nil), p.Vertices...)
	}
	return out
}
func (r *CertifiedMixedAlgebraicShellBoundary) Work() (segments, operations int) {
	if r == nil || r.source == nil {
		return 0, 0
	}
	return r.network.Segments, r.network.ExactOperations
}
func (r *CertifiedMixedAlgebraicShellBoundary) DisplayEmbedded() (bool, int) {
	if r == nil || r.source == nil {
		return false, 0
	}
	return r.embedded, r.pairs
}
func (r *CertifiedMixedAlgebraicShellBoundary) EdgeCertificate(edge int) (MixedBoundaryEdgeCertificate, error) {
	if r == nil || r.source == nil || edge < 0 || edge >= len(r.network.Edges) {
		return MixedBoundaryEdgeCertificate{}, geom.ErrInvalid
	}
	c := MixedBoundaryEdgeCertificate{Segments: len(r.network.Edges[edge]) - 1, ErrorUpper: r.deflection}
	if _, ok := r.network.SourceCertificates[edge]; ok {
		c.Kind = MixedBoundarySourceCurve
		return c, nil
	}
	if _, ok := r.network.SectionCertificates[edge]; ok {
		c.Kind = MixedBoundaryTubeCurve
		return c, nil
	}
	if _, ok := r.restricted[edge]; ok {
		p, e := r.source.SectionPiece(edge)
		if e != nil {
			return c, e
		}
		d, e := p.Restricted.Piece(p.Index)
		if e != nil {
			return c, e
		}
		c.Kind = MixedBoundaryRestrictedCurve
		c.SourceDomain = d.Source
		return c, nil
	}
	return MixedBoundaryEdgeCertificate{}, geom.ErrInvalid
}
