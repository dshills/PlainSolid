package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeParabolaThreeCirclesCut = "trimmed-spline-graph-parabola-three-circles-cut"

type parabolaThreeCirclesCutRecipe struct {
	Version     int                                         `json:"version"`
	Kind        string                                      `json:"kind"`
	Sources     []json.RawMessage                           `json:"sources"`
	Axis        *int                                        `json:"freeAxis"`
	Tolerance   float64                                     `json:"tolerance"`
	Arrangement *parabolaThreeCirclesCutRecipeArrangement   `json:"arrangement"`
	Components  []parabolaThreeCirclesCutRecipeLayer        `json:"components"`
	Contacts    [][]parabolaCrossingCirclesCutRecipeContact `json:"contacts"`
}

type parabolaThreeCirclesCutRecipeArrangement struct {
	CrossingCircles []int `json:"crossingCircles"`
	SeparatedCircle *int  `json:"separatedCircle"`
	SeparatedRegion *int  `json:"separatedRegion"`
}

type parabolaThreeCirclesCutRecipeLayer struct {
	parabolaCrossingCirclesCutRecipeLayer
	SeparatedEdges    []int `json:"separatedEdges"`
	SeparatedVertices []int `json:"separatedVertices"`
}

func threeCirclesCutRecipeLedger(r *ParabolaThreeCirclesCut) (*parabolaThreeCirclesCutRecipeArrangement,
	[]parabolaThreeCirclesCutRecipeLayer, [][]parabolaCrossingCirclesCutRecipeContact) {
	a := r.arrangement
	arrangement := &parabolaThreeCirclesCutRecipeArrangement{CrossingCircles: a.CrossingCircles[:],
		SeparatedCircle: &a.SeparatedCircle, SeparatedRegion: &a.SeparatedRegion}
	layers := make([]parabolaThreeCirclesCutRecipeLayer, 0, len(r.history))
	for _, h := range r.history {
		layers = append(layers, parabolaThreeCirclesCutRecipeLayer{
			parabolaCrossingCirclesCutRecipeLayer: twoCrossingCircleCutLayerLedger(h.ParabolaTwoCrossingCirclesCutHistory),
			SeparatedEdges:                        h.SeparatedEdges[:], SeparatedVertices: h.SeparatedVertices[:],
		})
	}
	return arrangement, layers, crossingCircleCutContactLedger(r.contacts[:])
}

// WriteJSON retains the original ordered stocks, full curve ownership,
// positive layer order and all four separate source contacts.
func (r *ParabolaThreeCirclesCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	sources, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	arrangement, components, contacts := threeCirclesCutRecipeLedger(r)
	return writePolynomialGraphRecipe(ctx, w, parabolaThreeCirclesCutRecipe{
		Version: 1, Kind: nativeParabolaThreeCirclesCut, Sources: sources, Axis: &r.axis, Tolerance: r.tolerance,
		Arrangement: arrangement, Components: components, Contacts: contacts,
	})
}

// ReadParabolaThreeCirclesCutJSON re-proves the seven signed source regions
// and every separate native layer and contact before returning the Cut.
func ReadParabolaThreeCirclesCutJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaThreeCirclesCut, error) {
	var recipe parabolaThreeCirclesCutRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaThreeCirclesCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Arrangement == nil || (len(recipe.Components) != 3 && len(recipe.Components) != 4) ||
		len(recipe.Contacts) != 2 {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	r, err := CutParabolaThreeCirclesSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	arrangement, components, contacts := threeCirclesCutRecipeLedger(r)
	if !reflect.DeepEqual(recipe.Arrangement, arrangement) || !reflect.DeepEqual(recipe.Components, components) ||
		!reflect.DeepEqual(recipe.Contacts, contacts) || (limits.MaxEntities > 0 && polynomialGraphEntityCount(r.components) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
