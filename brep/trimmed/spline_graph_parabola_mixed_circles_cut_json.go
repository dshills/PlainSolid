//go:build research

package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeParabolaMixedCirclesCut = "trimmed-spline-graph-parabola-mixed-circles-cut"

type parabolaMixedCirclesCutRecipe struct {
	Version    int                                    `json:"version"`
	Kind       string                                 `json:"kind"`
	Sources    []json.RawMessage                      `json:"sources"`
	Axis       *int                                   `json:"freeAxis"`
	Tolerance  float64                                `json:"tolerance"`
	Components []parabolaMixedCirclesCutRecipeLayer   `json:"components"`
	Contacts   []parabolaMixedCirclesCutRecipeContact `json:"contacts"`
}

type parabolaMixedCirclesCutRecipeLayer struct {
	parabolaCircleCrossingsCutRecipeLayer
	SeparatedEdges    []int `json:"separatedEdges"`
	SeparatedVertices []int `json:"separatedVertices"`
}

type parabolaMixedCirclesCutRecipeContact struct {
	parabolaCircleCrossingsCutRecipeContact
	Components []int `json:"components"`
}

func parabolaMixedCirclesCutLedger(r *ParabolaMixedCirclesCut) ([]parabolaMixedCirclesCutRecipeLayer, []parabolaMixedCirclesCutRecipeContact) {
	layers := make([]parabolaMixedCirclesCutRecipeLayer, len(r.history))
	for i, h := range r.history {
		layers[i] = parabolaMixedCirclesCutRecipeLayer{
			parabolaCircleCrossingsCutRecipeLayer: parabolaCircleCrossingsCutRecipeLayer{
				Region: &h.Region, RoofFaces: h.RoofFaces[:], ParabolaEdges: h.ParabolaEdges[:],
				CircleEdges: h.CircleEdges[:], WallFaces: h.WallFaces[:], DomainSides: h.DomainSides[:],
			},
			SeparatedEdges: h.SeparatedEdges[:], SeparatedVertices: h.SeparatedVertices[:],
		}
	}
	contacts := make([]parabolaMixedCirclesCutRecipeContact, len(r.contacts))
	for i, c := range r.contacts {
		contact := parabolaCircleCrossingsCutRecipeContact{Vertices: c.Vertices[:], Rays: [][]int{c.Rays[0][:], c.Rays[1][:]}}
		for _, q := range c.Parameters {
			contact.Parameters = append(contact.Parameters, q.RatString())
		}
		for _, q := range c.Point {
			contact.Point = append(contact.Point, q.RatString())
		}
		contacts[i] = parabolaMixedCirclesCutRecipeContact{parabolaCircleCrossingsCutRecipeContact: contact, Components: []int{0, 1}}
	}
	return layers, contacts
}

// WriteJSON records the original supports, all positive components and their
// original wall, circular, and point-contact incidences.
func (r *ParabolaMixedCirclesCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	layers, contacts := parabolaMixedCirclesCutLedger(r)
	return writePolynomialGraphRecipe(ctx, w, parabolaMixedCirclesCutRecipe{
		Version: 1, Kind: nativeParabolaMixedCirclesCut, Sources: src, Axis: &r.axis,
		Tolerance: r.tolerance, Components: layers, Contacts: contacts,
	})
}

// ReadParabolaMixedCirclesCutJSON re-proves all exact original boundaries
// and authenticates the contact graph and component-local indices.
func ReadParabolaMixedCirclesCutJSON(ctx context.Context, in io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaMixedCirclesCut, error) {
	var recipe parabolaMixedCirclesCutRecipe
	if err := readPolynomialGraphRecipe(ctx, in, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaMixedCirclesCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || len(recipe.Components) < 2 || len(recipe.Components) > 3 || len(recipe.Contacts) != 2 {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CutParabolaMixedCirclesSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	layers, contacts := parabolaMixedCirclesCutLedger(out)
	if !reflect.DeepEqual(recipe.Components, layers) || !reflect.DeepEqual(recipe.Contacts, contacts) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount(out.components) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
