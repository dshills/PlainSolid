package offset

import (
	"context"
	"math/big"

	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedVerifiedSweepOpenChainMesh is the outward indexed boundary
// of the original sweep chain. Each vertex row is indexed by original
// chart owner and records the exact U, V and displacement station
// indices of each incident chart. A shared perimeter vertex has two
// adjacent owners. OriginalFace identifies each triangle's source face.
type CertifiedVerifiedSweepOpenChainMesh struct {
	Mesh                      mesh.Mesh
	UVStations                [][2][]*big.Rat
	DisplacementStations      []*big.Rat
	VertexStations            [][]VerifiedSweepPairMeshStation
	OriginalFace              []VerifiedSweepPairMeshFace
	DistanceUpperSquared      *big.Rat
	SquaredFacetNormalTangent *big.Rat `json:",omitempty"`
	SignedDisplayVolume       *big.Rat
	GeometryWork              int
	Collision                 collision.Report
}

// TessellateVerifiedNormalOffsetSweepOpenChain propagates the original
// rational stations across each complete wall map and meshes only the
// retained analytic faces. The shared-grid proof bounds both directed
// distances and, when requested, continuous outward facet deviation.
// Exact indexed collision and shell checks certify one embedded positive
// boundary. Failure returns an empty result.
func TessellateVerifiedNormalOffsetSweepOpenChain(ctx context.Context, chain *geom.VerifiedNormalOffsetSweepOpenChain,
	o VerifiedSweepMeshOptions) (CertifiedVerifiedSweepOpenChainMesh, error) {
	var empty CertifiedVerifiedSweepOpenChainMesh
	if ctx == nil || chain == nil {
		return empty, geom.ErrInvalid
	}
	sweeps := chain.Sweeps()
	if len(sweeps) < 2 {
		return empty, geom.ErrInvalid
	}
	out, err := tessellateVerifiedSweepChain[[][2][]*big.Rat, []VerifiedSweepPairMeshStation](
		ctx, sweeps, chain.Joins(), chain.Boundary(), o)
	return CertifiedVerifiedSweepOpenChainMesh(out), err
}
