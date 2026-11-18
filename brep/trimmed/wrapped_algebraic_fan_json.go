//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// WrappedAlgebraicFanJSONOptions bounds a source-only three-/four-face fan recipe.
// MaxBytes defaults to 8MiB (maximum 32MiB). MaxWork defaults to 64 million
// operations and is cumulative over face reconstruction, all cyclic pairs, global
// embedding, and all native uses. Face/Assembly/Embedding add per-stage limits.
// Writing conservatively reserves each face writer's full configured work
// allowance before reloading the complete recipe within the remaining budget.
type WrappedAlgebraicFanJSONOptions struct {
	MaxBytes  int64
	MaxWork   int
	Face      WrappedQuadraticFaceJSONOptions
	Assembly  edgecheck.Options
	Embedding geom.SurfaceInjectivityOptions
}
type wrappedFanRecipe struct {
	Version        int                      `json:"version"`
	Kind           string                   `json:"kind"`
	Faces          []json.RawMessage        `json:"faces"`
	Mates          []wrappedChainMateRecipe `json:"mates"`
	PeriodShifts   [][]string               `json:"periodShifts"`
	ProjectionAxes []int                    `json:"projectionAxes"`
	Center         int                      `json:"center"`
	Centers        []int                    `json:"centers"`
	Witness        wrappedShellWitness      `json:"witness"`
}

func wrappedFanJSONOptions(o WrappedAlgebraicFanJSONOptions) (WrappedAlgebraicFanJSONOptions, error) {
	base, e := wrappedChainJSONOptions(WrappedAlgebraicChainJSONOptions(o))
	return WrappedAlgebraicFanJSONOptions(base), e
}

// WriteWrappedAlgebraicFanShellJSON re-proves all stored geometry and indexed
// ancestry before emitting bytes. Each original source is stored exactly once.
func WriteWrappedAlgebraicFanShellJSON(ctx context.Context, dst io.Writer, s *WrappedAlgebraicFanShell, o WrappedAlgebraicFanJSONOptions) error {
	if ctx == nil || dst == nil || s == nil || s.embedding == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	o, e := wrappedFanJSONOptions(o)
	if e != nil {
		return e
	}
	if e := wrappedFanSourceBinding(ctx, s); e != nil {
		return e
	}
	p := s.embedding.proof
	n := wrappedFanRecipe{Version: 1, Kind: "wrapped-algebraic-cyclic-fan", ProjectionAxes: []int{p.axes[0], p.axes[1]}, Center: s.center, Centers: append([]int(nil), s.embedding.centers...)}
	source := make([]*WrappedAlgebraicQuadraticFace, len(p.pairs))
	for i, pair := range p.pairs {
		source[i] = pair.sources[0]
	}
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
	if _, e := decodeWrappedFan(ctx, n, o); e != nil {
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

// ReadWrappedAlgebraicFanShellJSON rebuilds all original immutable source faces,
// all cyclic mate proofs, their global embedding and the indexed native shell. Stored
// lift and ancestry witnesses must equal this independent reconstruction.
func ReadWrappedAlgebraicFanShellJSON(ctx context.Context, src io.Reader, o WrappedAlgebraicFanJSONOptions) (*WrappedAlgebraicFanShell, error) {
	if ctx == nil || src == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := wrappedFanJSONOptions(o)
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
	var n wrappedFanRecipe
	if e := d.Decode(&n); e != nil {
		return nil, e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return nil, geom.ErrInvalid
	}
	return decodeWrappedFan(ctx, n, o)
}
func decodeWrappedFan(ctx context.Context, n wrappedFanRecipe, o WrappedAlgebraicFanJSONOptions) (*WrappedAlgebraicFanShell, error) {
	nf := len(n.Faces)
	if n.Version != 1 || n.Kind != "wrapped-algebraic-cyclic-fan" || (nf != 3 && nf != 4) || len(n.Mates) != nf || len(n.PeriodShifts) != nf || len(n.Centers) != nf || len(n.ProjectionAxes) != 2 {
		return nil, geom.ErrInvalid
	}
	for _, v := range n.Centers {
		if v < 0 || v >= 32 {
			return nil, geom.ErrInvalid
		}
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
	if len(w.Vertices) < 4 || len(w.Vertices) > 128 || len(w.Edges) < 6 || len(w.Edges) > 128 || len(w.FaceUses) != nf || len(w.Boundary) != len(w.Vertices)-1 || n.Center < 0 || n.Center >= len(w.Vertices) {
		return nil, geom.ErrInvalid
	}
	for _, v := range w.Vertices {
		if len(v.Point) != 3 || len(v.Sources) < 1 || len(v.Sources) > nf {
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
	faces := make([]*WrappedAlgebraicQuadraticFace, nf)
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
	pairs := make([]*WrappedAlgebraicOpenShell, nf)
	for i, m := range n.Mates {
		if remaining < 1 {
			return nil, geom.ErrConvergence
		}
		opts := o.Assembly
		opts.MaxWork = min(opts.MaxWork, remaining)
		p, e := NewWrappedAlgebraicOpenShell(ctx, faces[i], faces[(i+1)%nf], [2]int{m.Edges[0][0], m.Edges[0][1]}, [2]int{m.Edges[1][0], m.Edges[1][1]}, m.ReverseB, opts)
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
	p, e := CertifyWrappedAlgebraicFanEmbedding(ctx, pairs, opts)
	if e != nil {
		return nil, e
	}
	remaining -= p.ExactOperations()
	for i, shift := range n.PeriodShifts {
		for j, v := range shift {
			if p.proof.shifts[i][j].String() != v {
				return nil, geom.ErrDomain
			}
		}
	}
	if remaining < 1 {
		return nil, geom.ErrConvergence
	}
	assembly := o.Assembly
	assembly.MaxWork = min(assembly.MaxWork, remaining)
	s, e := NewWrappedAlgebraicFanShell(ctx, p, assembly)
	if e != nil {
		return nil, e
	}
	if s.center != n.Center || !reflect.DeepEqual(p.centers, n.Centers) {
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

func wrappedFanSourceBinding(ctx context.Context, s *WrappedAlgebraicFanShell) error {
	if s == nil || s.graph == nil || s.embedding == nil || s.embedding.proof == nil {
		return geom.ErrInvalid
	}
	p := s.embedding.proof
	nf := len(p.pairs)
	if (nf != 3 && nf != 4) || len(s.graph.faces) != nf {
		return geom.ErrInvalid
	}
	source := make([]*WrappedAlgebraicQuadraticFace, nf)
	for i, pair := range p.pairs {
		if pair == nil {
			return geom.ErrInvalid
		}
		source[i] = pair.sources[0]
	}
	return wrappedIndexedSourceBinding(ctx, s.graph.faces, s.graph.edges, s.graph.edgeSources, source)
}
