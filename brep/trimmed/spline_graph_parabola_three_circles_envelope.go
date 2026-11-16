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

// ParabolaThreeCirclesEnvelopeHistory identifies the four original crossing
// links and the separated disk face sharing a circular seam with its
// enclosing exterior region. Circle indices use the source center order.
type ParabolaThreeCirclesEnvelopeHistory struct {
	ParabolaTwoCrossingCirclesHistory
	CrossingCircles   [2]int
	SeparatedCircle   int
	SeparatedRegion   int
	SeparatedFace     int
	SeparatedOwner    int
	SeparatedEdges    [4]int
	SeparatedVertices [4]int
}

// ParabolaThreeCirclesEnvelope holds one source-proved original Common or
// Fuse with seven roof regions and a nested circular roof seam.
type ParabolaThreeCirclesEnvelope struct {
	solid     Solid
	history   ParabolaThreeCirclesEnvelopeHistory
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

func (r *ParabolaThreeCirclesEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *ParabolaThreeCirclesEnvelope) History() ParabolaThreeCirclesEnvelopeHistory {
	if r == nil {
		return ParabolaThreeCirclesEnvelopeHistory{}
	}
	h := r.history
	for i, edges := range h.CircleEdges {
		h.CircleEdges[i] = slices.Clone(edges)
	}
	return h
}

// CombineParabolaThreeCirclesSplineGraphs builds Common or Fuse from the
// original graph stocks after proving both transverse pairs and the
// strictly separated circle in their complete source arrangement.
func CombineParabolaThreeCirclesSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, op parametric.BooleanOp,
	tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaThreeCirclesEnvelope, error) {
	if op != parametric.Intersection && op != parametric.Union {
		return nil, geom.ErrInvalid
	}
	sources := [3]*geom.BSplineSurface{base, a, b}
	p, err := proveParabolaThreeCirclesPartition(ctx, sources, axis, tolerance, o)
	if err != nil {
		return nil, fmt.Errorf("trimmed: three-circle partition: %w", err)
	}
	solid, assembly, err := combineOpenGraphRegions(ctx, base, a, b, p.regions[:], op, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, fmt.Errorf("trimmed: three-circle envelope: %w", err)
	}
	h, err := bindParabolaThreeCirclesEnvelope(ctx, solid, assembly, p, sources, op, o.Proof)
	if err != nil {
		return nil, fmt.Errorf("trimmed: three-circle original face ownership: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &ParabolaThreeCirclesEnvelope{solid: solid, history: h, sources: sources, axis: axis, op: op, tolerance: tolerance}, nil
}

func bindParabolaThreeCirclesEnvelope(ctx context.Context, s Solid, assembly SplineGraphBooleanHistory,
	p parabolaThreeCirclesPartition, sources [3]*geom.BSplineSurface, op parametric.BooleanOp,
	opts geom.GraphPointContactOptions) (h ParabolaThreeCirclesEnvelopeHistory, err error) {
	topo := s.Topology()
	n := len(p.crossing.pieces) + 4
	if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 || topo.InnerWires != 1 ||
		topo.Faces != 14 || topo.Edges != n+18 || topo.Vertices != n+7 || assembly.Base != 7 ||
		len(assembly.WallFaces) != 6 || len(assembly.ContourEdges) != n {
		return h, geom.ErrConvergence
	}
	h.ParabolaTwoCrossingCirclesHistory, err = bindTwoCrossingCircleEnvelopeRegions(ctx, s, assembly, p.crossing, sources, op)
	if err != nil {
		return h, err
	}
	h.CrossingCircles = p.proof.CrossingCircles
	h.SeparatedCircle, h.SeparatedRegion, h.SeparatedFace = p.proof.SeparatedCircle, p.proof.SeparatedRegion, 6
	if (p.regions[6].sign > 0) != (op == parametric.Union) {
		h.SeparatedOwner = 1
	}
	if h.RoofOwners[h.SeparatedRegion] == h.SeparatedOwner {
		return h, geom.ErrConvergence
	}
	disk, exterior := s.faces[h.SeparatedFace], s.faces[h.SeparatedRegion]
	if err := bindParabolaCirclesRegionFace(ctx, disk, sources[1+h.SeparatedOwner], false, p.regions[6], p.proof.RegionWitnesses[6]); err != nil {
		return h, err
	}
	edges, err := bindParabolaCircleCrossingEdges(s, assembly.ContourEdges, p.regions[6].loop)
	if err != nil {
		return h, err
	}
	for _, id := range edges {
		if !opposedGraphContourUses(exterior.Holes[0], disk.Outer, id) {
			return h, geom.ErrConvergence
		}
	}
	h.SeparatedEdges = [4]int(edges)
	h.SeparatedVertices, err = bindOriginalSeparatedCircleRing(s, edges, p.regions[6].loop, p.proof.SeparatedExtrema)
	if err != nil {
		return h, err
	}
	if err := materialVertexLinks(ctx, s, true); err != nil {
		return h, err
	}
	return h, validateExactGraphPartitionEdges(ctx, s, opts)
}

const nativeParabolaThreeCirclesEnvelope = "trimmed-spline-graph-parabola-three-circles-envelope"

type parabolaThreeCirclesEnvelopeRecipe struct {
	parabolaTwoCrossingCirclesEnvelopeRecipe
	Arrangement *parabolaThreeCirclesEnvelopeLedger `json:"arrangement"`
}

type parabolaThreeCirclesEnvelopeLedger struct {
	CrossingCircles []int `json:"crossingCircles"`
	SeparatedCircle *int  `json:"separatedCircle"`
	SeparatedRegion *int  `json:"separatedRegion"`
	Face            *int  `json:"face"`
	Owner           *int  `json:"owner"`
	Edges           []int `json:"edges"`
	Vertices        []int `json:"vertices"`
}

func threeCirclesEnvelopeLedger(h ParabolaThreeCirclesEnvelopeHistory) *parabolaThreeCirclesEnvelopeLedger {
	return &parabolaThreeCirclesEnvelopeLedger{CrossingCircles: h.CrossingCircles[:], SeparatedCircle: &h.SeparatedCircle,
		SeparatedRegion: &h.SeparatedRegion, Face: &h.SeparatedFace, Owner: &h.SeparatedOwner,
		Edges: h.SeparatedEdges[:], Vertices: h.SeparatedVertices[:]}
}

// WriteJSON records all original supports, crossing links, and separated
// disk incidences for whole-source replay.
func (r *ParabolaThreeCirclesEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	sources, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaThreeCirclesEnvelopeRecipe{
		parabolaTwoCrossingCirclesEnvelopeRecipe: parabolaTwoCrossingCirclesEnvelopeRecipe{
			Version: 1, Kind: nativeParabolaThreeCirclesEnvelope, Sources: sources, Axis: &r.axis,
			Operation: &r.op, Tolerance: r.tolerance, Ledger: twoCrossingCirclesEnvelopeLedgerFor(r.history.ParabolaTwoCrossingCirclesHistory),
		},
		Arrangement: threeCirclesEnvelopeLedger(r.history),
	})
}

// ReadParabolaThreeCirclesEnvelopeJSON re-proves the complete arrangement of
// all three original circle factors, original roof owners, and native links.
func ReadParabolaThreeCirclesEnvelopeJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaThreeCirclesEnvelope, error) {
	var recipe parabolaThreeCirclesEnvelopeRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaThreeCirclesEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || recipe.Ledger == nil || recipe.Arrangement == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	r, err := CombineParabolaThreeCirclesSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, twoCrossingCirclesEnvelopeLedgerFor(r.history.ParabolaTwoCrossingCirclesHistory)) ||
		!reflect.DeepEqual(recipe.Arrangement, threeCirclesEnvelopeLedger(r.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{r.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
