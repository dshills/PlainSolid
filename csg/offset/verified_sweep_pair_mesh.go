package offset

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"

	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// VerifiedSweepPairMeshStation indexes one original chart's exact
// (U,V,displacement) grid. Only shared seam vertices have both owners.
type VerifiedSweepPairMeshStation struct {
	Index   [3]int
	Present bool
}

// VerifiedSweepPairMeshFace identifies the original analytic face of a facet.
type VerifiedSweepPairMeshFace struct {
	Owner int
	Side  geom.NormalOffsetSweepSide
}

// CertifiedVerifiedSweepPairMesh is the single outward, embedded, closed
// boundary of a verified two-chart union. UVStations is indexed by original
// owner and parameter direction. All three station axes are exact rationals.
// The squared distance and optional normal tangent bound are in model space.
type CertifiedVerifiedSweepPairMesh struct {
	Mesh                      mesh.Mesh
	UVStations                [2][2][]*big.Rat
	DisplacementStations      []*big.Rat
	VertexStations            [][2]VerifiedSweepPairMeshStation
	OriginalFace              []VerifiedSweepPairMeshFace
	DistanceUpperSquared      *big.Rat
	SquaredFacetNormalTangent *big.Rat `json:",omitempty"`
	SignedDisplayVolume       *big.Rat
	GeometryWork              int
	Collision                 collision.Report
}

// TessellateVerifiedNormalOffsetSweepPair builds a common exact station grid
// on both sides of an authenticated swept wall and retains only the outer
// faces. Each source cell and its actual rounded facets share one rational
// bounding box. Its diameter bounds both directed distances. Optional facet
// angles use continuous outward normals on the original cells. Indexed
// collision and shell validation prove an embedded, positive boundary.
// Failure is atomic and uses the same budgets as a single-sweep display.
func TessellateVerifiedNormalOffsetSweepPair(ctx context.Context, pair *geom.VerifiedNormalOffsetSweepPair,
	o VerifiedSweepMeshOptions) (CertifiedVerifiedSweepPairMesh, error) {
	var empty CertifiedVerifiedSweepPairMesh
	if ctx == nil || pair == nil {
		return empty, geom.ErrInvalid
	}
	join, separator := pair.Join()
	if separator == nil {
		return empty, geom.ErrInvalid
	}
	o, err := checkedVerifiedSweepMeshOptions(o)
	if err != nil {
		return empty, err
	}
	sweeps, faces := pair.Sweeps(), pair.Boundary()
	domainA, displacement := sweeps[0].Domain()
	domainB, _ := sweeps[1].Domain()
	domains := [2]geom.ParameterRectangle{domainA, domainB}
	limit := sweepRat(o.Deflection)
	half := new(big.Rat).Quo(limit, big.NewRat(2, 1))
	limit.Mul(limit, limit)
	span := new(big.Rat).Sub(displacement.Upper, displacement.Lower)
	maxT := 1
	for maxT < 1<<o.MaxDepth && span.Cmp(half) > 0 {
		maxT *= 2
		span.Quo(span, big.NewRat(2, 1))
	}
	work := verifiedSweepMeshWork{options: o}
	for depth := 0; depth <= o.MaxDepth; depth++ {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		n := 1 << depth
		nT := min(n, maxT)
		if o.NormalAngle > 0 {
			// Wall normals can also vary with displacement. Refine
			// that axis after the first few UV levels and check
			// each cell over its entire displacement interval.
			nT = max(nT, n/8)
		}
		count := 8*n*n + 12*n*nT
		if count > o.MaxTriangles || work.remaining() == 0 {
			return empty, ErrLimit
		}
		grids := makeVerifiedSweepPairGrids(domains, displacement, join, n, nT)
		out, err := sampleVerifiedSweepPairMesh(ctx, &work, sweeps, faces, grids, join, limit, n, count)
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
			return empty, fmt.Errorf("offset: verified sweep pair mesh embedding: %w", err)
		}
		shells := locator.Shells()
		if len(shells) != 1 || shells[0].Parent != -1 || shells[0].Depth != 0 ||
			len(shells[0].Triangles) != count || shells[0].SignedVolume.Sign() <= 0 {
			return empty, geom.ErrConvergence
		}
		out.Collision, out.SignedDisplayVolume = locator.BoundaryCollision(), shells[0].SignedVolume
		out.GeometryWork = work.operations
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		return out, nil
	}
	return empty, geom.ErrConvergence
}

type verifiedSweepPairGrid struct {
	verifiedSweepGrid
	// Binary enclosures are used only for source cell bounds. They do
	// not select a vertex or replace any original exact station.
	brackets [2][][2]float64
}

func makeVerifiedSweepPairGrids(domains [2]geom.ParameterRectangle, displacement geom.ExactParameterInterval,
	join geom.VerifiedNormalOffsetSweepJoin, n, nT int) (grids [2]verifiedSweepPairGrid) {
	displacements := sweepRationalStations(displacement.Lower, displacement.Upper, nT)
	grids[0] = makeVerifiedSweepChartGrid(domains[0], displacements, n, [2][]*big.Rat{})
	mappedAxis := 1 - int(join.Edges[1])/2
	domainB := sweepChartUVExtents(domains[1])[mappedAxis]
	stationsA := grids[0].stations[1-int(join.Edges[0])/2]
	var mapped [2][]*big.Rat
	mapped[mappedAxis] = remapVerifiedSweepStations(stationsA, domainB, join)
	grids[1] = makeVerifiedSweepChartGrid(domains[1], displacements, n, mapped)
	return grids
}

func sweepChartUVExtents(domain geom.ParameterRectangle) [2][2]float64 {
	return [2][2]float64{{domain.UMin, domain.UMax}, {domain.VMin, domain.VMax}}
}

func makeVerifiedSweepChartGrid(domain geom.ParameterRectangle, displacements []*big.Rat,
	n int, mapped [2][]*big.Rat) (g verifiedSweepPairGrid) {
	g.segments = [3]int{n, n, len(displacements) - 1}
	g.stations[2] = displacements
	for axis, interval := range sweepChartUVExtents(domain) {
		lo, hi := sweepRat(interval[0]), sweepRat(interval[1])
		g.stations[axis] = mapped[axis]
		if mapped[axis] == nil {
			g.stations[axis] = sweepRationalStations(lo, hi, n)
		}
		g.brackets[axis] = make([][2]float64, n+1)
		for i, station := range g.stations[axis] {
			g.brackets[axis][i] = verifiedSweepPairStationBracket(station, interval)
		}
		collar := new(big.Rat).Sub(hi, lo)
		collar.Quo(collar, big.NewRat(int64(32*n), 1))
		left, _ := new(big.Rat).Add(lo, collar).Float64()
		right, _ := new(big.Rat).Sub(hi, collar).Float64()
		if left <= interval[0] {
			left = math.Nextafter(interval[0], interval[1])
		}
		if right >= interval[1] {
			right = math.Nextafter(interval[1], interval[0])
		}
		g.interiorSide[axis] = [2]float64{left, right}
	}
	return g
}

// The source stations may already have come from another original chart.
func remapVerifiedSweepStations(source []*big.Rat, target [2]float64, join geom.VerifiedNormalOffsetSweepJoin) []*big.Rat {
	out := make([]*big.Rat, len(source))
	first, last := source[0], source[len(source)-1]
	sourceWidth := new(big.Rat).Sub(last, first)
	lo, hi := sweepRat(target[0]), sweepRat(target[1])
	width := new(big.Rat).Sub(hi, lo)
	one := big.NewRat(1, 1)
	for i, p := range source {
		s := new(big.Rat).Sub(p, first)
		s.Quo(s, sourceWidth)
		if join.BStationRatio != nil {
			scaled := new(big.Rat).Mul(s, join.BStationRatio)
			denominator := new(big.Rat).Sub(scaled, s)
			s.Quo(scaled, denominator.Add(denominator, one))
		}
		j := i
		if join.ReverseB {
			j = len(source) - 1 - i
			s.Sub(one, s)
		}
		out[j] = s.Add(lo, s.Mul(width, s))
	}
	return out
}

func verifiedSweepPairStationBracket(station *big.Rat, domain [2]float64) [2]float64 {
	value, exact := station.Float64()
	lo, hi := value, value
	if !exact {
		if sweepRat(value).Cmp(station) > 0 {
			lo = math.Nextafter(value, math.Inf(-1))
		} else {
			hi = math.Nextafter(value, math.Inf(1))
		}
	}
	return [2]float64{max(lo, domain[0]), min(hi, domain[1])}
}

func (g verifiedSweepPairGrid) cellRectangle(lo, hi [3]int, side int) geom.ParameterRectangle {
	limits := [2][2]float64{
		{g.brackets[0][lo[0]][0], g.brackets[0][hi[0]][1]},
		{g.brackets[1][lo[1]][0], g.brackets[1][hi[1]][1]},
	}
	if axis := side / 2; axis < 2 {
		limits[axis][1-side%2] = g.interiorSide[axis][side%2]
	}
	return geom.ParameterRectangle{UMin: limits[0][0], UMax: limits[0][1], VMin: limits[1][0], VMax: limits[1][1]}
}

type verifiedSweepPairSeamGrid struct {
	wallAxis [2]int
	wallEnd  [2]int
	reverse  bool
	width    int
}

func makeVerifiedSweepPairSeamGrid(join geom.VerifiedNormalOffsetSweepJoin, n int) (seam verifiedSweepPairSeamGrid) {
	seam.reverse, seam.width = join.ReverseB, n
	for owner, edge := range join.Edges {
		seam.wallAxis[owner] = int(edge) / 2
		seam.wallEnd[owner] = (int(edge) % 2) * n
	}
	return seam
}

func (s verifiedSweepPairSeamGrid) paired(owner int, index [3]int) (peer [3]int, shared bool) {
	axis := s.wallAxis[owner]
	if index[axis] != s.wallEnd[owner] {
		return peer, false
	}
	other := 1 - owner
	peerAxis := s.wallAxis[other]
	station := index[1-axis]
	if s.reverse {
		station = s.width - station
	}
	peer[peerAxis], peer[1-peerAxis], peer[2] = s.wallEnd[other], station, index[2]
	return peer, true
}

func sampleVerifiedSweepPairMesh(ctx context.Context, w *verifiedSweepMeshWork, sweeps [2]*geom.VerifiedNormalOffsetSweep,
	faces []geom.VerifiedNormalOffsetSweepBoundaryFace, grids [2]verifiedSweepPairGrid,
	join geom.VerifiedNormalOffsetSweepJoin, limit *big.Rat, n, count int) (CertifiedVerifiedSweepPairMesh, error) {
	var empty CertifiedVerifiedSweepPairMesh
	vertexCount := count/2 + 2
	builder := verifiedSweepPairMeshBuilder{
		ids:         [2]map[[3]int]int{make(map[[3]int]int, vertexCount/2), make(map[[3]int]int, vertexCount/2)},
		represented: make([][3]*big.Rat, 0, vertexCount),
		seam:        makeVerifiedSweepPairSeamGrid(join, n),
		out: CertifiedVerifiedSweepPairMesh{
			Mesh:                 mesh.Mesh{Vertices: make([]geom.Vec3, 0, vertexCount), Triangles: make([][3]int, 0, count)},
			DisplacementStations: grids[0].stations[2],
			VertexStations:       make([][2]VerifiedSweepPairMeshStation, 0, vertexCount),
			OriginalFace:         make([]VerifiedSweepPairMeshFace, 0, count),
			DistanceUpperSquared: new(big.Rat),
		},
	}
	for owner := range grids {
		builder.out.UVStations[owner] = [2][]*big.Rat{grids[owner].stations[0], grids[owner].stations[1]}
	}
	if w.options.NormalAngle > 0 {
		builder.out.SquaredFacetNormalTangent = new(big.Rat)
	}
	for _, face := range faces {
		side := int(face.Side)
		grid := grids[face.Owner]
		axis := side / 2
		free := [3][2]int{{1, 2}, {0, 2}, {0, 1}}[axis]
		for i := 0; i < grid.segments[free[0]]; i++ {
			for j := 0; j < grid.segments[free[1]]; j++ {
				lo := [3]int{}
				lo[axis] = (side % 2) * grid.segments[axis]
				lo[free[0]], lo[free[1]] = i, j
				if err := builder.cell(ctx, w, sweeps[face.Owner], grid, face, lo, free, limit); err != nil {
					return empty, err
				}
			}
		}
	}
	if len(builder.out.Mesh.Vertices) != vertexCount || len(builder.out.Mesh.Triangles) != count {
		return empty, geom.ErrConvergence
	}
	return builder.out, nil
}

type verifiedSweepPairMeshBuilder struct {
	out         CertifiedVerifiedSweepPairMesh
	ids         [2]map[[3]int]int
	represented [][3]*big.Rat
	seam        verifiedSweepPairSeamGrid
}

func (b *verifiedSweepPairMeshBuilder) vertex(ctx context.Context, w *verifiedSweepMeshWork,
	sweep *geom.VerifiedNormalOffsetSweep, grid verifiedSweepPairGrid, owner int, station [3]int) (int, error) {
	if id, ok := b.ids[owner][station]; ok {
		return id, nil
	}
	point, exact, err := w.vertex(ctx, sweep, grid.verifiedSweepGrid, station)
	if err != nil {
		return 0, err
	}
	id := len(b.out.Mesh.Vertices)
	b.ids[owner][station] = id
	var original [2]VerifiedSweepPairMeshStation
	original[owner] = VerifiedSweepPairMeshStation{Index: station, Present: true}
	if peer, shared := b.seam.paired(owner, station); shared {
		b.ids[1-owner][peer] = id
		original[1-owner] = VerifiedSweepPairMeshStation{Index: peer, Present: true}
	}
	b.out.VertexStations = append(b.out.VertexStations, original)
	b.out.Mesh.Vertices = append(b.out.Mesh.Vertices, point)
	b.represented = append(b.represented, exact)
	return id, nil
}

func (b *verifiedSweepPairMeshBuilder) cell(ctx context.Context, w *verifiedSweepMeshWork,
	sweep *geom.VerifiedNormalOffsetSweep, grid verifiedSweepPairGrid, face geom.VerifiedNormalOffsetSweepBoundaryFace,
	lo [3]int, free [2]int, limit *big.Rat) error {
	cell, err := certifyVerifiedSweepOuterCell(ctx, w, sweep, grid, face, lo, free, limit, &b.out.Mesh.Vertices,
		func(station [3]int) (int, [3]*big.Rat, error) {
			id, err := b.vertex(ctx, w, sweep, grid, face.Owner, station)
			if err != nil {
				return 0, [3]*big.Rat{}, err
			}
			return id, b.represented[id], nil
		})
	if err != nil {
		return err
	}
	if cell.distance.Cmp(b.out.DistanceUpperSquared) > 0 {
		b.out.DistanceUpperSquared = cell.distance
	}
	if cell.angle != nil && cell.angle.Cmp(b.out.SquaredFacetNormalTangent) > 0 {
		b.out.SquaredFacetNormalTangent = cell.angle
	}
	b.out.Mesh.Triangles = append(b.out.Mesh.Triangles, cell.triangles[:]...)
	source := VerifiedSweepPairMeshFace{Owner: face.Owner, Side: face.Side}
	b.out.OriginalFace = append(b.out.OriginalFace, source, source)
	return nil
}

type certifiedVerifiedSweepOuterCell struct {
	triangles       [2][3]int
	distance, angle *big.Rat
}

func certifyVerifiedSweepOuterCell(ctx context.Context, w *verifiedSweepMeshWork, sweep *geom.VerifiedNormalOffsetSweep,
	grid verifiedSweepPairGrid, face geom.VerifiedNormalOffsetSweepBoundaryFace, lo [3]int, free [2]int, limit *big.Rat,
	vertices *[]geom.Vec3, vertex func([3]int) (int, [3]*big.Rat, error)) (c certifiedVerifiedSweepOuterCell, err error) {
	if err := ctx.Err(); err != nil {
		return c, err
	}
	if w.remaining() == 0 {
		return c, ErrLimit
	}
	hi := lo
	hi[free[0]]++
	hi[free[1]]++
	rect := grid.cellRectangle(lo, hi, int(face.Side))
	t0, t1 := grid.stations[2][lo[2]], grid.stations[2][hi[2]]
	box, err := sweep.BoundsRational(ctx, rect, t0, t1, geom.SurfacePatchOptions{MaxOperations: w.remaining()})
	w.charge(box.ExactOperations)
	if err != nil {
		return c, err
	}
	if verifiedSweepBoxDiameterSquared(box, nil).Cmp(limit) > 0 {
		return c, geom.ErrConvergence
	}
	corners := [4][3]int{lo, lo, hi, lo}
	corners[1][free[0]]++
	corners[3][free[1]]++
	var ids [4]int
	var rounded [4][3]*big.Rat
	for i, station := range corners {
		id, exact, err := vertex(station)
		if err != nil {
			return c, err
		}
		ids[i], rounded[i] = id, exact
	}
	c.distance = verifiedSweepBoxDiameterSquared(box, rounded[:])
	if c.distance.Cmp(limit) > 0 {
		return c, geom.ErrConvergence
	}
	reverse := face.Face.ReverseWinding()
	c.triangles = verifiedSweepCellTriangles(ids, reverse, false)
	if w.options.NormalAngle > 0 {
		if w.remaining() == 0 {
			return c, ErrLimit
		}
		box, err := face.Face.NormalBoundsRational(ctx, rect, t0, t1, geom.SurfacePatchOptions{MaxOperations: w.remaining()})
		w.charge(box.ExactOperations)
		if err != nil {
			return c, err
		}
		c.angle, err = verifiedSweepFacetBound(box.Bounds, c.triangles, *vertices, w.options.NormalAngle)
		if errors.Is(err, geom.ErrConvergence) {
			c.triangles = verifiedSweepCellTriangles(ids, reverse, true)
			c.angle, err = verifiedSweepFacetBound(box.Bounds, c.triangles, *vertices, w.options.NormalAngle)
		}
		if err != nil {
			return c, err
		}
	}
	return c, nil
}
