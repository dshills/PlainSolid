//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedWrappedAnisotropicStationMesh preserves original source face and
// station identities with a two-sided source/display distance bound. The exact
// display volume is diagnostic, not a certified source-volume error estimate.
type CertifiedWrappedAnisotropicStationMesh struct {
	Mesh                                            mesh.Mesh
	Source                                          WrappedAnisotropicStationProof
	OriginalFace                                    []int
	StationVertices                                 [][]int
	SquaredErrors                                   []*big.Rat
	SourceImageSquaredDistance, SignedDisplayVolume *big.Rat
	Collision                                       collision.Report
	Subdivisions, TriangleChecks, StationSamples    int
}

// TessellateWrappedAnisotropicStationLoftCertified authenticates original
// independently scaled weighted stations and uses original exact knot-grid
// residuals. Adjacent rims share one index only after exact rational evaluation
// on both source charts and bit-identical rounding. Cap fans use the same rim;
// their radial contraction preserves the boundary error bound. Global manifold
// links, positive orientation and nonincident-contact rejection remain mandatory.
// Existing station budgets, representable cap-center gate and failure atomicity
// apply unchanged. No circular, homothetic or angular-normal claim is substituted.
func (s Solid) TessellateWrappedAnisotropicStationLoftCertified(ctx context.Context, o CertifiedWrappedStationMeshOptions) (CertifiedWrappedAnisotropicStationMesh, error) {
	empty := CertifiedWrappedAnisotropicStationMesh{}
	proof, bands, e := s.validateWrappedAnisotropicStationLoft(ctx, o.Band.Embedding)
	if e != nil {
		return empty, e
	}
	if !proof.Complete || len(bands) == 0 {
		return empty, geom.ErrConvergence
	}
	q, e := tessellateProvedWrappedStationBands(ctx, o, 1, nil, bands, proof.History, tessellateWrappedAnisotropicBandFrom)
	if e != nil {
		return empty, e
	}
	return CertifiedWrappedAnisotropicStationMesh{q.Mesh, proof, q.OriginalFace, q.StationVertices, q.SquaredErrors, q.SourceImageSquaredDistance, q.SignedDisplayVolume, q.Collision, q.Subdivisions, q.TriangleChecks, q.StationSamples}, nil
}

func tessellateWrappedAnisotropicBandFrom(ctx context.Context, w *geom.WrappedSurfacePatch, o CertifiedWrappedFrustumMeshOptions, minimum int) (CertifiedWrappedFrustumMesh, error) {
	p, e := geom.CertifyWrappedAnisotropicEmbedding(ctx, w, o.Embedding)
	if e != nil {
		return CertifiedWrappedFrustumMesh{}, e
	}
	if !p.Complete {
		return CertifiedWrappedFrustumMesh{}, geom.ErrConvergence
	}
	return tessellateProvedWrappedStarBandFrom(ctx, w, o, minimum, p.Center, [2]*big.Rat{p.LowerZ, p.UpperZ})
}
