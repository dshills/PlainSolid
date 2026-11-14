package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphStockCentroid encloses the retained volume, three first
// moments about the model origin and the Cartesian centroid. Every bound is
// exact rational; the volume must be proved strictly positive.
type AlgebraicGraphStockCentroid struct {
	Volume            geom.ExactParameterInterval
	FirstMoments      [3]geom.ExactParameterInterval
	Coordinates       [3]geom.ExactParameterInterval
	ExactOperations   int
	ExactSubdivisions int
}

// AlgebraicGraphStockMass encloses raw second moments in order
// XX, YY, ZZ, XY, XZ, YZ, plus centroid and inertia about the centroid.
// Density is one. Inertia uses Cartesian axes through the centroid.
type AlgebraicGraphStockMass struct {
	AlgebraicGraphStockCentroid
	SecondMoments   [6]geom.ExactParameterInterval
	CentroidInertia [3][3]geom.ExactParameterInterval
}

// BoundVerticalMass extends BoundVerticalCentroid with all second moments
// and the tensor about the centroid. Width applies to the raw signed flux
// measures. Derived inertia accuracy also depends on size and placement.
func (c AlgebraicGraphCapPair) BoundVerticalMass(ctx context.Context, width *big.Rat, o RationalGraphShellSectionOptions,
	roots geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockMass, error) {
	return c.boundVerticalMass(ctx, width, o, roots, true)
}

// BoundVerticalCentroid integrates closed stock bounded by two graph caps
// and zero XY-Jacobian original walls. It asks for the given width on the
// individual flux measures. Coordinate widths also depend on volume and
// position; callers can tighten the input width if needed.
func (c AlgebraicGraphCapPair) BoundVerticalCentroid(ctx context.Context, width *big.Rat, o RationalGraphShellSectionOptions,
	roots geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockCentroid, error) {
	v, e := c.boundVerticalMass(ctx, width, o, roots, false)
	return v.AlgebraicGraphStockCentroid, e
}

func (c AlgebraicGraphCapPair) boundVerticalMass(ctx context.Context, width *big.Rat, o RationalGraphShellSectionOptions,
	roots geom.ExactCurvePlaneEventOptions, second bool) (AlgebraicGraphStockMass, error) {

	var zero AlgebraicGraphStockMass
	if ctx == nil || width == nil || width.Sign() <= 0 || roots.MaxOperations < 0 || roots.MaxSubdivisions < 0 || roots.MaxCoefficientBits < 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if roots.MaxOperations == 0 {
		roots.MaxOperations = 4000000
	}
	if roots.MaxSubdivisions == 0 {
		roots.MaxSubdivisions = 8192
	}
	if roots.MaxCoefficientBits == 0 {
		roots.MaxCoefficientBits = 32768
	}
	if roots.MaxCoefficientBits < 64 || max(width.Num().BitLen(), width.Denom().BitLen()) > roots.MaxCoefficientBits {
		return zero, geom.ErrInvalid
	}
	b := capClipBudget{ctx, roots.MaxOperations, roots.MaxSubdivisions, roots.MaxCoefficientBits}
	boundary, e := c.exactIndexedBoundary(o, &b)
	if e != nil {
		return zero, e
	}
	if e := c.proveZeroStockXYFlux(boundary, o, &b, true); e != nil {
		return zero, e
	}
	edges := 0
	for _, cap := range c.caps {
		for _, wire := range cap.wires {
			edges += len(wire)
		}
	}
	if edges < 1 {
		return zero, geom.ErrConvergence
	}
	count, maxDegree := 4, 2
	if second {
		count, maxDegree = 10, 3
	}
	sum := make([]geom.ExactParameterInterval, count)
	for i := range sum {
		sum[i] = geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
	}
	for _, cap := range c.caps {
		p := cap.regions.plane
		if p.dropped != 2 {
			return zero, geom.ErrUnrepresentable
		}
		axes := orientedRationalGraphProjection(p)
		// Divergence fields have only a vertical component. For density
		// 1, x, y, z their cap values are z, x*z, y*z, z²/2.
		h := [3]*big.Rat{p.offset, new(big.Rat).Neg(p.normal[axes[0]]), new(big.Rat).Neg(p.normal[axes[1]])}
		coeff := graphCapCentroidDensities(h, axes, second)
		monomials := (maxDegree + 1) * (maxDegree + 2) / 2
		type term struct {
			exponent   [2]int
			multiplier [10]*big.Rat
			width      *big.Rat
		}
		var terms []term
		for i := 0; i <= maxDegree; i++ {
			for j := 0; i+j <= maxDegree; j++ {
				key := [2]int{i, j}
				m := term{exponent: key}
				scale := new(big.Rat)
				for k := range coeff {
					m.multiplier[k] = coeff[k][key]
					if x := m.multiplier[k]; x != nil {
						scale.Add(scale, new(big.Rat).Abs(x))
					}
				}
				if scale.Sign() == 0 {
					continue
				}
				m.width = new(big.Rat).Quo(width, new(big.Rat).Mul(scale, big.NewRat(int64(monomials*edges), 1)))
				terms = append(terms, m)
			}
		}
		for _, wire := range cap.wires {
			for _, use := range wire {
				for _, m := range terms {
					g, e := use.Cell.BoundProjectedMonomialGreen(ctx, axes, m.exponent, m.width, b.limits())
					if e != nil {
						return zero, e
					}
					if e := b.charge(g.Operations, g.Subdivisions); e != nil {
						return zero, e
					}
					for i, x := range m.multiplier {
						if i >= len(sum) || x == nil || x.Sign() == 0 {
							continue
						}
						v := new(big.Rat).Set(x)
						if use.Reversed != (p.orientation < 0) {
							v.Neg(v)
						}
						lo, hi := g.Interval.Lower, g.Interval.Upper
						if v.Sign() < 0 {
							lo, hi = hi, lo
						}
						sum[i].Lower.Add(sum[i].Lower, new(big.Rat).Mul(v, lo))
						sum[i].Upper.Add(sum[i].Upper, new(big.Rat).Mul(v, hi))
						if !graphStockMomentFits(sum[i], b.bits) {
							return zero, geom.ErrConvergence
						}
					}
				}
			}
		}
	}
	for _, s := range sum {
		if s.Upper.Cmp(s.Lower) < 0 || new(big.Rat).Sub(s.Upper, s.Lower).Cmp(width) > 0 {
			return zero, geom.ErrConvergence
		}
	}
	if sum[0].Lower.Sign() <= 0 {
		return zero, geom.ErrConvergence
	}
	out := AlgebraicGraphStockMass{AlgebraicGraphStockCentroid: AlgebraicGraphStockCentroid{Volume: sum[0], FirstMoments: [3]geom.ExactParameterInterval{sum[1], sum[2], sum[3]},
		ExactOperations: roots.MaxOperations - b.work, ExactSubdivisions: roots.MaxSubdivisions - b.subdiv}}
	for i, s := range out.FirstMoments {
		den := sum[0]
		lo, hi := den.Upper, den.Lower
		if s.Lower.Sign() < 0 {
			lo = den.Lower
		}
		if s.Upper.Sign() < 0 {
			hi = den.Upper
		}
		out.Coordinates[i] = geom.ExactParameterInterval{Lower: new(big.Rat).Quo(s.Lower, lo), Upper: new(big.Rat).Quo(s.Upper, hi)}
		if !graphStockMomentFits(out.Coordinates[i], b.bits) {
			return zero, geom.ErrConvergence
		}
	}
	if second {
		copy(out.SecondMoments[:], sum[4:])
		out.CentroidInertia, e = graphStockCentroidInertia(out, b.bits)
		if e != nil {
			return zero, e
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

func graphCapCentroidDensities(h [3]*big.Rat, axes [2]int, second bool) (coeff [10]map[[2]int]*big.Rat) {
	for i := range coeff {
		coeff[i] = map[[2]int]*big.Rat{}
	}
	powers := [3][2]int{{0, 0}, {1, 0}, {0, 1}}
	for i, p := range powers {
		coeff[0][p] = new(big.Rat).Set(h[i])
		for j, axis := range axes {
			key := p
			key[j]++
			coeff[axis+1][key] = new(big.Rat).Set(h[i])
		}
		for j, q := range powers {
			key := [2]int{p[0] + q[0], p[1] + q[1]}
			if coeff[3][key] == nil {
				coeff[3][key] = new(big.Rat)
			}
			coeff[3][key].Add(coeff[3][key], new(big.Rat).Quo(new(big.Rat).Mul(h[i], h[j]), big.NewRat(2, 1)))
		}
	}
	if second {
		// Pure XY factors multiply z; one Z factor contributes z²/2,
		// and two Z factors contribute z³/3.
		positions := [2]int{}
		for i, a := range axes {
			positions[a] = i
		}
		for i, p := range powers {
			x, y, xy := p, p, p
			x[positions[0]] += 2
			y[positions[1]] += 2
			xy[0]++
			xy[1]++
			coeff[4][x] = new(big.Rat).Set(h[i])
			coeff[5][y] = new(big.Rat).Set(h[i])
			coeff[7][xy] = new(big.Rat).Set(h[i])
			for j, q := range powers {
				for k, v := range powers {
					target := [2]int{p[0] + q[0] + v[0], p[1] + q[1] + v[1]}
					if coeff[6][target] == nil {
						coeff[6][target] = new(big.Rat)
					}
					x := new(big.Rat).Mul(h[i], h[j])
					x.Mul(x, h[k]).Quo(x, big.NewRat(3, 1))
					coeff[6][target].Add(coeff[6][target], x)
				}
			}
		}
		for p, q := range coeff[3] {
			x, y := p, p
			x[positions[0]]++
			y[positions[1]]++
			coeff[8][x] = new(big.Rat).Set(q)
			coeff[9][y] = new(big.Rat).Set(q)
		}
	}

	return coeff
}

func graphStockMomentFits(s geom.ExactParameterInterval, bits int) bool {
	return max(s.Lower.Num().BitLen(), s.Lower.Denom().BitLen(), s.Upper.Num().BitLen(), s.Upper.Denom().BitLen()) <= bits
}
