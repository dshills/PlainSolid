package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"slices"

	"plainsolid/geom"
)

const nativeTwoPolynomialNodeCut = "trimmed-two-polynomial-node-cut-components"

type twoPolynomialNodeCutRecipe struct {
	Version   int                              `json:"version"`
	Kind      string                           `json:"kind"`
	Sources   []json.RawMessage                `json:"sources"`
	Axis      *int                             `json:"freeAxis"`
	Tolerance float64                          `json:"tolerance"`
	Regions   []int                            `json:"regions"`
	Contacts  []twoPolynomialNodeRecipeContact `json:"contacts"`
}

type twoPolynomialNodeRecipeContact struct {
	Parameters []string `json:"parameters"`
	Point      []string `json:"point"`
	Components []int    `json:"components"`
	Vertices   []int    `json:"vertices"`
}

func (c TwoPolynomialNodeComponentContact) recipeContact() twoPolynomialNodeRecipeContact {
	r := twoPolynomialNodeRecipeContact{Components: c.Components[:], Vertices: c.Vertices[:]}
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
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	sources, e := encodePolynomialCutSupports(ctx, r.sources)
	if e != nil {
		return e
	}
	recipe := twoPolynomialNodeCutRecipe{Version: 1, Kind: nativeTwoPolynomialNodeCut, Sources: sources,
		Axis: &r.axis, Tolerance: r.tolerance, Regions: r.regions}
	for _, c := range r.contacts {
		recipe.Contacts = append(recipe.Contacts, c.recipeContact())
	}
	return writePolynomialCutRecipe(ctx, w, recipe)
}

// ReadTwoPolynomialNodeCutJSON rebuilds the original layers, independently
// re-proves their embeddings, and checks both exact point-contact identities.
func ReadTwoPolynomialNodeCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*TwoPolynomialNodeCut, error) {
	var recipe twoPolynomialNodeCutRecipe
	if e := readPolynomialCutRecipe(ctx, r, limits, &recipe); e != nil {
		return nil, e
	}
	if recipe.Version != 1 || recipe.Kind != nativeTwoPolynomialNodeCut || len(recipe.Sources) != 3 || recipe.Axis == nil ||
		len(recipe.Contacts) != 2 || len(recipe.Regions) < 2 || len(recipe.Regions) > 3 {
		return nil, geom.ErrInvalid
	}
	for _, c := range recipe.Contacts {
		if len(c.Parameters) != 2 || len(c.Point) != 3 || len(c.Components) != 2 || len(c.Vertices) != 2 {
			return nil, geom.ErrInvalid
		}
	}
	sources, e := decodePolynomialCutSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphTwoPolynomialNodeComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if !slices.Equal(out.regions, recipe.Regions) {
		return nil, geom.ErrConvergence
	}
	for i, encoded := range recipe.Contacts {
		proved := out.contacts[i].recipeContact()
		if !slices.Equal(encoded.Parameters, proved.Parameters) || !slices.Equal(encoded.Point, proved.Point) ||
			!slices.Equal(encoded.Components, proved.Components) || !slices.Equal(encoded.Vertices, proved.Vertices) {
			return nil, geom.ErrConvergence
		}
	}
	if limits.MaxEntities > 0 && polynomialCutEntityCount(out.components) > limits.MaxEntities {
		return nil, geom.ErrConvergence
	}
	return out, nil
}
