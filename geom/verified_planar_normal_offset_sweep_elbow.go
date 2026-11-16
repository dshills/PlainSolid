package geom

import (
	"context"
	"fmt"
	"math/big"
)

// VerifiedPlanarNormalOffsetSweepElbow is the union of three original
// planar sweep charts with two complete adjacent walls. The two end
// charts share exactly the swept edge common to those middle walls.
type VerifiedPlanarNormalOffsetSweepElbow struct {
	sweeps  [3]*VerifiedNormalOffsetSweep
	pairs   [2]*VerifiedNormalOffsetSweepPair
	contact PlanarSweepElbowContact
}

// PlanarSweepElbowContact identifies the original normalized U,V corner
// of each owner and the common world edge in signed-displacement order.
type PlanarSweepElbowContact struct {
	Corners   [3][2]int
	Endpoints [2][3]*big.Rat
}

// PlanarSweepElbowProof reports exact work and whether all sectors sealed.
type PlanarSweepElbowProof struct {
	Complete        bool
	ExactOperations int
}

// NewVerifiedPlanarNormalOffsetSweepElbow reuses two proved complete
// wall maps on intersecting original middle walls. Each strictly
// convex planar prism must occupy its designated sector of the two
// nonparallel wall planes. Its joined walls and their intersections
// with the other plane have exact original ancestry. The fourth open
// sector is empty.
// The exact arithmetic budget is shared across all three prisms and
// the complete sector proof.
func NewVerifiedPlanarNormalOffsetSweepElbow(ctx context.Context, ab, bc *VerifiedNormalOffsetSweepPair,
	o SurfacePatchOptions) (_ *VerifiedPlanarNormalOffsetSweepElbow, r PlanarSweepElbowProof, err error) {
	if ctx == nil || ab == nil || bc == nil || ab.coordinate == nil || bc.coordinate == nil {
		return nil, r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, r, err
	}
	limits, err := patchOptions(o)
	if err != nil {
		return nil, r, err
	}
	incoming, outgoing := ab.join.Edges[1], bc.join.Edges[0]
	if ab.sweeps[1] != bc.sweeps[0] || incoming/2 == outgoing/2 {
		return nil, r, ErrDomain
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	planes := [2][3]*big.Rat{sweepJoinPlaneCovector(ab.join), sweepJoinPlaneCovector(bc.join)}
	constants := [2]*big.Rat{ab.coordinate, bc.coordinate}
	for i, plane := range planes {
		if err := checkSweepSeparatorCovector(plane, b); err != nil {
			return nil, r, err
		}
		if err := b.check(constants[i]); err != nil {
			return nil, r, err
		}
	}
	exact := boundedRatArithmetic{b: b}
	parallel := sweepLineParallel(&exact, planes[0], planes[1])
	if exact.err != nil {
		return nil, r, exact.err
	}
	if parallel {
		return nil, r, ErrDomain
	}

	sweeps := [3]*VerifiedNormalOffsetSweep{ab.sweeps[0], ab.sweeps[1], bc.sweeps[1]}
	var prisms [3]VerifiedNormalOffsetSweepPlanarPrism
	for i, sweep := range sweeps {
		if err := ctx.Err(); err != nil {
			return nil, r, err
		}
		prism, err := sweep.planarPrismRational(b)
		if err != nil {
			return nil, r, fmt.Errorf("geom: planar sweep elbow owner %d: %w", i, err)
		}
		prisms[i] = prism
	}

	var contact PlanarSweepElbowContact
	middle := &contact.Corners[1]
	middle[incoming/2], middle[outgoing/2] = int(incoming%2), int(outgoing%2)
	contact.Corners[0] = elbowWallPeerCorner(*middle, incoming, ab.join.Edges[0], ab.join.ReverseB)
	contact.Corners[2] = elbowWallPeerCorner(*middle, outgoing, bc.join.Edges[1], bc.join.ReverseB)
	contact.Endpoints = prisms[1].Corners[middle[0]][middle[1]]
	for owner, corner := range contact.Corners {
		edge := prisms[owner].Corners[corner[0]][corner[1]]
		for d, endpoint := range edge {
			for axis, q := range endpoint {
				if q.Cmp(contact.Endpoints[d][axis]) != 0 {
					return nil, r, ErrDomain
				}
			}
		}
	}

	walls := [3][2]int{
		{int(ab.join.Edges[0]), -1},
		{int(incoming), int(outgoing)},
		{-1, int(bc.join.Edges[1])},
	}
	sa, sb := ab.join.AInteriorSign, bc.join.AInteriorSign
	signs := [3][2]int{{sa, sb}, {-sa, sb}, {-sa, -sb}}
	// Every supported original sweep equals the convex hull of its
	// exact corners. A strict sign away from a joined wall proves its
	// exact equality hull. Each end stays in the closed sector of the
	// other plane. On its joined wall the only additional equality is
	// the common swept edge, even if its outer wall extends that plane.
	for owner, prism := range prisms {
		if err := ctx.Err(); err != nil {
			return nil, r, err
		}
		for u, row := range prism.Corners {
			for v, stations := range row {
				uv := [2]int{u, v}
				for j, plane := range planes {
					onPlane := uv == contact.Corners[owner]
					strict := true
					if wall := walls[owner][j]; wall >= 0 {
						onPlane = uv[wall/2] == wall%2
					} else {
						wall := walls[owner][1-j]
						strict = uv[wall/2] == wall%2
					}
					want := signs[owner][j]
					if onPlane {
						want = 0
					}
					for _, p := range stations {
						value := exact.sub(sweepLineDot(&exact, plane, p), constants[j])
						if exact.err != nil {
							return nil, r, exact.err
						}
						if value.Sign() != want && (strict || value.Sign() != 0) {
							return nil, r, ErrDomain
						}
					}
				}
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, r, err
	}
	r.Complete = true
	return &VerifiedPlanarNormalOffsetSweepElbow{sweeps: sweeps, pairs: [2]*VerifiedNormalOffsetSweepPair{ab, bc}, contact: contact}, r, nil
}

func elbowWallPeerCorner(corner [2]int, from, to SurfaceEdge, reversed bool) (peer [2]int) {
	peer[to/2], peer[1-to/2] = int(to%2), corner[1-from/2]
	if reversed {
		peer[1-to/2] = 1 - peer[1-to/2]
	}
	return peer
}

// Sweeps returns the three immutable original sweeps in owner order.
func (e *VerifiedPlanarNormalOffsetSweepElbow) Sweeps() [3]*VerifiedNormalOffsetSweep {
	if e == nil {
		return [3]*VerifiedNormalOffsetSweep{}
	}
	return e.sweeps
}

// Joins returns both exact original wall maps and their owner indices.
func (e *VerifiedPlanarNormalOffsetSweepElbow) Joins() [2]VerifiedNormalOffsetSweepChainJoin {
	var result [2]VerifiedNormalOffsetSweepChainJoin
	if e == nil || e.pairs[0] == nil {
		return result
	}
	for i, pair := range e.pairs {
		join, coordinate := pair.Join()
		result[i] = VerifiedNormalOffsetSweepChainJoin{Owners: [2]int{i, i + 1}, Original: join, Coordinate: coordinate}
	}
	return result
}

// Contact returns the original edge ancestry and owned exact endpoints.
func (e *VerifiedPlanarNormalOffsetSweepElbow) Contact() (r PlanarSweepElbowContact) {
	if e == nil || e.pairs[0] == nil {
		return r
	}
	r.Corners = e.contact.Corners
	for i, endpoint := range e.contact.Endpoints {
		for axis, q := range endpoint {
			r.Endpoints[i][axis] = new(big.Rat).Set(q)
		}
	}
	return r
}

// Boundary returns the fourteen original faces retained after both joins.
func (e *VerifiedPlanarNormalOffsetSweepElbow) Boundary() []VerifiedNormalOffsetSweepBoundaryFace {
	if e == nil || e.pairs[0] == nil {
		return nil
	}
	return verifiedSweepChainBoundary(e.sweeps[:], e.pairs[:])
}

// PlanarPrismsRational returns owned exact corners and volumes for all
// three original prisms, under a common arithmetic budget.
func (e *VerifiedPlanarNormalOffsetSweepElbow) PlanarPrismsRational(ctx context.Context,
	o SurfacePatchOptions) ([]VerifiedNormalOffsetSweepPlanarPrism, error) {
	if ctx == nil || e == nil || e.pairs[0] == nil {
		return nil, ErrInvalid
	}
	return planarSweepPrismsRational(ctx, e.sweeps[:], o)
}
