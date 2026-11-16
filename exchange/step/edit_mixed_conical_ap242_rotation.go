package step

import (
	"math/big"

	rigidplacement "plainsolid/brep/placement"
	"plainsolid/geom"
)

func setMixedConicalOccurrenceRotation(pose *mappedRationalBasis, rotation [3][3]*big.Rat) (bool, error) {
	var matrix [3][4]*big.Rat
	for i, row := range rotation {
		copy(matrix[i][:3], row[:])
		matrix[i][3] = new(big.Rat)
	}
	rigid, err := rigidplacement.NewRationalRigid(matrix, 256)
	if err != nil {
		return false, geom.ErrUnrepresentable
	}
	matrix, err = rigid.Matrix()
	if err != nil {
		return false, err
	}
	changed := false
	for j, column := range []*[3]*big.Rat{&pose.X, &pose.Y, &pose.Z} {
		for i := range column {
			q := matrix[i][j]
			changed = changed || q.Cmp(column[i]) != 0
			column[i] = q
		}
	}
	return changed, nil
}

// STEP stores direction ratios. Where a unit component has no terminating
// decimal, write the axis's equivalent exact integer ratios and let the
// strict reader recover the rational unit vector. Decimal axes retain their
// established encoding.
func mixedConicalDirectionRatios(axis [3]*big.Rat) [3]*big.Rat {
	var ratio [3]*big.Rat
	for _, q := range axis {
		if _, err := mappedTerminatingDecimal(q); err == nil {
			continue
		}
		scale := big.NewInt(1)
		for _, component := range axis {
			d := component.Denom()
			gcd := new(big.Int).GCD(nil, nil, scale, d)
			scale.Quo(scale, gcd).Mul(scale, d)
		}
		multiplier := new(big.Rat).SetInt(scale)
		for i, component := range axis {
			ratio[i] = new(big.Rat).Mul(component, multiplier)
		}
		return ratio
	}
	return axis
}

// Verify the emitted target using the same exact and ordinary projections as
// the specialized reader before staging the edited graph.
func mixedConicalRotationProjectsExactly(f *File, id int64, want mappedRationalBasis) bool {
	exact, err := mappedRationalFrame(f, id)
	if err != nil {
		return false
	}
	got := [4][3]*big.Rat{exact.X, exact.Y, exact.Z, exact.Origin}
	for j, axis := range [4][3]*big.Rat{want.X, want.Y, want.Z, want.Origin} {
		for i, q := range axis {
			if got[j][i].Cmp(q) != 0 {
				return false
			}
		}
	}
	frame, err := placement(f, id)
	return err == nil && mixedConicalFrameProjection(f, id, frame, exact)
}
