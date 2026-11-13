package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type nativeTubeGraphDifference struct {
	Version  int             `json:"version"`
	Kind     string          `json:"kind"`
	Base     json.RawMessage `json:"base"`
	Section  json.RawMessage `json:"section"`
	Minuend  int             `json:"minuend"`
	Exterior bool            `json:"exterior,omitempty"`
}

// WriteAlgebraicTubeGraphDifferenceJSON stores the common base and original
// section recipe. No selected-region list or proof result is persisted.
func WriteAlgebraicTubeGraphDifferenceJSON(w io.Writer, d AlgebraicTubeGraphDifference) error {
	if w == nil || d.verifiedBase == nil || d.regions.section.native == nil || d.verifiedMinuend < 0 || d.verifiedMinuend > 1 {
		return geom.ErrInvalid
	}
	base, e := parametric.EncodeSurface(d.verifiedBase)
	if e != nil {
		return e
	}
	var section bytes.Buffer
	if e := WriteTrimmedSplinePairSectionJSON(&section, d.regions.section); e != nil {
		return e
	}
	p, e := json.Marshal(nativeTubeGraphDifference{1, "trimmed-algebraic-tube-graph-difference", base, section.Bytes(), d.verifiedMinuend, d.exterior})
	if e != nil {
		return e
	}
	p = append(p, '\n')
	n, e := w.Write(p)
	if e == nil && n != len(p) {
		return io.ErrShortWrite
	}
	return e
}

func ReadAlgebraicTubeGraphDifferenceJSON(ctx context.Context, r io.Reader, l JSONLimits,
	section AlgebraicTrimmedPairSectionOptions, o AlgebraicTubeGraphBooleanOptions) (out AlgebraicTubeGraphDifference, err error) {
	if ctx == nil || r == nil || l.MaxBytes < 0 || l.MaxBytes > 1<<30 || l.MaxControlPoints < 0 || l.MaxEntities < 0 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 32 << 20
	}
	if l.MaxControlPoints == 0 {
		l.MaxControlPoints = 4096
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return out, e
	}
	if int64(len(b)) > l.MaxBytes {
		return out, geom.ErrConvergence
	}
	p := json.NewDecoder(bytes.NewReader(b))
	p.DisallowUnknownFields()
	var n nativeTubeGraphDifference
	if e := p.Decode(&n); e != nil {
		return out, e
	}
	if e := p.Decode(new(any)); e != io.EOF || n.Version != 1 || n.Kind != "trimmed-algebraic-tube-graph-difference" ||
		n.Minuend < 0 || n.Minuend > 1 {
		return out, geom.ErrInvalid
	}
	x, e := parametric.DecodeSurface(n.Base, l.MaxControlPoints)
	if e != nil {
		return out, e
	}
	base, ok := x.(*geom.BSplineSurface)
	if !ok {
		return out, geom.ErrInvalid
	}
	s, e := ReadTrimmedSplinePairSectionJSON(ctx, bytes.NewReader(n.Section), l, section)
	if e != nil {
		return out, e
	}
	out, e = s.subtractAlgebraicTubeGraphSolids(ctx, base, n.Minuend, o, n.Exterior)
	if e != nil {
		return out, e
	}
	if out.exterior != n.Exterior {
		return AlgebraicTubeGraphDifference{}, geom.ErrInvalid
	}
	return out, nil
}
