package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// mixedAxisPlaneProposal recognizes only exact coordinate planes. Its
// coefficient identity checks every term of N-coordinate*W; comparing
// represented corner heights alone would miss saddle and bowed patches.
func mixedAxisPlaneProposal(ctx context.Context, patch *geom.SurfacePatch, face int, budget *int) (MixedPlanarFragmentProposal, bool, error) {
	var zero MixedPlanarFragmentProposal
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}
	// The coefficient conversion is limited inside this fresh view.
	// Its fixed allowance covers all degree-four tensor coefficients
	// plus their subsequent exact plane comparisons.
	const work = 6400
	if *budget <= work {
		return zero, false, geom.ErrConvergence
	}
	*budget -= work
	patch, err := patch.WithLimits(geom.SurfacePatchOptions{MaxOperations: work - 256, MaxCoefficientBits: 4096})
	if err != nil {
		return zero, false, err
	}
	h, _, ok := patch.RationalBezierCoefficients(4)
	if !ok || len(h) == 0 || len(h[0]) == 0 || h[0][0][3].Sign() <= 0 {
		return zero, false, geom.ErrUnrepresentable
	}
	for axis := range 3 {
		c := new(big.Rat).Quo(h[0][0][axis], h[0][0][3])
		matches := true
		for _, row := range h {
			for _, entry := range row {
				if new(big.Rat).Mul(entry[3], c).Cmp(entry[axis]) != 0 {
					matches = false
					break
				}
			}
			if !matches {
				break
			}
		}
		if !matches {
			continue
		}
		v, exact := c.Float64()
		if !exact || !finite(v) {
			return zero, false, geom.ErrUnrepresentable
		}
		return MixedPlanarFragmentProposal{Face: face, Plane: mixedCoordinatePlane(axis, v),
			ProjectionAxes: [2]int{(axis + 1) % 3, (axis + 2) % 3}}, true, nil
	}
	return zero, false, nil
}
