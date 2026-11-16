package offset

import (
	"context"
	"math/big"

	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedVerifiedFourChartSweepChainMesh retains the four original
// UV grids and their indexed source stations on a single embedded shell.
// Each shared wall perimeter vertex has two adjacent original owners.
type CertifiedVerifiedFourChartSweepChainMesh struct {
	Mesh                      mesh.Mesh
	UVStations                [4][2][]*big.Rat
	DisplacementStations      []*big.Rat
	VertexStations            [][4]VerifiedSweepPairMeshStation
	OriginalFace              []VerifiedSweepPairMeshFace
	DistanceUpperSquared      *big.Rat
	SquaredFacetNormalTangent *big.Rat `json:",omitempty"`
	SignedDisplayVolume       *big.Rat
	GeometryWork              int
	Collision                 collision.Report
}

// TessellateVerifiedFourChartNormalOffsetSweepChain meshes the eighteen
// original outer faces. It propagates shared stations through all three
// verified wall maps and certifies two-sided distance, optional continuous
// facet deviation, and a single closed outward embedded mesh.
func TessellateVerifiedFourChartNormalOffsetSweepChain(ctx context.Context, chain *geom.VerifiedFourChartNormalOffsetSweepChain,
	o VerifiedSweepMeshOptions) (CertifiedVerifiedFourChartSweepChainMesh, error) {
	var empty CertifiedVerifiedFourChartSweepChainMesh
	if ctx == nil || chain == nil {
		return empty, geom.ErrInvalid
	}
	sweeps := chain.Sweeps()
	if sweeps[0] == nil {
		return empty, geom.ErrInvalid
	}
	joins := chain.Joins()
	out, err := tessellateVerifiedSweepChain[[4][2][]*big.Rat, [4]VerifiedSweepPairMeshStation](
		ctx, sweeps[:], joins[:], chain.Boundary(), o)
	return CertifiedVerifiedFourChartSweepChainMesh(out), err
}
