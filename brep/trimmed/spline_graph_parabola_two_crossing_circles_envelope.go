package trimmed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ParabolaTwoCrossingCirclesHistory binds the six signed regions to the
// original roof owners. Parabola edges follow increasing free parameter.
// Each circle's edges begin at physical +X and follow counterclockwise.
type ParabolaTwoCrossingCirclesHistory struct {
	Base          int
	RoofFaces     [6]int
	RoofOwners    [6]int
	WallFaces     [6]int
	ParabolaEdges [twoCrossingCirclesParabolaPieces]int
	CircleEdges   [2][]int
	Junctions     [2][2]ParabolaCircleCrossingsJunction
}

// ParabolaTwoCrossingCirclesEnvelope is an exact original-source Common or
// Fuse with six roof regions and four transverse junctions.
type ParabolaTwoCrossingCirclesEnvelope struct {
	solid     Solid
	history   ParabolaTwoCrossingCirclesHistory
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

func (r *ParabolaTwoCrossingCirclesEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *ParabolaTwoCrossingCirclesEnvelope) History() (h ParabolaTwoCrossingCirclesHistory) {
	if r == nil {
		return h
	}
	h = r.history
	for i, edges := range h.CircleEdges {
		h.CircleEdges[i] = slices.Clone(edges)
	}
	return h
}

// CombineParabolaTwoCrossingCirclesSplineGraphs builds one native Common or
// Fuse shell. All four junctions must be exactly representable as native
// vertices, and the original oriented coedges must close in physical XY.
func CombineParabolaTwoCrossingCirclesSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	axis int, op parametric.BooleanOp, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaTwoCrossingCirclesEnvelope, error) {
	if op != parametric.Intersection && op != parametric.Union {
		return nil, geom.ErrInvalid
	}
	sources := [3]*geom.BSplineSurface{base, a, b}
	p, err := proveParabolaTwoCrossingCirclesPartition(ctx, sources, axis, tolerance, o)
	if err != nil {
		return nil, fmt.Errorf("trimmed: two crossing circles partition: %w", err)
	}
	solid, assembly, err := combineOpenGraphRegions(ctx, base, a, b, p.regions[:], op, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, fmt.Errorf("trimmed: two crossing circles shell: %w", err)
	}
	h, err := bindTwoCrossingCirclesEnvelope(ctx, solid, assembly, p, sources, op, o.Proof)
	if err != nil {
		return nil, fmt.Errorf("trimmed: two crossing circles envelope: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &ParabolaTwoCrossingCirclesEnvelope{solid: solid, history: h, sources: sources, axis: axis, op: op, tolerance: tolerance}, nil
}

func bindTwoCrossingCirclesEnvelope(ctx context.Context, solid Solid, assembly SplineGraphBooleanHistory,
	p parabolaTwoCrossingCirclesPartition, sources [3]*geom.BSplineSurface, op parametric.BooleanOp,
	opts geom.GraphPointContactOptions) (h ParabolaTwoCrossingCirclesHistory, err error) {
	topo := solid.Topology()
	n := len(p.pieces)
	if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 ||
		topo.Faces != 13 || topo.Edges != n+18 || topo.Vertices != n+7 || topo.InnerWires != 0 ||
		assembly.Base != 6 || len(assembly.WallFaces) != len(h.WallFaces) || len(assembly.ContourEdges) != n {
		return h, geom.ErrConvergence
	}
	h, err = bindTwoCrossingCircleEnvelopeRegions(ctx, solid, assembly, p, sources, op)
	if err != nil {
		return h, err
	}
	if err := materialVertexLinks(ctx, solid, false); err != nil {
		return h, err
	}
	return h, validateExactGraphPartitionEdges(ctx, solid, opts)
}

func bindTwoCrossingCircleEnvelopeRegions(ctx context.Context, solid Solid, assembly SplineGraphBooleanHistory,
	p parabolaTwoCrossingCirclesPartition, sources [3]*geom.BSplineSurface, op parametric.BooleanOp) (h ParabolaTwoCrossingCirclesHistory, err error) {
	h.Base = assembly.Base
	copy(h.WallFaces[:], assembly.WallFaces)
	for i := range h.RoofFaces {
		h.RoofFaces[i] = i
	}
	if err := bindCrossingCircleEnvelopeFaces(ctx, solid, assembly, sources, op, p.regions[:], p.proof.RegionWitnesses[:], h.RoofOwners[:]); err != nil {
		return h, err
	}
	part := parabolaCrossingCirclesPartition{pieces: p.pieces, circles: p.circles[:], rays: p.rays[:]}
	edges, err := bindCrossingCircleEnvelopeContours(solid, assembly, part, p.proof.Nodes[:], h.CircleEdges[:], h.Junctions[:])
	if err != nil {
		return h, err
	}
	h.ParabolaEdges = [twoCrossingCirclesParabolaPieces]int(edges[:twoCrossingCirclesParabolaPieces])
	return h, nil
}

func bindCrossingCircleEnvelopeFaces(ctx context.Context, solid Solid, assembly SplineGraphBooleanHistory,
	sources [3]*geom.BSplineSurface, op parametric.BooleanOp, regions []openGraphRegion,
	witnesses [][2]*big.Rat, owners []int) error {
	if solid.faces[assembly.Base].Surface != sources[0] || !solid.faces[assembly.Base].Reversed {
		return geom.ErrConvergence
	}
	if err := validateGraphEnvelopeSplineTrims(ctx, solid.faces[assembly.Base:assembly.Base+1]); err != nil {
		return err
	}
	for i, region := range regions {
		if (region.sign > 0) != (op == parametric.Union) {
			owners[i] = 1
		}
		if err := bindParabolaCirclesRegionFace(ctx, solid.faces[i], sources[1+owners[i]], false, region, witnesses[i]); err != nil {
			return fmt.Errorf("trimmed: original roof region %d: %w", i, err)
		}
	}
	for _, id := range assembly.WallFaces {
		wall := solid.faces[id]
		if len(wall.Outer) != 4 {
			return geom.ErrConvergence
		}
		bottom, hasBase := graphWallSourceTrim(wall.Outer[0].Curve, false, false)
		top, hasRoof := graphWallSourceTrim(wall.Outer[2].Curve, true, false)
		if !hasBase || !hasRoof || bottom.surface != sources[0] || bottom.a != top.a || bottom.b != top.b {
			return geom.ErrConvergence
		}
		ownerFound := false
		for i, region := range regions[:2] {
			for _, piece := range region.loop {
				if piece.side >= 0 && top.a == piece.a && top.b == piece.b && top.surface == sources[1+owners[i]] {
					ownerFound = true
				}
			}
		}
		if !ownerFound {
			return geom.ErrConvergence
		}
	}
	return nil
}

func bindCrossingCircleEnvelopeContours(solid Solid, assembly SplineGraphBooleanHistory, part parabolaCrossingCirclesPartition,
	nodes [][2]geom.ParabolaCircleCrossing, circleEdges [][]int, junctions [][2]ParabolaCircleCrossingsJunction) ([]int, error) {
	edges, err := bindParabolaCircleCrossingEdges(solid, assembly.ContourEdges, part.pieces)
	if err != nil {
		return nil, err
	}
	for i, ring := range part.circles {
		for _, piece := range ring {
			circleEdges[i] = append(circleEdges[i], edges[piece])
		}
		for j, node := range nodes[i] {
			rays := part.rays[i][j]
			ordered := [4]int{edges[rays[0]], edges[rays[1]], edges[rays[2]], edges[rays[3]]}
			junctions[i][j], err = bindTwoCrossingCirclesEnvelopeJunction(solid, node, ordered)
			if err != nil {
				return nil, err
			}
		}
	}
	return edges, nil
}

func bindTwoCrossingCirclesEnvelopeJunction(solid Solid, node geom.ParabolaCircleCrossing,
	rays [4]int) (h ParabolaCircleCrossingsJunction, err error) {
	vertex, err := exactParabolaCircleCrossingVertex(solid, node.Point)
	if err != nil {
		return h, err
	}
	for i, id := range rays {
		before, after := node.SectorRegions[(i+3)%4], node.SectorRegions[i]
		if !slices.Equal(nodalCubicIncidentFaces(solid, id), []int{min(before, after), max(before, after)}) ||
			!opposedGraphContourUses(solid.faces[before].Outer, solid.faces[after].Outer, id) ||
			!matchesParabolaCircleCrossingRay(solid, id, vertex, node.Rays[i]) {
			return h, geom.ErrConvergence
		}
		model := solid.edges[id].Curve.(surfaceTrimCurve)
		boundOwner := false
		for _, face := range []Face{solid.faces[before], solid.faces[after]} {
			boundOwner = boundOwner || (face.Surface == model.surface && slices.ContainsFunc(face.Outer, func(use Use) bool {
				return use.Edge == id && !use.Reversed && use.UVStart == model.a && use.UVEnd == model.b &&
					sameParabolaCircleCrossingTrim(use.Curve, model.curve)
			}))
		}
		if !boundOwner {
			return h, geom.ErrConvergence
		}
	}
	for id, edge := range solid.edges {
		if (edge.Start == vertex || edge.End == vertex) && !slices.Contains(rays[:], id) {
			return h, geom.ErrConvergence
		}
	}
	return ParabolaCircleCrossingsJunction{Vertex: vertex, Rays: rays, Sectors: node.SectorRegions}, nil
}

const nativeParabolaTwoCrossingCirclesEnvelope = "trimmed-spline-graph-parabola-two-crossing-circles-envelope"

type parabolaCrossingCirclesEnvelopeRecipe struct {
	Version   int                                    `json:"version"`
	Kind      string                                 `json:"kind"`
	Sources   []json.RawMessage                      `json:"sources"`
	Axis      *int                                   `json:"freeAxis"`
	Operation *parametric.BooleanOp                  `json:"operation"`
	Tolerance float64                                `json:"tolerance"`
	Ledger    *parabolaCrossingCirclesEnvelopeLedger `json:"ledger"`
}

type parabolaCrossingCirclesEnvelopeLedger struct {
	Base          *int                                        `json:"base"`
	RoofFaces     []int                                       `json:"roofFaces"`
	RoofOwners    []int                                       `json:"roofOwners"`
	ParabolaEdges []int                                       `json:"parabolaEdges"`
	CircleEdges   [][]int                                     `json:"circleEdges"`
	WallFaces     []int                                       `json:"wallFaces"`
	Junctions     [][]parabolaCircleCrossingsEnvelopeJunction `json:"junctions"`
}

func twoCrossingCirclesEnvelopeLedgerFor(h ParabolaTwoCrossingCirclesHistory) *parabolaCrossingCirclesEnvelopeLedger {
	return crossingCirclesEnvelopeLedgerFor(h.Base, h.RoofFaces[:], h.RoofOwners[:], h.ParabolaEdges[:], h.WallFaces[:], h.CircleEdges[:], h.Junctions[:])
}

func crossingCirclesEnvelopeLedgerFor(base int, roofFaces, owners, parabolaEdges, walls []int, circleEdges [][]int,
	junctions [][2]ParabolaCircleCrossingsJunction) *parabolaCrossingCirclesEnvelopeLedger {
	out := &parabolaCrossingCirclesEnvelopeLedger{Base: &base, RoofFaces: roofFaces, RoofOwners: owners,
		ParabolaEdges: parabolaEdges, CircleEdges: circleEdges, WallFaces: walls}
	for _, circle := range junctions {
		out.Junctions = append(out.Junctions, parabolaCircleEnvelopeJunctionLedger(circle[:]))
	}
	return out
}

func parabolaCircleEnvelopeJunctionLedger(junctions []ParabolaCircleCrossingsJunction) []parabolaCircleCrossingsEnvelopeJunction {
	out := make([]parabolaCircleCrossingsEnvelopeJunction, 0, len(junctions))
	for _, node := range junctions {
		out = append(out, parabolaCircleCrossingsEnvelopeJunction{Vertex: &node.Vertex, Rays: node.Rays[:], Sectors: node.Sectors[:]})
	}
	return out
}

// WriteJSON records the original stocks and the complete native contour and
// vertex incidence ledger.
func (r *ParabolaTwoCrossingCirclesEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaCrossingCirclesEnvelopeRecipe{
		Version: 1, Kind: nativeParabolaTwoCrossingCirclesEnvelope, Sources: src, Axis: &r.axis, Operation: &r.op,
		Tolerance: r.tolerance, Ledger: twoCrossingCirclesEnvelopeLedgerFor(r.history),
	})
}

// ReadParabolaTwoCrossingCirclesEnvelopeJSON rebuilds the native envelope
// and re-proves every source and all four vertex links before publication.
func ReadParabolaTwoCrossingCirclesEnvelopeJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaTwoCrossingCirclesEnvelope, error) {
	var recipe parabolaCrossingCirclesEnvelopeRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaTwoCrossingCirclesEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	r, err := CombineParabolaTwoCrossingCirclesSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, twoCrossingCirclesEnvelopeLedgerFor(r.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{r.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
