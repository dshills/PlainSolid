package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"reflect"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// WrappedCovectorChainJSONOptions bounds a source-only three-face recipe.
// MaxBytes defaults to 8MiB (maximum 32MiB). MaxWork defaults to 64 million
// operations and is cumulative over face reconstruction, both pairs, strict
// separation and all native uses. Face/Assembly/Separation add per-stage limits.
// Writing conservatively reserves each face writer's full configured work
// allowance before reloading the complete recipe within the remaining budget.
type WrappedCovectorChainJSONOptions struct {
	MaxBytes   int64
	MaxWork    int
	Face       WrappedQuadraticFaceJSONOptions
	Assembly   edgecheck.Options
	Separation geom.SurfaceAffineCoordinateOptions
}
type wrappedCovectorChainRecipe struct {
	Version    int                              `json:"version"`
	Kind       string                           `json:"kind"`
	Faces      []json.RawMessage                `json:"faces"`
	Mates      []wrappedChainMateRecipe         `json:"mates"`
	Separators []wrappedCovectorSeparatorRecipe `json:"separators"`
	Margins    []string                         `json:"margins"`
	Witness    wrappedShellWitness              `json:"witness"`
}

func wrappedCovectorChainJSONOptions(o WrappedCovectorChainJSONOptions) (WrappedCovectorChainJSONOptions, error) {
	p, e := wrappedSeparatedChainJSONOptions(WrappedSeparatedChainJSONOptions(o))
	return WrappedCovectorChainJSONOptions(p), e
}

type wrappedCovectorSeparatorRecipe struct {
	Normal        []string `json:"normal"`
	Level         string   `json:"level"`
	ParameterAxes []int    `json:"parameterAxes"`
	Scale         []string `json:"scale"`
	Offset        []string `json:"offset"`
}

func encodeWrappedCovector(q WrappedCovectorSeparator) wrappedCovectorSeparatorRecipe {
	return wrappedCovectorSeparatorRecipe{Normal: []string{q.Normal[0].RatString(), q.Normal[1].RatString(), q.Normal[2].RatString()}, Level: q.Level.RatString(), ParameterAxes: []int{q.ParameterAxes[0], q.ParameterAxes[1]}, Scale: []string{q.Scale[0].RatString(), q.Scale[1].RatString()}, Offset: []string{q.Offset[0].RatString(), q.Offset[1].RatString()}}
}

// WriteWrappedAlgebraicCovectorChainJSON re-proves all stored geometry and indexed
// ancestry before emitting bytes. Original middle source is stored exactly once.
func WriteWrappedAlgebraicCovectorChainJSON(ctx context.Context, dst io.Writer, s *WrappedAlgebraicCovectorChain, o WrappedCovectorChainJSONOptions) error {
	if ctx == nil || dst == nil || s == nil || s.graph == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	o, e := wrappedCovectorChainJSONOptions(o)
	if e != nil {
		return e
	}
	if e := wrappedCovectorChainSourceBinding(ctx, s); e != nil {
		return e
	}
	n := wrappedCovectorChainRecipe{Version: 1, Kind: "wrapped-three-support-covector-chain", Margins: []string{s.outer.margins[0].RatString(), s.outer.margins[1].RatString()}}
	pairs := [2]*WrappedAlgebraicOpenShell{s.adjacent[0].Shell(), s.adjacent[1].Shell()}
	source := [3]*WrappedAlgebraicQuadraticFace{pairs[0].sources[0], pairs[0].sources[1], pairs[1].sources[1]}
	reserved := 0
	for _, f := range source {
		if o.MaxWork-reserved <= o.Face.Face.MaxOperations {
			return geom.ErrConvergence
		}
		var buf bytes.Buffer
		if e := WriteWrappedAlgebraicQuadraticFaceJSON(ctx, &buf, f, o.Face); e != nil {
			return e
		}
		reserved += o.Face.Face.MaxOperations
		n.Faces = append(n.Faces, append(json.RawMessage(nil), buf.Bytes()...))
	}
	for i, pair := range pairs {
		q := s.adjacent[i].Separator()
		n.Separators = append(n.Separators, encodeWrappedCovector(q))
		m := wrappedChainMateRecipe{ReverseB: pair.pair.ReversedParameter()}
		for i := 0; i < 2; i++ {
			owner, _ := pair.pair.Owner(i)
			m.Edges = append(m.Edges, []int{owner.Loop, owner.Edge})
		}
		n.Mates = append(n.Mates, m)
	}
	n.Separators = append(n.Separators, encodeWrappedCovector(s.outer.Separator()))
	n.Witness, e = wrappedIndexedShellWitness(s.graph.vertices, s.graph.edges, s.graph.faces, s.graph.vertexSources, s.graph.edgeSources, s.graph.boundary)
	if e != nil {
		return e
	}
	raw, e := json.Marshal(n)
	if e != nil {
		return e
	}
	if int64(len(raw)) > o.MaxBytes {
		return geom.ErrConvergence
	}
	o.MaxWork -= reserved
	if _, e := decodeWrappedCovectorChain(ctx, n, o); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	count, e := dst.Write(raw)
	if e == nil && count != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

// ReadWrappedAlgebraicCovectorChainJSON rebuilds all three immutable source faces,
// both mate proofs, their global embedding and the indexed native shell. Stored
// margins and ancestry witnesses must equal this independent reconstruction.
func ReadWrappedAlgebraicCovectorChainJSON(ctx context.Context, src io.Reader, o WrappedCovectorChainJSONOptions) (*WrappedAlgebraicCovectorChain, error) {
	if ctx == nil || src == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := wrappedCovectorChainJSONOptions(o)
	if e != nil {
		return nil, e
	}
	raw, e := io.ReadAll(io.LimitReader(src, o.MaxBytes+1))
	if e != nil {
		return nil, e
	}
	if int64(len(raw)) > o.MaxBytes {
		return nil, geom.ErrConvergence
	}
	if e := checkWrappedRecipeJSON(ctx, raw); e != nil {
		return nil, e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var n wrappedCovectorChainRecipe
	if e := d.Decode(&n); e != nil {
		return nil, e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return nil, geom.ErrInvalid
	}
	return decodeWrappedCovectorChain(ctx, n, o)
}
func decodeWrappedCovectorChain(ctx context.Context, n wrappedCovectorChainRecipe, o WrappedCovectorChainJSONOptions) (*WrappedAlgebraicCovectorChain, error) {
	if n.Version != 1 || n.Kind != "wrapped-three-support-covector-chain" || len(n.Faces) != 3 || len(n.Mates) != 2 || len(n.Separators) != 3 || len(n.Margins) != 2 {
		return nil, geom.ErrInvalid
	}
	for _, m := range n.Mates {
		if len(m.Edges) != 2 {
			return nil, geom.ErrInvalid
		}
		for _, edge := range m.Edges {
			if len(edge) != 2 || edge[0] != 0 || edge[1] < 0 || edge[1] >= 32 {
				return nil, geom.ErrInvalid
			}
		}
	}
	for _, m := range n.Margins {
		if len(m) > 2*o.Separation.MaxCoefficientBits+8 {
			return nil, geom.ErrConvergence
		}
	}
	w := n.Witness
	if len(w.Vertices) < 5 || len(w.Vertices) > 92 || len(w.Edges) < 7 || len(w.Edges) > 94 || len(w.FaceUses) != 3 || len(w.Boundary) != len(w.Vertices) {
		return nil, geom.ErrInvalid
	}
	for _, v := range w.Vertices {
		if len(v.Point) != 3 || len(v.Sources) < 1 || len(v.Sources) > 2 {
			return nil, geom.ErrInvalid
		}
		for _, p := range v.Point {
			if len(p) > 2*o.Face.Face.MaxCoefficientBits+8 {
				return nil, geom.ErrConvergence
			}
		}
		for _, s := range v.Sources {
			if len(s) != 2 {
				return nil, geom.ErrInvalid
			}
		}
	}
	for _, e := range w.Edges {
		if len(e.Vertices) != 2 || len(e.Range) != 2 || len(e.Sources) < 1 || len(e.Sources) > 2 {
			return nil, geom.ErrInvalid
		}
		for _, s := range e.Sources {
			if len(s) != 5 {
				return nil, geom.ErrInvalid
			}
		}
		for _, r := range e.Range {
			if len(r) > 2*o.Face.Face.MaxCoefficientBits+8 {
				return nil, geom.ErrConvergence
			}
		}
	}
	for _, uses := range w.FaceUses {
		if len(uses) < 3 || len(uses) > 32 {
			return nil, geom.ErrInvalid
		}
		for _, u := range uses {
			if len(u) != 2 {
				return nil, geom.ErrInvalid
			}
		}
	}
	for _, u := range w.Boundary {
		if len(u) != 7 {
			return nil, geom.ErrInvalid
		}
	}
	remaining := o.MaxWork
	var separators [3]WrappedCovectorSeparator
	for i, r := range n.Separators {
		q, e := parseCovectorChainSeparator(r, o.Separation.MaxCoefficientBits, &remaining)
		if e != nil {
			return nil, e
		}
		separators[i] = q
	}
	var faces [3]*WrappedAlgebraicQuadraticFace
	for i, raw := range n.Faces {
		if remaining < 1 {
			return nil, geom.ErrConvergence
		}
		opts := o.Face
		opts.Face.MaxOperations = min(opts.Face.MaxOperations, remaining)
		f, e := ReadWrappedAlgebraicQuadraticFaceJSON(ctx, bytes.NewReader(raw), opts)
		if e != nil {
			return nil, e
		}
		remaining -= f.ExactOperations()
		faces[i] = f
	}
	var pairs [2]*WrappedAlgebraicOpenShell
	for i, m := range n.Mates {
		if remaining < 1 {
			return nil, geom.ErrConvergence
		}
		opts := o.Assembly
		opts.MaxWork = min(opts.MaxWork, remaining)
		p, e := NewWrappedAlgebraicOpenShell(ctx, faces[i], faces[i+1], [2]int{m.Edges[0][0], m.Edges[0][1]}, [2]int{m.Edges[1][0], m.Edges[1][1]}, m.ReverseB, opts)
		if e != nil {
			return nil, e
		}
		remaining -= p.ExactOperations()
		pairs[i] = p
	}
	if remaining < 1 {
		return nil, geom.ErrConvergence
	}
	opts := o.Separation
	opts.MaxOperations = min(opts.MaxOperations, remaining)
	s, e := NewWrappedAlgebraicCovectorChain(ctx, pairs[0], pairs[1], [2]WrappedCovectorSeparator{separators[0], separators[1]}, separators[2], opts)
	if e != nil {
		return nil, e
	}
	if s.outer.margins[0].RatString() != n.Margins[0] || s.outer.margins[1].RatString() != n.Margins[1] {
		return nil, geom.ErrDomain
	}
	expected, e := wrappedIndexedShellWitness(s.graph.vertices, s.graph.edges, s.graph.faces, s.graph.vertexSources, s.graph.edgeSources, s.graph.boundary)
	if e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(w, expected) {
		return nil, geom.ErrDomain
	}
	return s, nil
}

func parseCovectorChainSeparator(r wrappedCovectorSeparatorRecipe, bits int, remaining *int) (WrappedCovectorSeparator, error) {
	var q WrappedCovectorSeparator
	if len(r.Normal) != 3 || len(r.ParameterAxes) != 2 || len(r.Scale) != 2 || len(r.Offset) != 2 {
		return q, geom.ErrInvalid
	}
	parse := func(text string) (*big.Rat, error) {
		*remaining -= 1
		if *remaining < 1 || len(text) > 2*bits+8 {
			return nil, geom.ErrConvergence
		}
		v, ok := new(big.Rat).SetString(text)
		if !ok || v.RatString() != text {
			return nil, geom.ErrInvalid
		}
		if v.Num().BitLen() > bits || v.Denom().BitLen() > bits {
			return nil, geom.ErrConvergence
		}
		return v, nil
	}

	var e error
	for i := range q.Normal {
		q.Normal[i], e = parse(r.Normal[i])
		if e != nil {
			return q, e
		}
	}
	q.Level, e = parse(r.Level)
	if e != nil {
		return q, e
	}
	for i := 0; i < 2; i++ {
		q.ParameterAxes[i] = r.ParameterAxes[i]
		if q.ParameterAxes[i] < 0 || q.ParameterAxes[i] > 1 {
			return q, geom.ErrInvalid
		}
		q.Scale[i], e = parse(r.Scale[i])
		if e != nil {
			return q, e
		}
		q.Offset[i], e = parse(r.Offset[i])
		if e != nil {
			return q, e
		}
	}
	return q, nil
}
func wrappedCovectorChainSourceBinding(ctx context.Context, s *WrappedAlgebraicCovectorChain) error {
	if s == nil || s.graph == nil || s.adjacent[0] == nil || s.adjacent[1] == nil {
		return geom.ErrInvalid
	}
	a, b := s.adjacent[0].Shell(), s.adjacent[1].Shell()
	if a == nil || b == nil {
		return geom.ErrInvalid
	}
	source := []*WrappedAlgebraicQuadraticFace{a.sources[0], a.sources[1], b.sources[1]}
	return wrappedIndexedSourceBinding(ctx, s.graph.faces, s.graph.edges, s.graph.edgeSources, source)
}
