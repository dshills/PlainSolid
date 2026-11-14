package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// The additional witnesses are checked against freshly reconstructed native
// topology, never accepted as geometry or asserted proof. Owner tuples are
// (original loop, original edge, native start vertex, native end vertex).
type wrappedAlgebraicFaceRecipe struct {
	wrappedQuadraticFaceRecipe
	ModelVertices [][]string `json:"modelVertices"`
	SourceOwners  [][]int    `json:"sourceOwners"`
}

// WriteWrappedAlgebraicQuadraticFaceJSON writes a distinct source-only schema
// with redundant exact original vertex/owner witnesses. It re-proves the recipe
// and actual input model curves before any bytes are emitted. Interior seam
// roots are recomputed, not stored as rounded cuts or trusted certificates.
// Limits are those of WrappedQuadraticFaceJSONOptions, with MaxPieces<=8192.
func WriteWrappedAlgebraicQuadraticFaceJSON(ctx context.Context, dst io.Writer, r *WrappedAlgebraicQuadraticFace, o WrappedQuadraticFaceJSONOptions) error {
	if ctx == nil || dst == nil || r == nil || r.face == nil || len(r.face.loops) == 0 {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	o, e := wrappedQuadraticJSONOptions(o)
	if e != nil {
		return e
	}
	if o.Face.MaxPieces > 8192 {
		return geom.ErrInvalid
	}
	face := r.face
	w, ok := face.native.Face.Surface.(*geom.WrappedSurfacePatch)
	if !ok || w.FundamentalPatch() == nil {
		return geom.ErrInvalid
	}
	cu, cv := w.FundamentalPatch().ControlShape()
	if cu < 1 || cv < 1 || cu > o.MaxControlPoints || cv > o.MaxControlPoints || cu > o.MaxControlPoints/cv {
		return geom.ErrConvergence
	}
	source, e := w.ExportRecipe()
	if e != nil {
		return e
	}
	n := wrappedAlgebraicFaceRecipe{wrappedQuadraticFaceRecipe: wrappedQuadraticFaceRecipe{Version: 1, Kind: "wrapped-algebraic-quadratic-open-face", Surface: source.Surface.Data(), WrapU: source.Options.WrapU, WrapV: source.Options.WrapV, ContinuityOrder: source.Options.ContinuityOrder, ProjectionAxes: []int{face.projectionAxes[0], face.projectionAxes[1]}, Reversed: face.native.Face.Reversed, Loops: make([][][][]string, len(face.loops))}}
	if len(face.native.Face.Holes) != len(face.loops)-1 {
		return geom.ErrInvalid
	}
	preflightWork := 0
	index := 0
	for li, loop := range face.loops {
		uses := face.native.Face.Outer
		if li > 0 {
			uses = face.native.Face.Holes[li-1]
		}
		if len(uses) != len(loop) {
			return geom.ErrInvalid
		}
		for ei, q := range loop {
			use := uses[ei]
			actual, ok := use.Curve.(ExactQuadraticUV)
			if !ok || use.Edge != index || use.Reversed {
				return geom.ErrInvalid
			}
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					preflightWork++
					if preflightWork >= o.Face.MaxOperations {
						return geom.ErrConvergence
					}
					if actual.control[i][j] == nil || q.control[i][j] == nil || actual.control[i][j].Cmp(q.control[i][j]) != 0 {
						return geom.ErrInvalid
					}
				}
			}
			_, endpoints := q.Endpoints()
			if use.UVStart != endpoints[0] || use.UVEnd != endpoints[1] {
				return geom.ErrInvalid
			}
			h := make([][]string, 3)
			for i, row := range q.control {
				h[i] = make([]string, 3)
				for j, value := range row {
					if value == nil {
						return geom.ErrInvalid
					}
					if value.Num().BitLen() > o.Face.MaxCoefficientBits || value.Denom().BitLen() > o.Face.MaxCoefficientBits {
						return geom.ErrConvergence
					}
					h[i][j] = value.RatString()
				}
			}
			n.Loops[li] = append(n.Loops[li], h)
			if index >= len(face.native.Edges) {
				return geom.ErrInvalid
			}
			edge := face.native.Edges[index]
			n.SourceOwners = append(n.SourceOwners, []int{li, ei, edge.Start, edge.End})
			index++
		}
	}
	if index != len(face.native.Edges) || index != len(face.native.Vertices) {
		return geom.ErrInvalid
	}
	for _, v := range face.native.Vertices {
		if !v.IsFinite() {
			return geom.ErrInvalid
		}
		n.ModelVertices = append(n.ModelVertices, []string{ratInset(v.X).RatString(), ratInset(v.Y).RatString(), ratInset(v.Z).RatString()})
	}
	raw, e := json.Marshal(n)
	if e != nil {
		return e
	}
	if int64(len(raw)) > o.MaxBytes {
		return geom.ErrConvergence
	}
	child := o
	child.Face.MaxOperations -= preflightWork
	rebuilt, e := decodeWrappedAlgebraicFaceRecipe(ctx, n, child)
	if e != nil {
		return e
	}
	used := preflightWork + rebuilt.ExactOperations()
	index = 0
	for li, loop := range face.loops {
		for ei := range loop {
			if used >= o.Face.MaxOperations {
				return geom.ErrConvergence
			}
			uses := rebuilt.face.native.Face.Outer
			if li > 0 {
				uses = rebuilt.face.native.Face.Holes[li-1]
			}
			same, work, e := compareWrappedAlgebraicQuadraticUse(ctx, face.native.Edges[index], rebuilt.face.native.Face.Surface, uses[ei], edgecheck.Options{MaxWork: o.Face.MaxOperations - used, MaxBits: o.Face.MaxCoefficientBits, MaxIntervals: o.Face.MaxPieces})
			used += work
			if e != nil {
				return e
			}
			if !same {
				return geom.ErrConvergence
			}
			index++
		}
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

// ReadWrappedAlgebraicQuadraticFaceJSON validates bounded canonical rational
// data, original owner/vertex witnesses, support seam continuity, exact loop
// embedding and whole native edge association. Unknown/duplicate fields and
// legacy kinds reject. Only a completely re-proved open face is published.
func ReadWrappedAlgebraicQuadraticFaceJSON(ctx context.Context, src io.Reader, o WrappedQuadraticFaceJSONOptions) (*WrappedAlgebraicQuadraticFace, error) {
	if ctx == nil || src == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := wrappedQuadraticJSONOptions(o)
	if e != nil {
		return nil, e
	}
	if o.Face.MaxPieces > 8192 {
		return nil, geom.ErrInvalid
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
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var n wrappedAlgebraicFaceRecipe
	if e := decoder.Decode(&n); e != nil {
		return nil, e
	}
	var trailing any
	if e := decoder.Decode(&trailing); e != io.EOF {
		return nil, geom.ErrInvalid
	}
	return decodeWrappedAlgebraicFaceRecipe(ctx, n, o)
}
func decodeWrappedAlgebraicFaceRecipe(ctx context.Context, n wrappedAlgebraicFaceRecipe, o WrappedQuadraticFaceJSONOptions) (*WrappedAlgebraicQuadraticFace, error) {
	if len(n.Loops) < 1 || len(n.Loops) > 10 || len(n.ModelVertices) > o.Face.MaxEdges || len(n.SourceOwners) != len(n.ModelVertices) {
		return nil, geom.ErrInvalid
	}
	total := 0
	for li, loop := range n.Loops {
		if len(loop) < 3 || len(loop) > o.Face.MaxEdges-total {
			return nil, geom.ErrInvalid
		}
		base := total
		for ei := range loop {
			if total >= len(n.SourceOwners) {
				return nil, geom.ErrInvalid
			}
			owner := n.SourceOwners[total]
			if len(owner) != 4 || owner[0] != li || owner[1] != ei || owner[2] != total || owner[3] != base+(ei+1)%len(loop) {
				return nil, geom.ErrInvalid
			}
			total++
		}
	}
	if total != len(n.ModelVertices) {
		return nil, geom.ErrInvalid
	}
	witnesses := make([][3]*big.Rat, total)
	used := 0
	for i, row := range n.ModelVertices {
		if len(row) != 3 {
			return nil, geom.ErrInvalid
		}
		for axis, text := range row {
			used++
			if used >= o.Face.MaxOperations || len(text) > 2*o.Face.MaxCoefficientBits+8 {
				return nil, geom.ErrConvergence
			}
			slash := false
			for j, ch := range text {
				if ch >= '0' && ch <= '9' {
					continue
				}
				if ch == '-' && j == 0 {
					continue
				}
				if ch == '/' && !slash {
					slash = true
					continue
				}
				return nil, geom.ErrInvalid
			}
			q, ok := new(big.Rat).SetString(text)
			if !ok || q.RatString() != text {
				return nil, geom.ErrInvalid
			}
			if q.Num().BitLen() > o.Face.MaxCoefficientBits || q.Denom().BitLen() > o.Face.MaxCoefficientBits {
				return nil, geom.ErrConvergence
			}
			witnesses[i][axis] = q
		}
	}
	child := o
	child.Face.MaxOperations -= used
	face, e := decodeWrappedQuadraticFaceRecipeMode(ctx, n.wrappedQuadraticFaceRecipe, child, true)
	if e != nil {
		return nil, e
	}
	for i, v := range face.native.Vertices {
		for axis, q := range [3]*big.Rat{ratInset(v.X), ratInset(v.Y), ratInset(v.Z)} {
			if q.Cmp(witnesses[i][axis]) != 0 {
				return nil, geom.ErrDomain
			}
		}
	}
	face.operations += used
	return &WrappedAlgebraicQuadraticFace{face: face}, nil
}
