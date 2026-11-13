package iges

import (
	"io"
	"math/big"
	"plainsolid/geom"
)

// ReadAffinePartitionedSplineLensSolid additionally accepts a positive oriented
// represented conjugate basis (Radii.X,Tilt),(Shear,Radii.Y). Both diagonal
// components must be positive. The source quarter parameterization is retained.
func ReadAffinePartitionedSplineLensSolid(r io.Reader, l Limits) (PartitionedRationalSplineLens, error) {
	return readPartitionedLens(r, l, true)
}

// NewAffinePartitionedSplineLensSolid constructs lossless source edge pieces
// from original charts; unrepresentable coefficients and irrational cuts fail.
func NewAffinePartitionedSplineLensSolid(m SplineLensSolid) (PartitionedRationalSplineLens, error) {
	return newPartitionedLens(m, true)
}

// WriteAffinePartitionedSplineLensSolid re-proves material, source identities,
// exact chart cuts and exclusive closed topology before writing any bytes.
func WriteAffinePartitionedSplineLensSolid(w io.Writer, m PartitionedRationalSplineLens) error {
	return writePartitionedLens(w, m, true)
}

func recoverIGESLensBasis(points []geom.Vec3, tangent geom.Vec3) (geom.Vec2, geom.Vec2, float64, float64, error) {
	fail := func() (geom.Vec2, geom.Vec2, float64, float64, error) {
		return geom.Vec2{}, geom.Vec2{}, 0, 0, geom.ErrInvalid
	}
	if len(points) < 4 {
		return fail()
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	// The first positive curve tangent is parallel to the second basis axis.
	// det(point,tangent) is extremal at the +/- first-axis quarter vertices.
	score := func(p geom.Vec3, a, b *big.Rat) *big.Rat {
		return new(big.Rat).Sub(new(big.Rat).Mul(rat(p.X), b), new(big.Rat).Mul(rat(p.Y), a))
	}
	tx, ty := rat(tangent.X), rat(tangent.Y)
	if tx == nil || ty == nil || tx.Sign() == 0 && ty.Sign() == 0 {
		return fail()
	}
	west := points[0]
	for _, p := range points[1:] {
		if score(p, tx, ty).Cmp(score(west, tx, ty)) < 0 {
			west = p
		}
	}
	cx, e := lensAverage(points[0].X, west.X)
	if e != nil {
		return fail()
	}
	cy, e := lensAverage(points[0].Y, west.Y)
	if e != nil {
		return fail()
	}
	ax, e := lensExact(points[0].X, cx, -1)
	if e != nil {
		return fail()
	}
	ay, e := lensExact(points[0].Y, cy, -1)
	if e != nil {
		return fail()
	}
	// det(firstAxis,point) is maximal at the positive second-axis endpoint.
	north := points[0]
	for _, p := range points[1:] {
		if score(p, rat(ax), rat(ay)).Cmp(score(north, rat(ax), rat(ay))) < 0 {
			north = p
		}
	}
	bx, e := lensExact(north.X, cx, -1)
	if e != nil {
		return fail()
	}
	by, e := lensExact(north.Y, cy, -1)
	if e != nil {
		return fail()
	}
	if ax <= 0 || by <= 0 {
		return fail()
	}
	return geom.Vec2{X: cx, Y: cy}, geom.Vec2{X: ax, Y: by}, bx, ay, nil
}
