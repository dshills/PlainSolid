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
	branches []RationalGraphShellBranch
}

type nativeAlgebraicGraphCapClip struct {
	Version   int             `json:"version"`
	Kind      string          `json:"kind"`
	Stock     json.RawMessage `json:"stock"`
	Section   [][]*float64    `json:"section"`
	Cut       [][]*float64    `json:"cut"`
	Positive  *bool           `json:"keepPositive"`
	Compose   bool            `json:"compose"`
	Separated bool            `json:"separated,omitempty"`
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
	out.recipe = &algebraicGraphCapRecipe{s, section, cut, keepPositive, o.ComposeSectionCurves, q.Branches}
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
	return c.applyClipLocation(point, loc), nil
}

// locateRationalBudget applies the query ledger to the existing certified
// topology. The caller already proved that the point is on the cap plane.
func (c AlgebraicGraphCapClip) locateRationalBudget(p [3]*big.Rat, b *capClipBudget) (trimvalidate.Location, error) {
	for _, v := range p {
		if v.Num().BitLen() > b.bits || v.Denom().BitLen() > b.bits {
			return trimvalidate.Unknown, geom.ErrConvergence
		}
	}
	axes := orientedRationalGraphProjection(c.regions.plane)
	where, r, e := c.regions.certificate.LocateBounded(b.ctx, [2]*big.Rat{p[axes[0]], p[axes[1]]}, b.work, b.subdiv, b.bits)
	if e != nil {
		return trimvalidate.Unknown, e
	}
	if e = b.charge(r.Work, r.Subdivisions); e != nil {
		return trimvalidate.Unknown, e
	}
	if r.Status != trimvalidate.Certified {
		return trimvalidate.Unknown, geom.ErrConvergence
	}
	return c.applyClipLocation(p, where), nil
}

func (c AlgebraicGraphCapClip) applyClipLocation(point [3]*big.Rat, loc trimvalidate.Location) trimvalidate.Location {
	if loc == trimvalidate.Outside {
		return loc
	}
	sign := new(big.Rat).Sub(c.cut.dot(point), c.cut.offset).Sign() * c.keepSign
	switch {
	case sign < 0:
		return trimvalidate.Outside
	case sign == 0:
		return trimvalidate.Boundary
	default:
		return loc
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
	f := nativeAlgebraicGraphCapClip{Version: 1, Kind: "trimmed-algebraic-graph-cap",
		Stock: stock.Bytes(), Section: graphCapEncodePlane(recipe.section), Cut: graphCapEncodePlane(recipe.cut),
		Positive: &recipe.positive, Compose: recipe.compose, Separated: c.separated}
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
	section, e := graphCapDecodePlane(f.Section)
	if e != nil {
		return zero, e
	}
	cut, e := graphCapDecodePlane(f.Cut)
	if e != nil {
		return zero, e
	}
	stock, e := ReadJSONContext(ctx, bytes.NewReader(f.Stock), l)
	if e != nil {
		return zero, e
	}
	o.ComposeSectionCurves = f.Compose
	if f.Separated {
		if *f.Positive {
			return zero, geom.ErrInvalid
		}
		return stock.separatedAlgebraicCap(ctx, section, cut, o, clip)
	}
	return stock.ClipAlgebraicGraphCap(ctx, section, cut, *f.Positive, o, clip)
}

func graphCapEncodePlane(q [3]geom.Vec3) [][]*float64 {
	out := make([][]*float64, 3)
	for i, p := range q {
		out[i] = []*float64{&p.X, &p.Y, &p.Z}
	}
	return out
}

func graphCapDecodePlane(raw [][]*float64) ([3]geom.Vec3, error) {
	var dst [3]geom.Vec3
	if len(raw) != len(dst) {
		return dst, geom.ErrInvalid
	}
	for i, p := range raw {
		if len(p) != 3 || p[0] == nil || p[1] == nil || p[2] == nil {
			return [3]geom.Vec3{}, geom.ErrInvalid
		}
		dst[i] = geom.Vec3{X: *p[0], Y: *p[1], Z: *p[2]}
	}
	return dst, nil
}
