package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeParabolaCircleCrossingsCut = "trimmed-spline-graph-parabola-circle-crossings-cut"

type parabolaCircleCrossingsCutRecipe struct {
	Version    int                                       `json:"version"`
	Kind       string                                    `json:"kind"`
	Sources    []json.RawMessage                         `json:"sources"`
	Axis       *int                                      `json:"freeAxis"`
	Tolerance  float64                                   `json:"tolerance"`
	Components []parabolaCircleCrossingsCutRecipeLayer   `json:"components"`
	Contacts   []parabolaCircleCrossingsCutRecipeContact `json:"contacts"`
}

type parabolaCircleCrossingsCutRecipeLayer struct {
	Region        *int  `json:"region"`
	RoofFaces     []int `json:"roofFaces"`
	ParabolaEdges []int `json:"parabolaEdges"`
	CircleEdges   []int `json:"circleEdges"`
	WallFaces     []int `json:"wallFaces"`
	DomainSides   []int `json:"domainSides"`
}

type parabolaCircleCrossingsCutRecipeContact struct {
	Parameters []string `json:"parameters"`
	Point      []string `json:"point"`
	Vertices   []int    `json:"vertices"`
	Rays       [][]int  `json:"rays"`
}

func parabolaCircleCrossingsCutRecipeLedger(result *ParabolaCircleCrossingsCut) ([]parabolaCircleCrossingsCutRecipeLayer, []parabolaCircleCrossingsCutRecipeContact) {
	var components []parabolaCircleCrossingsCutRecipeLayer
	for _, h := range result.history {
		components = append(components, parabolaCircleCrossingsCutRecipeLayer{Region: &h.Region, RoofFaces: h.RoofFaces[:],
			ParabolaEdges: h.ParabolaEdges[:], CircleEdges: h.CircleEdges[:], WallFaces: h.WallFaces[:], DomainSides: h.DomainSides[:]})
	}
	var contacts []parabolaCircleCrossingsCutRecipeContact
	for _, contact := range result.contacts {
		entry := parabolaCircleCrossingsCutRecipeContact{Vertices: contact.Vertices[:], Rays: [][]int{contact.Rays[0][:], contact.Rays[1][:]}}
		for _, q := range contact.Parameters {
			entry.Parameters = append(entry.Parameters, q.RatString())
		}
		for _, q := range contact.Point {
			entry.Point = append(entry.Point, q.RatString())
		}
		contacts = append(contacts, entry)
	}
	return components, contacts
}

// WriteJSON records the original supports and ordered component, edge and
// contact indices. The positive layers and contact proof are rebuilt on load.
func (r *ParabolaCircleCrossingsCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	components, contacts := parabolaCircleCrossingsCutRecipeLedger(r)
	return writePolynomialGraphRecipe(ctx, w, parabolaCircleCrossingsCutRecipe{Version: 1, Kind: nativeParabolaCircleCrossingsCut,
		Sources: src, Axis: &r.axis, Tolerance: r.tolerance, Components: components, Contacts: contacts})
}

// ReadParabolaCircleCrossingsCutJSON re-proves both source components and their
// only two shared closure points before accepting the original native indices.
func ReadParabolaCircleCrossingsCutJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCircleCrossingsCut, error) {
	var recipe parabolaCircleCrossingsCutRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCircleCrossingsCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || len(recipe.Components) != 2 || len(recipe.Contacts) != 2 {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	result, err := CutParabolaCircleCrossingsSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	components, contacts := parabolaCircleCrossingsCutRecipeLedger(result)
	if !reflect.DeepEqual(recipe.Components, components) || !reflect.DeepEqual(recipe.Contacts, contacts) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount(result.components[:]) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
