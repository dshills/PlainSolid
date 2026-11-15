package step

import (
	"math/big"
	"plainsolid/geom"
)

func affineSplineSourceRat(v Value) (*big.Rat, error) {
	q, e := liftSourceRat(v)
	if e != nil || q.Num().BitLen() > 256 || q.Denom().BitLen() > 256 {
		return nil, geom.ErrInvalid
	}
	return q, nil
}
func affineSplineGreville(m, k Value, d, n int) ([17]string, [17]int, []*big.Rat, error) {
	var out [17]string
	var mult [17]int
	bad := func() ([17]string, [17]int, []*big.Rat, error) { return out, mult, nil, geom.ErrInvalid }
	if n < d+1 || n > 17 || m.Kind != Aggregate || k.Kind != Aggregate || len(k.Items) < 2 || len(k.Items) > 17 || len(m.Items) != len(k.Items) {
		return bad()
	}
	var full []*big.Rat
	var last *big.Rat
	for i, val := range k.Items {
		q, e := affineSplineSourceRat(val)
		if e != nil || (last != nil && q.Cmp(last) <= 0) {
			return bad()
		}
		mu, e := affineSplineSourceRat(m.Items[i])
		if e != nil || !mu.IsInt() || !mu.Num().IsInt64() {
			return bad()
		}
		x := mu.Num().Int64()
		if x < 1 || x > int64(d) && i != 0 && i != len(k.Items)-1 || (i == 0 || i == len(k.Items)-1) && x != int64(d+1) {
			return bad()
		}
		out[i] = q.RatString()
		mult[i] = int(x)
		last = q
		for j := int64(0); j < x; j++ {
			full = append(full, q)
		}
	}
	if len(full) != n+d+1 {
		return bad()
	}
	g := make([]*big.Rat, n)
	span := new(big.Rat).Sub(full[len(full)-1], full[0])
	for i := range g {
		g[i] = new(big.Rat)
		for j := 1; j <= d; j++ {
			g[i].Add(g[i], full[i+j])
		}
		g[i].Quo(g[i], big.NewRat(int64(d), 1))
		g[i].Sub(g[i], full[0])
		g[i].Quo(g[i], span)
		if g[i].Num().BitLen() > 1024 || g[i].Denom().BitLen() > 1024 {
			return bad()
		}
	}
	return out, mult, g, nil
}
func affineSplineKnotSpec(spec, m, k Value, d int) bool {
	if spec.Kind != Enumeration {
		return false
	}
	switch spec.Text {
	case "UNSPECIFIED":
		return true
	case "PIECEWISE_BEZIER_KNOTS":
		for i := 1; i < len(m.Items)-1; i++ {
			q, e := affineSplineSourceRat(m.Items[i])
			if e != nil || q.Cmp(big.NewRat(int64(d), 1)) != 0 {
				return false
			}
		}
		return true
	case "QUASI_UNIFORM_KNOTS":
		var delta *big.Rat
		for i := 1; i < len(k.Items); i++ {
			a, e := affineSplineSourceRat(k.Items[i])
			b, e2 := affineSplineSourceRat(k.Items[i-1])
			if e != nil || e2 != nil {
				return false
			}
			v := new(big.Rat).Sub(a, b)
			if delta != nil && v.Cmp(delta) != 0 {
				return false
			}
			delta = v
			if i < len(m.Items)-1 {
				q, e := affineSplineSourceRat(m.Items[i])
				if e != nil || q.Cmp(big.NewRat(1, 1)) != 0 {
					return false
				}
			}
		}
		return true
	}
	return false
}
