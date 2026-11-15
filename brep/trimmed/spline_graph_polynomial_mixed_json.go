package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativePolynomialMixedEnvelope = "trimmed-polynomial-mixed-envelope"

type polynomialMixedEnvelopeRecipe struct {
	Version   int                           `json:"version"`
	Kind      string                        `json:"kind"`
	Sources   []json.RawMessage             `json:"sources"`
	Axis      *int                          `json:"freeAxis"`
	Operation *parametric.BooleanOp         `json:"operation"`
	Tolerance float64                       `json:"tolerance"`
	Ledger    polynomialMixedEnvelopeLedger `json:"ledger"`
}

type polynomialMixedEnvelopeLedger struct {
	Edges      [][]int                                `json:"edges"`
	Vertices   []int                                  `json:"vertices"`
	Midpoints  []int                                  `json:"midpoints"`
	RoofFaces  []int                                  `json:"roofFaces"`
	RoofOwners []int                                  `json:"roofOwners"`
	Base       int                                    `json:"base"`
	Contacts   []polynomialMixedEnvelopeRecipeContact `json:"contacts"`
}

type polynomialMixedEnvelopeRecipeContact struct {
	Parameters    []string `json:"parameters"`
	Point         []string `json:"point"`
	Order         int      `json:"order"`
	DerivativeGap string   `json:"derivativeGap"`
	Vertex        int      `json:"vertex"`
	Rays          []int    `json:"rays"`
	Sectors       []int    `json:"sectors"`
}

func (r *PolynomialMixedEnvelope) recipeLedger() polynomialMixedEnvelopeLedger {
	h := r.history
	ledger := polynomialMixedEnvelopeLedger{
		Vertices: h.Vertices[:], Midpoints: h.Midpoints[:], RoofFaces: h.RoofFaces[:], RoofOwners: h.RoofOwners[:], Base: h.Base,
	}
	for _, chain := range h.Edges {
		if chain[1] < 0 {
			ledger.Edges = append(ledger.Edges, []int{chain[0]})
		} else {
			ledger.Edges = append(ledger.Edges, []int{chain[0], chain[1]})
		}
	}
	for _, c := range r.contacts {
		entry := polynomialMixedEnvelopeRecipeContact{
			Order: c.Order, DerivativeGap: c.DerivativeGap.RatString(), Vertex: c.Vertex, Rays: c.Rays[:], Sectors: c.Sectors[:],
		}
		for _, p := range c.Parameters {
			entry.Parameters = append(entry.Parameters, p.RatString())
		}
		for _, p := range c.Point {
			entry.Point = append(entry.Point, p.RatString())
		}
		ledger.Contacts = append(ledger.Contacts, entry)
	}
	return ledger
}

// WriteJSON records the original roofs and the bound topology of both
// contacts. Reload reconstructs the boundary and verifies this ledger.
func (r *PolynomialMixedEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	sources, e := encodePolynomialGraphSupports(ctx, r.sources)
	if e != nil {
		return e
	}
	recipe := polynomialMixedEnvelopeRecipe{
		Version: 1, Kind: nativePolynomialMixedEnvelope, Sources: sources, Axis: &r.axis,
		Operation: &r.op, Tolerance: r.tolerance, Ledger: r.recipeLedger(),
	}
	return writePolynomialGraphRecipe(ctx, w, recipe)
}

// ReadPolynomialMixedEnvelopeJSON re-proves the original crossing, tangency
// and both owner coedges before binding their recorded native identities.
func ReadPolynomialMixedEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialMixedEnvelopeOptions) (*PolynomialMixedEnvelope, error) {
	var recipe polynomialMixedEnvelopeRecipe
	if e := readPolynomialGraphRecipe(ctx, r, limits, &recipe); e != nil {
		return nil, e
	}
	if recipe.Version != 1 || recipe.Kind != nativePolynomialMixedEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || len(recipe.Ledger.Contacts) != 2 {
		return nil, geom.ErrInvalid
	}
	sources, e := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if e != nil {
		return nil, e
	}
	out, e := CombineSplineGraphPolynomialMixedContact(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(recipe.Ledger, out.recipeLedger()) {
		return nil, geom.ErrConvergence
	}
	if limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{out.solid}) > limits.MaxEntities {
		return nil, geom.ErrConvergence
	}
	return out, nil
}
