package trimmed

import (
	"context"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeParabolaThreeCrossingCirclesCut = "trimmed-spline-graph-parabola-three-crossing-circles-cut"

func threeCrossingCirclesCutRecipeLedger(r *ParabolaThreeCrossingCirclesCut) ([]parabolaCrossingCirclesCutRecipeLayer,
	[][]parabolaCrossingCirclesCutRecipeContact) {
	layers := make([]parabolaCrossingCirclesCutRecipeLayer, 0, len(r.history))
	for _, h := range r.history {
		layers = append(layers, crossingCircleCutLayerLedger(h.Region, h.RoofFaces[:], h.ParabolaEdges[:], h.WallFaces, h.DomainSides, h.CircleEdges[:]))
	}
	return layers, crossingCircleCutContactLedger(r.contacts[:])
}

// WriteJSON records the original stocks, all four positive components and
// their six separately indexed source contacts.
func (r *ParabolaThreeCrossingCirclesCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	sources, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	components, contacts := threeCrossingCirclesCutRecipeLedger(r)
	return writePolynomialGraphRecipe(ctx, w, parabolaCrossingCirclesCutRecipe{
		Version: 1, Kind: nativeParabolaThreeCrossingCirclesCut, Sources: sources, Axis: &r.axis, Tolerance: r.tolerance,
		Components: components, Contacts: contacts,
	})
}

// ReadParabolaThreeCrossingCirclesCutJSON re-proves the original positive
// regions, roofs, wall ownership and separate contacts before publication.
func ReadParabolaThreeCrossingCirclesCutJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaThreeCrossingCirclesCut, error) {
	var recipe parabolaCrossingCirclesCutRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaThreeCrossingCirclesCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || len(recipe.Components) != 4 || len(recipe.Contacts) != 3 {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	r, err := CutParabolaThreeCrossingCirclesSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	components, contacts := threeCrossingCirclesCutRecipeLedger(r)
	if !reflect.DeepEqual(recipe.Components, components) || !reflect.DeepEqual(recipe.Contacts, contacts) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount(r.components[:]) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
