package trimmed

import (
	"math"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func nativeQuarticRoofPairFixture(t *testing.T, axis int, scale *big.Rat, factors [4][2]int64) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}
	g := []*big.Rat{new(big.Rat).Set(scale)}
	for _, root := range factors {
		next := make([]*big.Rat, len(g)+1)
		for i := range next {
			next[i] = new(big.Rat)
		}
		for i, c := range g {
			next[i].Sub(next[i], new(big.Rat).Mul(c, big.NewRat(root[0], 1)))
			next[i+1].Add(next[i+1], new(big.Rat).Mul(c, big.NewRat(root[1], 1)))
		}
		g = next
	}
	for i, c := range f {
		g[i].Add(g[i], c)
	}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func polynomialPairSourceEnvelopeArea(fuse bool, stations []float64, branches func(float64) (f, g, df, dg float64)) float64 {
	return polynomialPairSourceEnvelopeAreaScaled(fuse, stations, branches, 1, 1, [3]float64{})
}

func polynomialPairSourceEnvelopeAreaScaled(fuse bool, stations []float64, branches func(float64) (f, g, df, dg float64),
	freeLength, dependentLength float64, base [3]float64) float64 {
	return polynomialPairSourceEnvelopeAreaOnVectors(fuse, stations, branches, geom.Vec2{X: freeLength}, geom.Vec2{Y: dependentLength}, base)
}

func polynomialPairSourceEnvelopeAreaOnVectors(fuse bool, stations []float64, branches func(float64) (f, g, df, dg float64),
	uVector, vVector geom.Vec2, base [3]float64) float64 {
	// Independent Gauss-Legendre integration directly on the original
	// stock equations. Each quadrature rectangle maps to one signed
	// strip over a slab and evaluates the original roof derivatives.
	x := [8]float64{.09501250983763744, .2816035507792589, .4580167776572274, .6178762444026438, .7554044083550030, .8656312023878318, .9445750230732326, .9894009349916499}
	w := [8]float64{.1894506104550685, .1826034150449236, .1691565193950025, .1495959888165767, .1246289712555339, .09515851168249278, .06225352393864789, .027152459411754095}
	integrate := func(a, b float64, value func(float64) float64) float64 {
		half, center, sum := (b-a)/2, (a+b)/2, 0.0
		for i, t := range x {
			delta := half * t
			sum += w[i] * (value(center-delta) + value(center+delta))
		}
		return sum * half
	}
	roofHeight := func(u, v float64, raised bool) float64 {
		base := 3 + u*v/64
		if raised {
			f, g, _, _ := branches(u)
			base += (105.0 / 128) * (v - f) * (v - g)
		}
		return base
	}
	freeLength, dependentLength := math.Hypot(uVector.X, uVector.Y), math.Hypot(vVector.X, vVector.Y)
	xy := uVector.X*vVector.Y - uVector.Y*vVector.X
	areaScale := func(du, dv float64) float64 {
		x := uVector.Y*dv - vVector.Y*du
		y := vVector.X*du - uVector.X*dv
		return math.Sqrt(xy*xy + x*x + y*y)
	}
	roofJacobian := func(u, v float64, raised bool) float64 {
		du, dv := v/64, u/64
		if raised {
			f, g, df, dg := branches(u)
			du -= (105.0 / 128) * (df*(v-g) + dg*(v-f))
			dv += (105.0 / 128) * (2*v - f - g)
		}
		return areaScale(du, dv)
	}
	fiber := func(u float64, value func(float64, float64, bool) float64) float64 {
		f, g, _, _ := branches(u)
		lo, hi := min(f, g), max(f, g)
		exterior := func(v float64) float64 { return value(u, v, fuse) }
		interior := func(v float64) float64 { return value(u, v, !fuse) }
		return integrate(0, lo, exterior) + integrate(lo, hi, interior) + integrate(hi, 1, exterior)
	}
	uMin, uMax := stations[0], stations[len(stations)-1]
	area := areaScale(base[1], base[2]) * (uMax - uMin)
	for i := range stations[:len(stations)-1] {
		area += integrate(stations[i], stations[i+1], func(u float64) float64 { return fiber(u, roofJacobian) })
	}
	// Each wall is a vertical fiber over an original domain side.
	thickness := func(u, v float64, raised bool) float64 {
		return roofHeight(u, v, raised) - (base[0] + base[1]*u + base[2]*v)
	}
	for i, uEdge := range [2]float64{uMin, uMax} {
		vEdge := float64(i)
		area += freeLength * integrate(uMin, uMax, func(u float64) float64 { return thickness(u, vEdge, fuse) })
		area += dependentLength * fiber(uEdge, thickness)
	}
	return area
}
