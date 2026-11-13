package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// PerpendicularCylinderSplitCutResult keeps two separately closed bodies.
// The cutters' minimum lateral reach provides an exact squared lower bound
// on their separating axial gap.
type PerpendicularCylinderSplitCutResult struct {
	Solids                                         [2]Solid
	StockWallFaces, CutterWallFaces, StockCapFaces [2][]int
	MinimumGapSquared                              *big.Rat
}

// PerpendicularCylinderSplitCut cuts X-axis cylinder stock with a larger
// Z-axis cylinder. Their axes intersect at center. Both stock ends must
// lie beyond the cutter, and the cutter must extend across the stock.
func PerpendicularCylinderSplitCut(ctx context.Context, center geom.Vec3, stockRadius, cutterRadius, stockHalfLength, cutterHalfLength, tolerance float64) (PerpendicularCylinderSplitCutResult, error) {
	var empty PerpendicularCylinderSplitCutResult
	if ctx == nil || !center.IsFinite() || !finite(stockRadius) || !finite(cutterRadius) || !finite(stockHalfLength) ||
		!finite(cutterHalfLength) || !finite(tolerance) || stockRadius <= 0 || cutterRadius <= stockRadius || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	pad := mul(big.NewRat(8, 1), q(tolerance))
	if q(stockRadius).Cmp(pad) <= 0 || sub(q(cutterRadius), q(stockRadius)).Cmp(pad) <= 0 ||
		sub(q(stockHalfLength), q(cutterRadius)).Cmp(pad) <= 0 || sub(q(cutterHalfLength), q(stockRadius)).Cmp(pad) <= 0 {
		return empty, geom.ErrConvergence
	}
	var out PerpendicularCylinderSplitCutResult
	for i, side := range []int{-1, 1} {
		var faces []Face
		u, v := Interval{-math.Pi / 2, math.Pi / 2}, Interval{0, 1}
		for _, half := range []int{-1, 1} {
			for cap := range 2 {
				k := splitCylinderCutChart{center, stockRadius, cutterRadius, stockHalfLength, side, half, cap == 1}
				reverse := side*half > 0
				if k.cap {
					reverse = !reverse
					out.StockCapFaces[i] = append(out.StockCapFaces[i], len(faces))
				} else {
					out.StockWallFaces[i] = append(out.StockWallFaces[i], len(faces))
				}
				faces = append(faces, Face{Surface: k, U: u, V: v, Reversed: reverse})
			}
		}
		c := unequalCylinderCommonChart{center, stockRadius, cutterRadius, 1, side, true}
		out.CutterWallFaces[i] = []int{len(faces)}
		faces = append(faces, Face{Surface: c, U: u, V: Interval{-1, 1}, Reversed: side > 0})
		var e error
		out.Solids[i], e = joinPrimitivePatches(faces, tolerance)
		if e != nil {
			return empty, e
		}
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	out.MinimumGapSquared = mul(big.NewRat(4, 1), sub(mul(q(cutterRadius), q(cutterRadius)), mul(q(stockRadius), q(stockRadius))))
	return out, nil
}

type splitCylinderCutChart struct {
	center             geom.Vec3
	stock, cutter, end float64
	side, half         int
	cap                bool
}

func (s splitCylinderCutChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s splitCylinderCutChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s splitCylinderCutChart) evaluate(u, v float64) (p, du, dv geom.Vec3, err error) {
	if !finite(u) || !finite(v) || u < -math.Pi/2 || u > math.Pi/2 || v < 0 || v > 1 {
		return p, du, dv, geom.ErrDomain
	}
	sn, cs := math.Sincos(u)
	if math.Abs(u) == math.Pi/2 {
		cs = 0
	}
	y, w := s.stock*sn, s.stock*cs
	a, b := float64(s.side), float64(s.half)
	if s.cap {
		p = geom.Vec3{X: a * s.end, Y: y, Z: b * w * v}
		du, dv = geom.Vec3{Y: w, Z: -b * y * v}, geom.Vec3{Z: b * w}
	} else {
		r := math.Sqrt((1-y/s.cutter)*(1+y/s.cutter)) * s.cutter
		if r <= 0 || !finite(r) {
			return p, du, dv, geom.ErrConvergence
		}
		first := -y / r * w
		p = geom.Vec3{X: a * ((1-v)*r + v*s.end), Y: y, Z: b * w}
		du, dv = geom.Vec3{X: a * (1 - v) * first, Y: w, Z: -b * y}, geom.Vec3{X: a * (s.end - r)}
	}
	p = p.Add(s.center)
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}

func (s splitCylinderCutChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 {
		return 0, 0, 0, geom.ErrDomain
	}
	if s.cap {
		return upMul(2, s.stock), s.stock, 0, nil
	}
	return (unequalCylinderCommonChart{small: s.stock, large: s.cutter}).secondDerivativeBounds(ctx, u, v)
}
