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

func (r *CuspidalCubicCommon) recipeLedger() *cuspidalCubicCommonLedger {
	h := r.history
	cusp := r.section.Vertices[0]
	stringPair := func(v [2]*big.Rat) []string { return []string{v[0].RatString(), v[1].RatString()} }
	l := &cuspidalCubicCommonLedger{
		Base: h.Base, RoofFaces: h.RoofFaces[:], RoofOwners: h.RoofOwners[:], Vertices: h.Vertices[:],
		Point: []string{cusp.Point[0].RatString(), cusp.Point[1].RatString(), cusp.Point[2].RatString()},
		UV:    [][]string{stringPair(cusp.UV[0]), stringPair(cusp.UV[1])},
	}
	for i, g := range r.section.Germs {
		l.Germs = append(l.Germs, cuspidalCubicCommonGerm{
			Edge: h.Germs[i], Range: stringPair(g.Range),
			UVSecond: [][]string{stringPair(g.UVSecond[0]), stringPair(g.UVSecond[1])},
			UVThird:  [][]string{stringPair(g.UVThird[0]), stringPair(g.UVThird[1])},
		})
	}
	return l
}

// WriteJSON records the original stocks, both signed faces, and the exact
// cusp incidence and owner jets needed for the source replay.
func (r *CuspidalCubicCommon) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.section == nil {
		return geom.ErrInvalid
	}
	stock, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, cuspidalCubicCommonRecipe{
		Version: 1, Kind: nativeCuspidalCubicCommon, Sources: stock, Axis: &r.freeAxis, Tolerance: r.tolerance,
		Ledger: r.recipeLedger(),
	})
}

// ReadCuspidalCubicCommonJSON re-proves the exact original cusp and rebuilds
// the native roof, wall, and base faces before verifying their ledger.
func ReadCuspidalCubicCommonJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o CuspidalCubicCommonOptions) (*CuspidalCubicCommon, error) {
	var recipe cuspidalCubicCommonRecipe
	if err := readPolynomialGraphRecipe(ctx, r, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeCuspidalCubicCommon || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	stock, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CommonCuspidalCubicSplineGraphs(ctx, stock[0], stock[1], stock[2], *recipe.Axis, recipe.Tolerance, o)
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
