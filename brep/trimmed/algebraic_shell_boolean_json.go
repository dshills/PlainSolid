package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type nativeAlgebraicShellBoolean struct {
	Version   int                  `json:"version"`
	Kind      string               `json:"kind"`
	Operation parametric.BooleanOp `json:"operation"`
	Section   json.RawMessage      `json:"section"`
}

// WriteAlgebraicShellBooleanJSON stores only the original section sources,
// verified tube proposals and operation. No mutable output topology is trusted.
func WriteAlgebraicShellBooleanJSON(w io.Writer, s AlgebraicShellBoolean) error {
	if w == nil || s.source == nil {
		return geom.ErrInvalid
	}
	var b bytes.Buffer
	if e := WriteAlgebraicShellSectionJSON(&b, AlgebraicShellSectionResult{source: s.source}); e != nil {
		return e
	}
	p, e := json.Marshal(nativeAlgebraicShellBoolean{1, "trimmed-algebraic-shell-boolean", s.op, b.Bytes()})
	if e != nil {
		return e
	}
	p = append(p, '\n')
	n, e := w.Write(p)
	if e == nil && n != len(p) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadAlgebraicShellBooleanJSON proves the entire original shell section,
// reconstructs imprints, classifies material, and rechecks sewn vertex links.
func ReadAlgebraicShellBooleanJSON(ctx context.Context, r io.Reader, l JSONLimits,
	section AlgebraicShellSectionOptions, o AlgebraicShellBooleanOptions) (AlgebraicShellBoolean, error) {
	var out AlgebraicShellBoolean
	if ctx == nil || r == nil || l.MaxBytes < 0 || l.MaxBytes > 1<<30 || l.MaxControlPoints < 0 || l.MaxEntities < 0 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 32 << 20
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return out, e
	}
	if int64(len(b)) > l.MaxBytes {
		return out, geom.ErrConvergence
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var f nativeAlgebraicShellBoolean
	if e = d.Decode(&f); e != nil {
		return out, e
	}
	if e = d.Decode(new(any)); e != io.EOF || f.Version != 1 || f.Kind != "trimmed-algebraic-shell-boolean" ||
		len(f.Section) == 0 || f.Operation > parametric.Difference {
		return out, geom.ErrInvalid
	}
	s, e := ReadAlgebraicShellSectionJSON(ctx, bytes.NewReader(f.Section), l, section)
	if e != nil {
		return out, e
	}
	return s.CombineAlgebraic(ctx, f.Operation, o)
}
