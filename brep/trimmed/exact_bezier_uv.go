package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// ExactBezierUV is an immutable cubic or quartic rational Bézier trim.
// It retains exact homogeneous coefficients and requires exactly
// representable endpoint coordinates. Its parameter range is [0,1].
type ExactBezierUV struct {
	control [][3]*big.Rat
	ends    [2]geom.Vec2
}

// NewExactBezierUV copies positive-weight (w*u,w*v,w) coefficients.
func NewExactBezierUV(h [][3]*big.Rat) (ExactBezierUV, error) {
	var zero ExactBezierUV
	if len(h) < 4 || len(h) > 5 {
		return zero, geom.ErrInvalid
	}
	for _, p := range h {
		for _, v := range p {
			if v == nil {
				return zero, geom.ErrInvalid
			}
			if v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
				return zero, geom.ErrConvergence
			}
		}
		if p[2].Sign() <= 0 {
			return zero, geom.ErrInvalid
		}
	}
	q := ExactBezierUV{control: copyHomogeneousUV(h)}
	for i, p := range [2][3]*big.Rat{h[0], h[len(h)-1]} {
		u, e := exactGraphPartitionUV([2]*big.Rat{new(big.Rat).Quo(p[0], p[2]), new(big.Rat).Quo(p[1], p[2])})
		if e != nil {
			return zero, e
		}
		q.ends[i] = u
	}
	if q.ends[0] == q.ends[1] {
		return zero, geom.ErrDegenerate
	}
	return q, nil
}

func copyHomogeneousUV(h [][3]*big.Rat) [][3]*big.Rat {
	if len(h) == 0 {
		return nil
	}
	out := make([][3]*big.Rat, len(h))
	for i, control := range h {
		for j, x := range control {
			out[i][j] = new(big.Rat).Set(x)
		}
	}
	return out
}

// HomogeneousControls returns caller-owned copies.
func (q ExactBezierUV) HomogeneousControls() [][3]*big.Rat { return copyHomogeneousUV(q.control) }

func (q ExactBezierUV) reversed() ExactBezierUV {
	h := make([][3]*big.Rat, len(q.control))
	for i, p := range q.control {
		h[len(h)-1-i] = p
	}
	return ExactBezierUV{control: h, ends: [2]geom.Vec2{q.ends[1], q.ends[0]}}
}

// EvaluateRational returns exact chart coordinates and their derivative.
func (q ExactBezierUV) EvaluateRational(t *big.Rat) (p, d [2]*big.Rat, err error) {
	if len(q.control) == 0 || t == nil {
		return p, d, geom.ErrInvalid
	}
	if t.Sign() < 0 || t.Cmp(big.NewRat(1, 1)) > 0 {
		return p, d, geom.ErrDomain
	}
	if t.Num().BitLen() > 4096 || t.Denom().BitLen() > 4096 {
		return p, d, geom.ErrConvergence
	}
	h := copyHomogeneousUV(q.control)
	u := new(big.Rat).Sub(big.NewRat(1, 1), t)
	blend := func(a, b *big.Rat) { a.Add(a.Mul(a, u), new(big.Rat).Mul(b, t)) }
	for n := len(h) - 1; n > 1; n-- {
		for i := range n {
			for j := range h[i] {
				blend(h[i][j], h[i+1][j])
			}
		}
	}
	var derivative [3]*big.Rat
	degree := big.NewRat(int64(len(h)-1), 1)
	for j := range derivative {
		derivative[j] = new(big.Rat).Mul(degree, new(big.Rat).Sub(h[1][j], h[0][j]))
		blend(h[0][j], h[1][j])
	}
	for j := range p {
		p[j] = new(big.Rat).Quo(h[0][j], h[0][2])
		d[j] = new(big.Rat).Quo(new(big.Rat).Sub(derivative[j], new(big.Rat).Mul(p[j], derivative[2])), h[0][2])
	}
	return p, d, nil
}

func (q ExactBezierUV) EvalUV(t float64) (geom.Vec2, error) {
	if !finite(t) {
		return geom.Vec2{}, geom.ErrDomain
	}
	p, _, e := q.EvaluateRational(ratInset(t))
	return exactBezierUVDisplay(p, e)
}

func (q ExactBezierUV) DerivativeUV(t float64) (geom.Vec2, error) {
	if !finite(t) {
		return geom.Vec2{}, geom.ErrDomain
	}
	_, d, e := q.EvaluateRational(ratInset(t))
	return exactBezierUVDisplay(d, e)
}

func exactBezierUVDisplay(v [2]*big.Rat, err error) (geom.Vec2, error) {
	if err != nil {
		return geom.Vec2{}, err
	}
	x, _ := v[0].Float64()
	y, _ := v[1].Float64()
	p := geom.Vec2{X: x, Y: y}
	if !uvFinite(p) {
		return geom.Vec2{}, geom.ErrUnrepresentable
	}
	return p, nil
}

// BoundsUV encloses the complete curve using the positive control hull.
func (q ExactBezierUV) BoundsUV() (geom.UVBox, error) {
	var box geom.UVBox
	if len(q.control) == 0 {
		return box, geom.ErrInvalid
	}
	var lo, hi [2]float64
	for i, p := range q.control {
		for j := range lo {
			a, b, e := outwardTrimRational(new(big.Rat).Quo(p[j], p[2]))
			if e != nil {
				return box, e
			}
			if i == 0 {
				lo[j], hi[j] = a, b
			} else {
				lo[j], hi[j] = min(lo[j], a), max(hi[j], b)
			}
		}
	}
	return geom.UVBox{Min: geom.Vec2{X: lo[0], Y: lo[1]}, Max: geom.Vec2{X: hi[0], Y: hi[1]}}, nil
}

func (q ExactBezierUV) descriptor(ctx context.Context, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	h := make([][4]*big.Rat, len(q.control))
	for i, p := range q.control {
		h[i] = [4]*big.Rat{p[0], p[1], new(big.Rat), p[2]}
	}
	return geom.NewExactHomogeneousBezier(ctx, h, o)
}
