package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"slices"

	"plainsolid/geom"
)

const nativeTwoPolynomialNodeCut = "trimmed-two-polynomial-node-cut-components"

type polynomialNodeComponentsRecipe struct {
	Version   int                                     `json:"version"`
	Kind      string                                  `json:"kind"`
	Sources   []json.RawMessage                       `json:"sources"`
	Axis      *int                                    `json:"freeAxis"`
	Tolerance float64                                 `json:"tolerance"`
	Regions   []int                                   `json:"regions"`
	Contacts  []polynomialNodeComponentsRecipeContact `json:"contacts"`
}

type polynomialNodeComponentsRecipeContact struct {
	Parameters []string `json:"parameters"`
	Point      []string `json:"point"`
	Components []int    `json:"components"`
	Vertices   []int    `json:"vertices"`
}

func (c TwoPolynomialNodeComponentContact) recipeContact() polynomialNodeComponentsRecipeContact {
	r := polynomialNodeComponentsRecipeContact{Components: c.Components[:], Vertices: c.Vertices[:]}
	for _, p := range c.Parameters {
		r.Parameters = append(r.Parameters, p.RatString())
	}
	for _, p := range c.Point {
		r.Point = append(r.Point, p.RatString())
	}
	return r
}

// WriteJSON writes the original graph supports and their indexed two-contact
// ledger. Reload reconstructs the component boundaries from these sources.
func (r *TwoPolynomialNodeCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil {
		return geom.ErrInvalid
	}
	return writePolynomialNodeComponentsRecipe(ctx, w, nativeTwoPolynomialNodeCut, r.sources, r.axis, r.tolerance, r.regions, r.contacts[:])
}

// ReadTwoPolynomialNodeCutJSON rebuilds the original layers, independently
// re-proves their embeddings, and checks both exact point-contact identities.
func ReadTwoPolynomialNodeCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*TwoPolynomialNodeCut, error) {
	recipe, sources, e := readPolynomialNodeComponentsRecipe(ctx, r, limits, nativeTwoPolynomialNodeCut, 2)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphTwoPolynomialNodeComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.regions, out.contacts[:], out.components, limits); e != nil {
		return nil, e
	}
	return out, nil
}

func writePolynomialNodeComponentsRecipe(ctx context.Context, w io.Writer, kind string, stock [3]*geom.BSplineSurface, axis int, tolerance float64, regions []int, contacts []TwoPolynomialNodeComponentContact) error {
	if ctx == nil || w == nil || stock[0] == nil {
		return geom.ErrInvalid
	}
	sources, e := encodePolynomialGraphSupports(ctx, stock)
	if e != nil {
		return e
	}
	recipe := polynomialNodeComponentsRecipe{Version: 1, Kind: kind, Sources: sources,
		Axis: &axis, Tolerance: tolerance, Regions: regions}
	for _, c := range contacts {
		recipe.Contacts = append(recipe.Contacts, c.recipeContact())
	}
	return writePolynomialGraphRecipe(ctx, w, recipe)
}

func readPolynomialNodeComponentsRecipe(ctx context.Context, r io.Reader, limits JSONLimits, kind string, nodes int) (polynomialNodeComponentsRecipe, [3]*geom.BSplineSurface, error) {
	var recipe polynomialNodeComponentsRecipe
	if e := readPolynomialGraphRecipe(ctx, r, limits, &recipe); e != nil {
		return polynomialNodeComponentsRecipe{}, [3]*geom.BSplineSurface{}, e
	}
	if recipe.Version != 1 || recipe.Kind != kind || len(recipe.Sources) != 3 || recipe.Axis == nil ||
		len(recipe.Contacts) != nodes || (len(recipe.Regions) != 2 && len(recipe.Regions) != nodes+1) {
		return polynomialNodeComponentsRecipe{}, [3]*geom.BSplineSurface{}, geom.ErrInvalid
	}
	for _, c := range recipe.Contacts {
		if len(c.Parameters) != 2 || len(c.Point) != 3 || len(c.Components) != 2 || len(c.Vertices) != 2 {
			return polynomialNodeComponentsRecipe{}, [3]*geom.BSplineSurface{}, geom.ErrInvalid
		}
	}
	sources, e := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	return recipe, sources, e
}

func (recipe polynomialNodeComponentsRecipe) verify(regions []int, contacts []TwoPolynomialNodeComponentContact, components []Solid, limits JSONLimits) error {
	if !slices.Equal(regions, recipe.Regions) {
		return geom.ErrConvergence
	}
	for i, encoded := range recipe.Contacts {
		proved := contacts[i].recipeContact()
		if !slices.Equal(encoded.Parameters, proved.Parameters) || !slices.Equal(encoded.Point, proved.Point) ||
			!slices.Equal(encoded.Components, proved.Components) || !slices.Equal(encoded.Vertices, proved.Vertices) {
			return geom.ErrConvergence
		}
	}
	if limits.MaxEntities > 0 && polynomialGraphEntityCount(components) > limits.MaxEntities {
		return geom.ErrConvergence
	}
	return nil
}
