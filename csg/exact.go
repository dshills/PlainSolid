package csg

import (
	"fmt"
	"math"
	"math/big"
	"plainsolid/geom"
)

// Exactly parallel equally oriented planes can be replaced by the stricter one.
// Otherwise, angular proximity alone is insufficient: at long lever arms it can
// correspond to large model-space displacement. A linear distance difference
// attains its extrema on the vertices of the inputs' convex hull.
func equivalentPlanes(p, q plane, av, bv []geom.Vec3, origin geom.Vec3, tolerance float64) bool {
	if p.n == q.n {
		return true
	}
	delta := p.n.Sub(q.n)
	if delta.Length() > angularTolerance {
		return false
	}
	coefficients := [3]*big.Rat{difference(rational(p.n.X), rational(q.n.X)), difference(rational(p.n.Y), rational(q.n.Y)), difference(rational(p.n.Z), rational(q.n.Z))}
	offset := difference(rational(p.d), rational(q.d))
	limit := rational(tolerance)
	for _, vertices := range [][]geom.Vec3{av, bv} {
		for _, v := range vertices {
			local := [3]*big.Rat{difference(rational(v.X), rational(origin.X)), difference(rational(v.Y), rational(origin.Y)), difference(rational(v.Z), rational(origin.Z))}
			signed := new(big.Rat).Neg(offset)
			for i := 0; i < 3; i++ {
				signed.Add(signed, product(coefficients[i], local[i]))
			}
			signed.Abs(signed)
			if signed.Cmp(limit) > 0 {
				return false
			}
		}
	}
	return true
}

type rationalMatrix [3][3]*big.Rat

func rational(x float64) *big.Rat       { return new(big.Rat).SetFloat64(x) }
func product(a, b *big.Rat) *big.Rat    { return new(big.Rat).Mul(a, b) }
func difference(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
func determinant(a rationalMatrix) *big.Rat {
	first := product(a[0][0], difference(product(a[1][1], a[2][2]), product(a[1][2], a[2][1])))
	second := product(a[0][1], difference(product(a[1][0], a[2][2]), product(a[1][2], a[2][0])))
	third := product(a[0][2], difference(product(a[1][0], a[2][1]), product(a[1][1], a[2][0])))
	return new(big.Rat).Add(difference(first, second), third)
}

// exactIntersection skips a triple only when its represented determinant is
// exactly zero, or its unique intersection is outside at least one halfspace.
// Checking halfspaces before float conversion avoids overflow for remote points.
func exactIntersection(p, q, r plane, halfspaces []plane, tolerance float64) (geom.Vec3, bool, error) {
	var matrix rationalMatrix
	rhs := [3]*big.Rat{}
	for i, h := range []plane{p, q, r} {
		matrix[i] = [3]*big.Rat{rational(h.n.X), rational(h.n.Y), rational(h.n.Z)}
		rhs[i] = rational(h.d)
	}
	det := determinant(matrix)
	if det.Sign() == 0 {
		return geom.Vec3{}, false, nil
	}
	var coordinate [3]*big.Rat
	for column := 0; column < 3; column++ {
		m := matrix
		for row := 0; row < 3; row++ {
			m[row][column] = rhs[row]
		}
		coordinate[column] = new(big.Rat).Quo(determinant(m), det)
	}
	limit := rational(tolerance)
	for _, h := range halfspaces {
		signed := new(big.Rat).Add(product(rational(h.n.X), coordinate[0]), product(rational(h.n.Y), coordinate[1]))
		signed.Add(signed, product(rational(h.n.Z), coordinate[2]))
		signed.Sub(signed, rational(h.d))
		if signed.Cmp(limit) > 0 {
			return geom.Vec3{}, false, nil
		}
	}
	x, _ := coordinate[0].Float64()
	y, _ := coordinate[1].Float64()
	z, _ := coordinate[2].Float64()
	point := geom.Vec3{X: x, Y: y, Z: z}
	if !point.IsFinite() {
		return geom.Vec3{}, false, fmt.Errorf("csg: exact plane intersection exceeds float64 coordinate range")
	}
	for _, h := range halfspaces {
		distance := h.n.Dot(point) - h.d
		if math.IsNaN(distance) || math.IsInf(distance, 0) || distance > tolerance {
			return geom.Vec3{}, false, fmt.Errorf("csg: exact plane intersection cannot be represented within tolerance")
		}
	}
	return point, true, nil
}
