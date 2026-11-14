package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"plainsolid/geom"
)

// WrappedSeparatedShellJSONOptions bounds the distinct separated-pair recipe.
// MaxBytes defaults16MiB, maximum32MiB. MaxWork defaults128 million. Nested pair
// stages conservatively reserve their configured limits: reading reserves two
// face allowances plus one assembly allowance; writing additionally reserves
// two face writer allowances. All remaining separator/native work is cumulative.
// No older pair or chain schema gains an asserted global-embedding flag.
type WrappedSeparatedShellJSONOptions struct {
	MaxBytes   int64
	MaxWork    int
	Pair       WrappedAlgebraicOpenShellJSONOptions
	Separation geom.SurfaceAffineCoordinateOptions
}
type wrappedSeparatorRecipe struct {
	Axis          int      `json:"axis"`
	Level         string   `json:"level"`
	ParameterAxes []int    `json:"parameterAxes"`
	Scale         []string `json:"scale"`
	Offset        []string `json:"offset"`
}
type wrappedSeparatedShellRecipe struct {
	Version   int                    `json:"version"`
	Kind      string                 `json:"kind"`
	Pair      json.RawMessage        `json:"pair"`
	Separator wrappedSeparatorRecipe `json:"separator"`
}

func wrappedSeparatedJSONOptions(o WrappedSeparatedShellJSONOptions) (WrappedSeparatedShellJSONOptions, error) {
	if o.MaxBytes == 0 {
		o.MaxBytes = 16 << 20
	}
	if o.MaxWork == 0 {
		o.MaxWork = 128000000
	}
	if o.MaxBytes < 1 || o.MaxBytes > 32<<20 || o.MaxWork < 1 || o.MaxWork > 1000000000 {
		return o, geom.ErrInvalid
	}
	var e error
	o.Pair, e = wrappedShellJSONOptions(o.Pair)
	if e != nil {
		return o, e
	}
	if o.Separation.MaxOperations == 0 {
		o.Separation.MaxOperations = 16000000
	}
	if o.Separation.MaxCoefficientBits == 0 {
		o.Separation.MaxCoefficientBits = 16384
	}
	if o.Separation.MaxOperations < 1 || o.Separation.MaxCoefficientBits < 64 || o.Separation.MaxCoefficientBits > 65536 {
		return o, geom.ErrInvalid
	}
	return o, nil
}
func reserveWrappedPairWork(o WrappedAlgebraicOpenShellJSONOptions, write bool, available int) (int, error) {
	count := 2
	if write {
		count = 4
	}
	if o.Assembly.MaxWork >= available {
		return 0, geom.ErrConvergence
	}
	left := available - o.Assembly.MaxWork
	if o.Face.Face.MaxOperations >= (left+count-1)/count {
		return 0, geom.ErrConvergence
	}
	return available - o.Assembly.MaxWork - count*o.Face.Face.MaxOperations, nil
}

// WriteWrappedAlgebraicSeparatedShellJSON serializes source geometry and an
// exact separator proposal. Both native and global proofs are rebuilt before
// the first output byte, including original mate/endpoint ownership witnesses.
func WriteWrappedAlgebraicSeparatedShellJSON(ctx context.Context, dst io.Writer, s *WrappedAlgebraicSeparatedShell, o WrappedSeparatedShellJSONOptions) error {
	if ctx == nil || dst == nil || s == nil || s.proof == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	o, e := wrappedSeparatedJSONOptions(o)
	if e != nil {
		return e
	}
	remaining, e := reserveWrappedPairWork(o.Pair, true, o.MaxWork)
	if e != nil {
		return e
	}
	var pair bytes.Buffer
	if e := WriteWrappedAlgebraicOpenShellJSON(ctx, &pair, s.PairShell(), o.Pair); e != nil {
		return e
	}
	q := s.proof.Separator()
	r := wrappedSeparatorRecipe{Axis: q.Axis, Level: q.Level.RatString(), ParameterAxes: []int{q.ParameterAxes[0], q.ParameterAxes[1]}}
	for i := 0; i < 2; i++ {
		r.Scale = append(r.Scale, q.Scale[i].RatString())
		r.Offset = append(r.Offset, q.Offset[i].RatString())
	}
	n := wrappedSeparatedShellRecipe{Version: 1, Kind: "wrapped-coordinate-separated-open-shell", Pair: append(json.RawMessage(nil), pair.Bytes()...), Separator: r}
	raw, e := json.Marshal(n)
	if e != nil {
		return e
	}
	if int64(len(raw)) > o.MaxBytes {
		return geom.ErrConvergence
	}
	o.MaxWork = remaining
	if _, e := decodeWrappedSeparatedShell(ctx, n, o); e != nil {
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

// ReadWrappedAlgebraicSeparatedShellJSON reconstructs the old strict native
// pair, then independently proves the supplied separator and all native uses.
func ReadWrappedAlgebraicSeparatedShellJSON(ctx context.Context, src io.Reader, o WrappedSeparatedShellJSONOptions) (*WrappedAlgebraicSeparatedShell, error) {
	if ctx == nil || src == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := wrappedSeparatedJSONOptions(o)
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
	var n wrappedSeparatedShellRecipe
	if e := d.Decode(&n); e != nil {
		return nil, e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return nil, geom.ErrInvalid
	}
	return decodeWrappedSeparatedShell(ctx, n, o)
}
func decodeWrappedSeparatedShell(ctx context.Context, n wrappedSeparatedShellRecipe, o WrappedSeparatedShellJSONOptions) (*WrappedAlgebraicSeparatedShell, error) {
	if n.Version != 1 || n.Kind != "wrapped-coordinate-separated-open-shell" || n.Separator.Axis < 0 || n.Separator.Axis > 2 || len(n.Separator.ParameterAxes) != 2 || len(n.Separator.Scale) != 2 || len(n.Separator.Offset) != 2 {
		return nil, geom.ErrInvalid
	}
	remaining, e := reserveWrappedPairWork(o.Pair, false, o.MaxWork)
	if e != nil {
		return nil, e
	}
	parse := func(text string) (*big.Rat, error) {
		remaining--
		if remaining < 1 || len(text) > 2*o.Separation.MaxCoefficientBits+8 {
			return nil, geom.ErrConvergence
		}
		q, ok := new(big.Rat).SetString(text)
		if !ok || q.RatString() != text {
			return nil, geom.ErrInvalid
		}
		if q.Num().BitLen() > o.Separation.MaxCoefficientBits || q.Denom().BitLen() > o.Separation.MaxCoefficientBits {
			return nil, geom.ErrConvergence
		}
		return q, nil
	}
	q := WrappedCoordinateSeparator{Axis: n.Separator.Axis}
	q.Level, e = parse(n.Separator.Level)
	if e != nil {
		return nil, e
	}
	for i := 0; i < 2; i++ {
		q.ParameterAxes[i] = n.Separator.ParameterAxes[i]
		if q.ParameterAxes[i] < 0 || q.ParameterAxes[i] > 1 {
			return nil, geom.ErrInvalid
		}
		q.Scale[i], e = parse(n.Separator.Scale[i])
		if e != nil {
			return nil, e
		}
		q.Offset[i], e = parse(n.Separator.Offset[i])
		if e != nil {
			return nil, e
		}
	}
	pair, e := ReadWrappedAlgebraicOpenShellJSON(ctx, bytes.NewReader(n.Pair), o.Pair)
	if e != nil {
		return nil, e
	}
	opts := o.Separation
	opts.MaxOperations = min(opts.MaxOperations, remaining)
	return NewWrappedAlgebraicSeparatedShell(ctx, pair, q, opts)
}
