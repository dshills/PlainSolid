package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativeParabolaCircleCrossingsEnvelope = "trimmed-spline-graph-parabola-circle-crossings-envelope"

type parabolaCircleCrossingsEnvelopeRecipe struct {
	Version   int                                    `json:"version"`
	Kind      string                                 `json:"kind"`
	Sources   []json.RawMessage                      `json:"sources"`
	Axis      *int                                   `json:"freeAxis"`
	Operation *parametric.BooleanOp                  `json:"operation"`
	Tolerance float64                                `json:"tolerance"`
	Ledger    *parabolaCircleCrossingsEnvelopeLedger `json:"ledger"`
}

type parabolaCircleCrossingsEnvelopeLedger struct {
	Base          *int                                      `json:"base"`
	RoofFaces     []int                                     `json:"roofFaces"`
	RoofOwners    []int                                     `json:"roofOwners"`
	ParabolaEdges []int                                     `json:"parabolaEdges"`
	CircleEdges   []int                                     `json:"circleEdges"`
	WallFaces     []int                                     `json:"wallFaces"`
	Junctions     []parabolaCircleCrossingsEnvelopeJunction `json:"junctions"`
}

type parabolaCircleCrossingsEnvelopeJunction struct {
	Vertex  *int  `json:"vertex"`
	Rays    []int `json:"rays"`
	Sectors []int `json:"sectors"`
}

func parabolaCircleCrossingsEnvelopeLedgerFor(h ParabolaCircleCrossingsHistory) *parabolaCircleCrossingsEnvelopeLedger {
	out := &parabolaCircleCrossingsEnvelopeLedger{Base: &h.Base, RoofFaces: h.RoofFaces[:], RoofOwners: h.RoofOwners[:],
		ParabolaEdges: h.ParabolaEdges[:], CircleEdges: h.CircleEdges[:], WallFaces: h.WallFaces[:]}
	for _, node := range h.Junctions {
		out.Junctions = append(out.Junctions, parabolaCircleCrossingsEnvelopeJunction{Vertex: &node.Vertex, Rays: node.Rays[:], Sectors: node.Sectors[:]})
	}
	return out
}

// WriteJSON records the three original supports, operation and certified
// native ownership and junction ledger.
func (r *ParabolaCircleCrossingsEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaCircleCrossingsEnvelopeRecipe{Version: 1, Kind: nativeParabolaCircleCrossingsEnvelope,
		Sources: src, Axis: &r.axis, Operation: &r.op, Tolerance: r.tolerance, Ledger: parabolaCircleCrossingsEnvelopeLedgerFor(r.history)})
}

// ReadParabolaCircleCrossingsEnvelopeJSON rebuilds the whole original source
// partition and authenticates the native face, edge and node incidences.
func ReadParabolaCircleCrossingsEnvelopeJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCircleCrossingsEnvelope, error) {
	var recipe parabolaCircleCrossingsEnvelopeRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCircleCrossingsEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	result, err := CombineParabolaCircleCrossingsSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, parabolaCircleCrossingsEnvelopeLedgerFor(result.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{result.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
