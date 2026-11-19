//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedAlgebraicChainJSONOptions bounds a source-only three-face recipe.
// MaxBytes defaults to 8MiB (maximum 32MiB). MaxWork defaults to 64 million
// operations and is cumulative over face reconstruction, both pairs, global
// embedding, and all native uses. Face/Assembly/Embedding add per-stage limits.
// Writing conservatively reserves each face writer's full configured work
// allowance before reloading the complete recipe within the remaining budget.
type WrappedAlgebraicChainJSONOptions struct {
	MaxBytes  int64
	MaxWork   int
	Face      WrappedQuadraticFaceJSONOptions
	Assembly  edgecheck.Options
	Embedding geom.SurfaceInjectivityOptions
}
type wrappedChainMateRecipe struct {
	Edges    [][]int `json:"edges"`
	ReverseB bool    `json:"reverseB"`
}
type wrappedChainRecipe struct {
	Version        int                      `json:"version"`
	Kind           string                   `json:"kind"`
	Faces          []json.RawMessage        `json:"faces"`
	Mates          []wrappedChainMateRecipe `json:"mates"`
	PeriodShifts   [][]string               `json:"periodShifts"`
	ProjectionAxes []int                    `json:"projectionAxes"`
	Witness        wrappedShellWitness      `json:"witness"`
}

func wrappedChainJSONOptions(o WrappedAlgebraicChainJSONOptions) (WrappedAlgebraicChainJSONOptions, error) {
	if o.MaxBytes == 0 {
		o.MaxBytes = 8 << 20
	}
	if o.MaxWork == 0 {
		o.MaxWork = 64000000
	}
	base, e := wrappedShellJSONOptions(WrappedAlgebraicOpenShellJSONOptions{MaxBytes: o.MaxBytes, Face: o.Face, Assembly: o.Assembly})
	if e != nil {
		return o, e
	}
	o.Face = base.Face
	o.Assembly = base.Assembly
	if o.Embedding.MaxOperations == 0 {
		o.Embedding.MaxOperations = 16000000
	}
	if o.Embedding.MaxCoefficientBits == 0 {
		o.Embedding.MaxCoefficientBits = 16384
	}
	if o.MaxWork < 1 || o.MaxWork > 1000000000 || o.Embedding.MaxOperations < 1 || o.Embedding.MaxCoefficientBits < 64 || o.Embedding.MaxCoefficientBits > 65536 {
		return o, geom.ErrInvalid
	}
	return o, nil
}

// WriteWrappedAlgebraicChainShellJSON re-proves all stored geometry and indexed
// ancestry before emitting bytes. Original middle source is stored exactly once.
func WriteWrappedAlgebraicChainShellJSON(ctx context.Context, dst io.Writer, s *WrappedAlgebraicChainShell, o WrappedAlgebraicChainJSONOptions) error {
	if ctx == nil || dst == nil || s == nil || s.embedding == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	o, e := wrappedChainJSONOptions(o)
	if e != nil {
		return e
	}
	if e := wrappedChainSourceBinding(ctx, s); e != nil {
		return e
	}
	p := s.embedding
	n := wrappedChainRecipe{Version: 1, Kind: "wrapped-algebraic-three-face-chain", ProjectionAxes: []int{p.axes[0], p.axes[1]}}
	source := [3]*WrappedAlgebraicQuadraticFace{p.pairs[0].sources[0], p.pairs[0].sources[1], p.pairs[1].sources[1]}
	reserved := 0
	for i, f := range source {
		if o.MaxWork-reserved <= o.Face.Face.MaxOperations {
			return geom.ErrConvergence
		}
		var buf bytes.Buffer
		if e := WriteWrappedAlgebraicQuadraticFaceJSON(ctx, &buf, f, o.Face); e != nil {
			return e
		}
		reserved += o.Face.Face.MaxOperations
		n.Faces = append(n.Faces, append(json.RawMessage(nil), buf.Bytes()...))
		n.PeriodShifts = append(n.PeriodShifts, []string{p.shifts[i][0].String(), p.shifts[i][1].String()})
	}
	for _, pair := range p.pairs {
		m := wrappedChainMateRecipe{ReverseB: pair.pair.ReversedParameter()}
		for i := 0; i < 2; i++ {
			owner, _ := pair.pair.Owner(i)
			m.Edges = append(m.Edges, []int{owner.Loop, owner.Edge})
		}
		n.Mates = append(n.Mates, m)
	}
	n.Witness, e = wrappedIndexedShellWitness(s.vertices, s.edges, s.faces, s.vertexSources, s.edgeSources, s.boundary)
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
	if _, e := decodeWrappedChain(ctx, n, o); e != nil {
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

// ReadWrappedAlgebraicChainShellJSON rebuilds all three immutable source faces,
// both mate proofs, their global embedding and the indexed native shell. Stored
// lift and ancestry witnesses must equal this independent reconstruction.
func ReadWrappedAlgebraicChainShellJSON(ctx context.Context, src io.Reader, o WrappedAlgebraicChainJSONOptions) (*WrappedAlgebraicChainShell, error) {
	if ctx == nil || src == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := wrappedChainJSONOptions(o)
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
	var n wrappedChainRecipe
	if e := d.Decode(&n); e != nil {
		return nil, e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return nil, geom.ErrInvalid
	}
	return decodeWrappedChain(ctx, n, o)
}
func decodeWrappedChain(ctx context.Context, n wrappedChainRecipe, o WrappedAlgebraicChainJSONOptions) (*WrappedAlgebraicChainShell, error) {
	if n.Version != 1 || n.Kind != "wrapped-algebraic-three-face-chain" || len(n.Faces) != 3 || len(n.Mates) != 2 || len(n.PeriodShifts) != 3 || len(n.ProjectionAxes) != 2 {
		return nil, geom.ErrInvalid
	}
	for _, m := range n.Mates {
		if len(m.Edges) != 2 || !m.ReverseB {
			return nil, geom.ErrInvalid
		}
		for _, edge := range m.Edges {
			if len(edge) != 2 || edge[0] != 0 || edge[1] < 0 || edge[1] >= 32 {
				return nil, geom.ErrInvalid
			}
		}
	}
	for _, shift := range n.PeriodShifts {
		if len(shift) != 2 {
			return nil, geom.ErrInvalid
		}
		for _, v := range shift {
			if len(v) > 2*o.Embedding.MaxCoefficientBits+8 {
				return nil, geom.ErrConvergence
			}
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
	opts := o.Embedding
	opts.MaxOperations = min(opts.MaxOperations, remaining)
	opts.ProjectionAxes = [2]int{n.ProjectionAxes[0], n.ProjectionAxes[1]}
	p, e := CertifyWrappedAlgebraicChainEmbedding(ctx, pairs[0], pairs[1], opts)
	if e != nil {
		return nil, e
	}
	remaining -= p.ExactOperations()
	for i, shift := range n.PeriodShifts {
		for j, v := range shift {
			if p.shifts[i][j].String() != v {
				return nil, geom.ErrDomain
			}
		}
	}
	if remaining < 1 {
		return nil, geom.ErrConvergence
	}
	assembly := o.Assembly
	assembly.MaxWork = min(assembly.MaxWork, remaining)
	s, e := NewWrappedAlgebraicChainShell(ctx, p, assembly)
	if e != nil {
		return nil, e
	}
	expected, e := wrappedIndexedShellWitness(s.vertices, s.edges, s.faces, s.vertexSources, s.edgeSources, s.boundary)
	if e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(w, expected) {
		return nil, geom.ErrDomain
	}
	return s, nil
}

func wrappedChainSourceBinding(ctx context.Context, s *WrappedAlgebraicChainShell) error {
	if len(s.faces) != 3 || len(s.edgeSources) != len(s.edges) || s.embedding.pairs[0] == nil || s.embedding.pairs[1] == nil {
		return geom.ErrInvalid
	}
	p := s.embedding
	source := [3]*WrappedAlgebraicQuadraticFace{p.pairs[0].sources[0], p.pairs[0].sources[1], p.pairs[1].sources[1]}
	return wrappedIndexedSourceBinding(ctx, s.faces, s.edges, s.edgeSources, source[:])
}

func wrappedIndexedSourceBinding(ctx context.Context, faces []Face, edges []parametric.Edge, edgeSources [][]WrappedOpenEdgeSource, source []*WrappedAlgebraicQuadraticFace) error {
	if len(faces) != len(source) || len(edges) != len(edgeSources) {
		return geom.ErrInvalid
	}
	// Reuse the exact source/evaluator binding check, with a temporary two-owner
	// view for each actual face and its owned canonical model edges. No geometry
	// or proof is generated by this view, and no indexed incidence is inferred.
	for fi, f := range faces {
		view := &WrappedAlgebraicOpenShell{faces: []Face{f, f}, sources: [2]*WrappedAlgebraicQuadraticFace{source[fi], source[fi]}}
		for ei, owners := range edgeSources {
			if len(owners) < 1 {
				return geom.ErrInvalid
			}
			owner := owners[0]
			if owner.Face == fi {
				owner.Face = 0
				view.edges = append(view.edges, edges[ei])
				view.edgeSources = append(view.edgeSources, []WrappedOpenEdgeSource{owner})
			}
		}
		if e := wrappedShellSourceBinding(ctx, view); e != nil {
			return e
		}
	}
	return ctx.Err()
}
