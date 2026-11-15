package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativePolynomialTangencyEnvelope = "trimmed-polynomial-tangency-envelope"

type polynomialTangencyEnvelopeRecipe struct {
	Version    int                   `json:"version"`
	Kind       string                `json:"kind"`
	Sources    []json.RawMessage     `json:"sources"`
	Axis       *int                  `json:"freeAxis"`
	Operation  *parametric.BooleanOp `json:"operation"`
	Tolerance  float64               `json:"tolerance"`
	Parameters []string              `json:"parameters"`
	Point      []string              `json:"point"`
	Vertices   []int                 `json:"vertices"`
	Edges      []int                 `json:"edges"`
	RoofFaces  []int                 `json:"roofFaces"`
	RoofOwners []int                 `json:"roofOwners"`
	Base       int                   `json:"base"`
}

// WriteJSON records the original supports and bound tangent vertex, edge and
// roof identities. It writes a proof recipe for ReadPolynomialTangencyEnvelopeJSON.
func (r *PolynomialTangencyEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	sources, e := encodePolynomialGraphSupports(ctx, r.sources)
	if e != nil {
		return e
	}
	recipe := polynomialTangencyEnvelopeRecipe{
		Version: 1, Kind: nativePolynomialTangencyEnvelope, Sources: sources, Axis: &r.axis,
		Operation: &r.op, Tolerance: r.tolerance, Vertices: r.history.Vertices[:], Edges: r.history.Edges[:],
		RoofFaces: r.history.RoofFaces[:], RoofOwners: r.history.RoofOwners[:], Base: r.history.Base,
	}
	for _, p := range r.parameters {
		recipe.Parameters = append(recipe.Parameters, p.RatString())
	}
	for _, p := range r.point {
		recipe.Point = append(recipe.Point, p.RatString())
	}
	return writePolynomialGraphRecipe(ctx, w, recipe)
}

// ReadPolynomialTangencyEnvelopeJSON reconstructs the original graph
// tangency and exact owner restrictions before binding the recorded topology.
func ReadPolynomialTangencyEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialTangencyEnvelopeOptions) (*PolynomialTangencyEnvelope, error) {
	var recipe polynomialTangencyEnvelopeRecipe
	if e := readPolynomialGraphRecipe(ctx, r, limits, &recipe); e != nil {
		return nil, e
	}
	if recipe.Version != 1 || recipe.Kind != nativePolynomialTangencyEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || len(recipe.Parameters) != 2 || len(recipe.Point) != 3 ||
		len(recipe.Vertices) != 5 || len(recipe.Edges) != 4 || len(recipe.RoofFaces) != 4 || len(recipe.RoofOwners) != 4 {
		return nil, geom.ErrInvalid
	}
	sources, e := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if e != nil {
		return nil, e
	}
	out, e := CombineSplineGraphPolynomialTangency(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	h := out.history
	if !slices.Equal(recipe.Vertices, h.Vertices[:]) || !slices.Equal(recipe.Edges, h.Edges[:]) ||
		!slices.Equal(recipe.RoofFaces, h.RoofFaces[:]) || !slices.Equal(recipe.RoofOwners, h.RoofOwners[:]) || recipe.Base != h.Base {
		return nil, geom.ErrConvergence
	}
	for i, p := range out.parameters {
		if recipe.Parameters[i] != p.RatString() {
			return nil, geom.ErrConvergence
		}
	}
	for i, p := range out.point {
		if recipe.Point[i] != p.RatString() {
			return nil, geom.ErrConvergence
		}
	}
	if limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{out.solid}) > limits.MaxEntities {
		return nil, geom.ErrConvergence
	}
	return out, nil
}
