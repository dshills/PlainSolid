package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
)

type nativeAlgebraicSew struct {
	Version   int                   `json:"version"`
	Kind      string                `json:"kind"`
	Inputs    [2]json.RawMessage    `json:"inputs"`
	Selection [2][]AlgebraicSewFace `json:"selection"`
	Pairs     []AlgebraicSewPair    `json:"pairs"`
}

// WriteAlgebraicSewJSON stores the original arrangements and explicit sewing
// recipe. It does not serialize derived geometry, incidence, or proof flags.
func WriteAlgebraicSewJSON(w io.Writer, s AlgebraicSewResult) error {
	if w == nil || len(s.recipeSelection[0])+len(s.recipeSelection[1]) == 0 {
		return geom.ErrInvalid
	}
	f := nativeAlgebraicSew{Version: 1, Kind: "trimmed-algebraic-sew", Selection: s.recipeSelection, Pairs: s.recipePairs}
	for i, a := range s.inputs {
		if len(f.Selection[i]) == 0 {
			continue
		}
		var b bytes.Buffer
		if e := WriteAlgebraicFaceArrangementJSON(&b, a); e != nil {
			return e
		}
		f.Inputs[i] = b.Bytes()
	}
	b, e := json.Marshal(f)
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

// ReadAlgebraicSewJSON reconstructs both original inputs, reproves each
// entire paired curve and reconstructs the result's closed vertex links.
func ReadAlgebraicSewJSON(ctx context.Context, r io.Reader, limits JSONLimits, o AlgebraicArrangementContactOptions) (AlgebraicSewResult, error) {
	var empty AlgebraicSewResult
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
	var f nativeAlgebraicSew
	if e = d.Decode(&f); e != nil {
		return empty, e
	}
	if e = d.Decode(new(any)); e != io.EOF || f.Version != 1 || f.Kind != "trimmed-algebraic-sew" || len(f.Pairs) > 256 ||
		len(f.Selection[0])+len(f.Selection[1]) < 1 || len(f.Selection[0])+len(f.Selection[1]) > 256 {
		return empty, geom.ErrInvalid
	}
	var in [2]AlgebraicFaceArrangement
	for i, x := range f.Inputs {
		if len(f.Selection[i]) == 0 {
			if len(x) != 0 && !bytes.Equal(x, []byte("null")) {
				return empty, geom.ErrInvalid
			}
			continue
		}
		in[i], e = ReadAlgebraicFaceArrangementJSON(ctx, bytes.NewReader(x), limits, o.Seams)
		if e != nil {
			return empty, e
		}
	}
	return SewAlgebraicArrangementFragments(ctx, in[0], in[1], f.Selection, f.Pairs, o)
}
