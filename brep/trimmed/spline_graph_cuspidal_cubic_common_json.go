package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"reflect"

	"plainsolid/geom"
)

const nativeCuspidalCubicCommon = "trimmed-cuspidal-cubic-common"

type cuspidalCubicCommonRecipe struct {
	Version   int                        `json:"version"`
	Kind      string                     `json:"kind"`
	Sources   []json.RawMessage          `json:"sources"`
	Axis      *int                       `json:"freeAxis"`
	Tolerance float64                    `json:"tolerance"`
	Ledger    *cuspidalCubicCommonLedger `json:"ledger"`
}

type cuspidalCubicCommonLedger struct {
	Base       int                       `json:"base"`
	RoofFaces  []int                     `json:"roofFaces"`
	RoofOwners []int                     `json:"roofOwners"`
	Vertices   []int                     `json:"vertices"`
	Point      []string                  `json:"point"`
	UV         [][]string                `json:"uv"`
	Germs      []cuspidalCubicCommonGerm `json:"germs"`
}

type cuspidalCubicCommonGerm struct {
	Edge     int        `json:"edge"`
	Range    []string   `json:"range"`
	UVSecond [][]string `json:"uvSecond"`
	UVThird  [][]string `json:"uvThird"`
}

func makeCuspidalCubicEnvelopeLedger(h CuspidalCubicCommonHistory, section *geom.CuspidalCubicSection) *cuspidalCubicCommonLedger {
	point, uv, germs := cuspidalCubicSeamLedger(h.Germs, section)
	return &cuspidalCubicCommonLedger{
		Base: h.Base, RoofFaces: h.RoofFaces[:], RoofOwners: h.RoofOwners[:], Vertices: h.Vertices[:],
		Point: point, UV: uv, Germs: germs,
	}
}

func cuspidalCubicSeamLedger(edges [2]int, section *geom.CuspidalCubicSection) ([]string, [][]string, []cuspidalCubicCommonGerm) {
	cusp := section.Vertices[0]
	stringPair := func(v [2]*big.Rat) []string { return []string{v[0].RatString(), v[1].RatString()} }
	point := []string{cusp.Point[0].RatString(), cusp.Point[1].RatString(), cusp.Point[2].RatString()}
	uv := [][]string{stringPair(cusp.UV[0]), stringPair(cusp.UV[1])}
	germs := make([]cuspidalCubicCommonGerm, 0, len(section.Germs))
	for i, g := range section.Germs {
		germs = append(germs, cuspidalCubicCommonGerm{
			Edge: edges[i], Range: stringPair(g.Range),
			UVSecond: [][]string{stringPair(g.UVSecond[0]), stringPair(g.UVSecond[1])},
			UVThird:  [][]string{stringPair(g.UVThird[0]), stringPair(g.UVThird[1])},
		})
	}
	return point, uv, germs
}

// WriteJSON records the original stocks, both signed faces, and the exact
// cusp incidence and owner jets needed for the source replay.
func (r *CuspidalCubicCommon) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.section == nil {
		return geom.ErrInvalid
	}
	return writeCuspidalCubicEnvelopeRecipe(ctx, w, nativeCuspidalCubicCommon, r.sources, r.freeAxis, r.tolerance, r.history, r.section)
}

func writeCuspidalCubicEnvelopeRecipe(ctx context.Context, w io.Writer, kind string, sources [3]*geom.BSplineSurface, axis int,
	tolerance float64, h CuspidalCubicCommonHistory, section *geom.CuspidalCubicSection) error {
	stock, err := encodePolynomialGraphSupports(ctx, sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, cuspidalCubicCommonRecipe{
		Version: 1, Kind: kind, Sources: stock, Axis: &axis, Tolerance: tolerance, Ledger: makeCuspidalCubicEnvelopeLedger(h, section),
	})
}

// ReadCuspidalCubicCommonJSON re-proves the exact original cusp and rebuilds
// the native roof, wall, and base faces before verifying their ledger.
func ReadCuspidalCubicCommonJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o CuspidalCubicCommonOptions) (*CuspidalCubicCommon, error) {
	recipe, stock, err := readCuspidalCubicEnvelopeRecipe(ctx, r, limits, nativeCuspidalCubicCommon)
	if err != nil {
		return nil, err
	}
	out, err := CommonCuspidalCubicSplineGraphs(ctx, stock[0], stock[1], stock[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if err := recipe.verify(ctx, out.solid, out.history, out.section, limits); err != nil {
		return nil, err
	}
	return out, nil
}

func readCuspidalCubicEnvelopeRecipe(ctx context.Context, r io.Reader, limits JSONLimits, kind string) (cuspidalCubicCommonRecipe, [3]*geom.BSplineSurface, error) {
	var recipe cuspidalCubicCommonRecipe
	if err := readPolynomialGraphRecipe(ctx, r, limits, &recipe); err != nil {
		return recipe, [3]*geom.BSplineSurface{}, err
	}
	if recipe.Version != 1 || recipe.Kind != kind || len(recipe.Sources) != 3 || recipe.Axis == nil || recipe.Ledger == nil {
		return recipe, [3]*geom.BSplineSurface{}, geom.ErrInvalid
	}
	stock, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	return recipe, stock, err
}

func (recipe cuspidalCubicCommonRecipe) verify(ctx context.Context, solid Solid, h CuspidalCubicCommonHistory,
	section *geom.CuspidalCubicSection, limits JSONLimits) error {
	if !reflect.DeepEqual(recipe.Ledger, makeCuspidalCubicEnvelopeLedger(h, section)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{solid}) > limits.MaxEntities) {
		return geom.ErrConvergence
	}
	return ctx.Err()
}
