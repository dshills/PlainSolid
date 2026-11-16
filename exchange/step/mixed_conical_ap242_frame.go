package step

import (
	"math/big"

	"plainsolid/geom"
)

func projectMixedConicalFrame(pose mappedRationalBasis) (out geom.Affine) {
	for j, axis := range [4][3]*big.Rat{pose.X, pose.Y, pose.Z, pose.Origin} {
		for i, q := range axis {
			out[i][j], _ = q.Float64()
		}
	}
	return out
}

// The exact source frame has unit, orthogonal X and Z axes and Y = Z×X.
// If one column equals its own positive Cartesian axis, the other two axes
// lie in its perpendicular plane and form a proper rotation there. Its
// exact coefficients are authority for the specialized binary64 view even
// when the ordinary normalization rounds them differently. The ordinary
// and exact readings must still agree on the translation.
func mixedConicalFrameProjection(frame geom.Frame, exact mappedRationalBasis) bool {
	ordinary, projected := frameTransform(frame), projectMixedConicalFrame(exact)
	if ordinary == projected {
		return true
	}
	for i := range ordinary {
		if ordinary[i][3] != projected[i][3] {
			return false
		}
	}
	for j, axis := range [3][3]*big.Rat{exact.X, exact.Y, exact.Z} {
		if axis[(j+1)%3].Sign() == 0 && axis[(j+2)%3].Sign() == 0 && axis[j].Cmp(big.NewRat(1, 1)) == 0 {
			return true
		}
	}
	return false
}
