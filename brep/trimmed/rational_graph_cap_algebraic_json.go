package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

type algebraicGraphCapRecipe struct {
	stock    Solid
	section  [3]geom.Vec3
	cut      [3]geom.Vec3
	positive bool
	compose  bool
}

type nativeAlgebraicGraphCapClip struct {
	Version  int             `json:"version"`
	Kind     string          `json:"kind"`
	Stock    json.RawMessage `json:"stock"`
	Section  [][]*float64    `json:"section"`
	Cut      [][]*float64    `json:"cut"`
	Positive *bool           `json:"keepPositive"`
	Compose  bool            `json:"compose"`
}

// ClipAlgebraicGraphCap sections an original native graph shell and clips the
// resulting planar material against a second plane. The result can be saved
// with WriteAlgebraicGraphCapJSON; it retains the source shell and the two
// exact planes induced by the given binary64 points.
func (s Solid) ClipAlgebraicGraphCap(ctx context.Context, section, cut [3]geom.Vec3, keepPositive bool, o RationalGraphShellSectionOptions, clip RationalGraphShellRegionOptions) (AlgebraicGraphCapClip, error) {
	var zero AlgebraicGraphCapClip
	if o.Regions == nil {
		o.Regions = &RationalGraphShellRegionOptions{}
	}
	q, e := s.SectionRationalGraphSplineShell(ctx, section, o)
	if e != nil {
		return zero, e
	}
	out, e := q.Regions.ClipAlgebraicHalfspace(ctx, cut, keepPositive, clip)
	if e != nil {
		return zero, e
	}
	out.recipe = &algebraicGraphCapRecipe{s, section, cut, keepPositive, o.ComposeSectionCurves}
	return out, nil
}

// LocateRational classifies an exact point of the clipped plane from the
// original certified ring forest and the exact clipping inequality. Both the
// original trim and a generated seam can be on the result boundary. A
// nonincident point returns ErrDomain. Public diagnostic edits are ignored.
func (c AlgebraicGraphCapClip) LocateRational(ctx context.Context, point [3]*big.Rat) (trimvalidate.Location, error) {
	if c.regions == nil {
		return trimvalidate.Unknown, geom.ErrInvalid
	}
	loc, report, e := c.regions.LocateRational(ctx, point)
	if e != nil {
		return trimvalidate.Unknown, e
	}
	if report.Status != trimvalidate.Certified {
		return trimvalidate.Unknown, geom.ErrConvergence
	}
	if loc == trimvalidate.Outside {
		return loc, nil
	}
	sign := new(big.Rat).Sub(c.cut.dot(point), c.cut.offset).Sign() * c.keepSign
	switch {
	case sign < 0:
		return trimvalidate.Outside, nil
	case sign == 0:
		return trimvalidate.Boundary, nil
	default:
		return loc, nil
	}
}

// WriteAlgebraicGraphCapJSON stores the original stock, both represented
// planes, and the selected side. Wires, brackets and their display values are
// never accepted as a source of geometry. Only a stock-backed clip made by
// ClipAlgebraicGraphCap has this persistence recipe.
func WriteAlgebraicGraphCapJSON(w io.Writer, c AlgebraicGraphCapClip) error {
	if w == nil || c.recipe == nil {
		return geom.ErrInvalid
	}
	recipe := c.recipe
	var stock bytes.Buffer
	if e := WriteJSON(&stock, recipe.stock); e != nil {
		return e
	}
	points := func(q [3]geom.Vec3) [][]*float64 {
		out := make([][]*float64, 3)
		for i, p := range q {
			out[i] = []*float64{&p.X, &p.Y, &p.Z}
		}
		return out
	}
	f := nativeAlgebraicGraphCapClip{Version: 1, Kind: "trimmed-algebraic-graph-cap",
		Stock: stock.Bytes(), Section: points(recipe.section), Cut: points(recipe.cut),
		Positive: &recipe.positive, Compose: recipe.compose}
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

// ReadAlgebraicGraphCapJSON revalidates the original native stock, then
// rediscovers its planar material, exact algebraic contacts and closed
// clipped boundaries. Neither a topology report nor a root bracket from disk
// is trusted; both are derived by the same bounded exact proofs.
func ReadAlgebraicGraphCapJSON(ctx context.Context, r io.Reader, l JSONLimits, o RationalGraphShellSectionOptions, clip RationalGraphShellRegionOptions) (AlgebraicGraphCapClip, error) {
	var zero AlgebraicGraphCapClip
	if ctx == nil || r == nil || l.MaxBytes < 0 || l.MaxBytes > 1<<30 || l.MaxEntities < 0 || l.MaxControlPoints < 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 32 << 20
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(b)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var f nativeAlgebraicGraphCapClip
	if e = d.Decode(&f); e != nil {
		return zero, e
	}
	if e = d.Decode(new(any)); e != io.EOF || f.Version != 1 || f.Kind != "trimmed-algebraic-graph-cap" || f.Positive == nil {
		return zero, geom.ErrInvalid
	}
	var section, cut [3]geom.Vec3
	for _, q := range []struct {
		raw [][]*float64
		dst *[3]geom.Vec3
	}{{f.Section, &section}, {f.Cut, &cut}} {
		if len(q.raw) != len(*q.dst) {
			return zero, geom.ErrInvalid
		}
		for i, p := range q.raw {
			if len(p) != 3 || p[0] == nil || p[1] == nil || p[2] == nil {
				return zero, geom.ErrInvalid
			}
			q.dst[i] = geom.Vec3{X: *p[0], Y: *p[1], Z: *p[2]}
		}
	}
	stock, e := ReadJSONContext(ctx, bytes.NewReader(f.Stock), l)
	if e != nil {
		return zero, e
	}
	o.ComposeSectionCurves = f.Compose
	return stock.ClipAlgebraicGraphCap(ctx, section, cut, *f.Positive, o, clip)
}
