package offset

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedVerifiedSweepChainMesh is the single outward and embedded
// boundary of three joined original sweep charts. A vertex records its
// original exact grid index in each incident owner. Only vertices of
// either common wall perimeter have two original owners.
type CertifiedVerifiedSweepChainMesh struct {
	Mesh                      mesh.Mesh
	UVStations                [3][2][]*big.Rat
	DisplacementStations      []*big.Rat
	VertexStations            [][3]VerifiedSweepPairMeshStation
	OriginalFace              []VerifiedSweepPairMeshFace
	DistanceUpperSquared      *big.Rat
	SquaredFacetNormalTangent *big.Rat `json:",omitempty"`
	SignedDisplayVolume       *big.Rat
	GeometryWork              int
	Collision                 collision.Report
}

type verifiedSweepChainUVStations interface {
	[3][2][]*big.Rat | [4][2][]*big.Rat | [][2][]*big.Rat
}

type verifiedSweepChainVertexStations interface {
	[3]VerifiedSweepPairMeshStation | [4]VerifiedSweepPairMeshStation | []VerifiedSweepPairMeshStation
}

// Each public result keeps its own station layout while sharing the
// same refinement, source proofs and indexed collision check.
type verifiedSweepChainMesh[UV verifiedSweepChainUVStations, Vertex verifiedSweepChainVertexStations] struct {
	Mesh                      mesh.Mesh
	UVStations                UV
	DisplacementStations      []*big.Rat
	VertexStations            []Vertex
	OriginalFace              []VerifiedSweepPairMeshFace
	DistanceUpperSquared      *big.Rat
	SquaredFacetNormalTangent *big.Rat `json:",omitempty"`
	SignedDisplayVolume       *big.Rat
	GeometryWork              int
	Collision                 collision.Report
}

// TessellateVerifiedNormalOffsetSweepChain transports one exact station
// grid through both original maps. The fourteen retained outer faces
// share indexed vertices along both joined wall perimeters. Each cell
// certifies both directed distances using its original continuous source
// and delivered rounded facets. Optional continuous facet deviation uses
// each original outer face's normal. Exact indexed collision and shell
// validation certify a single closed positive boundary. Failure is atomic.
func TessellateVerifiedNormalOffsetSweepChain(ctx context.Context, chain *geom.VerifiedNormalOffsetSweepChain,
	o VerifiedSweepMeshOptions) (CertifiedVerifiedSweepChainMesh, error) {
	var empty CertifiedVerifiedSweepChainMesh
	if ctx == nil || chain == nil {
		return empty, geom.ErrInvalid
	}
	sweeps := chain.Sweeps()
	if sweeps[0] == nil {
		return empty, geom.ErrInvalid
	}
	joins := chain.Joins()
	out, err := tessellateVerifiedSweepChain[[3][2][]*big.Rat, [3]VerifiedSweepPairMeshStation](
		ctx, sweeps[:], joins[:], chain.Boundary(), o)
	return CertifiedVerifiedSweepChainMesh(out), err
}

func tessellateVerifiedSweepChain[UV verifiedSweepChainUVStations, Vertex verifiedSweepChainVertexStations](ctx context.Context,
	sweeps []*geom.VerifiedNormalOffsetSweep, joins []geom.VerifiedNormalOffsetSweepChainJoin,
	faces []geom.VerifiedNormalOffsetSweepBoundaryFace, o VerifiedSweepMeshOptions) (verifiedSweepChainMesh[UV, Vertex], error) {
	var empty verifiedSweepChainMesh[UV, Vertex]
	o, err := checkedVerifiedSweepMeshOptions(o)
	if err != nil {
		return empty, err
	}
	domains := make([]geom.ParameterRectangle, len(sweeps))
	domain, displacement := sweeps[0].Domain()
	domains[0] = domain
	for owner := 1; owner < len(sweeps); owner++ {
		domains[owner], _ = sweeps[owner].Domain()
	}
	limit := sweepRat(o.Deflection)
	half := new(big.Rat).Quo(limit, big.NewRat(2, 1))
	limit.Mul(limit, limit)
	span := new(big.Rat).Sub(displacement.Upper, displacement.Lower)
	maxT := 1
	for maxT < 1<<o.MaxDepth && span.Cmp(half) > 0 {
		maxT *= 2
		span.Quo(span, big.NewRat(2, 1))
	}
	w := verifiedSweepMeshWork{options: o}
	for depth := 0; depth <= o.MaxDepth; depth++ {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		n := 1 << depth
		nT := min(n, maxT)
		if o.NormalAngle > 0 {
			nT = max(nT, n/8)
		}
		// Each chart contributes two cap faces. Each of the joins
		// removes two side faces from the original four per chart.
		count := 4*len(sweeps)*n*n + 2*(4*len(sweeps)-2*len(joins))*n*nT
		if count > o.MaxTriangles || w.remaining() == 0 {
			return empty, ErrLimit
		}
		grids := makeVerifiedSweepChainGrids(domains, displacement, joins, n, nT)
		out, err := sampleVerifiedSweepChainMesh[UV, Vertex](ctx, &w, sweeps, faces, grids, joins, limit, n, count)
		if errors.Is(err, geom.ErrConvergence) {
			continue
		}
		if err != nil {
			return empty, err
		}
		locator, err := collision.NewMeshPointLocator(ctx, out.Mesh, nil, collision.LocationOptions{
			MaxTriangles: o.MaxTriangles, MaxCollisionPairs: o.MaxCollisionPairs,
		})
		if err != nil {
			return empty, fmt.Errorf("offset: verified sweep chain mesh embedding: %w", err)
		}
		shells := locator.Shells()
		if len(shells) != 1 || shells[0].Parent != -1 || shells[0].Depth != 0 ||
			len(shells[0].Triangles) != count || shells[0].SignedVolume.Sign() <= 0 {
			return empty, geom.ErrConvergence
		}
		out.Collision, out.SignedDisplayVolume = locator.BoundaryCollision(), shells[0].SignedVolume
		out.GeometryWork = w.operations
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		return out, nil
	}
	return empty, geom.ErrConvergence
}

func makeVerifiedSweepChainGrids(domains []geom.ParameterRectangle, displacement geom.ExactParameterInterval,
	joins []geom.VerifiedNormalOffsetSweepChainJoin, n, nT int) []verifiedSweepPairGrid {
	grids := make([]verifiedSweepPairGrid, len(domains))
	displacements := sweepRationalStations(displacement.Lower, displacement.Upper, nT)
	grids[0] = makeVerifiedSweepChartGrid(domains[0], displacements, n, [2][]*big.Rat{})
	for i, join := range joins {
		sourceAxis := 1 - int(join.Original.Edges[0])/2
		targetAxis := 1 - int(join.Original.Edges[1])/2
		target := sweepChartUVExtents(domains[i+1])[targetAxis]
		var mapped [2][]*big.Rat
		mapped[targetAxis] = remapVerifiedSweepStations(grids[i].stations[sourceAxis], target, join.Original)
		grids[i+1] = makeVerifiedSweepChartGrid(domains[i+1], displacements, n, mapped)
	}
	return grids
}

type verifiedSweepChainMeshBuilder[UV verifiedSweepChainUVStations, Vertex verifiedSweepChainVertexStations] struct {
	out         verifiedSweepChainMesh[UV, Vertex]
	ids         []map[[3]int]int
	represented [][3]*big.Rat
	seams       []verifiedSweepPairSeamGrid
}

func sampleVerifiedSweepChainMesh[UV verifiedSweepChainUVStations, Vertex verifiedSweepChainVertexStations](
	ctx context.Context, w *verifiedSweepMeshWork, sweeps []*geom.VerifiedNormalOffsetSweep,
	faces []geom.VerifiedNormalOffsetSweepBoundaryFace, grids []verifiedSweepPairGrid,
	joins []geom.VerifiedNormalOffsetSweepChainJoin, limit *big.Rat, n, count int) (verifiedSweepChainMesh[UV, Vertex], error) {
	var empty verifiedSweepChainMesh[UV, Vertex]
	vertexCount := count/2 + 2
	builder := verifiedSweepChainMeshBuilder[UV, Vertex]{
		ids:         make([]map[[3]int]int, len(sweeps)),
		seams:       make([]verifiedSweepPairSeamGrid, len(joins)),
		represented: make([][3]*big.Rat, 0, vertexCount),
		out: verifiedSweepChainMesh[UV, Vertex]{
			Mesh:                 mesh.Mesh{Vertices: make([]geom.Vec3, 0, vertexCount), Triangles: make([][3]int, 0, count)},
			DisplacementStations: grids[0].stations[2],
			VertexStations:       make([]Vertex, vertexCount),
			OriginalFace:         make([]VerifiedSweepPairMeshFace, 0, count),
			DistanceUpperSquared: new(big.Rat),
		},
	}
	if dynamic, ok := any(&builder.out.UVStations).(*[][2][]*big.Rat); ok {
		*dynamic = make([][2][]*big.Rat, len(sweeps))
	}
	if dynamic, ok := any(builder.out.VertexStations).([][]VerifiedSweepPairMeshStation); ok {
		// Give the original owner rows independent spans of one ledger.
		width := len(sweeps)
		stations := make([]VerifiedSweepPairMeshStation, vertexCount*width)
		for i := range dynamic {
			lo, hi := i*width, (i+1)*width
			dynamic[i] = stations[lo:hi:hi]
		}
	}
	for i, join := range joins {
		builder.seams[i] = makeVerifiedSweepPairSeamGrid(join.Original, n)
	}
	for owner, grid := range grids {
		builder.ids[owner] = make(map[[3]int]int, vertexCount/len(sweeps))
		builder.out.UVStations[owner] = [2][]*big.Rat{grid.stations[0], grid.stations[1]}
	}
	if w.options.NormalAngle > 0 {
		builder.out.SquaredFacetNormalTangent = new(big.Rat)
	}
	for _, face := range faces {
		grid := grids[face.Owner]
		side := int(face.Side)
		axis := side / 2
		free := [3][2]int{{1, 2}, {0, 2}, {0, 1}}[axis]
		identity := VerifiedSweepPairMeshFace{Owner: face.Owner, Side: face.Side}
		vertex := func(station [3]int) (int, [3]*big.Rat, error) {
			return builder.vertex(ctx, w, sweeps[face.Owner], grid, face.Owner, station)
		}
		for i := 0; i < grid.segments[free[0]]; i++ {
			for j := 0; j < grid.segments[free[1]]; j++ {
				lo := [3]int{}
				lo[axis] = (side % 2) * grid.segments[axis]
				lo[free[0]], lo[free[1]] = i, j
				cell, err := certifyVerifiedSweepOuterCell(ctx, w, sweeps[face.Owner], grid, face, lo, free, limit,
					&builder.out.Mesh.Vertices, vertex)
				if err != nil {
					return empty, err
				}
				builder.out.Mesh.Triangles = append(builder.out.Mesh.Triangles, cell.triangles[:]...)
				builder.out.OriginalFace = append(builder.out.OriginalFace, identity, identity)
				if cell.distance.Cmp(builder.out.DistanceUpperSquared) > 0 {
					builder.out.DistanceUpperSquared = cell.distance
				}
				if cell.angle != nil && cell.angle.Cmp(builder.out.SquaredFacetNormalTangent) > 0 {
					builder.out.SquaredFacetNormalTangent = cell.angle
				}
			}
		}
	}
	if len(builder.out.Mesh.Vertices) != vertexCount || len(builder.out.Mesh.Triangles) != count {
		return empty, geom.ErrConvergence
	}
	return builder.out, nil
}

func (b *verifiedSweepChainMeshBuilder[UV, Vertex]) vertex(ctx context.Context, w *verifiedSweepMeshWork,
	sweep *geom.VerifiedNormalOffsetSweep, grid verifiedSweepPairGrid, owner int, station [3]int) (int, [3]*big.Rat, error) {
	if id, ok := b.ids[owner][station]; ok {
		return id, b.represented[id], nil
	}
	point, exact, err := w.vertex(ctx, sweep, grid.verifiedSweepGrid, station)
	if err != nil {
		return 0, exact, err
	}
	id := len(b.out.Mesh.Vertices)
	if id >= len(b.out.VertexStations) {
		return 0, exact, geom.ErrConvergence
	}
	b.ids[owner][station] = id
	original := b.out.VertexStations[id]
	original[owner] = VerifiedSweepPairMeshStation{Index: station, Present: true}
	for i, seam := range b.seams {
		local := owner - i
		if local < 0 || local > 1 {
			continue
		}
		if peer, shared := seam.paired(local, station); shared {
			peerOwner := i + 1 - local
			b.ids[peerOwner][peer] = id
			original[peerOwner] = VerifiedSweepPairMeshStation{Index: peer, Present: true}
			break
		}
	}
	b.out.VertexStations[id] = original
	b.out.Mesh.Vertices = append(b.out.Mesh.Vertices, point)
	b.represented = append(b.represented, exact)
	return id, exact, nil
}
