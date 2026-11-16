package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeParabolaTwoCrossingCirclesCut = "trimmed-spline-graph-parabola-two-crossing-circles-cut"

type parabolaTwoCrossingCirclesCutRecipe struct {
	Version    int                                            `json:"version"`
	Kind       string                                         `json:"kind"`
	Sources    []json.RawMessage                              `json:"sources"`
	Axis       *int                                           `json:"freeAxis"`
	Tolerance  float64                                        `json:"tolerance"`
	Components []parabolaTwoCrossingCirclesCutRecipeLayer     `json:"components"`
	Contacts   [][]parabolaTwoCrossingCirclesCutRecipeContact `json:"contacts"`
}

type parabolaTwoCrossingCirclesCutRecipeLayer struct {
	Region        *int    `json:"region"`
	RoofFaces     []int   `json:"roofFaces"`
	ParabolaEdges []int   `json:"parabolaEdges"`
	CircleEdges   [][]int `json:"circleEdges"`
	WallFaces     []int   `json:"wallFaces"`
	DomainSides   []int   `json:"domainSides"`
}

type parabolaTwoCrossingCirclesCutRecipeContact struct {
	parabolaCircleCrossingsCutRecipeContact
	Components []int `json:"components"`
}

func twoCrossingCirclesCutRecipeLedger(r *ParabolaTwoCrossingCirclesCut) ([]parabolaTwoCrossingCirclesCutRecipeLayer,
	[][]parabolaTwoCrossingCirclesCutRecipeContact) {
	var components []parabolaTwoCrossingCirclesCutRecipeLayer
	for _, h := range r.history {
		components = append(components, parabolaTwoCrossingCirclesCutRecipeLayer{Region: &h.Region, RoofFaces: h.RoofFaces[:],
			ParabolaEdges: h.ParabolaEdges[:], CircleEdges: h.CircleEdges[:], WallFaces: h.WallFaces, DomainSides: h.DomainSides})
	}
	var contacts [][]parabolaTwoCrossingCirclesCutRecipeContact
	for _, circle := range r.contacts {
		var entries []parabolaTwoCrossingCirclesCutRecipeContact
		for _, contact := range circle {
			entry := parabolaTwoCrossingCirclesCutRecipeContact{
				Components: contact.Components[:],
				parabolaCircleCrossingsCutRecipeContact: parabolaCircleCrossingsCutRecipeContact{
					Vertices: contact.Vertices[:], Rays: [][]int{contact.Rays[0][:], contact.Rays[1][:]},
				},
			}
			for _, q := range contact.Parameters {
				entry.Parameters = append(entry.Parameters, q.RatString())
			}
			for _, q := range contact.Point {
				entry.Point = append(entry.Point, q.RatString())
			}
			entries = append(entries, entry)
		}
		contacts = append(contacts, entries)
	}
	return components, contacts
}

// WriteJSON records the original stocks, all three ordered components and
// every indexed point contact.
func (r *ParabolaTwoCrossingCirclesCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	components, contacts := twoCrossingCirclesCutRecipeLedger(r)
	return writePolynomialGraphRecipe(ctx, w, parabolaTwoCrossingCirclesCutRecipe{Version: 1, Kind: nativeParabolaTwoCrossingCirclesCut,
		Sources: src, Axis: &r.axis, Tolerance: r.tolerance, Components: components, Contacts: contacts})
}

// ReadParabolaTwoCrossingCirclesCutJSON recomputes source signs, roof and
// wall ownership, and all four native contacts before returning components.
func ReadParabolaTwoCrossingCirclesCutJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaTwoCrossingCirclesCut, error) {
	var recipe parabolaTwoCrossingCirclesCutRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaTwoCrossingCirclesCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || len(recipe.Components) != 3 || len(recipe.Contacts) != 2 {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	r, err := CutParabolaTwoCrossingCirclesSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	components, contacts := twoCrossingCirclesCutRecipeLedger(r)
	if !reflect.DeepEqual(recipe.Components, components) || !reflect.DeepEqual(recipe.Contacts, contacts) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount(r.components[:]) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
