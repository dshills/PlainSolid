package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"plainsolid/geom"
)

type algebraicSubdivisionFile struct {
	Version int                   `json:"version"`
	Kind    string                `json:"kind"`
	Source  json.RawMessage       `json:"source"`
	Splits  []AlgebraicSeamRecipe `json:"splits"`
}

// WriteAlgebraicSubdivisionJSON stores the original native solid and exact
// subdivision recipes. All indices, root associations and display vertices
// are recomputed on read, not trusted as serialized proof flags.
func WriteAlgebraicSubdivisionJSON(w io.Writer, s AlgebraicSubdivision) error {
	if w == nil || len(s.recipes) == 0 {
		return geom.ErrInvalid
	}
	var source bytes.Buffer
	if e := WriteJSON(&source, s.source); e != nil {
		return e
	}
	file := algebraicSubdivisionFile{1, "trimmed-algebraic-subdivision", source.Bytes(), s.recipes}
	b, e := json.Marshal(file)
	if e != nil {
		return e
	}
	b = append(b, '\n')
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadAlgebraicSubdivisionJSON bounds both the envelope and native source.
// Exact plane constraints and the source's two owner charts rederive every
// split; a different valid root or stale source recipe is rejected.
func ReadAlgebraicSubdivisionJSON(ctx context.Context, r io.Reader, limits JSONLimits, o AlgebraicSeamOptions) (AlgebraicSubdivision, error) {
	var empty AlgebraicSubdivision
	if ctx == nil || r == nil || limits.MaxBytes < 0 || limits.MaxBytes > 1<<30 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if limits.MaxBytes == 0 {
		limits.MaxBytes = 32 << 20
	}
	b, e := io.ReadAll(io.LimitReader(r, limits.MaxBytes+1))
	if e != nil {
		return empty, e
	}
	if int64(len(b)) > limits.MaxBytes {
		return empty, geom.ErrConvergence
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var f algebraicSubdivisionFile
	if e = d.Decode(&f); e != nil {
		return empty, e
	}
	if e = d.Decode(new(any)); e != io.EOF || f.Version != 1 || f.Kind != "trimmed-algebraic-subdivision" ||
		len(f.Splits) == 0 || len(f.Splits) > 32 {
		return empty, geom.ErrInvalid
	}
	s, e := ReadJSONContext(ctx, bytes.NewReader(f.Source), limits)
	if e != nil {
		return empty, e
	}
	return s.SubdivideAlgebraicSeams(ctx, f.Splits, o)
}
