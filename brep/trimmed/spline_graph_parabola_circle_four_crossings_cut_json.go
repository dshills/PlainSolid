package trimmed

import (
	"context"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeParabolaCircleFourCrossingsCut = "trimmed-spline-graph-parabola-circle-four-crossings-cut"

func parabolaCircleFourCrossingsCutRecipeLedger(r *ParabolaCircleFourCrossingsCut) ([]parabolaCrossingCirclesCutRecipeLayer,
	[][]parabolaCrossingCirclesCutRecipeContact) {
	layers := make([]parabolaCrossingCirclesCutRecipeLayer, 0, len(r.history))
	for _, h := range r.history {
		layers = append(layers, crossingCircleCutLayerLedger(h.Region, h.RoofFaces[:], h.ParabolaEdges[:], h.WallFaces, h.DomainSides, [][]int{h.CircleEdges}))
	}
	return layers, [][]parabolaCrossingCirclesCutRecipeContact{crossingCircleCutContactRecipeEntries(r.contacts[:])}
}

// WriteJSON stores the original stocks, positive shells and all four
// separately indexed contacts.
func (r *ParabolaCircleFourCrossingsCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	components, contacts := parabolaCircleFourCrossingsCutRecipeLedger(r)
	return writePolynomialGraphRecipe(ctx, w, parabolaCrossingCirclesCutRecipe{
		Version: 1, Kind: nativeParabolaCircleFourCrossingsCut, Sources: src, Axis: &r.axis, Tolerance: r.tolerance,
		Components: components, Contacts: contacts,
	})
}

// ReadParabolaCircleFourCrossingsCutJSON recomputes every original layer and
// contact from the source stocks before publishing the rebuilt assembly.
func ReadParabolaCircleFourCrossingsCutJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCircleFourCrossingsCut, error) {
	var recipe parabolaCrossingCirclesCutRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCircleFourCrossingsCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || (len(recipe.Components) != 2 && len(recipe.Components) != 4) || len(recipe.Contacts) != 1 || len(recipe.Contacts[0]) != 4 {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	r, err := CutParabolaCircleFourCrossingsSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	components, contacts := parabolaCircleFourCrossingsCutRecipeLedger(r)
	if !reflect.DeepEqual(recipe.Components, components) || !reflect.DeepEqual(recipe.Contacts, contacts) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount(r.components) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
