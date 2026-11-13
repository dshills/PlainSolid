package step

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// PolynomialWindingOptions bounds exact radial-ray root isolation. Zero fields
// select 64 subdivision levels and 65536 cells. Exhaustion returns an error.
type PolynomialWindingOptions struct{ MaxDepth, MaxCells int }

// PolynomialWindingEvidence proves the signed turn count around positive Z for
// an exactly closed source with strictly monotone azimuth. Each crossing is an
// exact parameter (equal bounds) or an OPEN isolating interval containing
// precisely one root of
// y-origin.Y=0 with x-origin.X>0. It is not a BRep ownership certificate.
type PolynomialWindingEvidence struct {
	Turns     int
	Crossings [][2]*big.Rat
	Cells     int
}

// BoundPolynomialSplineWinding counts transverse crossings of the positive-X
// radial ray. Exact Bernstein sign variations isolate roots; subdivision proves
// their positive radial coordinate. Shared knot roots count once and the final
// endpoint is excluded, giving a half-open traversal of the closed source.
func BoundPolynomialSplineWinding(r io.Reader, l Limits, curveID int64, origin geom.Vec3, o PolynomialWindingOptions) (out PolynomialWindingEvidence, err error) {
	fail := func() (PolynomialWindingEvidence, error) {
		return PolynomialWindingEvidence{}, fmt.Errorf("step: winding proof unsupported or budget exhausted")
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 64
	}
	if o.MaxCells == 0 {
		o.MaxCells = 65536
	}
	if o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxCells > 1000000 || r == nil {
		return fail()
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return out, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return fail()
	}
	angular, e := BoundPolynomialSplineAzimuth(bytes.NewReader(raw), l, curveID, origin)
	if e != nil {
		return out, e
	}
	if !angular.Closed || !angular.Positive && !angular.Negative {
		return fail()
	}
	p, poles, knots, e := readResidualPolynomial(bytes.NewReader(raw), l, curveID)
	if e != nil {
		return out, e
	}
	inverse, e := residualBernsteinInverse(p)
	if e != nil {
		return out, e
	}
	end := knots[len(poles)]
	seen := map[string]bool{}
	endpoint := func(t, x, y *big.Rat) error {
		if t.Cmp(end) == 0 || y.Sign() != 0 {
			return nil
		}
		if x.Sign() == 0 {
			return geom.ErrDomain
		}
		if x.Sign() < 0 {
			return nil
		}
		key := t.RatString()
		if !seen[key] {
			seen[key] = true
			out.Crossings = append(out.Crossings, [2]*big.Rat{new(big.Rat).Set(t), new(big.Rat).Set(t)})
		}
		return nil
	}
	var visit func([]*big.Rat, []*big.Rat, *big.Rat, *big.Rat, int) error
	visit = func(x, y []*big.Rat, a, b *big.Rat, depth int) error {
		for i := range x {
			if !residualBits(x[i]) || !residualBits(y[i]) {
				return geom.ErrDomain
			}
		}
		out.Cells++
		if out.Cells > o.MaxCells {
			return geom.ErrDomain
		}
		if e := endpoint(a, x[0], y[0]); e != nil {
			return e
		}
		if e := endpoint(b, x[p], y[p]); e != nil {
			return e
		}
		variations, previous := 0, 0
		for _, q := range y {
			sign := q.Sign()
			if sign == 0 {
				continue
			}
			if previous != 0 && sign != previous {
				variations++
			}
			previous = sign
		}
		if previous == 0 {
			return geom.ErrDomain
		}
		if variations == 0 {
			return nil
		}
		positive, negative := true, true
		for _, q := range x {
			positive = positive && q.Sign() > 0
			negative = negative && q.Sign() < 0
		}
		if variations == 1 && (positive || negative) {
			if positive {
				out.Crossings = append(out.Crossings, [2]*big.Rat{new(big.Rat).Set(a), new(big.Rat).Set(b)})
			}
			return nil
		}
		if depth >= o.MaxDepth {
			return geom.ErrDomain
		}
		xl, xr := windingBisect(x)
		yl, yr := windingBisect(y)
		mid := new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1))
		if e := visit(xl, yl, a, mid, depth+1); e != nil {
			return e
		}
		return visit(xr, yr, mid, b, depth+1)
	}
	ox, oy := new(big.Rat).SetFloat64(origin.X), new(big.Rat).SetFloat64(origin.Y)
	for k := p; k < len(poles); k++ {
		a, b := knots[k], knots[k+1]
		if a.Cmp(b) == 0 {
			continue
		}
		samples := make([][2]*big.Rat, p+1)
		for j := 0; j <= p; j++ {
			t := new(big.Rat).Add(a, new(big.Rat).Mul(new(big.Rat).Sub(b, a), big.NewRat(int64(j), int64(p))))
			q, e := residualDeBoor(p, poles, knots, k, t)
			if e != nil {
				return fail()
			}
			samples[j] = [2]*big.Rat{new(big.Rat).Sub(q[0], ox), new(big.Rat).Sub(q[1], oy)}
		}
		controls := [2][]*big.Rat{make([]*big.Rat, p+1), make([]*big.Rat, p+1)}
		for c := 0; c < 2; c++ {
			for i := 0; i <= p; i++ {
				q := new(big.Rat)
				for j := 0; j <= p; j++ {
					q.Add(q, new(big.Rat).Mul(inverse[i][j], samples[j][c]))
				}
				if !residualBits(q) {
					return fail()
				}
				controls[c][i] = q
			}
		}
		if e = visit(controls[0], controls[1], a, b, 0); e != nil {
			return fail()
		}
	}
	if len(out.Crossings) == 0 {
		return fail()
	}
	sort.Slice(out.Crossings, func(i, j int) bool { return out.Crossings[i][0].Cmp(out.Crossings[j][0]) < 0 })
	out.Turns = len(out.Crossings)
	if angular.Negative {
		out.Turns = -out.Turns
	}
	return out, nil
}
func windingBisect(q []*big.Rat) ([]*big.Rat, []*big.Rat) {
	n := len(q)
	d := make([]*big.Rat, n)
	left, right := make([]*big.Rat, n), make([]*big.Rat, n)
	for i := range d {
		d[i] = new(big.Rat).Set(q[i])
	}
	left[0] = new(big.Rat).Set(d[0])
	right[n-1] = new(big.Rat).Set(d[n-1])
	for r := 1; r < n; r++ {
		for i := 0; i < n-r; i++ {
			d[i].Quo(new(big.Rat).Add(d[i], d[i+1]), big.NewRat(2, 1))
		}
		left[r] = new(big.Rat).Set(d[0])
		right[n-1-r] = new(big.Rat).Set(d[n-1-r])
	}
	return left, right
}
