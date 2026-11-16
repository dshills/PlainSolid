package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativeParabolaCircleApexEnvelope = "trimmed-spline-graph-parabola-circle-apex-tangency-envelope"

type parabolaCircleApexEnvelopeRecipe struct {
	Version   int                               `json:"version"`
	Kind      string                            `json:"kind"`
	Sources   []json.RawMessage                 `json:"sources"`
	Axis      *int                              `json:"freeAxis"`
	Operation *parametric.BooleanOp             `json:"operation"`
	Tolerance float64                           `json:"tolerance"`
	Ledger    *parabolaCircleApexEnvelopeLedger `json:"ledger"`
}

type parabolaCircleApexEnvelopeLedger struct {
	Base       *int    `json:"base"`
	RoofFaces  []int   `json:"roofFaces"`
	RoofOwners []int   `json:"roofOwners"`
	Circle     []int   `json:"circleEdges"`
	Bridge     *int    `json:"bridgeEdge"`
	Vertex     *int    `json:"vertex"`
	Rays       [][]int `json:"rays"`
	WallFaces  []int   `json:"wallFaces"`
}

func parabolaCircleApexEnvelopeLedgerFor(h ParabolaCircleApexEnvelopeHistory) *parabolaCircleApexEnvelopeLedger {
	return &parabolaCircleApexEnvelopeLedger{Base: &h.Base, RoofFaces: h.RoofFaces[:], RoofOwners: h.RoofOwners[:],
		Circle: h.Circle[:], Bridge: &h.Bridge, Vertex: &h.Vertex, Rays: [][]int{h.Rays[0][:], h.Rays[1][:]}, WallFaces: h.WallFaces[:]}
}

// WriteJSON records the original stock and proved native bridge, tangent
// vertex, section coedges and wall owners.
func (r *ParabolaCircleApexTangencyEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaCircleApexEnvelopeRecipe{
		Version: 1, Kind: nativeParabolaCircleApexEnvelope, Sources: src, Axis: &r.axis, Operation: &r.op,
		Tolerance: r.tolerance, Ledger: parabolaCircleApexEnvelopeLedgerFor(r.history),
	})
}

// ReadParabolaCircleApexTangencyEnvelopeJSON rebuilds the original source
// proof and all native incidences before accepting the recipe.
func ReadParabolaCircleApexTangencyEnvelopeJSON(ctx context.Context, input io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCircleApexTangencyEnvelope, error) {
	var recipe parabolaCircleApexEnvelopeRecipe
	if err := readPolynomialGraphRecipe(ctx, input, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCircleApexEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CombineParabolaCircleApexTangencySplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, parabolaCircleApexEnvelopeLedgerFor(out.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{out.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
