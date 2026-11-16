package offset

import (
	"context"

	"plainsolid/brep"
	"plainsolid/geom"
)

// VerifiedPlanarSweepElbowBRep is the embedded outward native boundary
// of the three original sweeps. The source normal sector is retained
// along with the native edge shared by the two original end faces.
type VerifiedPlanarSweepElbowBRep struct {
	sweepPlanarBRepBoundary
	original  *geom.VerifiedPlanarNormalOffsetSweepElbow
	reentrant int
}

func (r VerifiedPlanarSweepElbowBRep) Original() *geom.VerifiedPlanarNormalOffsetSweepElbow {
	return r.original
}

// ReentrantEdge returns the edge index of the certified end-end contact.
// The empty result has no edge and returns -1.
func (r VerifiedPlanarSweepElbowBRep) ReentrantEdge() int {
	if r.original == nil {
		return -1
	}
	return r.reentrant
}

// PlanarBRepFromVerifiedPlanarNormalOffsetSweepElbow builds the
// fourteen retained original faces using the full wall maps and the
// authenticated common swept edge. The corner coordinates must be
// exactly representable in binary64. An independent native boundary
// check proves one closed positive shell and exact volume agreement.
func PlanarBRepFromVerifiedPlanarNormalOffsetSweepElbow(ctx context.Context, elbow *geom.VerifiedPlanarNormalOffsetSweepElbow,
	o VerifiedSweepOpenChainBRepOptions) (VerifiedPlanarSweepElbowBRep, error) {
	prisms, err := elbow.PlanarPrismsRational(ctx, o.Source)
	if err != nil {
		return VerifiedPlanarSweepElbowBRep{}, err
	}
	joins, contact := elbow.Joins(), elbow.Contact()
	boundary, edge, err := buildSweepPlanarBRepBoundary(ctx, prisms, joins[:], elbow.Boundary(), &contact, o.Validation)
	if err != nil {
		return VerifiedPlanarSweepElbowBRep{}, err
	}
	return VerifiedPlanarSweepElbowBRep{sweepPlanarBRepBoundary: boundary, original: elbow, reentrant: edge}, nil
}

func validSweepBRepCornerOwners(previous []SweepBRepVertexSource, source SweepBRepVertexSource,
	contact *geom.PlanarSweepElbowContact) bool {
	if len(previous) == 1 {
		return true
	}
	if contact == nil || len(previous) != 2 || source.Owner != 2 {
		return false
	}
	for owner, reference := range [3]SweepBRepVertexSource{previous[0], previous[1], source} {
		uv := contact.Corners[owner]
		if reference.Owner != owner || reference.Corner != [3]int{uv[0], uv[1], source.Corner[2]} {
			return false
		}
	}
	return true
}

func (b *sweepPlanarBRepBuilder) elbowReentrantEdge(contact geom.PlanarSweepElbowContact,
	faces []brep.Face, sources []SweepBRepFaceSource) (int, bool) {
	uv := contact.Corners[1]
	v := b.corners[1][uv[0]][uv[1]]
	if len(b.sources[v[0]]) != 3 || len(b.sources[v[1]]) != 3 {
		return -1, false
	}
	id, ok := b.edgeIDs[[2]int{min(v[0], v[1]), max(v[0], v[1])}]
	if !ok {
		return -1, false
	}
	mask, count := uint8(0), 0
	for i, face := range faces {
		for _, use := range face.Boundary {
			if use.Edge == id {
				if sources[i].Side >= geom.NormalOffsetSweepLower {
					return -1, false
				}
				mask |= 1 << sources[i].Owner
				count++
			}
		}
	}
	return id, mask == (1<<0|1<<2) && count == 2
}
