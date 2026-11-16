package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"reflect"

	"plainsolid/geom"
)

const nativeAcnodalCubicCut = "trimmed-acnodal-cubic-cut"

type acnodalCubicCutRecipe struct {
	Version   int                    `json:"version"`
	Kind      string                 `json:"kind"`
	Sources   []json.RawMessage      `json:"sources"`
	Axis      *int                   `json:"freeAxis"`
	Tolerance float64                `json:"tolerance"`
	Ledger    *acnodalCubicCutLedger `json:"ledger"`
}

type acnodalCubicCutLedger struct {
	RoofFaces   []int                         `json:"roofFaces"`
	ArcVertices []int                         `json:"arcVertices"`
	ArcEdges    []int                         `json:"arcEdges"`
	WallFaces   []int                         `json:"wallFaces"`
	Volume      string                        `json:"volume"`
	Isolated    acnodalCubicCutLedgerPoint    `json:"isolated"`
	Arc         []acnodalCubicCutLedgerVertex `json:"arc"`
	Ranges      [][]string                    `json:"ranges"`
}

type acnodalCubicCutLedgerPoint struct {
	XYZ []string   `json:"xyz"`
	UV  [][]string `json:"uv"`
}

type acnodalCubicCutLedgerVertex struct {
	acnodalCubicCutLedgerPoint
	XYZJet []string   `json:"xyzJet"`
	UVJet  [][]string `json:"uvJet"`
}

func (r *AcnodalCubicCut) recipeLedger() *acnodalCubicCutLedger {
	h, p := r.history, r.section
	encode := func(v []*big.Rat) []string {
		out := make([]string, len(v))
		for i, q := range v {
			out[i] = q.RatString()
		}
		return out
	}
	ownerPair := func(uv [2][2]*big.Rat) [][]string {
		return [][]string{encode(uv[0][:]), encode(uv[1][:])}
	}
	point := func(v geom.NodalCubicVertex) acnodalCubicCutLedgerPoint {
		return acnodalCubicCutLedgerPoint{XYZ: encode(v.Point[:]), UV: ownerPair(v.UV)}
	}
	ledger := &acnodalCubicCutLedger{
		RoofFaces: h.RoofFaces[:], ArcVertices: h.ArcVertices[:], ArcEdges: h.ArcEdges[:], WallFaces: h.WallFaces[:],
		Volume: r.volume.RatString(), Isolated: point(p.Isolated),
	}
	for i, v := range p.Arc.Vertices {
		ledger.Arc = append(ledger.Arc, acnodalCubicCutLedgerVertex{
			acnodalCubicCutLedgerPoint: point(v), XYZJet: encode(p.Arc.ModelJet[i][:]), UVJet: ownerPair(p.Arc.UVJet[i]),
		})
	}
	for _, edge := range p.Arc.Edges {
		ledger.Ranges = append(ledger.Ranges, encode(edge.Range[:]))
	}
	return ledger
}

// WriteJSON records the original stocks, excluded isolated point, regular
// arc incidence and independent material integral.
func (r *AcnodalCubicCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.section == nil {
		return geom.ErrInvalid
	}
	stock, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, acnodalCubicCutRecipe{
		Version: 1, Kind: nativeAcnodalCubicCut, Sources: stock, Axis: &r.freeAxis, Tolerance: r.tolerance, Ledger: r.recipeLedger(),
	})
}

// ReadAcnodalCubicCutJSON re-proves the original signed regions, positive
// manifold cap, coedge jets and exact volume before verifying the receipt.
func ReadAcnodalCubicCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o AcnodalCubicCutOptions) (*AcnodalCubicCut, error) {
	var recipe acnodalCubicCutRecipe
	if err := readPolynomialGraphRecipe(ctx, r, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeAcnodalCubicCut || len(recipe.Sources) != 3 || recipe.Axis == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	sources, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CutAcnodalCubicSplineGraphs(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
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
