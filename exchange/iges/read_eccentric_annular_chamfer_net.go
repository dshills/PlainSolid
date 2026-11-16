package iges

import (
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// igesAnnularChamferNet proves four full rational quarter-circle arcs at
// either end of a finite meridian segment. The four cardinal quadratics
// have midpoint weight squared equal to half the endpoint weight product.
// Equal weights in both V rows make the entire wall affine in V. The
// azimuth and height signs coincide, giving an outward geometric normal.
func igesAnnularChamferNet(p []parameter) (geom.Vec3, [2]trimmed.WrappedLoftStation, error) {
	var center geom.Vec3
	var stations [2]trimmed.WrappedLoftStation
	fail := func() (geom.Vec3, [2]trimmed.WrappedLoftStation, error) {
		return geom.Vec3{}, [2]trimmed.WrappedLoftStation{}, errIGESAnnularChamfer
	}
	const header = 10
	const knots = 16
	const weights = 18
	const poles = 54
	const geometry = header + knots + weights + poles + 4
	if len(p) != geometry && len(p) != geometry+2 {
		return fail()
	}
	for i, expected := range [header]int{128, 8, 1, 2, 1, 1, 0, 0, 0, 0} {
		got, err := intValue(p[i])
		if err != nil || got != expected {
			return fail()
		}
	}
	for i, expected := range [knots]int64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4, 0, 0, 1, 1} {
		got, err := igesPlanarExact(p[header+i])
		if err != nil || got.Cmp(big.NewRat(expected, 1)) != 0 {
			return fail()
		}
	}
	for i, expected := range [4]int64{0, 4, 0, 1} {
		got, err := igesPlanarExact(p[geometry-4+i])
		if err != nil || got.Cmp(big.NewRat(expected, 1)) != 0 {
			return fail()
		}
	}
	for _, flag := range p[geometry:] {
		got, err := intValue(flag)
		if err != nil || got != 0 {
			return fail()
		}
	}
	expectedWeights := [9]int64{1, 1, 2, 2, 4, 2, 2, 1, 1}
	base, err := igesPlanarExact(p[header+knots])
	if err != nil || base.Sign() <= 0 {
		return fail()
	}
	for i, value := range p[header+knots : header+knots+weights] {
		got, err := igesPlanarExact(value)
		want := new(big.Rat).Mul(base, big.NewRat(expectedWeights[i%9], 1))
		if err != nil || got.Cmp(want) != 0 {
			return fail()
		}
	}
	var net [2][9]igesPlanarVec
	for i := range net {
		for j := range net[i] {
			offset := header + knots + weights + (i*9+j)*3
			net[i][j], err = igesPlanarCoordinates(p[offset : offset+3])
			if err != nil {
				return fail()
			}
		}
	}
	sign := net[1][0][2].Cmp(net[0][0][2])
	if sign == 0 {
		return fail()
	}
	x := new(big.Rat).Quo(new(big.Rat).Add(net[0][0][0], net[0][4][0]), big.NewRat(2, 1))
	y := net[0][0][1]
	center, err = geom.RepresentedExactPoint([3]*big.Rat{x, y, new(big.Rat)})
	if err != nil {
		return fail()
	}
	directions := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	for i, row := range net {
		z := row[0][2]
		radius := new(big.Rat).Sub(row[0][0], x)
		if radius.Sign() <= 0 {
			return fail()
		}
		for j, d := range directions {
			a := new(big.Rat).Add(x, new(big.Rat).Mul(radius, big.NewRat(d[0], 1)))
			b := new(big.Rat).Add(y, new(big.Rat).Mul(radius, big.NewRat(d[1]*int64(sign), 1)))
			if !row[j].equal(igesPlanarVec{a, b, z}) {
				return fail()
			}
		}
		zf, zOK := z.Float64()
		rf, rOK := radius.Float64()
		if !zOK || !rOK {
			return fail()
		}
		j := i
		if sign < 0 {
			j = 1 - i
		}
		stations[j] = trimmed.WrappedLoftStation{Z: zf, Radius: rf}
	}
	return center, stations, nil
}
