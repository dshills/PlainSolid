//go:build research

package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativeParabolaCircleEnvelope = "trimmed-spline-graph-parabola-circle-envelope"

type parabolaCircleEnvelopeRecipe struct {
	Version   int                           `json:"version"`
	Kind      string                        `json:"kind"`
	Sources   []json.RawMessage             `json:"sources"`
	Axis      *int                          `json:"freeAxis"`
	Operation *parametric.BooleanOp         `json:"operation"`
	Tolerance float64                       `json:"tolerance"`
	Ledger    *parabolaCircleEnvelopeLedger `json:"ledger"`
}

type parabolaCircleEnvelopeLedger struct {
	Base         int   `json:"base"`
	RoofFaces    []int `json:"roofFaces"`
	RoofOwners   []int `json:"roofOwners"`
	ParabolaEdge *int  `json:"parabolaEdge"`
	CircleEdges  []int `json:"circleEdges"`
	WallFaces    []int `json:"wallFaces"`
}

func parabolaCircleRecipeLedger(h ParabolaCircleGraphHistory) *parabolaCircleEnvelopeLedger {
	return &parabolaCircleEnvelopeLedger{Base: h.Base, RoofFaces: h.RoofFaces[:], RoofOwners: h.RoofOwners[:],
		ParabolaEdge: &h.ParabolaEdge, CircleEdges: h.CircleEdges[:], WallFaces: h.WallFaces[:]}
}

// WriteJSON records the original graph supports, operation and named edge
// and face owners of the certified parabola-circle envelope.
func (r *ParabolaCircleGraphEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaCircleEnvelopeRecipe{Version: 1, Kind: nativeParabolaCircleEnvelope,
		Sources: src, Axis: &r.axis, Operation: &r.op, Tolerance: r.tolerance, Ledger: parabolaCircleRecipeLedger(r.history)})
}

// ReadParabolaCircleGraphEnvelopeJSON rebuilds the complete original
// contact and material boundary and authenticates their native indices.
func ReadParabolaCircleGraphEnvelopeJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCircleGraphEnvelope, error) {
	var recipe parabolaCircleEnvelopeRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCircleEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	result, err := CombineParabolaCircleSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, parabolaCircleRecipeLedger(result.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{result.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	return result, ctx.Err()
}
