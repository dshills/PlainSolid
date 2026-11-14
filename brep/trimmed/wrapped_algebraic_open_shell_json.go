package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"reflect"
)

// WrappedAlgebraicOpenShellJSONOptions bounds a standalone two-face incidence
// recipe. MaxBytes defaults4MiB, max32MiB. Face limits apply to each of the two
// face proofs; Assembly limits apply to each complete shell reconstruction.
// Writers re-prove before publication; these are bounded per-stage limits,
// not one shared arithmetic allowance across all serialization stages.
type WrappedAlgebraicOpenShellJSONOptions struct {
	MaxBytes int64
	Face     WrappedQuadraticFaceJSONOptions
	Assembly edgecheck.Options
}
type wrappedShellVertexWitness struct {
	Point   []string `json:"point"`
	Sources [][]int  `json:"sources"`
}
type wrappedShellEdgeWitness struct {
	Vertices []int    `json:"vertices"`
	Range    []string `json:"range"`
	Sources  [][]int  `json:"sources"`
}
type wrappedShellWitness struct {
	Vertices []wrappedShellVertexWitness `json:"vertices"`
	Edges    []wrappedShellEdgeWitness   `json:"edges"`
	FaceUses [][][]int                   `json:"faceUses"`
	Boundary [][]int                     `json:"boundary"`
}
type wrappedAlgebraicShellRecipe struct {
	Version  int                 `json:"version"`
	Kind     string              `json:"kind"`
	Faces    []json.RawMessage   `json:"faces"`
	Mate     [][]int             `json:"mate"`
	ReverseB bool                `json:"reverseB"`
	Witness  wrappedShellWitness `json:"witness"`
}

func wrappedShellJSONOptions(o WrappedAlgebraicOpenShellJSONOptions) (WrappedAlgebraicOpenShellJSONOptions, error) {
	if o.MaxBytes == 0 {
		o.MaxBytes = 4 << 20
	}
	var e error
	o.Face, e = wrappedQuadraticJSONOptions(o.Face)
	if e != nil {
		return o, e
	}
	if o.Assembly.MaxWork == 0 {
		o.Assembly.MaxWork = 8000000
	}
	if o.Assembly.MaxBits == 0 {
		o.Assembly.MaxBits = 16384
	}
	if o.Assembly.MaxIntervals == 0 {
		o.Assembly.MaxIntervals = 4096
	}
	if o.MaxBytes < 1 || o.MaxBytes > 32<<20 || o.Face.Face.MaxPieces > 8192 || o.Assembly.MaxWork < 1 || o.Assembly.MaxBits < 64 || o.Assembly.MaxBits > 65536 || o.Assembly.MaxIntervals < 1 || o.Assembly.MaxIntervals > 8192 {
		return o, geom.ErrInvalid
	}
	return o, nil
}
func wrappedBool(x bool) int {
	if x {
		return 1
	}
	return 0
}
func wrappedShellNativeWitness(s *WrappedAlgebraicOpenShell) (wrappedShellWitness, error) {
	var out wrappedShellWitness
	if s == nil || len(s.vertexSources) != len(s.vertices) || len(s.edgeSources) != len(s.edges) || len(s.faces) != 2 {
		return out, geom.ErrInvalid
	}
	for i, p := range s.vertices {
		if !p.IsFinite() {
			return out, geom.ErrInvalid
		}
		v := wrappedShellVertexWitness{Point: []string{ratInset(p.X).RatString(), ratInset(p.Y).RatString(), ratInset(p.Z).RatString()}}
		for _, source := range s.vertexSources[i] {
			v.Sources = append(v.Sources, []int{source.Face, source.Vertex})
		}
		out.Vertices = append(out.Vertices, v)
	}
	for i, e := range s.edges {
		w := wrappedShellEdgeWitness{Vertices: []int{e.Start, e.End}, Range: []string{ratInset(e.Range.Min).RatString(), ratInset(e.Range.Max).RatString()}}
		for _, source := range s.edgeSources[i] {
			w.Sources = append(w.Sources, []int{source.Face, source.Edge, source.Loop, source.LocalEdge, wrappedBool(source.Reversed)})
		}
		out.Edges = append(out.Edges, w)
	}
	for _, face := range s.faces {
		var uses [][]int
		for _, u := range face.Outer {
			uses = append(uses, []int{u.Edge, wrappedBool(u.Reversed)})
		}
		out.FaceUses = append(out.FaceUses, uses)
	}
	for _, u := range s.boundary {
		out.Boundary = append(out.Boundary, []int{u.Face, u.LocalEdge, u.Use.Edge, wrappedBool(u.Use.Reversed), wrappedBool(u.ReversedTraversal), u.Start, u.End})
	}
	return out, nil
}

// WriteWrappedAlgebraicOpenShellJSON stores original immutable face recipes,
// original mate selectors and redundant native incidence/ownership witnesses.
// Rebuilding and validating every witness precedes the first output byte.
// This format retains the open shell's lack of a global nonoverlap claim.
func WriteWrappedAlgebraicOpenShellJSON(ctx context.Context, dst io.Writer, s *WrappedAlgebraicOpenShell, o WrappedAlgebraicOpenShellJSONOptions) error {
	if ctx == nil || dst == nil || s == nil || !s.pair.Proven() || s.sources[0] == nil || s.sources[1] == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	o, e := wrappedShellJSONOptions(o)
	if e != nil {
		return e
	}
	if e := wrappedShellSourceBinding(ctx, s); e != nil {
		return e
	}
	n := wrappedAlgebraicShellRecipe{Version: 1, Kind: "wrapped-algebraic-two-face-open-shell", ReverseB: s.pair.ReversedParameter()}
	for owner, face := range s.sources {
		var raw bytes.Buffer
		if e := WriteWrappedAlgebraicQuadraticFaceJSON(ctx, &raw, face, o.Face); e != nil {
			return e
		}
		n.Faces = append(n.Faces, append(json.RawMessage(nil), raw.Bytes()...))
		source, e := s.pair.Owner(owner)
		if e != nil {
			return e
		}
		n.Mate = append(n.Mate, []int{source.Loop, source.Edge})
	}
	n.Witness, e = wrappedShellNativeWitness(s)
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
	if _, e := decodeWrappedAlgebraicShell(ctx, n, o); e != nil {
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

// ReadWrappedAlgebraicOpenShellJSON reconstructs both original open faces,
// re-proves the complete edge mate, rebuilds indexed incidence, and compares
// exact witnesses. No stored certificate/complete flag is accepted. Duplicate
// or unknown fields, malformed ancestry and orientation mismatches reject.
func ReadWrappedAlgebraicOpenShellJSON(ctx context.Context, src io.Reader, o WrappedAlgebraicOpenShellJSONOptions) (*WrappedAlgebraicOpenShell, error) {
	if ctx == nil || src == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := wrappedShellJSONOptions(o)
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
	var n wrappedAlgebraicShellRecipe
	if e := d.Decode(&n); e != nil {
		return nil, e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return nil, geom.ErrInvalid
	}
	return decodeWrappedAlgebraicShell(ctx, n, o)
}
func decodeWrappedAlgebraicShell(ctx context.Context, n wrappedAlgebraicShellRecipe, o WrappedAlgebraicOpenShellJSONOptions) (*WrappedAlgebraicOpenShell, error) {
	if n.Version != 1 || n.Kind != "wrapped-algebraic-two-face-open-shell" || len(n.Faces) != 2 || len(n.Mate) != 2 {
		return nil, geom.ErrInvalid
	}
	for _, m := range n.Mate {
		if len(m) != 2 || m[0] != 0 || m[1] < 0 || m[1] >= 32 {
			return nil, geom.ErrInvalid
		}
	}
	w := n.Witness
	if len(w.Vertices) < 4 || len(w.Vertices) > 62 || len(w.Edges) < 5 || len(w.Edges) > 63 || len(w.FaceUses) != 2 || len(w.Boundary) != len(w.Vertices) {
		return nil, geom.ErrInvalid
	}
	for _, v := range w.Vertices {
		if len(v.Point) != 3 || len(v.Sources) < 1 || len(v.Sources) > 2 {
			return nil, geom.ErrInvalid
		}
		for _, text := range v.Point {
			if len(text) > 2*o.Face.Face.MaxCoefficientBits+8 {
				return nil, geom.ErrConvergence
			}
		}
		for _, source := range v.Sources {
			if len(source) != 2 || source[0] < 0 || source[0] > 1 || source[1] < 0 || source[1] >= 32 {
				return nil, geom.ErrInvalid
			}
		}
	}
	for _, edge := range w.Edges {
		if len(edge.Vertices) != 2 || len(edge.Range) != 2 || len(edge.Sources) < 1 || len(edge.Sources) > 2 {
			return nil, geom.ErrInvalid
		}
		for _, text := range edge.Range {
			if len(text) > 2*o.Face.Face.MaxCoefficientBits+8 {
				return nil, geom.ErrConvergence
			}
		}
		for _, source := range edge.Sources {
			if len(source) != 5 {
				return nil, geom.ErrInvalid
			}
		}
	}
	for _, uses := range w.FaceUses {
		if len(uses) < 3 || len(uses) > 32 {
			return nil, geom.ErrInvalid
		}
		for _, use := range uses {
			if len(use) != 2 {
				return nil, geom.ErrInvalid
			}
		}
	}
	for _, use := range w.Boundary {
		if len(use) != 7 {
			return nil, geom.ErrInvalid
		}
	}
	var faces [2]*WrappedAlgebraicQuadraticFace
	for i, raw := range n.Faces {
		if int64(len(raw)) > o.Face.MaxBytes {
			return nil, geom.ErrConvergence
		}
		face, e := ReadWrappedAlgebraicQuadraticFaceJSON(ctx, bytes.NewReader(raw), o.Face)
		if e != nil {
			return nil, e
		}
		faces[i] = face
	}
	s, e := NewWrappedAlgebraicOpenShell(ctx, faces[0], faces[1], [2]int{n.Mate[0][0], n.Mate[0][1]}, [2]int{n.Mate[1][0], n.Mate[1][1]}, n.ReverseB, o.Assembly)
	if e != nil {
		return nil, e
	}
	expected, e := wrappedShellNativeWitness(s)
	if e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(w, expected) {
		return nil, geom.ErrDomain
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return s, nil
}

// Bind exported native evaluators and pcurves to the private original sources
// before source-only encoding. Incidence indices are independently re-proved
// by decode; these checks prevent a stale source recipe hiding changed curves.
func wrappedShellSourceBinding(ctx context.Context, s *WrappedAlgebraicOpenShell) error {
	if len(s.faces) != 2 || len(s.edges) > 63 || len(s.edgeSources) != len(s.edges) {
		return geom.ErrInvalid
	}
	equalTrim := func(a, b PCurve) bool {
		x, ok := a.(ExactQuadraticUV)
		if !ok {
			return false
		}
		y, ok := b.(ExactQuadraticUV)
		if !ok {
			return false
		}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				if x.control[i][j] == nil || y.control[i][j] == nil || x.control[i][j].Cmp(y.control[i][j]) != 0 {
					return false
				}
			}
		}
		return true
	}
	for i, face := range s.faces {
		source := s.sources[i].face.native.Face
		if face.Surface != source.Surface || face.Reversed != source.Reversed || len(face.Holes) != 0 || len(face.Outer) != len(source.Outer) || len(face.Outer) > 32 {
			return geom.ErrInvalid
		}
		for j, use := range face.Outer {
			if e := ctx.Err(); e != nil {
				return e
			}
			original := source.Outer[j]
			if use.UVStart != original.UVStart || use.UVEnd != original.UVEnd || !equalTrim(use.Curve, original.Curve) {
				return geom.ErrDomain
			}
		}
	}
	for i, edge := range s.edges {
		if len(s.edgeSources[i]) < 1 {
			return geom.ErrInvalid
		}
		owner := s.edgeSources[i][0]
		if owner.Face < 0 || owner.Face > 1 || owner.Edge < 0 || owner.Edge >= len(s.sources[owner.Face].face.native.Edges) {
			return geom.ErrInvalid
		}
		original := s.sources[owner.Face].face.native.Edges[owner.Edge]
		a, ok := edge.Curve.(wrappedAlgebraicTrimCurve)
		if !ok {
			return geom.ErrInvalid
		}
		b, ok := original.Curve.(wrappedAlgebraicTrimCurve)
		if !ok {
			return geom.ErrInvalid
		}
		if edge.Range != original.Range || a.surface != b.surface || a.a != b.a || a.b != b.b || a.proof != b.proof || !equalTrim(a.curve, b.curve) {
			return geom.ErrDomain
		}
	}
	return ctx.Err()
}
