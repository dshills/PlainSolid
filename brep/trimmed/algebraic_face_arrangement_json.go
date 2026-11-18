//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"plainsolid/geom"
)

type nativeAlgebraicFaceCut struct {
	Face int       `json:"face"`
	Line [3]string `json:"line"`
}
type nativeAlgebraicFaceArrangement struct {
	Version     int                      `json:"version"`
	Kind        string                   `json:"kind"`
	Subdivision json.RawMessage          `json:"subdivision"`
	Cuts        []nativeAlgebraicFaceCut `json:"cuts"`
}

// WriteAlgebraicFaceArrangementJSON stores source subdivision recipes and
// exact chart cut lines. Fragment incidence and curves are derived on read.
func WriteAlgebraicFaceArrangementJSON(w io.Writer, a AlgebraicFaceArrangement) error {
	if w == nil || len(a.curves) == 0 {
		return geom.ErrInvalid
	}
	var source bytes.Buffer
	if e := WriteAlgebraicSubdivisionJSON(&source, a.subdivision); e != nil {
		return e
	}
	f := nativeAlgebraicFaceArrangement{Version: 1, Kind: "trimmed-algebraic-face-arrangement", Subdivision: source.Bytes()}
	for _, c := range a.curves {
		x := nativeAlgebraicFaceCut{Face: c.Partition.Face}
		for i, v := range c.Partition.Line {
			x.Line[i] = v.RatString()
		}
		f.Cuts = append(f.Cuts, x)
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

// ReadAlgebraicFaceArrangementJSON independently rederives the boundary
// partition and complete model cut curves from bounded source recipes.
func ReadAlgebraicFaceArrangementJSON(ctx context.Context, r io.Reader, limits JSONLimits, o AlgebraicSeamOptions) (AlgebraicFaceArrangement, error) {
	var empty AlgebraicFaceArrangement
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
	var f nativeAlgebraicFaceArrangement
	if e = d.Decode(&f); e != nil {
		return empty, e
	}
	if e = d.Decode(new(any)); e != io.EOF || f.Version != 1 || f.Kind != "trimmed-algebraic-face-arrangement" || len(f.Cuts) == 0 || len(f.Cuts) > 32 {
		return empty, geom.ErrInvalid
	}
	s, e := ReadAlgebraicSubdivisionJSON(ctx, bytes.NewReader(f.Subdivision), limits, o)
	if e != nil {
		return empty, e
	}
	var cuts []AlgebraicFaceCut
	for _, x := range f.Cuts {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		c := AlgebraicFaceCut{Face: x.Face}
		for i, v := range x.Line {
			q, e := parseAlgebraicSeamRational(v)
			if e != nil {
				return empty, e
			}
			c.Line[i] = q
		}
		cuts = append(cuts, c)
	}
	return s.ArrangeFacesByAlgebraicUVLines(ctx, cuts, o)
}
