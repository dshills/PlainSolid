package offset

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

// VerifiedSweepOpenChainBRepOptions bounds exact source arithmetic and
// independent validation of the assembled planar boundary.
type VerifiedSweepOpenChainBRepOptions struct {
	Source     geom.SurfacePatchOptions
	Validation collision.LocationOptions
}

// SweepBRepFaceSource identifies an original retained chart and side.
type SweepBRepFaceSource struct {
	Owner int
	Side  geom.NormalOffsetSweepSide
}

// SweepBRepVertexSource identifies an original U, V and displacement
// corner. The index on each axis is 0 for the original minimum or 1
// for the original maximum.
type SweepBRepVertexSource struct {
	Owner  int
	Corner [3]int
}

// VerifiedSweepOpenChainBRep is an immutable planar solid and its
// original analytic ancestry. The source charts retain their original
// parameter domains and complete joining maps. A joined wall corner
// records both original owners.
type VerifiedSweepOpenChainBRep struct {
	sweepPlanarBRepBoundary
	original *geom.VerifiedNormalOffsetSweepOpenChain
}

type sweepPlanarBRepBoundary struct {
	solid      brep.Solid
	faceSource []SweepBRepFaceSource
	vertices   [][]SweepBRepVertexSource
	volume     *big.Rat
}

func (r sweepPlanarBRepBoundary) Solid() brep.Solid { return r.solid }

func (r VerifiedSweepOpenChainBRep) Original() *geom.VerifiedNormalOffsetSweepOpenChain {
	return r.original
}

// FaceSources gives one source record per native face in solid order.
func (r sweepPlanarBRepBoundary) FaceSources() []SweepBRepFaceSource {
	return slices.Clone(r.faceSource)
}

// VertexSources gives the raw corner of each incident original owner at
// the native vertex.
func (r sweepPlanarBRepBoundary) VertexSources(vertex int) []SweepBRepVertexSource {
	if vertex < 0 || vertex >= len(r.vertices) {
		return nil
	}
	return slices.Clone(r.vertices[vertex])
}

// ExactVolume returns the positive volume agreed by the original
// analytic prisms and the native planar boundary.
func (r sweepPlanarBRepBoundary) ExactVolume() *big.Rat {
	if r.volume == nil {
		return nil
	}
	return new(big.Rat).Set(r.volume)
}

// PlanarBRepFromVerifiedNormalOffsetSweepOpenChain constructs one
// native planar face per retained original source face. Each source
// must prove an exact planar prism image, and the native vertices must
// represent their exact coordinates in binary64. Shared wall corners
// use the proved original endpoint maps. All faces are outward and the
// completed boundary must be a single embedded positive shell. Failure
// returns an empty result.
func PlanarBRepFromVerifiedNormalOffsetSweepOpenChain(ctx context.Context, chain *geom.VerifiedNormalOffsetSweepOpenChain,
	o VerifiedSweepOpenChainBRepOptions) (VerifiedSweepOpenChainBRep, error) {
	var empty VerifiedSweepOpenChainBRep
	prisms, err := chain.PlanarPrismsRational(ctx, o.Source)
	if err != nil {
		return empty, err
	}
	boundary, _, err := buildSweepPlanarBRepBoundary(ctx, prisms, chain.Joins(), chain.Boundary(), nil, o.Validation)
	if err != nil {
		return empty, err
	}
	return VerifiedSweepOpenChainBRep{sweepPlanarBRepBoundary: boundary, original: chain}, nil
}

func buildSweepPlanarBRepBoundary(ctx context.Context, prisms []geom.VerifiedNormalOffsetSweepPlanarPrism,
	joins []geom.VerifiedNormalOffsetSweepChainJoin, retained []geom.VerifiedNormalOffsetSweepBoundaryFace,
	contact *geom.PlanarSweepElbowContact, validation collision.LocationOptions) (sweepPlanarBRepBoundary, int, error) {
	var empty sweepPlanarBRepBoundary
	builder := sweepPlanarBRepBuilder{
		corners: make([][2][2][2]int, len(prisms)),
		seen:    make(map[geom.Vec3]bool, 4*(len(prisms)+1)),
		edgeIDs: make(map[[2]int]int, 8*len(prisms)+4),
	}
	volume := new(big.Rat)
	for owner, prism := range prisms {
		if err := ctx.Err(); err != nil {
			return empty, -1, err
		}
		if err := builder.addCorners(owner, prism, joins, contact); err != nil {
			return empty, -1, fmt.Errorf("offset: original sweep owner %d corners: %w", owner, err)
		}
		volume.Add(volume, prism.Volume)
	}
	faces := make([]brep.Face, 0, len(retained))
	sources := make([]SweepBRepFaceSource, 0, len(retained))
	for _, source := range retained {
		if err := ctx.Err(); err != nil {
			return empty, -1, err
		}
		faces = append(faces, builder.face(source))
		sources = append(sources, SweepBRepFaceSource{Owner: source.Owner, Side: source.Side})
	}
	if len(builder.points) != 4*(len(prisms)+1) || len(builder.edges) != 8*len(prisms)+4 || len(faces) != 4*len(prisms)+2 {
		return empty, -1, geom.ErrDomain
	}
	reentrant := -1
	if contact != nil {
		var ok bool
		reentrant, ok = builder.elbowReentrantEdge(*contact, faces, sources)
		if !ok {
			return empty, -1, geom.ErrDomain
		}
	}
	solid, err := brep.NewSolid(builder.points, builder.edges, faces)
	if err != nil {
		return empty, -1, fmt.Errorf("offset: planar sweep boundary: %w", err)
	}
	locator, err := collision.NewPointLocator(ctx, solid, validation)
	if err != nil {
		return empty, -1, fmt.Errorf("offset: planar sweep embedding: %w", err)
	}
	shells := locator.Shells()
	if len(shells) != 1 || shells[0].Parent != -1 || shells[0].Depth != 0 ||
		shells[0].SignedVolume.Sign() <= 0 || shells[0].SignedVolume.Cmp(volume) != 0 {
		return empty, -1, geom.ErrDomain
	}
	if err := ctx.Err(); err != nil {
		return empty, -1, err
	}
	return sweepPlanarBRepBoundary{
		solid: solid, faceSource: sources, vertices: builder.sources, volume: volume,
	}, reentrant, nil
}

type sweepPlanarBRepBuilder struct {
	corners [][2][2][2]int
	seen    map[geom.Vec3]bool
	points  []geom.Vec3
	sources [][]SweepBRepVertexSource
	edgeIDs map[[2]int]int
	edges   []brep.Edge
}

func (b *sweepPlanarBRepBuilder) addCorners(owner int, prism geom.VerifiedNormalOffsetSweepPlanarPrism,
	joins []geom.VerifiedNormalOffsetSweepChainJoin, contact *geom.PlanarSweepElbowContact) error {
	for u := range 2 {
		for v := range 2 {
			for d := range 2 {
				raw := [3]int{u, v, d}
				exact := prism.Corners[u][v][d]
				x, xExact := exact[0].Float64()
				y, yExact := exact[1].Float64()
				z, zExact := exact[2].Float64()
				point := geom.Vec3{X: x, Y: y, Z: z}
				if !xExact || !yExact || !zExact || !point.IsFinite() {
					return ErrPrecision
				}
				source := SweepBRepVertexSource{Owner: owner, Corner: raw}
				if owner > 0 {
					peer, shared := previousSweepBRepCorner(joins[owner-1].Original, raw)
					if shared {
						id := b.corners[owner-1][peer[0]][peer[1]][peer[2]]
						if b.points[id] != point || !validSweepBRepCornerOwners(b.sources[id], source, contact) {
							return geom.ErrDomain
						}
						b.corners[owner][u][v][d] = id
						b.sources[id] = append(b.sources[id], source)
						continue
					}
				}
				if b.seen[point] {
					return geom.ErrDomain
				}
				b.seen[point] = true
				b.corners[owner][u][v][d] = len(b.points)
				b.points = append(b.points, point)
				b.sources = append(b.sources, []SweepBRepVertexSource{source})
			}
		}
	}
	return nil
}

func previousSweepBRepCorner(join geom.VerifiedNormalOffsetSweepJoin, corner [3]int) (peer [3]int, shared bool) {
	a, b := int(join.Edges[0]), int(join.Edges[1])
	if corner[b/2] != b%2 {
		return peer, false
	}
	// The positive projective map fixes both normalized endpoints.
	peer[a/2], peer[1-a/2], peer[2] = a%2, corner[1-b/2], corner[2]
	if join.ReverseB {
		peer[1-a/2] = 1 - peer[1-a/2]
	}
	return peer, true
}

func (b *sweepPlanarBRepBuilder) face(source geom.VerifiedNormalOffsetSweepBoundaryFace) brep.Face {
	side := int(source.Side)
	fixed := side / 2
	free := [3][2]int{{1, 2}, {0, 2}, {0, 1}}[fixed]
	var ring [4]int
	for i, pair := range [4][2]int{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
		raw := [3]int{}
		raw[fixed], raw[free[0]], raw[free[1]] = side%2, pair[0], pair[1]
		ring[i] = b.corners[source.Owner][raw[0]][raw[1]][raw[2]]
	}
	if source.Face.ReverseWinding() {
		ring[1], ring[3] = ring[3], ring[1]
	}
	uses := make([]brep.EdgeUse, len(ring))
	for i, start := range ring {
		end := ring[(i+1)%len(ring)]
		key := [2]int{min(start, end), max(start, end)}
		id, exists := b.edgeIDs[key]
		if !exists {
			id = len(b.edges)
			b.edgeIDs[key] = id
			b.edges = append(b.edges, brep.Edge{Start: key[0], End: key[1]})
		}
		uses[i] = brep.EdgeUse{Edge: id, Reversed: start > end}
	}
	return brep.Face{Boundary: uses}
}
