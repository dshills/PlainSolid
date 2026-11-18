//go:build research

package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeNodalCubicCut = "trimmed-nodal-cubic-cut-components"

type nodalCubicCutRecipe struct {
	Version   int                  `json:"version"`
	Kind      string               `json:"kind"`
	Sources   []json.RawMessage    `json:"sources"`
	Axis      *int                 `json:"freeAxis"`
	Tolerance float64              `json:"tolerance"`
	Ledger    *nodalCubicCutLedger `json:"ledger"`
}

type nodalCubicCutLedger struct {
	UV         [][]string                  `json:"uv"`
	Point      []string                    `json:"point"`
	Vertices   []int                       `json:"vertices"`
	Rays       [][]int                     `json:"rays"`
	Components []nodalCubicCutRecipeRegion `json:"components"`
}

type nodalCubicCutRecipeRegion struct {
	Region  int    `json:"region"`
	Caps    []int  `json:"caps"`
	Contour []int  `json:"contour"`
	Walls   []int  `json:"walls"`
	Volume  string `json:"volume"`
}

func (r *NodalCubicCut) recipeLedger() *nodalCubicCutLedger {
	c := r.contact
	ledger := &nodalCubicCutLedger{Vertices: c.Vertices[:]}
	for i, uv := range c.UV {
		ledger.UV = append(ledger.UV, []string{uv[0].RatString(), uv[1].RatString()})
		ledger.Rays = append(ledger.Rays, c.Rays[i][:])
	}
	for _, q := range c.Point {
		ledger.Point = append(ledger.Point, q.RatString())
	}
	for i, h := range r.boundaries {
		ledger.Components = append(ledger.Components, nodalCubicCutRecipeRegion{
			Region: i + 1, Caps: []int{h.SplineFace, h.PlaneFace}, Contour: h.ContourEdges, Walls: h.WallFaces, Volume: r.volumes[i].RatString(),
		})
	}
	return ledger
}

// WriteJSON records the original supports, both positive region boundaries,
// their source volumes, and the common node with both local identities.
func (r *NodalCubicCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	raw, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, nodalCubicCutRecipe{
		Version: 1, Kind: nativeNodalCubicCut, Sources: raw, Axis: &r.axis, Tolerance: r.tolerance, Ledger: r.recipeLedger(),
	})
}

// ReadNodalCubicCutJSON re-proves both original supports, positive layers
// and their isolated contact before verifying the stored component ledger.
func ReadNodalCubicCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o NodalCubicCutOptions) (*NodalCubicCut, error) {
	var recipe nodalCubicCutRecipe
	if err := readPolynomialGraphRecipe(ctx, r, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeNodalCubicCut || len(recipe.Sources) != 3 || recipe.Axis == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	source, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CutNodalCubicSplineGraphs(ctx, source[0], source[1], source[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, out.recipeLedger()) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount(out.components[:]) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
