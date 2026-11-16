package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeCuspidalCubicCut = "trimmed-cuspidal-cubic-cut"

type cuspidalCubicCutRecipe struct {
	Version   int                     `json:"version"`
	Kind      string                  `json:"kind"`
	Sources   []json.RawMessage       `json:"sources"`
	Axis      *int                    `json:"freeAxis"`
	Tolerance float64                 `json:"tolerance"`
	Ledger    *cuspidalCubicCutLedger `json:"ledger"`
}

type cuspidalCubicCutLedger struct {
	Region    *int                      `json:"region"`
	RoofFaces []int                     `json:"roofFaces"`
	Vertices  []int                     `json:"vertices"`
	Point     []string                  `json:"point"`
	UV        [][]string                `json:"uv"`
	Germs     []cuspidalCubicCommonGerm `json:"germs"`
	WallFaces []int                     `json:"wallFaces"`
}

func (r *CuspidalCubicCut) recipeLedger() *cuspidalCubicCutLedger {
	h := r.history
	point, uv, germs := cuspidalCubicSeamLedger(h.Germs, r.section)
	return &cuspidalCubicCutLedger{Region: &h.Region, RoofFaces: h.RoofFaces[:], Vertices: h.Vertices[:],
		Point: point, UV: uv, Germs: germs, WallFaces: h.WallFaces}
}

// WriteJSON records the original base and roofs, exact cusp germs and signed
// region and wall incidence.
func (r *CuspidalCubicCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.section == nil {
		return geom.ErrInvalid
	}
	stock, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, cuspidalCubicCutRecipe{
		Version: 1, Kind: nativeCuspidalCubicCut, Sources: stock, Axis: &r.freeAxis, Tolerance: r.tolerance, Ledger: r.recipeLedger(),
	})
}

// ReadCuspidalCubicCutJSON re-proves the stock and cusp from the original
// supports, builds the positive layer, and verifies its stored incidence.
func ReadCuspidalCubicCutJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o CuspidalCubicCutOptions) (*CuspidalCubicCut, error) {
	var recipe cuspidalCubicCutRecipe
	if err := readPolynomialGraphRecipe(ctx, r, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeCuspidalCubicCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Ledger == nil || recipe.Ledger.Region == nil {
		return nil, geom.ErrInvalid
	}
	stock, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CutCuspidalCubicSplineGraphs(ctx, stock[0], stock[1], stock[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, out.recipeLedger()) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{out.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
