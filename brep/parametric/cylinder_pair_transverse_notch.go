package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// assembleTransverseInteriorNotch consumes two independently qualified
// equal-span footprint primitives. Restoring the lens below and above the
// original cutter caps leaves one regular side-open notch, not a cavity shell.
func assembleTransverseInteriorNotch(ctx context.Context, d, l []Face, axis geom.Vec3, h float64, z [2]float64, physical [2]*big.Rat, forward bool, tol float64) (FiniteCylinderPairDifferenceResult, error) {
	var zero FiniteCylinderPairDifferenceResult
	faces := make([]Face, 0, 16)
	for _, v := range []Interval{{0, z[0]}, {z[0], z[1]}, {z[1], h}} {
		f := d[0]
		f.V = v
		faces = append(faces, f)
	}
	cut := d[1]
	cut.V = Interval{z[0], z[1]}
	faces = append(faces, cut, d[2], d[3])
	for _, v := range []Interval{{0, z[0]}, {z[1], h}} {
		f := l[0]
		f.V = v
		faces = append(faces, f)
	}
	// Below the cut the exposed floor points forward; above it the ceiling
	// points backward. Each original cutter cap is represented by two lens fans.
	for end, ids := range [2][2]int{{2, 5}, {1, 4}} {
		for _, id := range ids {
			f := l[id]
			fan, ok := f.Surface.(arcFan)
			if !ok {
				return zero, geom.ErrDomain
			}
			for k, v := range []float64{axis.X, axis.Y, axis.Z} {
				if v != 0 {
					value, e := geom.RepresentedExactScalar(new(big.Rat).Mul(physical[end], new(big.Rat).SetFloat64(v)))
					if e != nil {
						return zero, e
					}
					*[3]*float64{&fan.center.X, &fan.center.Y, &fan.center.Z}[k] = value
				}
			}
			fan.height = z[end]
			f.Surface = fan
			faces = append(faces, f)
		}
	}
	faces = append(faces, l[1], l[4], l[2], l[5])
	solid, e := joinPrimitivePatches(faces, tol)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	caps := []int{8, 9, 10, 11}
	if !forward {
		caps = []int{10, 11, 8, 9}
	}
	return FiniteCylinderPairDifferenceResult{Solids: [2]Solid{solid, {}}, StockWallFaces: [2][]int{{0, 1, 2, 6, 7}}, CutterWallFaces: [2][]int{{3}}, StockCapFaces: [2][]int{{4, 5, 12, 13, 14, 15}}, CutterCapFaces: [2][]int{caps}}, nil
}
