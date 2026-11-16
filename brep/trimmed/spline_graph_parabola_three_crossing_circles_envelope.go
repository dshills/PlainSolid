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

const threeCrossingCirclesParabolaPieces = 10

// ParabolaThreeCrossingCirclesHistory binds eight material roof regions to
// their original owners. Parabola edges follow increasing free parameter,
// and circle edges follow physical counterclockwise order from +X.
type ParabolaThreeCrossingCirclesHistory struct {
	Base          int
	RoofFaces     [8]int
	RoofOwners    [8]int
	WallFaces     [6]int
	ParabolaEdges [threeCrossingCirclesParabolaPieces]int
	CircleEdges   [3][]int
	Junctions     [3][2]ParabolaCircleCrossingsJunction
}

// ParabolaThreeCrossingCirclesEnvelope is an original-source Common or Fuse
// with six transverse junctions on three strictly disjoint circles.
type ParabolaThreeCrossingCirclesEnvelope struct {
	solid     Solid
	history   ParabolaThreeCrossingCirclesHistory
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

func (r *ParabolaThreeCrossingCirclesEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *ParabolaThreeCrossingCirclesEnvelope) History() (h ParabolaThreeCrossingCirclesHistory) {
	if r == nil {
		return h
	}
	h = r.history
	for i, edges := range h.CircleEdges {
		h.CircleEdges[i] = slices.Clone(edges)
	}
	return h
}

// CombineParabolaThreeCrossingCirclesSplineGraphs builds a single exact
// Common or Fuse shell. Each original parabola and circle coedge and all six
// transverse vertex links must be representable on the original supports.
func CombineParabolaThreeCrossingCirclesSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	axis int, op parametric.BooleanOp, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaThreeCrossingCirclesEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		tolerance <= 0 || !finite(tolerance) || o.Stock.Domain != (geom.ParameterRectangle{}) ||
		(op != parametric.Intersection && op != parametric.Union) {
		return nil, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, base, a, b, o.Stock)
	if err != nil {
		return nil, err
	}
	proof, err := geom.DiscoverSplineGraphParabolaThreeCrossingCircles(ctx, patches[0], patches[1], axis, o.Proof)
	if err != nil {
		return nil, fmt.Errorf("trimmed: three crossing circles source: %w", err)
	}
	part, err := buildParabolaCrossingCirclesPartition(crossingCirclePartitionSource{
		axis: axis, parabola: proof.ParabolaControls, circles: proof.Circles[:], radii: proof.CircleRadii[:],
		order: proof.CircleOrder[:], nodes: proof.Nodes[:], ccw: proof.CircleCCWRays[:], signs: proof.RegionSides[:],
	})
	if err != nil {
		return nil, fmt.Errorf("trimmed: three crossing circles partition: %w", err)
	}
	s, assembly, err := combineOpenGraphRegions(ctx, base, a, b, part.regions, op, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, fmt.Errorf("trimmed: three crossing circles shell: %w", err)
	}
	src := [3]*geom.BSplineSurface{base, a, b}
	h, err := bindThreeCrossingCirclesEnvelope(ctx, s, assembly, part, proof, src, op, o.Proof)
	if err != nil {
		return nil, fmt.Errorf("trimmed: three crossing circles ownership: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &ParabolaThreeCrossingCirclesEnvelope{solid: s, history: h, sources: src, axis: axis, op: op, tolerance: tolerance}, nil
}

func bindThreeCrossingCirclesEnvelope(ctx context.Context, s Solid, assembly SplineGraphBooleanHistory,
	part parabolaCrossingCirclesPartition, proof *geom.SplineGraphParabolaThreeCrossingCircles,
	src [3]*geom.BSplineSurface, op parametric.BooleanOp, options geom.GraphPointContactOptions) (h ParabolaThreeCrossingCirclesHistory, err error) {
	n := len(part.pieces)
	topo := s.Topology()
	if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 || topo.InnerWires != 0 ||
		topo.Faces != 15 || topo.Edges != n+18 || topo.Vertices != n+5 || assembly.Base != 8 ||
		len(assembly.WallFaces) != len(h.WallFaces) || len(assembly.ContourEdges) != n {
		return h, geom.ErrConvergence
	}
	h.Base = assembly.Base
	h.WallFaces = [6]int(assembly.WallFaces)
	for i := range h.RoofFaces {
		h.RoofFaces[i] = i
	}
	if err := bindCrossingCircleEnvelopeFaces(ctx, s, assembly, src, op, part.regions, proof.RegionWitnesses[:], h.RoofOwners[:]); err != nil {
		return h, err
	}
	edges, err := bindCrossingCircleEnvelopeContours(s, assembly, part, proof.Nodes[:], h.CircleEdges[:], h.Junctions[:])
	if err != nil {
		return h, err
	}
	h.ParabolaEdges = [threeCrossingCirclesParabolaPieces]int(edges[:threeCrossingCirclesParabolaPieces])
	if err := materialVertexLinks(ctx, s, false); err != nil {
		return h, err
	}
	return h, validateExactGraphPartitionEdges(ctx, s, options)
}

const nativeParabolaThreeCrossingCirclesEnvelope = "trimmed-spline-graph-parabola-three-crossing-circles-envelope"

func threeCrossingCirclesEnvelopeLedgerFor(h ParabolaThreeCrossingCirclesHistory) *parabolaCrossingCirclesEnvelopeLedger {
	return crossingCirclesEnvelopeLedgerFor(h.Base, h.RoofFaces[:], h.RoofOwners[:], h.ParabolaEdges[:], h.WallFaces[:], h.CircleEdges[:], h.Junctions[:])
}

// WriteJSON records all original stocks and the complete native incidence.
func (r *ParabolaThreeCrossingCirclesEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaCrossingCirclesEnvelopeRecipe{
		Version: 1, Kind: nativeParabolaThreeCrossingCirclesEnvelope, Sources: src, Axis: &r.axis, Operation: &r.op,
		Tolerance: r.tolerance, Ledger: threeCrossingCirclesEnvelopeLedgerFor(r.history),
	})
}

// ReadParabolaThreeCrossingCirclesEnvelopeJSON re-proves the original
// arrangement and all six native links before publishing the rebuilt shell.
func ReadParabolaThreeCrossingCirclesEnvelopeJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaThreeCrossingCirclesEnvelope, error) {
	var recipe parabolaCrossingCirclesEnvelopeRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaThreeCrossingCirclesEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	r, err := CombineParabolaThreeCrossingCirclesSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, threeCrossingCirclesEnvelopeLedgerFor(r.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{r.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
