package trimmed

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ParabolaCircleFourCrossingsHistory records the original supports for
// all six regions, including the bounded pocket. Circle edges follow
// physical counterclockwise order from +X. Parabola edges and junctions
// follow increasing free coordinate.
type ParabolaCircleFourCrossingsHistory struct {
	Base          int
	RoofFaces     [6]int
	RoofOwners    [6]int
	WallFaces     [6]int
	ParabolaEdges [fourCrossingParabolaPieces]int
	CircleEdges   []int
	Junctions     [4]ParabolaCircleCrossingsJunction
}

// ParabolaCircleFourCrossingsEnvelope is an exact original-source Common or
// Fuse from a parabola meeting one interior circle in four transverse nodes.
type ParabolaCircleFourCrossingsEnvelope struct {
	solid     Solid
	history   ParabolaCircleFourCrossingsHistory
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

func (r *ParabolaCircleFourCrossingsEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *ParabolaCircleFourCrossingsEnvelope) History() (h ParabolaCircleFourCrossingsHistory) {
	if r == nil {
		return h
	}
	h = r.history
	h.CircleEdges = slices.Clone(h.CircleEdges)
	return h
}

// CombineParabolaCircleFourCrossingsSplineGraphs assembles one native Common
// or Fuse shell with the original pocket, both disk caps and all four
// original transverse vertex links. Any unresolved exact trim fails atomically.
func CombineParabolaCircleFourCrossingsSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	op parametric.BooleanOp, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCircleFourCrossingsEnvelope, error) {
	if op != parametric.Intersection && op != parametric.Union {
		return nil, geom.ErrInvalid
	}
	src := [3]*geom.BSplineSurface{base, a, b}
	p, err := proveParabolaCircleFourCrossingPartition(ctx, src, axis, tolerance, o)
	if err != nil {
		return nil, fmt.Errorf("trimmed: four crossings of one circle partition: %w", err)
	}
	s, assembly, err := combineOpenGraphRegions(ctx, base, a, b, p.regions[:], op, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, fmt.Errorf("trimmed: four crossings of one circle shell: %w", err)
	}
	h, err := bindParabolaCircleFourCrossingEnvelope(ctx, s, assembly, p, src, op, o.Proof)
	if err != nil {
		return nil, fmt.Errorf("trimmed: four crossings of one circle ownership: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &ParabolaCircleFourCrossingsEnvelope{solid: s, history: h, sources: src, axis: axis, op: op, tolerance: tolerance}, nil
}

func bindParabolaCircleFourCrossingEnvelope(ctx context.Context, s Solid, assembly SplineGraphBooleanHistory, p parabolaCircleFourCrossingPartition,
	src [3]*geom.BSplineSurface, op parametric.BooleanOp, opts geom.GraphPointContactOptions) (h ParabolaCircleFourCrossingsHistory, err error) {
	topo := s.Topology()
	n := len(p.pieces)
	if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 || topo.InnerWires != 0 ||
		topo.Faces != 13 || topo.Edges != n+18 || topo.Vertices != n+7 || assembly.Base != 6 ||
		len(assembly.WallFaces) != len(h.WallFaces) || len(assembly.ContourEdges) != n {
		return h, geom.ErrConvergence
	}
	h.Base = assembly.Base
	h.WallFaces = [6]int(assembly.WallFaces)
	for i := range h.RoofFaces {
		h.RoofFaces[i] = i
	}
	if err := bindCrossingCircleEnvelopeFaces(ctx, s, assembly, src, op, p.regions[:], p.proof.RegionWitnesses[:], h.RoofOwners[:]); err != nil {
		return h, err
	}
	edges, err := bindParabolaCircleCrossingEdges(s, assembly.ContourEdges, p.pieces)
	if err != nil {
		return h, err
	}
	h.ParabolaEdges = [fourCrossingParabolaPieces]int(edges[:fourCrossingParabolaPieces])
	h.CircleEdges = edges[fourCrossingParabolaPieces:]
	for i, node := range p.proof.Nodes {
		rays := p.rays[i]
		h.Junctions[i], err = bindTwoCrossingCirclesEnvelopeJunction(s, node, [4]int{edges[rays[0]], edges[rays[1]], edges[rays[2]], edges[rays[3]]})
		if err != nil {
			return h, err
		}
	}
	if err := materialVertexLinks(ctx, s, false); err != nil {
		return h, err
	}
	return h, validateExactGraphPartitionEdges(ctx, s, opts)
}

const nativeParabolaCircleFourCrossingsEnvelope = "trimmed-spline-graph-parabola-circle-four-crossings-envelope"

func parabolaCircleFourCrossingsEnvelopeLedgerFor(h ParabolaCircleFourCrossingsHistory) *parabolaCrossingCirclesEnvelopeLedger {
	return &parabolaCrossingCirclesEnvelopeLedger{Base: &h.Base, RoofFaces: h.RoofFaces[:], RoofOwners: h.RoofOwners[:],
		WallFaces: h.WallFaces[:], ParabolaEdges: h.ParabolaEdges[:], CircleEdges: [][]int{h.CircleEdges},
		Junctions: [][]parabolaCircleCrossingsEnvelopeJunction{parabolaCircleEnvelopeJunctionLedger(h.Junctions[:])}}
}

// WriteJSON records the original three stocks and the complete native roof,
// wall, curve and junction incidence ledger.
func (r *ParabolaCircleFourCrossingsEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaCrossingCirclesEnvelopeRecipe{
		Version: 1, Kind: nativeParabolaCircleFourCrossingsEnvelope, Sources: src, Axis: &r.axis, Operation: &r.op,
		Tolerance: r.tolerance, Ledger: parabolaCircleFourCrossingsEnvelopeLedgerFor(r.history),
	})
}

// ReadParabolaCircleFourCrossingsEnvelopeJSON re-proves the original
// six-region arrangement and native links before returning the envelope.
func ReadParabolaCircleFourCrossingsEnvelopeJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCircleFourCrossingsEnvelope, error) {
	var recipe parabolaCrossingCirclesEnvelopeRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCircleFourCrossingsEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	r, err := CombineParabolaCircleFourCrossingsSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, parabolaCircleFourCrossingsEnvelopeLedgerFor(r.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{r.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
