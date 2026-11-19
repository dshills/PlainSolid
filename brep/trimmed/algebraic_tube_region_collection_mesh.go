//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// AlgebraicTubeRegionCollectionMeshOptions shares boundary, polygon, residual,
// triangle, predicate and collision quotas across the entire selected set.
// Up to eight strictly increasing region indices are supported.
type AlgebraicTubeRegionCollectionMeshOptions struct {
	Component AlgebraicTubeRegionMeshOptions
}
type CertifiedAlgebraicTubeRegionCollectionMesh struct {
	Boundary                                                                                                                         CertifiedAlgebraicTubeRegionsBoundary
	Mesh                                                                                                                             mesh.Mesh
	Components                                                                                                                       []CertifiedAlgebraicTubeRegionMesh
	ComponentVertices                                                                                                                [][]int
	OriginalRegion, OriginalChart                                                                                                    []int
	GlobalBoundaryVertices                                                                                                           map[int]int // certified common network -> mesh
	SourceImageSquaredDistance, SignedDisplayVolume                                                                                  *big.Rat
	DisplayForest                                                                                                                    []trimvalidate.RingPlacement
	ForestProof                                                                                                                      trimvalidate.Report
	Collision                                                                                                                        collision.Report
	TriangleChecks, Predicates, PolygonWork, FootprintTriangles, SpanCellChecks, SpanPieceChecks, ResidualOperations, CollisionPairs int
}

// TessellateSelectedRegionsClosedCertified fills selected complete original
// regions, including disks and annuli nested in one another's holes. A single
// freshly proved boundary network is reused by exact index, never coordinate
// welded. All displayed rings must independently reproduce the complete source
// containment forest. Shared rims between selected regions reject; no singular
// zero-thickness union is inferred. Original region/chart IDs are retained.
// Positive closed manifold components and exact global absence of display
// contacts are mandatory. The image-distance certificate is not a volume-error
// or material-distance bound. Initial scope is original model-XY graph charts.
func (r AlgebraicTubeRegions) TessellateSelectedRegionsClosedCertified(ctx context.Context, selected []int, o AlgebraicTubeRegionCollectionMeshOptions) (CertifiedAlgebraicTubeRegionCollectionMesh, error) {
	empty := CertifiedAlgebraicTubeRegionCollectionMesh{}
	if ctx == nil || r.section.runSource() == nil || len(selected) == 0 || len(selected) > 8 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if r.section.runSource().projection != [2]int{0, 1} {
		return empty, geom.ErrConvergence
	}
	used := map[int]bool{}
	previous := -1
	for _, id := range selected {
		if id < 0 || id >= len(r.regions) || id <= previous {
			return empty, geom.ErrInvalid
		}
		previous = id
		for _, loop := range append([]int{r.regions[id].Outer}, r.regions[id].Holes...) {
			if loop < 0 || loop >= len(r.loops) || used[loop] {
				return empty, geom.ErrConvergence
			}
			used[loop] = true
		}
	}
	work, e := newTubeLensSpanWork(o.Component.Surface)
	if e != nil {
		return empty, e
	}
	c := &o.Component
	if c.Footprint.MaxTriangles == 0 {
		c.Footprint.MaxTriangles = 100000
	}
	if c.Footprint.MaxPredicates == 0 {
		c.Footprint.MaxPredicates = 10000000
	}
	if c.Surface.Closed.Interior.MaxTriangles == 0 {
		c.Surface.Closed.Interior.MaxTriangles = 100000
	}
	if c.Surface.Closed.Interior.MaxTriangleChecks == 0 {
		c.Surface.Closed.Interior.MaxTriangleChecks = 100000
	}
	if c.Surface.Closed.MaxPredicates == 0 {
		c.Surface.Closed.MaxPredicates = 10000000
	}
	if c.Surface.Closed.Collision.MaxPairs == 0 {
		c.Surface.Closed.Collision.MaxPairs = collision.DefaultMaxPairs
	}
	if c.Surface.Closed.Collision.MaxContacts == 0 {
		c.Surface.Closed.Collision.MaxContacts = collision.DefaultMaxContacts
	}
	if c.Footprint.MaxTriangles < 1 || c.Footprint.MaxTriangles > 1000000 || c.Footprint.MaxPredicates < 1 || c.Footprint.MaxPredicates > 100000000 || c.Surface.Closed.Interior.MaxTriangles < 1 || c.Surface.Closed.Interior.MaxTriangles > 1000000 || c.Surface.Closed.Interior.MaxTriangleChecks < 1 || c.Surface.Closed.Interior.MaxTriangleChecks > 1000000 || c.Surface.Closed.MaxPredicates < 1 || c.Surface.Closed.MaxPredicates > 100000000 || c.Surface.Closed.Collision.MaxPairs < 1 || c.Surface.Closed.Collision.MaxPairs > 1000000000 || c.Surface.Closed.Collision.MaxContacts < 1 || c.Surface.Closed.Collision.MaxContacts > 1000000 {
		return empty, geom.ErrInvalid
	}
	polygon, e := c.Footprint.Polygon.WithDefaults()
	if e != nil {
		return empty, e
	}
	c.Footprint.Polygon = polygon
	if c.Footprint.Boundary.Loop.Run.Display.Point.Tolerance == 0 {
		tol := c.Footprint.Boundary.Loop.Run.Display.Deflection * 1e-6
		if tol > 0 {
			c.Footprint.Boundary.Loop.Run.Display.Point.Tolerance = tol
		}
	}
	network, e := r.TessellateBoundaryCertified(ctx, c.Footprint.Boundary)
	if e != nil {
		return empty, e
	}
	forest, proof, e := certifyTubeDisplayForest(ctx, r, network, polygon)
	if e != nil {
		return empty, e
	}
	out := CertifiedAlgebraicTubeRegionCollectionMesh{Boundary: network, DisplayForest: forest, ForestProof: proof, PolygonWork: proof.Work, GlobalBoundaryVertices: map[int]int{}, SourceImageSquaredDistance: new(big.Rat), SignedDisplayVolume: new(big.Rat)}
	footprintPredicates := 0
	for _, region := range selected {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		options := *c
		options.Footprint.Polygon.MaxWork -= out.PolygonWork
		options.Footprint.MaxTriangles -= out.FootprintTriangles
		options.Footprint.MaxPredicates -= footprintPredicates
		options.Surface.Closed.Interior.MaxTriangles -= len(out.Mesh.Triangles)
		options.Surface.Closed.Interior.MaxTriangleChecks -= out.TriangleChecks
		options.Surface.Closed.MaxPredicates -= out.Predicates
		options.Surface.Closed.Collision.MaxPairs -= out.CollisionPairs
		if options.Footprint.Polygon.MaxWork < 1 || options.Footprint.MaxTriangles < 1 || options.Footprint.MaxPredicates < 1 || options.Surface.Closed.Interior.MaxTriangles < 1 || options.Surface.Closed.Interior.MaxTriangleChecks < 1 || options.Surface.Closed.MaxPredicates < 1 || options.Surface.Closed.Collision.MaxPairs < 1 {
			return empty, geom.ErrConvergence
		}
		loops := append([]int{r.regions[region].Outer}, r.regions[region].Holes...)
		footprint, e := triangulateTubeFootprintNetwork(ctx, network, loops, options.Footprint)
		if e != nil {
			return empty, e
		}
		footprint.Region = region
		footprint.Boundary = network
		out.PolygonWork += footprint.PolygonProof.Work
		out.FootprintTriangles += len(footprint.Mesh.Triangles)
		footprintPredicates += footprint.Predicates
		component, e := r.tessellateRegionFootprint(ctx, region, options, footprint, &work)
		if e != nil {
			return empty, e
		}
		out.TriangleChecks += component.TriangleChecks
		out.Predicates += component.Predicates
		out.CollisionPairs += component.Collision.BroadPhasePairs
		remap := make([]int, len(component.Mesh.Vertices))
		for i := range remap {
			remap[i] = -1
		}
		boundaryForLocal := make([]int, len(remap))
		for i := range boundaryForLocal {
			boundaryForLocal[i] = -1
		}
		for boundary, local := range component.GlobalBoundaryVertices {
			if boundary < 0 || boundary >= len(network.Vertices) || local < 0 || local >= len(remap) || component.Mesh.Vertices[local] != network.Vertices[boundary] || boundaryForLocal[local] >= 0 {
				return empty, geom.ErrConvergence
			}
			boundaryForLocal[local] = boundary
		}
		// Allocate deterministically in original local vertex order; maps supply
		// exact ancestry only, never coordinate welding or allocation order.
		for i, p := range component.Mesh.Vertices {
			boundary := boundaryForLocal[i]
			if boundary >= 0 {
				global, ok := out.GlobalBoundaryVertices[boundary]
				if !ok {
					global = len(out.Mesh.Vertices)
					out.Mesh.Vertices = append(out.Mesh.Vertices, p)
					out.GlobalBoundaryVertices[boundary] = global
				}
				remap[i] = global
			} else {
				remap[i] = len(out.Mesh.Vertices)
				out.Mesh.Vertices = append(out.Mesh.Vertices, p)
			}
		}
		for i, t := range component.Mesh.Triangles {
			out.Mesh.Triangles = append(out.Mesh.Triangles, [3]int{remap[t[0]], remap[t[1]], remap[t[2]]})
			out.OriginalRegion = append(out.OriginalRegion, region)
			out.OriginalChart = append(out.OriginalChart, component.OriginalChart[i])
		}
		if component.SourceImageSquaredDistance.Cmp(out.SourceImageSquaredDistance) > 0 {
			out.SourceImageSquaredDistance.Set(component.SourceImageSquaredDistance)
		}
		out.SignedDisplayVolume.Add(out.SignedDisplayVolume, component.SignedDisplayVolume)
		out.Components = append(out.Components, component)
		out.ComponentVertices = append(out.ComponentVertices, remap)
	}
	if e := out.Mesh.ValidateSolid(); e != nil {
		return empty, e
	}
	remaining := c.Surface.Closed.Collision.MaxPairs - out.CollisionPairs
	if remaining < 1 {
		return empty, geom.ErrConvergence
	}
	co := c.Surface.Closed.Collision
	co.MaxPairs = remaining
	report, e := collision.SelfIntersectionsBVHContext(ctx, out.Mesh, co)
	if e != nil {
		return empty, e
	}
	if !report.Complete || len(report.Contacts) != 0 {
		return empty, geom.ErrConvergence
	}
	out.Collision = report
	out.CollisionPairs += report.BroadPhasePairs
	out.SpanCellChecks, out.SpanPieceChecks, out.ResidualOperations = work.cells, work.pieces, work.operations
	return out, nil
}

func certifyTubeDisplayForest(ctx context.Context, r AlgebraicTubeRegions, n CertifiedAlgebraicTubeRegionsBoundary, o trimvalidate.Options) ([]trimvalidate.RingPlacement, trimvalidate.Report, error) {
	var rings []trimvalidate.Loop
	for li, loop := range n.Loops {
		var ids []int
		for _, path := range loop.Run.PieceVertices {
			if len(path) < 2 || len(ids) > 0 && ids[len(ids)-1] != path[0] {
				return nil, trimvalidate.Report{}, geom.ErrConvergence
			}
			if len(ids) == 0 {
				ids = append(ids, path[0])
			}
			ids = append(ids, path[1:]...)
		}
		if len(ids) < 4 || ids[0] != ids[len(ids)-1] {
			return nil, trimvalidate.Report{}, geom.ErrConvergence
		}
		ids = ids[:len(ids)-1]
		ring := make(trimvalidate.Loop, len(ids))
		for j, id := range ids {
			next := ids[(j+1)%len(ids)]
			if id < 0 || id >= len(n.LoopVertices[li]) || next < 0 || next >= len(n.LoopVertices[li]) {
				return nil, trimvalidate.Report{}, geom.ErrConvergence
			}
			a, b := n.Vertices[n.LoopVertices[li][id]], n.Vertices[n.LoopVertices[li][next]]
			ring[j] = trimvalidate.Segment{Start: geom.Vec2{X: a.X, Y: a.Y}, End: geom.Vec2{X: b.X, Y: b.Y}}
		}
		rings = append(rings, ring)
	}
	forest, proof, e := trimvalidate.ClassifyRingForest(ctx, rings, o)
	if e != nil {
		return nil, proof, e
	}
	if proof.Status != trimvalidate.Certified || len(forest) != len(r.loops) {
		return nil, proof, geom.ErrConvergence
	}
	for i, p := range forest {
		if p.Parent != r.loops[i].Parent {
			return nil, proof, geom.ErrConvergence
		}
	}
	return forest, proof, nil
}
