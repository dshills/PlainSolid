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

type polynomialEnvelopeRecipe struct {
	Version   int                      `json:"version"`
	Kind      string                   `json:"kind"`
	Sources   []json.RawMessage        `json:"sources"`
	Axis      *int                     `json:"freeAxis"`
	Operation *parametric.BooleanOp    `json:"operation"`
	Tolerance float64                  `json:"tolerance"`
	Ledger    polynomialEnvelopeLedger `json:"ledger"`
}

type polynomialEnvelopeLedger struct {
	Edges      [][]int                           `json:"edges"`
	Vertices   []int                             `json:"vertices"`
	Midpoints  []int                             `json:"midpoints"`
	RoofFaces  []int                             `json:"roofFaces"`
	RoofOwners []int                             `json:"roofOwners"`
	Base       int                               `json:"base"`
	Contacts   []polynomialEnvelopeRecipeContact `json:"contacts"`
}

type polynomialEnvelopeRecipeContact struct {
	Parameters    []string `json:"parameters"`
	Point         []string `json:"point"`
	Order         int      `json:"order"`
	DerivativeGap string   `json:"derivativeGap"`
	Vertex        int      `json:"vertex"`
	Rays          []int    `json:"rays"`
	Sectors       []int    `json:"sectors"`
}

func (r *PolynomialMixedEnvelope) recipeLedger() polynomialEnvelopeLedger {
	h := r.history
	return makePolynomialEnvelopeLedger(h.Edges[:], h.Vertices[:], h.Midpoints[:], h.RoofFaces[:], h.RoofOwners[:], h.Base, r.contacts[:])
}

func makePolynomialEnvelopeLedger(edges [][2]int, vertices, midpoints, faces, owners []int, base int, contacts []PolynomialMixedEnvelopeContact) polynomialEnvelopeLedger {
	ledger := polynomialEnvelopeLedger{
		Vertices: vertices, Midpoints: midpoints, RoofFaces: faces, RoofOwners: owners, Base: base,
	}
	for _, chain := range edges {
		if chain[1] < 0 {
			ledger.Edges = append(ledger.Edges, []int{chain[0]})
		} else {
			ledger.Edges = append(ledger.Edges, []int{chain[0], chain[1]})
		}
	}
	for _, c := range contacts {
		entry := polynomialEnvelopeRecipeContact{
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
	if r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writePolynomialEnvelopeRecipe(ctx, w, nativePolynomialMixedEnvelope, r.sources, r.axis, r.op, r.tolerance, r.recipeLedger())
}

func writePolynomialEnvelopeRecipe(ctx context.Context, w io.Writer, kind string, stock [3]*geom.BSplineSurface, axis int, op parametric.BooleanOp,
	tolerance float64, ledger polynomialEnvelopeLedger) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	sources, e := encodePolynomialGraphSupports(ctx, stock)
	if e != nil {
		return e
	}
	return writePolynomialGraphRecipe(ctx, w, polynomialEnvelopeRecipe{
		Version: 1, Kind: kind, Sources: sources, Axis: &axis, Operation: &op, Tolerance: tolerance, Ledger: ledger,
	})
}

// ReadPolynomialMixedEnvelopeJSON re-proves the original crossing, tangency
// and both owner coedges before binding their recorded native identities.
func ReadPolynomialMixedEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialMixedEnvelopeOptions) (*PolynomialMixedEnvelope, error) {
	recipe, sources, e := readPolynomialEnvelopeRecipe(ctx, r, limits, nativePolynomialMixedEnvelope, 2)
	if e != nil {
		return nil, e
	}
	out, e := CombineSplineGraphPolynomialMixedContact(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}

func readPolynomialEnvelopeRecipe(ctx context.Context, r io.Reader, limits JSONLimits, kind string, nodes int) (polynomialEnvelopeRecipe, [3]*geom.BSplineSurface, error) {
	var recipe polynomialEnvelopeRecipe
	if e := readPolynomialGraphRecipe(ctx, r, limits, &recipe); e != nil {
		return polynomialEnvelopeRecipe{}, [3]*geom.BSplineSurface{}, e
	}
	if recipe.Version != 1 || recipe.Kind != kind || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || len(recipe.Ledger.Contacts) != nodes {
		return polynomialEnvelopeRecipe{}, [3]*geom.BSplineSurface{}, geom.ErrInvalid
	}
	sources, e := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	return recipe, sources, e
}

func (recipe polynomialEnvelopeRecipe) verify(ledger polynomialEnvelopeLedger, solid Solid, limits JSONLimits) error {
	if !reflect.DeepEqual(recipe.Ledger, ledger) {
		return geom.ErrConvergence
	}
	if limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{solid}) > limits.MaxEntities {
		return geom.ErrConvergence
	}
	return nil
}
