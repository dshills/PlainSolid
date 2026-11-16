package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeParabolaCircleApexCut = "trimmed-spline-graph-parabola-circle-apex-tangency-cut"

type parabolaCircleApexCutRecipe struct {
	Version   int                                      `json:"version"`
	Kind      string                                   `json:"kind"`
	Sources   []json.RawMessage                        `json:"sources"`
	Axis      *int                                     `json:"freeAxis"`
	Tolerance float64                                  `json:"tolerance"`
	Layers    []parabolaCircleApexCutRecipeLayer       `json:"layers"`
	Contact   *parabolaCircleCrossingsCutRecipeContact `json:"contact"`
}

type parabolaCircleApexCutRecipeLayer struct {
	Region       *int  `json:"region"`
	RoofFaces    []int `json:"roofFaces"`
	SectionEdges []int `json:"sectionEdges"`
	WallFaces    []int `json:"wallFaces"`
	DomainSides  []int `json:"domainSides"`
}

func parabolaCircleApexCutLedger(r *ParabolaCircleApexTangencyCut) ([]parabolaCircleApexCutRecipeLayer, *parabolaCircleCrossingsCutRecipeContact) {
	var layers []parabolaCircleApexCutRecipeLayer
	for _, h := range r.history {
		layers = append(layers, parabolaCircleApexCutRecipeLayer{Region: &h.Region, RoofFaces: h.RoofFaces[:],
			SectionEdges: h.SectionEdges, WallFaces: h.WallFaces, DomainSides: h.DomainSides})
	}
	c := r.contact
	contact := &parabolaCircleCrossingsCutRecipeContact{Vertices: c.Vertices[:], Rays: [][]int{c.Rays[0][:], c.Rays[1][:]}}
	for _, q := range c.Parameters {
		contact.Parameters = append(contact.Parameters, q.RatString())
	}
	for _, q := range c.Point {
		contact.Point = append(contact.Point, q.RatString())
	}
	return layers, contact
}

// WriteJSON records the original supports and component-local incidences.
func (r *ParabolaCircleApexTangencyCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	layers, contact := parabolaCircleApexCutLedger(r)
	return writePolynomialGraphRecipe(ctx, w, parabolaCircleApexCutRecipe{Version: 1, Kind: nativeParabolaCircleApexCut,
		Sources: src, Axis: &r.axis, Tolerance: r.tolerance, Layers: layers, Contact: contact})
}

// ReadParabolaCircleApexTangencyCutJSON re-proves the original contact, both
// embedded components and their distinct local rays and tangent jets.
func ReadParabolaCircleApexTangencyCutJSON(ctx context.Context, input io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCircleApexTangencyCut, error) {
	var recipe parabolaCircleApexCutRecipe
	if err := readPolynomialGraphRecipe(ctx, input, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCircleApexCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || len(recipe.Layers) != 2 || recipe.Contact == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CutParabolaCircleApexTangencySplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	layers, contact := parabolaCircleApexCutLedger(out)
	if !reflect.DeepEqual(recipe.Layers, layers) || !reflect.DeepEqual(recipe.Contact, contact) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount(out.components[:]) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
