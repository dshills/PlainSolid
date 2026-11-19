//go:build research

package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"reflect"

	"plainsolid/geom"
)

const (
	nativeAcnodalCubicCommon = "trimmed-acnodal-cubic-common"
	nativeAcnodalCubicFuse   = "trimmed-acnodal-cubic-fuse"
)

type acnodalCubicEnvelopeRecipe struct {
	Version   int                         `json:"version"`
	Kind      string                      `json:"kind"`
	Sources   []json.RawMessage           `json:"sources"`
	Axis      *int                        `json:"freeAxis"`
	Tolerance float64                     `json:"tolerance"`
	Ledger    *acnodalCubicEnvelopeLedger `json:"ledger"`
}

type acnodalCubicEnvelopeLedger struct {
	Base             *int                     `json:"base"`
	RoofFaces        []int                    `json:"roofFaces"`
	RoofOwners       []int                    `json:"roofOwners"`
	IsolatedFace     *int                     `json:"isolatedFace"`
	IsolatedPoint    []string                 `json:"isolatedPoint"`
	IsolatedUV       [][]string               `json:"isolatedUV"`
	IsolatedTangents [][]string               `json:"isolatedTangents"`
	ArcVertices      []int                    `json:"arcVertices"`
	ArcEdges         []acnodalCubicLedgerEdge `json:"arcEdges"`
}

type acnodalCubicLedgerEdge struct {
	Edge int      `json:"edge"`
	Span []string `json:"span"`
}

func newAcnodalCubicEnvelopeLedger(r *AcnodalCubicEnvelope) *acnodalCubicEnvelopeLedger {
	h := r.history
	p := r.section
	pair := func(a [2]*big.Rat) []string { return []string{a[0].RatString(), a[1].RatString()} }
	triple := func(a [3]*big.Rat) []string { return []string{a[0].RatString(), a[1].RatString(), a[2].RatString()} }
	l := &acnodalCubicEnvelopeLedger{
		Base: &h.Base, RoofFaces: h.RoofFaces[:], RoofOwners: h.RoofOwners[:],
		IsolatedFace: &h.IsolatedFace, IsolatedPoint: triple(p.Isolated.Point),
		IsolatedUV:       [][]string{pair(p.Isolated.UV[0]), pair(p.Isolated.UV[1])},
		IsolatedTangents: [][]string{triple(p.IsolatedTangent[0]), triple(p.IsolatedTangent[1])},
	}
	if p.Arc != nil {
		l.ArcVertices = h.ArcVertices[:]
		for i, edge := range p.Arc.Edges {
			l.ArcEdges = append(l.ArcEdges, acnodalCubicLedgerEdge{Edge: h.ArcEdges[i], Span: pair(edge.Range)})
		}
	}
	return l
}

// WriteJSON stores the original stock and the isolated point separately from
// the optional material seam. No tessellation contributes to this receipt.
func (r *AcnodalCubicEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.section == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	kind := nativeAcnodalCubicCommon
	if r.upper {
		kind = nativeAcnodalCubicFuse
	}
	axis := r.axis
	return writePolynomialGraphRecipe(ctx, w, acnodalCubicEnvelopeRecipe{Version: 1, Kind: kind,
		Sources: src, Axis: &axis, Tolerance: r.tol, Ledger: newAcnodalCubicEnvelopeLedger(r)})
}

// ReadAcnodalCubicEnvelopeJSON rebuilds the original Common or Fuse and
// re-proves the isolated contact, signed roof ownership, and any regular arc.
func ReadAcnodalCubicEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o AcnodalCubicEnvelopeOptions) (*AcnodalCubicEnvelope, error) {
	var recipe acnodalCubicEnvelopeRecipe
	if err := readPolynomialGraphRecipe(ctx, r, limits, &recipe); err != nil {
		return nil, err
	}
	upper := recipe.Kind == nativeAcnodalCubicFuse
	if recipe.Version != 1 || (!upper && recipe.Kind != nativeAcnodalCubicCommon) || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	s, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := buildAcnodalCubicEnvelope(ctx, s[0], s[1], s[2], *recipe.Axis, recipe.Tolerance, upper, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, newAcnodalCubicEnvelopeLedger(out)) ||
		limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{out.solid}) > limits.MaxEntities {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
