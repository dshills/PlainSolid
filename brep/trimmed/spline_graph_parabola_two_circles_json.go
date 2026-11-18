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

const (
	nativeParabolaTwoCirclesEnvelope = "trimmed-spline-graph-parabola-two-circles-envelope"
	nativeParabolaTwoCirclesCut      = "trimmed-spline-graph-parabola-two-circles-cut"
)

type parabolaTwoCirclesEnvelopeRecipe struct {
	Version   int                               `json:"version"`
	Kind      string                            `json:"kind"`
	Sources   []json.RawMessage                 `json:"sources"`
	Axis      *int                              `json:"freeAxis"`
	Operation *parametric.BooleanOp             `json:"operation"`
	Tolerance float64                           `json:"tolerance"`
	Ledger    *parabolaTwoCirclesEnvelopeLedger `json:"ledger"`
}

type parabolaTwoCirclesEnvelopeLedger struct {
	Base                 int     `json:"base"`
	RoofFaces            []int   `json:"roofFaces"`
	RoofOwners           []int   `json:"roofOwners"`
	EnclosingRoof        *int    `json:"enclosingRoof"`
	CircleEnclosingRoofs []int   `json:"circleEnclosingRoofs,omitempty"`
	ParabolaEdge         *int    `json:"parabolaEdge"`
	CircleEdges          [][]int `json:"circleEdges"`
	WallFaces            []int   `json:"wallFaces"`
}

func parabolaTwoCirclesRecipeLedger(h ParabolaTwoCirclesGraphHistory) *parabolaTwoCirclesEnvelopeLedger {
	out := &parabolaTwoCirclesEnvelopeLedger{Base: h.Base, RoofFaces: h.RoofFaces[:], RoofOwners: h.RoofOwners[:],
		EnclosingRoof: &h.EnclosingRoof, ParabolaEdge: &h.ParabolaEdge, CircleEdges: [][]int{h.CircleEdges[0][:], h.CircleEdges[1][:]},
		WallFaces: h.WallFaces[:]}
	if h.EnclosingRoof < 0 {
		out.CircleEnclosingRoofs = h.CircleEnclosingRoofs[:]
	}
	return out
}

// WriteJSON stores the unchanged source supports and both circle coedge rings.
func (r *ParabolaTwoCirclesGraphEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaTwoCirclesEnvelopeRecipe{
		Version: 1, Kind: nativeParabolaTwoCirclesEnvelope, Sources: src, Axis: &r.axis, Operation: &r.op, Tolerance: r.tolerance,
		Ledger: parabolaTwoCirclesRecipeLedger(r.history),
	})
}

// ReadParabolaTwoCirclesGraphEnvelopeJSON re-proves the complete original
// contact, selected envelope and indexed owners before publishing a shell.
func ReadParabolaTwoCirclesGraphEnvelopeJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaTwoCirclesGraphEnvelope, error) {
	var recipe parabolaTwoCirclesEnvelopeRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaTwoCirclesEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	stock, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	r, err := CombineParabolaTwoCirclesSplineGraphs(ctx, stock[0], stock[1], stock[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, parabolaTwoCirclesRecipeLedger(r.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{r.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	return r, ctx.Err()
}

// WriteJSON stores the original sources and all selected native Cut components
// in the proved region order.
func (r *ParabolaTwoCirclesGraphCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaCircleCutRecipe{
		Version: 1, Kind: nativeParabolaTwoCirclesCut, Sources: src, Axis: &r.axis, Tolerance: r.tolerance,
		Ledger: parabolaCircleCutRecipeLedger(r.history),
	})
}

// ReadParabolaTwoCirclesGraphCutJSON re-proves the exact contact arrangement,
// positive layers and indexed component boundaries against the original sources.
func ReadParabolaTwoCirclesGraphCutJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaTwoCirclesGraphCut, error) {
	var recipe parabolaCircleCutRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaTwoCirclesCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || len(recipe.Ledger) < 1 || len(recipe.Ledger) > 3 {
		return nil, geom.ErrInvalid
	}
	stock, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	r, err := CutParabolaTwoCirclesSplineGraphs(ctx, stock[0], stock[1], stock[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, parabolaCircleCutRecipeLedger(r.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount(r.components) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	return r, ctx.Err()
}
