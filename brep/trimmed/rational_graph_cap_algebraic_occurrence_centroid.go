//go:build research

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

// BoundMass encloses the world first and second moments about the origin
// and the rotated world inertia about the centroid. The proper rational
// placement preserves volume; translation affects raw moments but cancels
// from the centroid inertia.
func (o AlgebraicGraphStockOccurrence) BoundMass(ctx context.Context, width *big.Rat, options RationalGraphShellSectionOptions,
	roots geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockMass, error) {
	var zero AlgebraicGraphStockMass
	matrix, e := o.pose.Matrix()
	if e != nil {
		return zero, e
	}
	m, e := o.pair.BoundVerticalMass(ctx, width, options, roots)
	if e != nil {
		return zero, e
	}
	bits := roots.MaxCoefficientBits
	if bits == 0 {
		bits = 32768
	}
	type interval = geom.ExactParameterInterval
	newInterval := func() interval { return interval{Lower: new(big.Rat), Upper: new(big.Rat)} }
	add := func(out interval, a *big.Rat, q interval) bool {
		lo, hi := q.Lower, q.Upper
		if a.Sign() < 0 {
			lo, hi = hi, lo
		}
		out.Lower.Add(out.Lower, new(big.Rat).Mul(a, lo))
		out.Upper.Add(out.Upper, new(big.Rat).Mul(a, hi))
		return graphStockMomentFits(out, bits)
	}
	pairs := [6][2]int{{0, 0}, {1, 1}, {2, 2}, {0, 1}, {0, 2}, {1, 2}}
	raw, inertia := m.SecondMoments, m.CentroidInertia
	first, coord := m.FirstMoments, m.Coordinates
	for index, ij := range pairs {
		i, j := ij[0], ij[1]
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		m2, g := newInterval(), newInterval()
		for k, ab := range pairs {
			a, b := ab[0], ab[1]
			weight := new(big.Rat).Mul(matrix[i][a], matrix[j][b])
			if a != b {
				weight.Add(weight, new(big.Rat).Mul(matrix[i][b], matrix[j][a]))
			}
			if !add(m2, weight, raw[k]) || !add(g, weight, inertia[a][b]) {
				return zero, geom.ErrConvergence
			}
		}
		for k := 0; k < 3; k++ {
			weight := new(big.Rat).Add(new(big.Rat).Mul(matrix[i][3], matrix[j][k]), new(big.Rat).Mul(matrix[j][3], matrix[i][k]))
			if !add(m2, weight, first[k]) {
				return zero, geom.ErrConvergence
			}
		}
		if !add(m2, new(big.Rat).Mul(matrix[i][3], matrix[j][3]), m.Volume) {
			return zero, geom.ErrConvergence
		}
		if i == j {
			if m2.Upper.Sign() < 0 || g.Upper.Sign() < 0 {
				return zero, geom.ErrConvergence
			}
			if m2.Lower.Sign() < 0 {
				m2.Lower.SetInt64(0)
			}
			if g.Lower.Sign() < 0 {
				g.Lower.SetInt64(0)
			}
		}
		m.SecondMoments[index] = m2
		m.CentroidInertia[i][j] = g
		m.CentroidInertia[j][i] = interval{Lower: new(big.Rat).Set(g.Lower), Upper: new(big.Rat).Set(g.Upper)}
	}
	for i := 0; i < 3; i++ {
		m1, world := newInterval(), newInterval()
		world.Lower.Set(matrix[i][3])
		world.Upper.Set(matrix[i][3])
		for j := 0; j < 3; j++ {
			if !add(m1, matrix[i][j], first[j]) || !add(world, matrix[i][j], coord[j]) {
				return zero, geom.ErrConvergence
			}
		}
		if !add(m1, matrix[i][3], m.Volume) {
			return zero, geom.ErrConvergence
		}
		m.FirstMoments[i], m.Coordinates[i] = m1, world
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return m, nil
}
