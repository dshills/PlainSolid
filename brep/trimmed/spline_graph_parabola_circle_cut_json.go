//go:build research

package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeParabolaCircleCut = "trimmed-spline-graph-parabola-circle-cut"

type parabolaCircleCutRecipe struct {
	Version   int                            `json:"version"`
	Kind      string                         `json:"kind"`
	Sources   []json.RawMessage              `json:"sources"`
	Axis      *int                           `json:"freeAxis"`
	Tolerance float64                        `json:"tolerance"`
	Ledger    []parabolaCircleCutRecipeLayer `json:"ledger"`
}

type parabolaCircleCutRecipeLayer struct {
	Region       *int  `json:"region"`
	RoofFaces    []int `json:"roofFaces"`
	ParabolaEdge *int  `json:"parabolaEdge"`
	CircleEdges  []int `json:"circleEdges"`
	WallFaces    []int `json:"wallFaces"`
	DomainSides  []int `json:"domainSides"`
}

func parabolaCircleCutRecipeLedger(h []ParabolaCircleCutComponentHistory) []parabolaCircleCutRecipeLayer {
	out := make([]parabolaCircleCutRecipeLayer, len(h))
	for i, x := range h {
		out[i] = parabolaCircleCutRecipeLayer{Region: &x.Region, RoofFaces: x.RoofFaces[:], ParabolaEdge: &x.ParabolaEdge,
			CircleEdges: x.CircleEdges, WallFaces: x.WallFaces, DomainSides: x.DomainSides}
	}
	return out
}

// WriteJSON records both original roofs, the shared base and each retained
// region's native topology in component order.
func (r *ParabolaCircleGraphCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	sources, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaCircleCutRecipe{
		Version: 1, Kind: nativeParabolaCircleCut, Sources: sources,
		Axis: &r.axis, Tolerance: r.tolerance, Ledger: parabolaCircleCutRecipeLedger(r.history),
	})
}

// ReadParabolaCircleGraphCutJSON re-proves the positive source regions,
// disjointness, and each closed component before checking the recipe ledger.
func ReadParabolaCircleGraphCutJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCircleGraphCut, error) {
	var recipe parabolaCircleCutRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCircleCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || len(recipe.Ledger) < 1 || len(recipe.Ledger) > 2 {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CutParabolaCircleSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, parabolaCircleCutRecipeLedger(out.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount(out.components) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	return out, ctx.Err()
}
