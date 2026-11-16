package offset

import (
	"context"
	"math/big"
	"math/bits"

	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// VerifiedAtlasMeshOptions bounds certified normal offset atlas display.
// Grid controls each source grid. The combined defaults allow 200 million
// grid work units, 100,000 final triangles and one million nonadjacent
// broadphase comparisons.
type VerifiedAtlasMeshOptions struct {
	Atlas                                geom.VerifiedNormalOffsetAtlasOptions
	Grid                                 mesh.NormalOffsetGridOptions
	MaxWork, MaxTriangles, MaxCrossPairs int
}

// CertifiedVerifiedAtlasMesh retains a stitched mesh of the exact
// offset atlas and the largest two-sided squared face distance bound.
// A closed result is oriented consistently with Faces' input winding.
type CertifiedVerifiedAtlasMesh struct {
	Mesh                            mesh.Mesh
	Atlas                           geom.VerifiedNormalOffsetAtlas
	DistanceUpperSquared            *big.Rat
	Segments, BoundaryEdges         int
	GridWork, NonadjacentCandidates int
}

// TessellateVerifiedNormalOffsetAtlas certifies each face, then stitches
// only matching original seam stations and identical rounded vertices.
// It checks the actual rounded vertices lie strictly on the correct sides
// of shared seam planes and tests all nonadjacent mesh pairs. A nonlinear
// original station map needs a coordinated non-tensor sampler and returns
// ErrDomain. Failure returns no mesh or partial assembly.
func TessellateVerifiedNormalOffsetAtlas(ctx context.Context, faces []geom.VerifiedNormalOffsetFace,
	joins []geom.VerifiedNormalOffsetJoin, o VerifiedAtlasMeshOptions) (CertifiedVerifiedAtlasMesh, error) {
	var empty CertifiedVerifiedAtlasMesh
	if ctx == nil || o.MaxWork < 0 || o.MaxWork > 1_000_000_000 ||
		o.MaxTriangles < 0 || o.MaxTriangles > collision.MaxTriangles || o.MaxCrossPairs < 0 {
		return empty, geom.ErrInvalid
	}
	if o.MaxWork == 0 {
		o.MaxWork = 200_000_000
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = collision.MaxTriangles
	}
	if o.MaxCrossPairs == 0 {
		o.MaxCrossPairs = collision.DefaultMaxPairs
	}
	one := big.NewRat(1, 1)
	for _, join := range joins {
		if join.BStationRatio != nil && join.BStationRatio.Cmp(one) != 0 {
			return empty, geom.ErrDomain
		}
	}
	proof, err := geom.CertifyVerifiedNormalOffsetAtlas(ctx, faces, joins, o.Atlas)
	if err != nil {
		return empty, err
	}
	if !proof.Complete {
		return empty, geom.ErrConvergence
	}
	out := CertifiedVerifiedAtlasMesh{Atlas: proof}
	grids := make([]mesh.CertifiedNormalOffsetGrid, len(faces))
	depth := 0
	makeGrid := func(i int, g mesh.NormalOffsetGridOptions) error {
		remaining := o.MaxWork - out.GridWork
		if remaining < 1 {
			return ErrLimit
		}
		cap := g.MaxWork
		if cap == 0 {
			cap = 100_000_000
		}
		g.MaxWork = min(cap, remaining)
		grid, err := mesh.TessellateVerifiedNormalOffsetGrid(ctx, faces[i].Patch, g)
		if err != nil {
			return err
		}
		out.GridWork += grid.GeometryWork
		grids[i] = grid
		return nil
	}
	for i := range grids {
		if err := makeGrid(i, o.Grid); err != nil {
			return empty, err
		}
		depth = max(depth, bits.Len(uint(len(grids[i].UStations)-1))-1)
	}
	out.Segments = 1 << depth
	if 2*out.Segments*out.Segments > o.MaxTriangles/len(faces) {
		return empty, ErrLimit
	}
	for i := range grids {
		if len(grids[i].UStations)-1 == out.Segments {
			continue
		}
		grid := o.Grid
		grid.MinDepth, grid.MaxDepth = depth, depth
		if err := makeGrid(i, grid); err != nil {
			return empty, err
		}
	}
	joined := make(map[[2]int]bool, len(joins))
	perFace := (out.Segments + 1) * (out.Segments + 1)
	roots := make(offsetAtlasMeshUnion, perFace*len(faces))
	for i := range roots {
		roots[i] = i
	}
	seams := make(map[[2]int]geom.VerifiedNormalOffsetAdjacency, len(joins))
	for _, join := range proof.JoinProofs {
		seams[join.Faces] = join.Adjacency
	}
	for _, join := range joins {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		a, b := join.Faces[0], join.Faces[1]
		joined[[2]int{min(a, b), max(a, b)}] = true
		separator := seams[join.Faces]
		if !separator.Complete || separator.Coordinate == nil {
			return empty, geom.ErrConvergence
		}
		for side, face := range join.Faces {
			if err := checkAtlasMeshHalfspace(grids[face], join.Edges[side], separator.Axis,
				separator.Coordinate, separator.InteriorSigns[side]); err != nil {
				return empty, err
			}
		}
		for station := 0; station <= out.Segments; station++ {
			other := station
			if join.ReverseB {
				other = out.Segments - station
			}
			ai, au := atlasGridEdgeStation(grids[a], join.Edges[0], station)
			bi, bu := atlasGridEdgeStation(grids[b], join.Edges[1], other)
			if join.ReverseB {
				bu.Sub(one, bu)
			}
			if au.Cmp(bu) != 0 || grids[a].Mesh.Vertices[ai] != grids[b].Mesh.Vertices[bi] {
				return empty, ErrPrecision
			}
			roots.merge(a*perFace+ai, b*perFace+bi)
		}
	}
	for a := range grids {
		boxA, err := grids[a].Mesh.Bounds()
		if err != nil {
			return empty, err
		}
		for b := a + 1; b < len(grids); b++ {
			if err := ctx.Err(); err != nil {
				return empty, err
			}
			if joined[[2]int{a, b}] {
				continue
			}
			boxB, err := grids[b].Mesh.Bounds()
			if err != nil {
				return empty, err
			}
			if _, overlap := boxA.Intersect(boxB); !overlap {
				continue
			}
			left := o.MaxCrossPairs - out.NonadjacentCandidates
			if left < 1 {
				return empty, ErrLimit
			}
			contact, err := collision.MeshIntersections(grids[a].Mesh, grids[b].Mesh, collision.Options{MaxPairs: left, MaxContacts: 1})
			if err != nil {
				return empty, err
			}
			out.NonadjacentCandidates += contact.BroadPhasePairs
			if !contact.Complete || len(contact.Contacts) != 0 {
				return empty, geom.ErrConvergence
			}
		}
	}
	index := make(map[int]int, len(roots))
	out.DistanceUpperSquared = new(big.Rat)
	for i, grid := range grids {
		if grid.DistanceUpperSquared.Cmp(out.DistanceUpperSquared) > 0 {
			out.DistanceUpperSquared.Set(grid.DistanceUpperSquared)
		}
		for _, triangle := range grid.Mesh.Triangles {
			for k, v := range triangle {
				root := roots.find(i*perFace + v)
				id, exists := index[root]
				if !exists {
					id = len(out.Mesh.Vertices)
					index[root] = id
					out.Mesh.Vertices = append(out.Mesh.Vertices, grid.Mesh.Vertices[v])
				}
				triangle[k] = id
			}
			if faces[i].ReverseWinding {
				triangle[1], triangle[2] = triangle[2], triangle[1]
			}
			out.Mesh.Triangles = append(out.Mesh.Triangles, triangle)
		}
	}
	if err := out.Mesh.Validate(); err != nil {
		return empty, err
	}
	out.BoundaryEdges, err = normalOffsetAtlasMeshIncidence(out.Mesh)
	if err != nil || out.BoundaryEdges != proof.BoundaryEdges*out.Segments {
		return empty, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	return out, nil
}

func atlasGridEdgeStation(g mesh.CertifiedNormalOffsetGrid, edge geom.SurfaceEdge, k int) (int, *big.Rat) {
	n := len(g.VStations)
	var id int
	free := g.VStations
	switch edge {
	case geom.SurfaceEdgeUMin:
		id = k
	case geom.SurfaceEdgeUMax:
		id = (len(g.UStations)-1)*n + k
	case geom.SurfaceEdgeVMin:
		id, free = k*n, g.UStations
	case geom.SurfaceEdgeVMax:
		id, free = k*n+n-1, g.UStations
	}
	lo := new(big.Rat).SetFloat64(free[0])
	span := new(big.Rat).Sub(new(big.Rat).SetFloat64(free[len(free)-1]), lo)
	t := new(big.Rat).Sub(new(big.Rat).SetFloat64(free[k]), lo)
	return id, t.Quo(t, span)
}

func checkAtlasMeshHalfspace(g mesh.CertifiedNormalOffsetGrid, edge geom.SurfaceEdge, axis int,
	coordinate *big.Rat, side int) error {
	n := len(g.VStations)
	for i, p := range g.Mesh.Vertices {
		onSeam := edge == geom.SurfaceEdgeUMin && i/n == 0 ||
			edge == geom.SurfaceEdgeUMax && i/n == len(g.UStations)-1 ||
			edge == geom.SurfaceEdgeVMin && i%n == 0 || edge == geom.SurfaceEdgeVMax && i%n == n-1
		value := [3]float64{p.X, p.Y, p.Z}[axis]
		sign := new(big.Rat).SetFloat64(value).Cmp(coordinate)
		if onSeam && sign != 0 || !onSeam && sign != side {
			return ErrPrecision
		}
	}
	return nil
}

type offsetAtlasMeshUnion []int

func (d offsetAtlasMeshUnion) find(x int) int {
	for d[x] != x {
		d[x] = d[d[x]]
		x = d[x]
	}
	return x
}

func (d offsetAtlasMeshUnion) merge(x, y int) {
	x, y = d.find(x), d.find(y)
	d[max(x, y)] = min(x, y)
}

func normalOffsetAtlasMeshIncidence(m mesh.Mesh) (boundary int, err error) {
	type record struct{ count, sign int }
	edges := make(map[[2]int]record, 2*len(m.Triangles))
	for _, triangle := range m.Triangles {
		for i, a := range triangle {
			b := triangle[(i+1)%3]
			id := [2]int{min(a, b), max(a, b)}
			r := edges[id]
			r.count++
			if a < b {
				r.sign++
			} else {
				r.sign--
			}
			if r.count > 2 || r.count == 2 && r.sign != 0 {
				return 0, geom.ErrConvergence
			}
			edges[id] = r
		}
	}
	for _, r := range edges {
		if r.count == 1 {
			boundary++
		}
	}
	return boundary, nil
}
