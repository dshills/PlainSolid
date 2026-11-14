package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// BoundBoundaryArea returns the original certified boundary area and face
// owners. A proper rigid occurrence preserves each face area exactly.
func (q WrappedRationalStationOccurrence) BoundBoundaryArea(ctx context.Context, o WrappedRationalStationAreaOptions) (WrappedRationalStationAreaBounds, error) {
	if q.bits == 0 {
		return WrappedRationalStationAreaBounds{}, geom.ErrInvalid
	}
	return q.source.BoundWrappedRationalStationArea(ctx, o)
}

// WrappedRationalStationOccurrenceInertia contains the original inertia proof
// and its independent centroidal tensor enclosure in world coordinate axes.
// The intervals in World meet the requested width. Off-diagonal intervals
// include their signs. The original wall and cap IDs remain attached to Original.
type WrappedRationalStationOccurrenceInertia struct {
	Original            WrappedRationalStationInertiaBounds
	World               [3][3]WrappedStationMassInterval
	TransformOperations int
}

// BoundCentroidalInertia rotates a certified original tensor using R*I*R^T.
// It uses the exact coefficient sum of the rotation to choose the local width
// needed for the requested world width. Rigid translations leave the centroidal
// tensor invariant. Source and transform use separate work budgets.
func (q WrappedRationalStationOccurrence) BoundCentroidalInertia(ctx context.Context, o WrappedRationalStationInertiaOptions) (WrappedRationalStationOccurrenceInertia, error) {
	var empty WrappedRationalStationOccurrenceInertia
	if q.bits == 0 || ctx == nil || o.InertiaTolerance == nil || o.InertiaTolerance.Sign() <= 0 {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	pose, err := q.pose.Matrix()
	if err != nil {
		return empty, err
	}
	b := &stationMassBudget{ctx: ctx, limit: 2048, bits: q.bits}
	tolerance := new(big.Rat).Set(b.check(o.InertiaTolerance))
	var coefficients [3][3][3][3]*big.Rat
	maxGrowth := new(big.Rat)
	for i := range coefficients {
		for j := i; j < 3; j++ {
			growth := new(big.Rat)
			for k := 0; k < 3; k++ {
				for l := k; l < 3; l++ {
					c := b.mul(pose[i][k], pose[j][l])
					if k != l {
						c = b.add(c, b.mul(pose[i][l], pose[j][k]))
					}
					coefficients[i][j][k][l] = c
					growth = b.add(growth, new(big.Rat).Abs(c))
				}
			}
			if growth.Cmp(maxGrowth) > 0 {
				maxGrowth = growth
			}
		}
	}
	o.InertiaTolerance = b.div(tolerance, maxGrowth)
	if b.err != nil {
		return empty, b.err
	}
	original, err := q.source.BoundWrappedRationalStationInertia(ctx, o)
	if err != nil {
		return empty, err
	}
	result := WrappedRationalStationOccurrenceInertia{Original: original}
	for i := range result.World {
		for j := i; j < 3; j++ {
			entry := WrappedStationMassInterval{new(big.Rat), new(big.Rat)}
			for k := 0; k < 3; k++ {
				for l := k; l < 3; l++ {
					coefficient := coefficients[i][j][k][l]
					if coefficient.Sign() != 0 {
						entry = b.plus(entry, b.scaled(original.CentroidalInertia[k][l], coefficient))
					}
				}
			}
			if b.err != nil {
				return empty, b.err
			}
			if i == j && entry.Lower.Sign() < 0 {
				entry.Lower = new(big.Rat)
			}
			result.World[i][j] = entry
			if i != j {
				result.World[j][i] = WrappedStationMassInterval{new(big.Rat).Set(entry.Lower), new(big.Rat).Set(entry.Upper)}
			}
		}
	}
	result.TransformOperations = b.used
	return result, nil
}
