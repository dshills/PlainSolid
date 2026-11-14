package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// BoundCentroid returns the world centroid and world first moments about
// the origin. The proper rational pose carries exact local measure intervals
// through rotation and translation. Width applies to local flux; rotated
// coordinate and moment widths also depend on the placement.
func (o AlgebraicGraphStockOccurrence) BoundCentroid(ctx context.Context, width *big.Rat, options RationalGraphShellSectionOptions,
	roots geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockCentroid, error) {
	var zero AlgebraicGraphStockCentroid
	matrix, e := o.pose.Matrix()
	if e != nil {
		return zero, e
	}
	out, e := o.pair.BoundVerticalCentroid(ctx, width, options, roots)
	if e != nil {
		return zero, e
	}
	local, coordinates := out.FirstMoments, out.Coordinates
	bits := roots.MaxCoefficientBits
	if bits == 0 {
		bits = 32768
	}
	for i := 0; i < 3; i++ {
		moment := geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
		for j, s := range [4]geom.ExactParameterInterval{local[0], local[1], local[2], out.Volume} {
			a := matrix[i][j]
			lo, hi := s.Lower, s.Upper
			if a.Sign() < 0 {
				lo, hi = hi, lo
			}
			moment.Lower.Add(moment.Lower, new(big.Rat).Mul(a, lo))
			moment.Upper.Add(moment.Upper, new(big.Rat).Mul(a, hi))
			if !graphStockMomentFits(moment, bits) {
				return zero, geom.ErrConvergence
			}
		}
		// Transform coordinate boxes directly. Dividing transformed
		// moments would also be valid but mixes correlated volume terms.
		world := geom.ExactParameterInterval{Lower: new(big.Rat).Set(matrix[i][3]), Upper: new(big.Rat).Set(matrix[i][3])}
		for j, s := range coordinates {
			a := matrix[i][j]
			lo, hi := s.Lower, s.Upper
			if a.Sign() < 0 {
				lo, hi = hi, lo
			}
			world.Lower.Add(world.Lower, new(big.Rat).Mul(a, lo))
			world.Upper.Add(world.Upper, new(big.Rat).Mul(a, hi))
			if !graphStockMomentFits(world, bits) {
				return zero, geom.ErrConvergence
			}
		}
		out.FirstMoments[i] = moment
		out.Coordinates[i] = world
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
