package trimmed

import (
	"math"
	"math/big"

	"plainsolid/geom"
)

// ExactQuadraticUV is an immutable positive-weight rational quadratic trim.
// Homogeneous coefficients are exact. Use NewRationalEndpointQuadraticUV
// when the UV endpoints need separate exact and display coordinates.
type ExactQuadraticUV struct {
	control           [3][3]*big.Rat
	ends              [2]geom.Vec2
	rationalEndpoints bool
}

// NewExactQuadraticUV copies the homogeneous controls (w*u,w*v,w).
func NewExactQuadraticUV(h [3][3]*big.Rat) (ExactQuadraticUV, error) {
	return newExactQuadraticUV(h, false)
}

// NewRationalEndpointQuadraticUV retains exact rational UV endpoints and
// their nearest float64 display coordinates. A trim whose display endpoints
// coincide cannot be used by the current display topology.
func NewRationalEndpointQuadraticUV(h [3][3]*big.Rat) (ExactQuadraticUV, error) {
	return newExactQuadraticUV(h, true)
}

func newExactQuadraticUV(h [3][3]*big.Rat, rationalEndpoints bool) (ExactQuadraticUV, error) {
	var empty ExactQuadraticUV
	for _, p := range h {
		for _, v := range p {
			if v == nil {
				return empty, geom.ErrInvalid
			}
			if v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
				return empty, geom.ErrConvergence
			}
		}
		if p[2].Sign() <= 0 {
			return empty, geom.ErrInvalid
		}
	}
	q := ExactQuadraticUV{rationalEndpoints: rationalEndpoints}
	for i, p := range h {
		for j, x := range p {
			q.control[i][j] = new(big.Rat).Set(x)
		}
	}
	for i := range q.ends {
		p := h[2*i]
		exact := [2]*big.Rat{new(big.Rat).Quo(p[0], p[2]), new(big.Rat).Quo(p[1], p[2])}
		if rationalEndpoints {
			q.ends[i].X, _ = exact[0].Float64()
			q.ends[i].Y, _ = exact[1].Float64()
			if !uvFinite(q.ends[i]) {
				return empty, geom.ErrUnrepresentable
			}
		} else {
			u, e := exactGraphPartitionUV(exact)
			if e != nil {
				return empty, e
			}
			q.ends[i] = u
		}
	}
	if q.ends[0] == q.ends[1] {
		return empty, geom.ErrDegenerate
	}
	return q, nil
}

// Endpoints returns independent exact chart endpoints and their display
// coordinates. The zero value has no exact endpoints.
func (q ExactQuadraticUV) Endpoints() (exact [2][2]*big.Rat, display [2]geom.Vec2) {
	if q.control[0][0] != nil {
		for i, j := range [2]int{0, 2} {
			for k := range exact[i] {
				exact[i][k] = new(big.Rat).Quo(q.control[j][k], q.control[j][2])
			}
		}
	}
	return exact, q.ends
}

// HomogeneousControls returns an independent copy of the exact coefficients.
func (q ExactQuadraticUV) HomogeneousControls() [3][3]*big.Rat {
	var h [3][3]*big.Rat
	if q.control[0][0] == nil {
		return h
	}
	for i, p := range q.control {
		for j, v := range p {
			h[i][j] = new(big.Rat).Set(v)
		}
	}
	return h
}

func (q ExactQuadraticUV) EvalUV(t float64) (geom.Vec2, error) {
	p, _, e := q.eval(t)
	return p, e
}

func (q ExactQuadraticUV) DerivativeUV(t float64) (geom.Vec2, error) {
	_, d, e := q.eval(t)
	return d, e
}

func (q ExactQuadraticUV) eval(t float64) (geom.Vec2, geom.Vec2, error) {
	if !finite(t) {
		return geom.Vec2{}, geom.Vec2{}, geom.ErrDomain
	}
	point, derivative, e := q.EvaluateRational(ratInset(t))
	if e != nil {
		return geom.Vec2{}, geom.Vec2{}, e
	}
	var p, d [2]float64
	for i := range point {
		p[i], _ = point[i].Float64()
		d[i], _ = derivative[i].Float64()
	}
	a, z := geom.Vec2{X: p[0], Y: p[1]}, geom.Vec2{X: d[0], Y: d[1]}
	if !uvFinite(a) || !uvFinite(z) {
		return geom.Vec2{}, geom.Vec2{}, geom.ErrUnrepresentable
	}
	return a, z, nil
}

// EvaluateRational gives the exact UV point and first derivative in the
// normalized trim parameter. Its input and returned coordinates do not
// share storage with the trim.
func (q ExactQuadraticUV) EvaluateRational(t *big.Rat) (point, derivative [2]*big.Rat, err error) {
	if q.control[0][0] == nil || t == nil {
		return point, derivative, geom.ErrInvalid
	}
	if t.Sign() < 0 || t.Cmp(big.NewRat(1, 1)) > 0 {
		return point, derivative, geom.ErrDomain
	}
	if t.Num().BitLen() > 4096 || t.Denom().BitLen() > 4096 {
		return point, derivative, geom.ErrConvergence
	}
	u := new(big.Rat).Sub(big.NewRat(1, 1), t)
	h := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	d := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	b := [3]*big.Rat{new(big.Rat).Mul(u, u), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(u, t)), new(big.Rat).Mul(t, t)}
	for i := range h {
		for j, w := range b {
			h[i].Add(h[i], new(big.Rat).Mul(w, q.control[j][i]))
		}
		a := new(big.Rat).Sub(q.control[1][i], q.control[0][i])
		z := new(big.Rat).Sub(q.control[2][i], q.control[1][i])
		d[i].Mul(big.NewRat(2, 1), new(big.Rat).Add(new(big.Rat).Mul(a, u), new(big.Rat).Mul(z, t)))
	}
	for i := range point {
		point[i] = new(big.Rat).Quo(h[i], h[2])
		derivative[i] = new(big.Rat).Quo(new(big.Rat).Sub(d[i], new(big.Rat).Mul(point[i], d[2])), h[2])
	}
	return point, derivative, nil
}

func (q ExactQuadraticUV) reversed() ExactQuadraticUV {
	q.control[0], q.control[2] = q.control[2], q.control[0]
	q.ends[0], q.ends[1] = q.ends[1], q.ends[0]
	return q
}

// BoundsUV encloses the entire exact curve in the positive homogeneous
// control hull. Nonbinary control coordinates are rounded outwards.
func (q ExactQuadraticUV) BoundsUV() (geom.UVBox, error) {
	if q.control[0][0] == nil {
		return geom.UVBox{}, geom.ErrInvalid
	}
	var lo, hi [2]float64
	for i, p := range q.control {
		for j := range lo {
			a, b, e := outwardTrimRational(new(big.Rat).Quo(p[j], p[2]))
			if e != nil {
				return geom.UVBox{}, e
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

func outwardTrimRational(q *big.Rat) (float64, float64, error) {
	v, _ := q.Float64()
	if !finite(v) {
		return 0, 0, geom.ErrUnrepresentable
	}
	a, b := v, v
	switch ratInset(v).Cmp(q) {
	case -1:
		b = math.Nextafter(v, math.Inf(1))
	case 1:
		a = math.Nextafter(v, math.Inf(-1))
	}
	if !finite(a) || !finite(b) {
		return 0, 0, geom.ErrUnrepresentable
	}
	return a, b, nil
}

// Restrict returns an exact reparameterized subtrim. Its new endpoints
// must fit the current native UV vertex representation.
func (q ExactQuadraticUV) Restrict(start, end *big.Rat) (ExactQuadraticUV, error) {
	return q.restrict(start, end, q.rationalEndpoints)
}

// RestrictRationalEndpoints retains exact rational split coordinates even
// when the new UV display vertices differ from the exact endpoints.
func (q ExactQuadraticUV) RestrictRationalEndpoints(start, end *big.Rat) (ExactQuadraticUV, error) {
	return q.restrict(start, end, true)
}

func (q ExactQuadraticUV) restrict(start, end *big.Rat, rationalEndpoints bool) (ExactQuadraticUV, error) {
	if q.control[0][0] == nil || start == nil || end == nil || start.Sign() < 0 ||
		end.Cmp(big.NewRat(1, 1)) > 0 || start.Cmp(end) >= 0 {
		return ExactQuadraticUV{}, geom.ErrDomain
	}
	for _, t := range [2]*big.Rat{start, end} {
		if t.Num().BitLen() > 4096 || t.Denom().BitLen() > 4096 {
			return ExactQuadraticUV{}, geom.ErrConvergence
		}
	}
	blossom := func(a, b *big.Rat) [3]*big.Rat {
		x, y := new(big.Rat).Sub(big.NewRat(1, 1), a), new(big.Rat).Sub(big.NewRat(1, 1), b)
		weights := [3]*big.Rat{new(big.Rat).Mul(x, y), new(big.Rat).Add(new(big.Rat).Mul(x, b), new(big.Rat).Mul(y, a)),
			new(big.Rat).Mul(a, b)}
		p := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		for j := range p {
			for i, w := range weights {
				p[j].Add(p[j], new(big.Rat).Mul(w, q.control[i][j]))
			}
		}
		return p
	}
	return newExactQuadraticUV([3][3]*big.Rat{blossom(start, start), blossom(start, end), blossom(end, end)}, rationalEndpoints)
}
