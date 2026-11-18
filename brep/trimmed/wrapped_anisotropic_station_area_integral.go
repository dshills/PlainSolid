//go:build research

package trimmed

import "math/big"

func anisotropicAreaChoose(n, k int) int64 {
	if k < 0 || k > n {
		return 0
	}
	v := int64(1)
	for i := 1; i <= k; i++ {
		v = v * int64(n-i+1) / int64(i)
	}
	return v
}
func anisotropicAreaProduct(a, c []*big.Rat, b *stationMassBudget) []*big.Rat {
	n, m := len(a)-1, len(c)-1
	out := make([]*big.Rat, n+m+1)
	for k := range out {
		out[k] = new(big.Rat)
		for i := max(0, k-m); i <= min(n, k); i++ {
			factor := big.NewRat(anisotropicAreaChoose(n, i)*anisotropicAreaChoose(m, k-i), anisotropicAreaChoose(n+m, k))
			out[k] = b.add(out[k], b.mul(factor, b.mul(a[i], c[k-i])))
		}
	}
	return out
}

type anisotropicAreaProfile struct {
	aw, bw, ay, bx []*big.Rat
	coeff          [5]*big.Rat
	remainder      *big.Rat
}

func anisotropicAreaProfileFrom(q stationMassSegment, b *stationMassBudget) anisotropicAreaProfile {
	p := anisotropicAreaProfile{}
	var x, y, w [3]*big.Rat
	for i := 0; i < 3; i++ {
		x[i], y[i], w[i] = q[i][0], q[i][1], q[i][2]
	}
	var a, c [3]*big.Rat
	for k, pair := range [3][2]int{{0, 1}, {0, 2}, {1, 2}} {
		i, j := pair[0], pair[1]
		a[k] = b.sub(b.mul(w[i], x[j]), b.mul(x[i], w[j]))
		c[k] = b.sub(b.mul(w[i], y[j]), b.mul(y[i], w[j]))
		if k != 1 {
			a[k] = b.add(a[k], a[k])
			c[k] = b.add(c[k], c[k])
		}
	}
	p.aw = anisotropicAreaProduct(a[:], w[:], b)
	p.bw = anisotropicAreaProduct(c[:], w[:], b)
	p.ay = anisotropicAreaProduct(a[:], y[:], b)
	p.bx = anisotropicAreaProduct(c[:], x[:], b)
	den := anisotropicAreaProduct(anisotropicAreaProduct(w[:], w[:], b), w[:], b)
	lo, hi := den[0], den[0]
	for _, d := range den[1:] {
		if d.Cmp(lo) < 0 {
			lo = d
		}
		if d.Cmp(hi) > 0 {
			hi = d
		}
	}
	if b.err != nil {
		return p
	}
	mean := b.div(b.add(lo, hi), big.NewRat(2, 1))
	msq := b.mul(mean, mean)
	radius := b.sub(mean, lo)
	p.remainder = b.div(b.mul(radius, radius), b.mul(msq, lo))
	for i := 0; i < 5; i++ {
		p.coeff[i] = new(big.Rat)
		for j, d := range den {
			linear := b.div(b.sub(b.add(lo, hi), d), msq)
			integral := big.NewRat(anisotropicAreaChoose(4, i)*anisotropicAreaChoose(6, j), 11*anisotropicAreaChoose(10, i+j))
			p.coeff[i] = b.add(p.coeff[i], b.mul(linear, integral))
		}
	}
	return p
}

// Exact cell cross product has degree(4,1) numerator over positive W³.
// 1/D=(2m-D)/m²+rho, 0<=rho<=(D-m)²/(m² minD).
// Reverse triangle bounds the integral below, convexity bounds it above;
// the signed vector remainder is never assumed to increase its norm.
func (p anisotropicAreaProfile) enclose(x, y [2]*big.Rat, h *big.Rat, b *stationMassBudget) WrappedStationMassInterval {
	dx, dy := b.sub(x[1], x[0]), b.sub(y[1], y[0])
	integral := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	upper, largest := new(big.Rat), new(big.Rat)
	norm := func(v [3]*big.Rat, up bool) *big.Rat {
		sq := new(big.Rat)
		for _, a := range v {
			sq = b.add(sq, b.mul(a, a))
		}
		return stationAreaSqrt(sq, up, b)
	}
	for i := 0; i < 5; i++ {
		for j := 0; j < 2; j++ {
			v := [3]*big.Rat{b.mul(b.mul(h, y[j]), p.bw[i]), b.mul(b.mul(h, x[j]), p.aw[i]), b.sub(b.mul(b.mul(x[j], dy), p.ay[i]), b.mul(b.mul(y[j], dx), p.bx[i]))}
			v[1] = b.sub(new(big.Rat), v[1])
			coefficient := b.div(p.coeff[i], big.NewRat(2, 1))
			for axis := 0; axis < 3; axis++ {
				integral[axis] = b.add(integral[axis], b.mul(coefficient, v[axis]))
			}
			m := norm(v, true)
			upper = b.add(upper, b.mul(coefficient, m))
			if m.Cmp(largest) > 0 {
				largest = m
			}
		}
	}
	rem := b.mul(p.remainder, largest)
	lower := b.sub(norm(integral, false), rem)
	if lower.Sign() < 0 {
		lower = new(big.Rat)
	}
	upper = b.add(upper, rem)
	return WrappedStationMassInterval{stationAreaDyadic(lower, false, b), stationAreaDyadic(upper, true, b)}
}
