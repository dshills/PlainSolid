package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

// polynomialPlaneEquation expresses the dropped coordinate as an affine
// function of the remaining two. Its residual has the sign of the raw
// outward halfspace multiplied by orientation.
type polynomialPlaneEquation struct {
	axes        [2]int
	dropped     int
	normal      [3]*big.Rat // normalized so normal[dropped] == 1
	offset      *big.Rat
	orientation int
}

func horizontalPolynomialPlane(height *big.Rat) polynomialPlaneEquation {
	return polynomialPlaneEquation{
		axes: [2]int{0, 1}, dropped: 2, orientation: 1, offset: height,
		normal: [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)},
	}
}

func polynomialPlaneFromPoints(points [3]geom.Vec3) (polynomialPlaneEquation, error) {
	var a, b, n, origin [3]*big.Rat
	p := [3][3]float64{
		{points[0].X, points[0].Y, points[0].Z},
		{points[1].X, points[1].Y, points[1].Z},
		{points[2].X, points[2].Y, points[2].Z},
	}
	for i := range a {
		origin[i] = ratInset(p[0][i])
		a[i] = new(big.Rat).Sub(ratInset(p[1][i]), origin[i])
		b[i] = new(big.Rat).Sub(ratInset(p[2][i]), origin[i])
	}
	for i := range n {
		j, k := (i+1)%3, (i+2)%3
		n[i] = new(big.Rat).Sub(new(big.Rat).Mul(a[j], b[k]), new(big.Rat).Mul(a[k], b[j]))
	}
	// Prefer XY so horizontal and oblique cuts of Z graphs share a path.
	for _, axis := range []int{2, 0, 1} {
		if n[axis].Sign() == 0 {
			continue
		}
		q := polynomialPlaneEquation{axes: [2]int{(axis + 1) % 3, (axis + 2) % 3},
			dropped: axis, orientation: n[axis].Sign(), offset: new(big.Rat)}
		for i := range n {
			q.normal[i] = new(big.Rat).Quo(n[i], n[axis])
			q.offset.Add(q.offset, new(big.Rat).Mul(q.normal[i], origin[i]))
		}
		return q, nil
	}
	return polynomialPlaneEquation{}, geom.ErrConvergence
}

func (q polynomialPlaneEquation) dot(p [3]*big.Rat) *big.Rat {
	v := new(big.Rat).Set(p[q.dropped])
	for _, i := range q.axes {
		if q.normal[i].Sign() != 0 {
			v.Add(v, new(big.Rat).Mul(q.normal[i], p[i]))
		}
	}
	return v
}
