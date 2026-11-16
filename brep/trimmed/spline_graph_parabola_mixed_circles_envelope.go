package trimmed

import (
	"context"
	"fmt"
	"io"
	"reflect"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ParabolaMixedCirclesEnvelopeHistory binds the crossing sectors and the
// original separated circle shared by its disk face and exterior roof hole.
type ParabolaMixedCirclesEnvelopeHistory struct {
	ParabolaCircleCrossingsHistory
	CrossingCircle     int
	SeparatedFace      int
	SeparatedOwner     int
	SeparatedEnclosing int
	SeparatedEdges     [4]int
	SeparatedVertices  [4]int
}

// ParabolaMixedCirclesEnvelope holds one native source-proved Common or Fuse
// with two transverse junctions and one disjoint circular roof seam.
type ParabolaMixedCirclesEnvelope struct {
	solid     Solid
	history   ParabolaMixedCirclesEnvelopeHistory
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

func (r *ParabolaMixedCirclesEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *ParabolaMixedCirclesEnvelope) History() ParabolaMixedCirclesEnvelopeHistory {
	if r == nil {
		return ParabolaMixedCirclesEnvelopeHistory{}
	}
	return r.history
}

// CombineParabolaMixedCirclesSplineGraphs builds Common or Fuse from the
// original graph stocks and retains their five exact signed source regions.
func CombineParabolaMixedCirclesSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	axis int, op parametric.BooleanOp, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaMixedCirclesEnvelope, error) {
	if op != parametric.Intersection && op != parametric.Union {
		return nil, geom.ErrInvalid
	}
	sources := [3]*geom.BSplineSurface{base, a, b}
	p, err := proveParabolaMixedCirclesPartition(ctx, sources, axis, tolerance, o)
	if err != nil {
		return nil, fmt.Errorf("trimmed: mixed parabola-circle partition: %w", err)
	}
	solid, assembly, err := combineOpenGraphRegions(ctx, base, a, b, p.regions[:], op, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, fmt.Errorf("trimmed: mixed parabola-circle shell: %w", err)
	}
	h, err := bindParabolaMixedCirclesEnvelope(ctx, solid, assembly, p, sources, op, o.Proof)
	if err != nil {
		return nil, fmt.Errorf("trimmed: mixed parabola-circle envelope: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &ParabolaMixedCirclesEnvelope{solid: solid, history: h, sources: sources, axis: axis, op: op, tolerance: tolerance}, nil
}

func bindParabolaMixedCirclesEnvelope(ctx context.Context, s Solid, assembly SplineGraphBooleanHistory, p parabolaMixedCirclesPartition,
	sources [3]*geom.BSplineSurface, op parametric.BooleanOp, o geom.GraphPointContactOptions) (h ParabolaMixedCirclesEnvelopeHistory, err error) {
	topo := s.Topology()
	if len(topo.Shells) != 1 || topo.CollapsedEdges != 0 || topo.EulerCharacteristic != 2 || topo.InnerWires != 1 ||
		topo.Faces != 12 || topo.Edges != 29 || topo.Vertices != 20 || assembly.Base != 5 ||
		len(assembly.ContourEdges) != 11 || len(assembly.WallFaces) != 6 {
		return h, geom.ErrConvergence
	}
	h.ParabolaCircleCrossingsHistory, err = bindParabolaCrossingEnvelopeRegions(ctx, s, assembly, p.crossing, sources, op, o, true)
	if err != nil {
		return h, fmt.Errorf("trimmed: original crossing regions: %w", err)
	}
	h.CrossingCircle = p.proof.CrossingCircle
	h.SeparatedFace, h.SeparatedEnclosing = 4, p.proof.SeparatedRegion
	if (p.regions[4].sign > 0) != (op == parametric.Union) {
		h.SeparatedOwner = 1
	}
	if h.SeparatedOwner == h.RoofOwners[h.SeparatedEnclosing] {
		return h, geom.ErrConvergence
	}
	disk, exterior := s.faces[h.SeparatedFace], s.faces[h.SeparatedEnclosing]
	if err := bindParabolaCirclesRegionFace(ctx, disk, sources[1+h.SeparatedOwner], false, p.regions[4], p.proof.RegionWitnesses[4]); err != nil {
		return h, fmt.Errorf("trimmed: original separated disk region: %w", err)
	}
	edges, err := bindParabolaCircleCrossingEdges(s, assembly.ContourEdges, p.regions[4].loop)
	if err != nil {
		return h, err
	}
	for _, id := range edges {
		if !opposedGraphContourUses(exterior.Holes[0], disk.Outer, id) {
			return h, geom.ErrConvergence
		}
	}
	h.SeparatedEdges = [4]int(edges)
	h.SeparatedVertices, err = bindParabolaMixedCircleRing(s, edges, p)
	if err != nil {
		return h, fmt.Errorf("trimmed: original separated circle rays: %w", err)
	}
	return h, nil
}

const nativeParabolaMixedCirclesEnvelope = "trimmed-spline-graph-parabola-mixed-circles-envelope"

type parabolaMixedCirclesEnvelopeRecipe struct {
	parabolaCircleCrossingsEnvelopeRecipe
	Separated *parabolaMixedCirclesEnvelopeLedger `json:"separated"`
}

type parabolaMixedCirclesEnvelopeLedger struct {
	CrossingCircle *int  `json:"crossingCircle"`
	Face           *int  `json:"face"`
	Owner          *int  `json:"owner"`
	Enclosing      *int  `json:"enclosing"`
	Edges          []int `json:"edges"`
	Vertices       []int `json:"vertices"`
}

func parabolaMixedCirclesLedger(h ParabolaMixedCirclesEnvelopeHistory) *parabolaMixedCirclesEnvelopeLedger {
	return &parabolaMixedCirclesEnvelopeLedger{CrossingCircle: &h.CrossingCircle, Face: &h.SeparatedFace, Owner: &h.SeparatedOwner,
		Enclosing: &h.SeparatedEnclosing, Edges: h.SeparatedEdges[:], Vertices: h.SeparatedVertices[:]}
}

// WriteJSON records all three original supports and both circle incidences.
func (r *ParabolaMixedCirclesEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaMixedCirclesEnvelopeRecipe{
		parabolaCircleCrossingsEnvelopeRecipe: parabolaCircleCrossingsEnvelopeRecipe{
			Version: 1, Kind: nativeParabolaMixedCirclesEnvelope, Sources: src, Axis: &r.axis, Operation: &r.op,
			Tolerance: r.tolerance, Ledger: parabolaCircleCrossingsEnvelopeLedgerFor(r.history.ParabolaCircleCrossingsHistory),
		},
		Separated: parabolaMixedCirclesLedger(r.history),
	})
}

// ReadParabolaMixedCirclesEnvelopeJSON re-proves the complete original
// partition, both crossing links and the separated nested roof seam.
func ReadParabolaMixedCirclesEnvelopeJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaMixedCirclesEnvelope, error) {
	var recipe parabolaMixedCirclesEnvelopeRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaMixedCirclesEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || recipe.Ledger == nil || recipe.Separated == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	r, err := CombineParabolaMixedCirclesSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, parabolaCircleCrossingsEnvelopeLedgerFor(r.history.ParabolaCircleCrossingsHistory)) ||
		!reflect.DeepEqual(recipe.Separated, parabolaMixedCirclesLedger(r.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{r.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
