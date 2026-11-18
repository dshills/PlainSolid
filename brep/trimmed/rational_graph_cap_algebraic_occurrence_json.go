//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

type nativeAlgebraicGraphStockOccurrence struct {
	Version int             `json:"version"`
	Kind    string          `json:"kind"`
	Pose    [][]string      `json:"pose"`
	Source  json.RawMessage `json:"source"`
}

// WriteAlgebraicGraphStockOccurrenceJSON persists the exact proper rigid
// pose with the original clipped stock recipe. World triangulation and
// approximate display vertices are not part of the document.
func WriteAlgebraicGraphStockOccurrenceJSON(w io.Writer, o AlgebraicGraphStockOccurrence) error {
	if w == nil || o.pair.caps[0].recipe == nil {
		return geom.ErrInvalid
	}
	p, e := o.Pose()
	if e != nil {
		return e
	}
	var s bytes.Buffer
	if e := WriteAlgebraicGraphStockJSON(&s, o.pair); e != nil {
		return e
	}
	q := nativeAlgebraicGraphStockOccurrence{Version: 1, Kind: "trimmed-algebraic-graph-stock-occurrence", Source: s.Bytes()}
	for _, row := range p {
		var raw []string
		for _, x := range row {
			raw = append(raw, x.RatString())
		}
		q.Pose = append(q.Pose, raw)
	}
	b, e := json.Marshal(q)
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

// ReadAlgebraicGraphStockOccurrenceJSON authenticates the exact pose, then
// re-proves the original stock, both caps, closed indexed topology and
// source fiber query. Invalid or unsupported inputs publish nothing.
func ReadAlgebraicGraphStockOccurrenceJSON(ctx context.Context, r io.Reader, l JSONLimits,
	section RationalGraphShellSectionOptions, cap RationalGraphShellRegionOptions, roots geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockOccurrence, error) {
	var zero AlgebraicGraphStockOccurrence
	if ctx == nil || r == nil || l.MaxBytes < 0 || l.MaxBytes > 1<<30 || l.MaxEntities < 0 || l.MaxControlPoints < 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 32 << 20
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var q nativeAlgebraicGraphStockOccurrence
	if e := d.Decode(&q); e != nil {
		return zero, e
	}
	if e := d.Decode(new(any)); e != io.EOF || q.Version != 1 || q.Kind != "trimmed-algebraic-graph-stock-occurrence" ||
		len(q.Source) == 0 || len(q.Pose) != 3 {
		return zero, geom.ErrInvalid
	}
	var matrix [3][4]*big.Rat
	for i, row := range q.Pose {
		if len(row) != 4 {
			return zero, geom.ErrInvalid
		}
		for j, s := range row {
			if len(s) > 2500 {
				return zero, geom.ErrConvergence
			}
			r, ok := new(big.Rat).SetString(s)
			if !ok || r.RatString() != s {
				return zero, geom.ErrInvalid
			}
			matrix[i][j] = r
		}
	}
	p, e := placement.NewRationalRigid(matrix, 4096)
	if e != nil {
		return zero, e
	}
	c, _, e := ReadAlgebraicGraphStockJSON(ctx, bytes.NewReader(q.Source), l, section, cap, roots)
	if e != nil {
		return zero, e
	}
	return c.PlaceVerticalMaterialRigid(ctx, p, section, roots)
}
