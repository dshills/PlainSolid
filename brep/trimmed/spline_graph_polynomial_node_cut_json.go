package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativePolynomialNodeCut = "trimmed-polynomial-node-cut-components"

type polynomialNodeCutRecipe struct {
	Version    int               `json:"version"`
	Kind       string            `json:"kind"`
	Sources    []json.RawMessage `json:"sources"`
	Axis       *int              `json:"freeAxis"`
	Tolerance  float64           `json:"tolerance"`
	Parameters []string          `json:"parameters"`
	Point      []string          `json:"point"`
	Vertices   []int             `json:"vertices"`
	Sectors    []int             `json:"sectors"`
}

// WriteJSON records the three original supports and the contact ledger of a
// certified native Cut. Component surfaces and trim loops are reconstructed
// from this source recipe on load.
func (r *PolynomialNodeCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	recipe := polynomialNodeCutRecipe{
		Version: 1, Kind: nativePolynomialNodeCut, Axis: &r.axis, Tolerance: r.tolerance,
		Vertices: r.contact.Vertices[:], Sectors: r.contact.Sectors[:],
	}
	var e error
	recipe.Sources, e = encodePolynomialGraphSupports(ctx, r.sources)
	if e != nil {
		return e
	}
	for _, p := range r.contact.Parameters {
		recipe.Parameters = append(recipe.Parameters, p.RatString())
	}
	for _, p := range r.contact.Point {
		recipe.Point = append(recipe.Point, p.RatString())
	}
	return writePolynomialGraphRecipe(ctx, w, recipe)
}

// ReadPolynomialNodeCutJSON rebuilds both original-graph Cut shells and
// independently re-proves their embeddings and exact shared vertex contact.
// The recorded point, sectors and local vertex IDs must match the rebuilt cut.
func ReadPolynomialNodeCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*PolynomialNodeCut, error) {
	var recipe polynomialNodeCutRecipe
	if e := readPolynomialGraphRecipe(ctx, r, limits, &recipe); e != nil {
		return nil, e
	}
	if recipe.Version != 1 || recipe.Kind != nativePolynomialNodeCut ||
		len(recipe.Sources) != 3 || recipe.Axis == nil || len(recipe.Parameters) != 2 || len(recipe.Point) != 3 ||
		len(recipe.Vertices) != 2 || len(recipe.Sectors) != 2 {
		return nil, geom.ErrInvalid
	}
	sources, e := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphPolynomialNodeComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if !slices.Equal(recipe.Vertices, out.contact.Vertices[:]) || !slices.Equal(recipe.Sectors, out.contact.Sectors[:]) {
		return nil, geom.ErrConvergence
	}
	for i, p := range out.contact.Parameters {
		if recipe.Parameters[i] != p.RatString() {
			return nil, geom.ErrConvergence
		}
	}
	for i, p := range out.contact.Point {
		if recipe.Point[i] != p.RatString() {
			return nil, geom.ErrConvergence
		}
	}
	if limits.MaxEntities > 0 && polynomialGraphEntityCount(out.components[:]) > limits.MaxEntities {
		return nil, geom.ErrConvergence
	}
	return out, nil
}

func encodePolynomialGraphSupports(ctx context.Context, sources [3]*geom.BSplineSurface) ([]json.RawMessage, error) {
	var result []json.RawMessage
	for _, source := range sources {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		raw, e := parametric.EncodeSurface(source)
		if e != nil {
			return nil, e
		}
		result = append(result, raw)
	}
	return result, nil
}

func decodePolynomialGraphSupports(ctx context.Context, encoded []json.RawMessage, maxControls int) ([3]*geom.BSplineSurface, error) {
	var sources [3]*geom.BSplineSurface
	if maxControls == 0 {
		maxControls = 4096
	}
	for i, raw := range encoded {
		if e := ctx.Err(); e != nil {
			return sources, e
		}
		decoded, e := parametric.DecodeSurface(raw, maxControls)
		if e != nil {
			return sources, e
		}
		var ok bool
		sources[i], ok = decoded.(*geom.BSplineSurface)
		if !ok {
			return sources, geom.ErrInvalid
		}
	}
	return sources, nil
}

func writePolynomialGraphRecipe(ctx context.Context, w io.Writer, recipe any) error {
	raw, e := json.Marshal(recipe)
	if e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	raw = append(raw, '\n')
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

func readPolynomialGraphRecipe(ctx context.Context, r io.Reader, limits JSONLimits, recipe any) error {
	if ctx == nil || r == nil || limits.MaxBytes < 0 || limits.MaxBytes > 1<<30 || limits.MaxControlPoints < 0 || limits.MaxEntities < 0 {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	if limits.MaxBytes == 0 {
		limits.MaxBytes = 32 << 20
	}
	raw, e := io.ReadAll(io.LimitReader(r, limits.MaxBytes+1))
	if e != nil {
		return e
	}
	if int64(len(raw)) > limits.MaxBytes {
		return geom.ErrConvergence
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(recipe); e != nil {
		return e
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return geom.ErrInvalid
	}
	return nil
}

func polynomialGraphEntityCount(parts []Solid) int {
	count := 3 // original base and two tops
	for _, part := range parts {
		count += len(part.vertices) + len(part.edges) + len(part.faces)
		for _, face := range part.faces {
			count += len(face.Outer)
		}
	}
	return count
}
