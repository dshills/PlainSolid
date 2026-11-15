package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativeClippedPolynomialFuse = "trimmed-clipped-polynomial-fuse"

// ClippedPolynomialFuseWall binds an exposed wall to an original planar
// stock-side atom, ordered in its outward face-loop direction. Low and
// High name the two original height supports. OwnerUV records their exact
// source clocks in that direction. Stations gives the bottom-to-top edge
// chains at the start and end, including subdivisions shared by other walls.
type ClippedPolynomialFuseWall struct {
	Face        int                               `json:"face"`
	Atom        int                               `json:"atom"`
	EndingRoofs uint8                             `json:"endingRoofs"`
	Low         geom.ClippedPolynomialHeightOwner `json:"lowerOwner"`
	High        geom.ClippedPolynomialHeightOwner `json:"upperOwner"`
	Points      [2]geom.Vec2                      `json:"points"`
	TraceEdges  [2]int                            `json:"traceEdges"`
	OwnerUV     [2][2]geom.Vec2                   `json:"ownerUV"`
	Stations    [2][]int                          `json:"stations"`
}

// ClippedPolynomialFuse is a source-backed native volume on two unequal
// projected stocks. Its original base and both roof charts are intact.
type ClippedPolynomialFuse struct {
	solid      Solid
	sources    [3]*geom.BSplineSurface
	tolerance  float64
	axis       int
	ledger     polynomialEnvelopeLedger
	walls      []ClippedPolynomialFuseWall
	footprints [2]geom.ParameterRectangle
	volume     *big.Rat
}

// Boundary gives the native closed union shell.
func (r *ClippedPolynomialFuse) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// ExactVolume gives the source-integrated physical material volume.
func (r *ClippedPolynomialFuse) ExactVolume() *big.Rat {
	if r == nil || r.volume == nil {
		return nil
	}
	return new(big.Rat).Set(r.volume)
}

// Walls gives the original incidence and coedge binding of each exposed wall.
func (r *ClippedPolynomialFuse) Walls() []ClippedPolynomialFuseWall {
	if r == nil {
		return nil
	}
	out := append([]ClippedPolynomialFuseWall(nil), r.walls...)
	for i := range out {
		for j, station := range r.walls[i].Stations {
			out[i].Stations[j] = append([]int(nil), station...)
		}
	}
	return out
}

// FuseClippedPolynomialEnvelope builds the union over exactly representable
// axis-aligned footprints with the same span in one physical axis. The cubic
// crossing and quadratic tangency must both be interior to the overlap. Roof
// step walls keep the original upper and lower traces, including tapered ends.
func FuseClippedPolynomialEnvelope(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	tolerance float64, o PolynomialCubicCrossingTangencyEnvelopeOptions) (*ClippedPolynomialFuse, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	sources := [3]*geom.BSplineSurface{base, a, b}
	var patches [3]*geom.SurfacePatch
	for i, source := range sources {
		p, e := geom.NewSurfacePatch(source, o.Section.Patch)
		if e != nil {
			return nil, e
		}
		patches[i] = p
	}
	material, e := geom.IntegrateClippedSplineGraphPolynomialMaterial(ctx, patches[0], patches[1], patches[2], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	first, second := material.Footprints[0], material.Footprints[1]
	if first == second || !(first.UMin == second.UMin && first.UMax == second.UMax ||
		first.VMin == second.VMin && first.VMax == second.VMax) {
		return nil, geom.ErrDomain
	}
	proof := material.Overlap.Projection.Arrangement
	if !isNativeAlignedPolynomialArrangement(proof, axis) {
		return nil, geom.ErrConvergence
	}
	baseChart, e := polynomialEnvelopeBaseChart(material.Overlap.BaseAlignment)
	if e != nil {
		return nil, e
	}
	roofCharts := projectedPolynomialCharts(material.Overlap.Projection)
	charts := [3]graphEnvelopeChart{baseChart, roofCharts[0], roofCharts[1]}
	builder := clippedFuseBuilder{
		sources: sources, evaluate: exactGraphVertexEvaluator(ctx, o.Proof),
		vertexIDs: make(map[geom.Vec3]int), edgeIDs: make(map[[2]int]int),
	}
	if e = builder.prepareSpans(ctx, material); e != nil {
		return nil, fmt.Errorf("trimmed: clipped Fuse wall stations: %w", e)
	}
	uv, xyz, e := proof.RepresentedVertices()
	if e != nil {
		return nil, e
	}
	pieces, e := polynomialNodePieces(proof.Edges, uv)
	if e != nil {
		return nil, e
	}
	ownerPieces, e := polynomialOwnerPieces(proof, material.Overlap.Projection.Owners)
	if e != nil {
		return nil, e
	}
	faceIDs, ownerIDs, section, baseID, e := builder.addCaps(material, uv, ownerPieces, charts)
	if e != nil {
		return nil, fmt.Errorf("trimmed: clipped Fuse original cap loops: %w", e)
	}
	walls, e := builder.addWalls(ctx)
	if e != nil {
		return nil, fmt.Errorf("trimmed: clipped Fuse exposed wall loops: %w", e)
	}
	solid, e := NewSolidContext(ctx, builder.vertices, builder.edges, builder.faces, tolerance)
	if e != nil {
		return nil, fmt.Errorf("trimmed: clipped Fuse boundary: %w", e)
	}
	// Selected cap interiors have disjoint projections. The material
	// ledger gives exactly the exposed height interval over every wall
	// atom, and station subdivisions bind their closures. Check that this
	// embedded arrangement has one cyclic link at each native vertex.
	top := solid.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 {
		return nil, geom.ErrConvergence
	}
	if e = validateExactGraphPartitionEdges(ctx, solid, o.Proof); e != nil {
		return nil, e
	}
	if e = validateExactSewnVertexLinks(ctx, solid); e != nil {
		return nil, e
	}
	binding, e := bindPolynomialNodeEdgesOnChart(solid, section, proof.Edges, xyz, pieces, true, nil)
	if e != nil {
		return nil, e
	}
	junctionBinding := polynomialEnvelopeBinding{polynomialNodeBinding: binding, roofFaces: faceIDs}
	var contacts [2]PolynomialMixedEnvelopeContact
	for i, node := range proof.Nodes {
		c, e := bindPolynomialEnvelopeJunction(solid, junctionBinding, proof.Edges, node, proof.Vertices[node.Vertex], xyz[node.Vertex])
		if e != nil {
			return nil, e
		}
		c.Order, c.DerivativeGap = proof.ContactOrder[i], proof.DerivativeGap[i]
		contacts[i] = c
	}
	chains := make([][2]int, len(binding.edges))
	for i, chain := range binding.edges {
		chains[i] = [2]int{chain[0], -1}
		if len(chain) == 2 {
			chains[i][1] = chain[1]
		}
	}
	ledger := makePolynomialEnvelopeLedger(chains, binding.vertices, []int{binding.midpoints[1], binding.midpoints[4]}, faceIDs, ownerIDs, baseID, contacts[:])
	return &ClippedPolynomialFuse{solid: solid, sources: sources, axis: axis, tolerance: tolerance, ledger: ledger, walls: walls,
		footprints: material.Footprints, volume: material.Fuse}, nil
}

type clippedFuseEndpoint struct {
	uv    geom.Vec2
	point geom.Vec3
}

type clippedFuseSpan struct {
	atom   int
	points [2]geom.Vec2
	traces [3][2]clippedFuseEndpoint
	source geom.ClippedPolynomialStockBoundary
}

type clippedFuseDirectedSpan struct {
	id      int
	reverse bool
}

func (x clippedFuseDirectedSpan) ends() (int, int) {
	if x.reverse {
		return 1, 0
	}
	return 0, 1
}

type clippedFuseBuilder struct {
	sources   [3]*geom.BSplineSurface
	evaluate  graphVertexEvaluator
	spans     []clippedFuseSpan
	vertices  []geom.Vec3
	edges     []parametric.Edge
	faces     []Face
	vertexIDs map[geom.Vec3]int
	edgeIDs   map[[2]int]int
}
