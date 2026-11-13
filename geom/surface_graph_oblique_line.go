package geom

import (
	"context"
	"math/big"
)

// GraphLineRegion is the closure of one strict-side open parameter region.
// Vertices follow the parameter rectangle's counterclockwise orientation.
type GraphLineRegion struct {
	Side     int
	Vertices [][2]*big.Rat
}

// SplineGraphObliqueLine proves exactly one straight oblique contact segment.
// Coefficients encode C+U*u+V*v in original UV units; PositiveSide is the height
// sign where that expression is positive. TerminalMasks use UMin,UMax,VMin,VMax
// bits 1,2,4,8. Regions have disjoint interiors and include their zero boundary.
type SplineGraphObliqueLine struct {
	Proven                                          bool
	Coefficients                                    [3]*big.Rat
	PositiveSide                                    int
	Terminals                                       [2][2]*big.Rat
	TerminalMasks                                   [2]uint8
	Regions                                         [2]GraphLineRegion
	Domain                                          ParameterRectangle
	Graph                                           SurfaceInjectivity
	CandidatesTested, ExactOperations, Subdivisions int
}

// DiscoverSplineGraphObliqueLine supports a union of knot-aligned Bézier cells.
// The exact affine factor is shared; the strictly signed quotient may differ
// on every cell. Only original-domain boundary segments supply proposals.
// Rational boundary roots supply proposals only. Exact polynomial division and
// strict Bernstein quotient sign prove that no additional contact exists.
// MaxCandidates bounds boundary proposals and pairs; candidate hints are unused.
func DiscoverSplineGraphObliqueLine(ctx context.Context, a, c *SurfacePatch, o GraphFactorAxisLineOptions) (r SplineGraphObliqueLine, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxRootSubdivisions == 0 {
		o.MaxRootSubdivisions = 8192
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 128
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 || o.MaxRootSubdivisions < 1 || o.MaxCandidates < 1 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	p, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, p, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	if len(cells) == 0 {
		return r, ErrConvergence
	}
	r.Domain = p.Domain()
	globalBounds := [2][2]float64{{r.Domain.UMin, r.Domain.UMax}, {r.Domain.VMin, r.Domain.VMax}}
	normalizedCell := func(cell *SurfacePatch) (origin, width [2]*big.Rat) {
		d := cell.Domain()
		bounds := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
		for axis := 0; axis < 2; axis++ {
			base := new(big.Rat).SetFloat64(globalBounds[axis][0])
			span := new(big.Rat).Sub(new(big.Rat).SetFloat64(globalBounds[axis][1]), base)
			origin[axis] = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(bounds[axis][0]), base), span)
			width[axis] = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(bounds[axis][1]), new(big.Rat).SetFloat64(bounds[axis][0])), span)
		}
		return
	}
	var points [][2]*big.Rat
	seen := map[string]bool{}
	for _, p := range cells {
		cellDomain := p.Domain()
		outer := [4]bool{cellDomain.UMin == r.Domain.UMin, cellDomain.UMax == r.Domain.UMax, cellDomain.VMin == r.Domain.VMin, cellDomain.VMax == r.Domain.VMax}
		cellOrigin, cellWidth := normalizedCell(p)
		for side := 0; side < 4; side++ {
			if !outer[side] {
				continue
			}
			n := p.countV
			if side >= 2 {
				n = p.countU
			}
			values := make([]*big.Rat, n)
			for i := range values {
				index := i
				if side == 1 {
					index = (p.countU-1)*p.countV + i
				}
				if side == 2 {
					index = i * p.countV
				}
				if side == 3 {
					index = i*p.countV + p.countV - 1
				}
				values[i] = p.net[index][2]
			}
			poly, e := bernsteinToPower(values, b)
			if e != nil {
				return r, e
			}
			if len(poly) == 0 {
				return r, ErrConvergence
			}
			remaining := o.MaxRootSubdivisions - r.Subdivisions
			if remaining < 1 {
				return r, ErrConvergence
			}
			steps, e := isolatePolynomialRoots(poly, big.NewRat(0, 1), big.NewRat(1, 1), b, remaining, func(lo, hi *big.Rat, _ int) (bool, error) {
				if e := ctx.Err(); e != nil {
					return false, e
				}
				root, e := recoverRationalPolynomialRoot(poly, lo, hi, b)
				if e != nil {
					return false, e
				}
				if root == nil {
					return new(big.Rat).Sub(hi, lo).Cmp(big.NewRat(1, 1000000000000)) <= 0, nil
				}
				uv := [2]*big.Rat{new(big.Rat), new(big.Rat)}
				if side < 2 {
					uv[0].SetInt64(int64(side))
					uv[1].Set(root)
				} else {
					uv[0].Set(root)
					uv[1].SetInt64(int64(side - 2))
				}
				for axis := 0; axis < 2; axis++ {
					uv[axis].Add(cellOrigin[axis], new(big.Rat).Mul(cellWidth[axis], uv[axis]))
				}
				key := uv[0].RatString() + "," + uv[1].RatString()
				if !seen[key] {
					if len(points) >= o.MaxCandidates {
						return false, ErrConvergence
					}
					seen[key] = true
					points = append(points, uv)
				}
				return true, nil
			})
			r.Subdivisions += steps
			if e != nil {
				return r, e
			}
		}
	}
	for i, x := range points {
		for _, y := range points[i+1:] {
			if r.CandidatesTested >= o.MaxCandidates {
				return r, ErrConvergence
			}
			r.CandidatesTested++
			u := new(big.Rat).Sub(y[1], x[1])
			v := new(big.Rat).Sub(x[0], y[0])
			if u.Sign() == 0 || v.Sign() == 0 {
				continue
			}
			constant := new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(u, x[0]), new(big.Rat).Mul(v, x[1])))
			sign := 0
			for _, cell := range cells {
				origin, width := normalizedCell(cell)
				localC := new(big.Rat).Add(constant, new(big.Rat).Add(new(big.Rat).Mul(u, origin[0]), new(big.Rat).Mul(v, origin[1])))
				localU, localV := new(big.Rat).Mul(u, width[0]), new(big.Rat).Mul(v, width[1])
				current, e := obliqueGraphQuotientSign(ctx, cell, localC, localU, localV, b)
				if e != nil {
					return r, e
				}
				if current == 0 || (sign != 0 && sign != current) {
					sign = 0
					break
				}
				sign = current
			}
			if sign == 0 {
				continue
			}
			remaining := o.MaxOperations - b.operations
			if remaining < 1 {
				return r, ErrConvergence
			}
			r.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
			b.operations += r.Graph.ExactOperations
			if e != nil {
				return r, e
			}
			if !r.Graph.ProvenInjective {
				return r, ErrConvergence
			}
			origin := [2]*big.Rat{new(big.Rat).SetFloat64(r.Domain.UMin), new(big.Rat).SetFloat64(r.Domain.VMin)}
			width := [2]*big.Rat{new(big.Rat).Sub(new(big.Rat).SetFloat64(r.Domain.UMax), origin[0]), new(big.Rat).Sub(new(big.Rat).SetFloat64(r.Domain.VMax), origin[1])}
			physical := func(uv [2]*big.Rat) [2]*big.Rat {
				return [2]*big.Rat{new(big.Rat).Add(origin[0], new(big.Rat).Mul(width[0], uv[0])), new(big.Rat).Add(origin[1], new(big.Rat).Mul(width[1], uv[1]))}
			}
			r.Coefficients[1] = new(big.Rat).Quo(u, width[0])
			r.Coefficients[2] = new(big.Rat).Quo(v, width[1])
			r.Coefficients[0] = new(big.Rat).Sub(constant, new(big.Rat).Add(new(big.Rat).Mul(r.Coefficients[1], origin[0]), new(big.Rat).Mul(r.Coefficients[2], origin[1])))
			r.PositiveSide = sign
			for k, uv := range [][2]*big.Rat{x, y} {
				r.Terminals[k] = physical(uv)
				for axis := 0; axis < 2; axis++ {
					if uv[axis].Sign() == 0 {
						r.TerminalMasks[k] |= 1 << uint(2*axis)
					}
					if uv[axis].Cmp(big.NewRat(1, 1)) == 0 {
						r.TerminalMasks[k] |= 2 << uint(2*axis)
					}
				}
			}
			value := func(uv [2]*big.Rat) *big.Rat {
				return new(big.Rat).Add(constant, new(big.Rat).Add(new(big.Rat).Mul(u, uv[0]), new(big.Rat).Mul(v, uv[1])))
			}
			square := [][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(1, 1)}}
			for region, side := range []int{-1, 1} {
				r.Regions[region].Side = side * sign
				var polygon [][2]*big.Rat
				for k, a := range square {
					z := square[(k+1)%4]
					fa, fz := value(a), value(z)
					if fa.Sign()*side >= 0 {
						polygon = append(polygon, a)
					}
					if fa.Sign()*fz.Sign() < 0 {
						t := new(big.Rat).Quo(fa, new(big.Rat).Sub(fa, fz))
						uv := [2]*big.Rat{}
						for axis := 0; axis < 2; axis++ {
							uv[axis] = new(big.Rat).Add(a[axis], new(big.Rat).Mul(t, new(big.Rat).Sub(z[axis], a[axis])))
						}
						polygon = append(polygon, uv)
					}
				}
				if len(polygon) < 3 {
					return r, ErrConvergence
				}
				for _, uv := range polygon {
					r.Regions[region].Vertices = append(r.Regions[region].Vertices, physical(uv))
				}
			}

			for _, value := range r.Coefficients {
				if e := b.check(value); e != nil {
					return r, e
				}
			}
			for _, region := range r.Regions {
				for _, uv := range region.Vertices {
					for _, value := range uv {
						if e := b.check(value); e != nil {
							return r, e
						}
					}
				}
			}
			if e := ctx.Err(); e != nil {
				return r, e
			}
			r.Proven = true
			return r, nil
		}
	}
	return r, ErrConvergence
}

func obliqueGraphQuotientSign(ctx context.Context, p *SurfacePatch, c, a, v *big.Rat, b *polynomialBudget) (int, error) {
	q, e := divideObliqueGraphBernstein(ctx, p, c, a, v, b)
	if e != nil || q == nil {
		return 0, e
	}
	sign := 0
	for _, h := range q.net {
		s := h[2].Sign()
		if s == 0 || (sign != 0 && sign != s) {
			return 0, nil
		}
		sign = s
	}
	return sign, nil
}

func divideObliqueGraphBernstein(ctx context.Context, p *SurfacePatch, c, a, v *big.Rat, b *polynomialBudget) (*SurfacePatch, error) {
	du, dv := p.countU-1, p.countV-1
	if du < 1 || dv < 1 {
		return nil, nil
	}
	maxV := du + dv + 1
	rem := make([][]*big.Rat, du+1)
	q := make([][]*big.Rat, du)
	for i := range rem {
		rem[i] = make([]*big.Rat, maxV)
		for j := range rem[i] {
			rem[i][j] = new(big.Rat)
		}
		if i < du {
			q[i] = make([]*big.Rat, maxV)
			for j := range q[i] {
				q[i][j] = new(big.Rat)
			}
		}
	}
	rows := make([]rationalPolynomial, du+1)
	for i := range rows {
		values := make([]*big.Rat, dv+1)
		for j := range values {
			values[j] = p.net[i*p.countV+j][2]
		}
		var e error
		rows[i], e = bernsteinToPower(values, b)
		if e != nil {
			return nil, e
		}
	}
	for j := 0; j <= dv; j++ {
		values := make([]*big.Rat, du+1)
		for i := range values {
			values[i] = new(big.Rat)
			if j < len(rows[i]) {
				values[i].Set(rows[i][j])
			}
		}
		poly, e := bernsteinToPower(values, b)
		if e != nil {
			return nil, e
		}
		for i, x := range poly {
			rem[i][j].Set(x)
		}
	}
	for i := du; i >= 1; i-- {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for j := 0; j < maxV; j++ {
			if rem[i][j].Sign() == 0 {
				continue
			}
			x := new(big.Rat).Quo(rem[i][j], a)
			q[i-1][j].Set(x)
			rem[i][j].SetInt64(0)
			rem[i-1][j].Sub(rem[i-1][j], new(big.Rat).Mul(c, x))
			if j+1 == maxV {
				return nil, nil
			}
			rem[i-1][j+1].Sub(rem[i-1][j+1], new(big.Rat).Mul(v, x))
			for _, value := range []*big.Rat{x, rem[i-1][j], rem[i-1][j+1]} {
				if e := b.check(value); e != nil {
					return nil, e
				}
			}
		}
	}
	for _, x := range rem[0] {
		if x.Sign() != 0 {
			return nil, nil
		}
	}
	for i := range q {
		for j := dv; j < maxV; j++ {
			if q[i][j].Sign() != 0 {
				return nil, nil
			}
		}
	}
	result := *p
	result.degreeU--
	result.degreeV--
	result.countU--
	result.countV--
	result.knotsU = append([]float64(nil), p.knotsU[1:len(p.knotsU)-1]...)
	result.knotsV = append([]float64(nil), p.knotsV[1:len(p.knotsV)-1]...)
	result.net = make([]exactHomogeneous, result.countU*result.countV)
	for i := 0; i < du; i++ {
		for j := 0; j < dv; j++ {
			coefficient := new(big.Rat)
			for k := 0; k <= i; k++ {
				fu := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(int64(du-1), int64(k)))
				for l := 0; l <= j; l++ {
					fv := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(j), int64(l)), new(big.Int).Binomial(int64(dv-1), int64(l)))
					coefficient.Add(coefficient, new(big.Rat).Mul(q[k][l], new(big.Rat).Mul(fu, fv)))
					if e := b.check(coefficient); e != nil {
						return nil, e
					}
				}
			}
			result.net[i*result.countV+j][2] = coefficient
		}
	}
	return &result, nil
}
