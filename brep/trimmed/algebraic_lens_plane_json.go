//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type nativeAlgebraicLensPlaneSplit struct {
	Version   int                `json:"version"`
	Kind      string             `json:"kind"`
	Supports  [3]json.RawMessage `json:"supports"`
	Ellipse   [6]string          `json:"ellipse"`
	Point     [2]string          `json:"point"`
	Plane     [3]string          `json:"plane"`
	Tolerance float64            `json:"tolerance"`
}

// WriteAlgebraicLensPlaneJSON stores original supports and exact constructive
// parameters. Neither represented cut vertices nor material flags are trusted.
func WriteAlgebraicLensPlaneJSON(w io.Writer, s AlgebraicLensPlaneSplit) error {
	if w == nil || s.base == nil || s.contact[0] == nil || len(s.source.curves) != 2 {
		return geom.ErrInvalid
	}
	source := s.source.subdivision.source
	f := nativeAlgebraicLensPlaneSplit{Version: 1, Kind: "trimmed-algebraic-lens-plane", Tolerance: source.tolerance}
	for i, x := range [3]geom.Surface{s.base, source.faces[0].Surface, source.faces[1].Surface} {
		b, e := parametric.EncodeSurface(x)
		if e != nil {
			return e
		}
		f.Supports[i] = b
	}
	for i, x := range s.contact {
		f.Ellipse[i] = x.RatString()
	}
	for i, x := range s.point {
		f.Point[i] = x.RatString()
	}
	for i, x := range s.source.curves[0].Partition.Line {
		f.Plane[i] = x.RatString()
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

// ReadAlgebraicLensPlaneJSON independently reproves the exact lens, both
// arrangement fragments and the shared oriented cap from bounded input.
func ReadAlgebraicLensPlaneJSON(ctx context.Context, r io.Reader, l JSONLimits, p geom.GraphPointContactOptions,
	o AlgebraicSeamOptions) (AlgebraicLensPlaneSplit, error) {
	var empty AlgebraicLensPlaneSplit
	if ctx == nil || r == nil || l.MaxBytes < 0 || l.MaxBytes > 1<<30 || l.MaxControlPoints < 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 32 << 20
	}
	if l.MaxControlPoints == 0 {
		l.MaxControlPoints = 4096
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return empty, e
	}
	if int64(len(b)) > l.MaxBytes {
		return empty, geom.ErrConvergence
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var f nativeAlgebraicLensPlaneSplit
	if e = d.Decode(&f); e != nil {
		return empty, e
	}
	if e = d.Decode(new(any)); e != io.EOF || f.Version != 1 || f.Kind != "trimmed-algebraic-lens-plane" {
		return empty, geom.ErrInvalid
	}
	var surfaces [3]*geom.BSplineSurface
	for i, b := range f.Supports {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		s, e := parametric.DecodeSurface(b, l.MaxControlPoints)
		if e != nil {
			return empty, e
		}
		var ok bool
		surfaces[i], ok = s.(*geom.BSplineSurface)
		if !ok {
			return empty, geom.ErrInvalid
		}
	}
	var ellipse geom.GraphQuadraticEllipseHint
	var point [2]*big.Rat
	var line [3]*big.Rat
	for _, pair := range []struct {
		raw []string
		dst []*big.Rat
	}{{f.Ellipse[:], ellipse.Coefficients[:]}, {f.Point[:], point[:]}, {f.Plane[:], line[:]}} {
		for i, x := range pair.raw {
			q, e := parseAlgebraicSeamRational(x)
			if e != nil {
				return empty, e
			}
			pair.dst[i] = q
		}
	}
	return SplitRationalEllipseLensByVerticalPlane(ctx, surfaces[0], surfaces[1], surfaces[2], ellipse, point, line, f.Tolerance, p, o)
}
