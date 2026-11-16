package iges

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// Equal homogeneous boundary rows give the same position and tangential
// derivative. For the normal derivative, compare the first row differences
// divided by the actual physical width. Matching degree cancels on both
// sides. The cells carry positive weights, so these identities also prove
// both first derivatives of the Cartesian surface.
func provePeriodicTrimPhysicalSeam(ctx context.Context, lower, upper geom.ExactPeriodicSurfaceBezierCell, axis int) error {
	degree := [2]int{lower.DegreeU, lower.DegreeV}
	below := [2]geom.ExactParameterInterval{lower.U.Domain, lower.V.Domain}
	above := [2]geom.ExactParameterInterval{upper.U.Domain, upper.V.Domain}
	along := 1 - axis
	if degree != ([2]int{upper.DegreeU, upper.DegreeV}) || degree[axis] < 1 ||
		below[axis].Upper.Cmp(above[axis].Lower) != 0 ||
		below[along].Lower.Cmp(above[along].Lower) != 0 || below[along].Upper.Cmp(above[along].Upper) != 0 {
		return fmt.Errorf("iges: seam cells have different physical boundary charts: %w", geom.ErrUnrepresentable)
	}
	widthBelow := new(big.Rat).Sub(below[axis].Upper, below[axis].Lower)
	widthAbove := new(big.Rat).Sub(above[axis].Upper, above[axis].Lower)
	stride := [2]int{degree[1] + 1, 1}
	for i := 0; i <= degree[along]; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		atUpper := i * stride[along]
		atLower := atUpper + degree[axis]*stride[axis]
		for j, point := range lower.Control[atLower] {
			seamPoint := upper.Control[atUpper][j]
			if point.Cmp(seamPoint) != 0 {
				return fmt.Errorf("iges: homogeneous seam positions disagree: %w", geom.ErrUnrepresentable)
			}
			a := new(big.Rat).Sub(point, lower.Control[atLower-stride[axis]][j])
			b := new(big.Rat).Sub(upper.Control[atUpper+stride[axis]][j], seamPoint)
			if a.Mul(a, widthAbove).Cmp(b.Mul(b, widthBelow)) != 0 {
				return fmt.Errorf("iges: physical first partials disagree at the seam: %w", geom.ErrUnrepresentable)
			}
		}
	}
	return nil
}
