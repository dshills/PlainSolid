//go:build research

package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeParabolaTwoCrossingCirclesCut = "trimmed-spline-graph-parabola-two-crossing-circles-cut"

type parabolaCrossingCirclesCutRecipe struct {
	Version    int                                         `json:"version"`
	Kind       string                                      `json:"kind"`
	Sources    []json.RawMessage                           `json:"sources"`
	Axis       *int                                        `json:"freeAxis"`
	Tolerance  float64                                     `json:"tolerance"`
	Components []parabolaCrossingCirclesCutRecipeLayer     `json:"components"`
	Contacts   [][]parabolaCrossingCirclesCutRecipeContact `json:"contacts"`
}

type parabolaCrossingCirclesCutRecipeLayer struct {
	Region        *int    `json:"region"`
	RoofFaces     []int   `json:"roofFaces"`
	ParabolaEdges []int   `json:"parabolaEdges"`
	CircleEdges   [][]int `json:"circleEdges"`
	WallFaces     []int   `json:"wallFaces"`
	DomainSides   []int   `json:"domainSides"`
}

type parabolaCrossingCirclesCutRecipeContact struct {
	parabolaCircleCrossingsCutRecipeContact
	Components []int `json:"components"`
}

func twoCrossingCirclesCutRecipeLedger(r *ParabolaTwoCrossingCirclesCut) ([]parabolaCrossingCirclesCutRecipeLayer,
	[][]parabolaCrossingCirclesCutRecipeContact) {
	var components []parabolaCrossingCirclesCutRecipeLayer
	for _, h := range r.history {
		components = append(components, twoCrossingCircleCutLayerLedger(h))
	}
	return components, crossingCircleCutContactLedger(r.contacts[:])
}

func twoCrossingCircleCutLayerLedger(h ParabolaTwoCrossingCirclesCutHistory) parabolaCrossingCirclesCutRecipeLayer {
	return crossingCircleCutLayerLedger(h.Region, h.RoofFaces[:], h.ParabolaEdges[:], h.WallFaces, h.DomainSides, h.CircleEdges[:])
}

func crossingCircleCutLayerLedger(region int, roofFaces, parabolaEdges, walls, sides []int,
	circleEdges [][]int) parabolaCrossingCirclesCutRecipeLayer {
	return parabolaCrossingCirclesCutRecipeLayer{Region: &region, RoofFaces: roofFaces,
		ParabolaEdges: parabolaEdges, CircleEdges: circleEdges, WallFaces: walls, DomainSides: sides}
}

func crossingCircleCutContactLedger(items [][2]ParabolaTwoCrossingCirclesCutContact) [][]parabolaCrossingCirclesCutRecipeContact {
	var contacts [][]parabolaCrossingCirclesCutRecipeContact
	for _, circle := range items {
		contacts = append(contacts, crossingCircleCutContactRecipeEntries(circle[:]))
	}
	return contacts
}

func crossingCircleCutContactRecipeEntries(items []ParabolaTwoCrossingCirclesCutContact) []parabolaCrossingCirclesCutRecipeContact {
	var entries []parabolaCrossingCirclesCutRecipeContact
	for _, contact := range items {
		entry := parabolaCrossingCirclesCutRecipeContact{
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
	return entries
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
	return writePolynomialGraphRecipe(ctx, w, parabolaCrossingCirclesCutRecipe{Version: 1, Kind: nativeParabolaTwoCrossingCirclesCut,
		Sources: src, Axis: &r.axis, Tolerance: r.tolerance, Components: components, Contacts: contacts})
}

// ReadParabolaTwoCrossingCirclesCutJSON recomputes source signs, roof and
// wall ownership, and all four native contacts before returning components.
func ReadParabolaTwoCrossingCirclesCutJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaTwoCrossingCirclesCut, error) {
	var recipe parabolaCrossingCirclesCutRecipe
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
