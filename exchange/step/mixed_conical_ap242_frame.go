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
// when ordinary normalization rounds them differently. A spatial rotation
// can also use that exact projection when both original direction triples
// are primitive integers represented exactly by the ordinary parser. The
// ordinary and exact readings must still agree on the translation.
func mixedConicalFrameProjection(f *File, placementID int64, frame geom.Frame, exact mappedRationalBasis) bool {
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
	source, err := simple(f, placementID, "AXIS2_PLACEMENT_3D", 4)
	return err == nil && mixedConicalPrimitiveIntegerDirection(f, source.Args[2].Ref) &&
		mixedConicalPrimitiveIntegerDirection(f, source.Args[3].Ref)
}

// The exact frame reader has already proved each direction's positive norm
// and their orthogonality. Here the integer content proves a canonical
// source ray, and 53 bits ensure binary64 parses each original component
// exactly before its separate normalization.
func mixedConicalPrimitiveIntegerDirection(f *File, id int64) bool {
	axis, err := mappedRationalCoordinate(f, id, "DIRECTION", 3)
	if err != nil {
		return false
	}
	var content big.Int
	for _, component := range axis {
		if !component.IsInt() || component.Num().BitLen() > 53 {
			return false
		}
		content.GCD(nil, nil, &content, component.Num())
	}
	return content.IsInt64() && content.Int64() == 1
}
