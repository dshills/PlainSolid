//go:build research

package trimmed

import (
	"context"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeParabolaCirclePocketDiskCut = "trimmed-spline-graph-parabola-circle-pocket-disk-cut"

type parabolaCirclePocketDiskCutRecipe struct {
	parabolaCrossingCirclesCutRecipe
	Arrangement *parabolaCirclePocketDiskCutLedger `json:"arrangement"`
}

type parabolaCirclePocketDiskCutLedger struct {
	CrossingCircle                *int    `json:"crossingCircle"`
	PocketCircle                  *int    `json:"pocketCircle"`
	DiskVertices                  [][]int `json:"diskVertices"`
	DiskSignedVolumePiCoefficient string  `json:"diskSignedVolumePiCoefficient"`
}

func parabolaCirclePocketDiskCutRecipeLedger(r *ParabolaCirclePocketDiskCut) ([]parabolaCrossingCirclesCutRecipeLayer,
	[][]parabolaCrossingCirclesCutRecipeContact, *parabolaCirclePocketDiskCutLedger) {
	components := make([]parabolaCrossingCirclesCutRecipeLayer, 0, len(r.history))
	vertices := make([][]int, 0, len(r.history))
	for _, h := range r.history {
		components = append(components, crossingCircleCutLayerLedger(h.Region, h.RoofFaces[:], h.ParabolaEdges[:], h.WallFaces, h.DomainSides,
			[][]int{h.CircleEdges, h.DiskEdges[:]}))
		vertices = append(vertices, h.DiskVertices[:])
	}
	arrangement := &parabolaCirclePocketDiskCutLedger{CrossingCircle: &r.circles[0], PocketCircle: &r.circles[1],
		DiskVertices: vertices, DiskSignedVolumePiCoefficient: r.diskVolume.RatString()}
	return components, [][]parabolaCrossingCirclesCutRecipeContact{crossingCircleCutContactRecipeEntries(r.contacts[:])}, arrangement
}

// WriteJSON stores both original roofs, every retained component and
// its exact rim and complete junction ownership ledgers.
func (r *ParabolaCirclePocketDiskCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	components, contacts, arrangement := parabolaCirclePocketDiskCutRecipeLedger(r)
	return writePolynomialGraphRecipe(ctx, w, parabolaCirclePocketDiskCutRecipe{
		parabolaCrossingCirclesCutRecipe: parabolaCrossingCirclesCutRecipe{
			Version: 1, Kind: nativeParabolaCirclePocketDiskCut, Sources: src, Axis: &r.axis, Tolerance: r.tolerance,
			Components: components, Contacts: contacts,
		},
		Arrangement: arrangement,
	})
}

// ReadParabolaCirclePocketDiskCutJSON re-proves the signed source
// partition, separate native shells and indexed point contacts.
func ReadParabolaCirclePocketDiskCutJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCirclePocketDiskCut, error) {
	var recipe parabolaCirclePocketDiskCutRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCirclePocketDiskCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Arrangement == nil || (len(recipe.Components) != 3 && len(recipe.Components) != 4) ||
		len(recipe.Contacts) != 1 || len(recipe.Contacts[0]) != 4 {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	r, err := CutParabolaCirclePocketDiskSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	components, contacts, arrangement := parabolaCirclePocketDiskCutRecipeLedger(r)
	if !reflect.DeepEqual(recipe.Components, components) || !reflect.DeepEqual(recipe.Contacts, contacts) ||
		!reflect.DeepEqual(recipe.Arrangement, arrangement) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount(r.components) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
