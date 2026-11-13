package geom

import (
	"context"
	"math/big"
)

// SplineGraphParallelObliqueLines certifies exactly two distinct full oblique
// contact segments. Coefficients[i] encode original-UV C+U*u+V*v. Regions are
// ordered by increasing common normalized projection u+slope*v. Adjacent closed
// polygons share a contact segment; their interiors have strict height Side.
type SplineGraphParallelObliqueLines struct {
	Proven                                          bool
	Domain                                          ParameterRectangle
	Coefficients                                    [2][3]*big.Rat
	Terminals                                       [2][2][2]*big.Rat
	Regions                                         [3]GraphLineRegion
	Graph                                           SurfaceInjectivity
	CandidatesTested, ExactOperations, Subdivisions int
}

// DiscoverSplineGraphParallelObliqueLines recovers exact rational boundary
// proposals, then divides every union-knot cell by both common affine factors.
// A uniformly strict Bernstein quotient proves exhaustive contact and side
// signs. Unsupported multiplicities, further contacts and irrational proposals
// remain unresolved. Candidate hints are unused; MaxCandidates bounds both
// distinct proposed lines and tested line pairs.
func DiscoverSplineGraphParallelObliqueLines(ctx context.Context, a, c *SurfacePatch, o GraphFactorAxisLineOptions) (r SplineGraphParallelObliqueLines, err error) {
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

	type line struct{ c, v *big.Rat }
	var lines []line
	lineSeen := map[string]bool{}
	for i, x := range points {
		for _, y := range points[i+1:] {
			u := new(big.Rat).Sub(y[1], x[1])
			v := new(big.Rat).Sub(x[0], y[0])
			if u.Sign() == 0 || v.Sign() == 0 {
				continue
			}
			v.Quo(v, u)
			constant := new(big.Rat).Neg(new(big.Rat).Add(x[0], new(big.Rat).Mul(v, x[1])))
			key := constant.RatString() + "," + v.RatString()
			if lineSeen[key] {
				continue
			}
			if len(lines) >= o.MaxCandidates {
				return r, ErrConvergence
			}
			lineSeen[key] = true
			lines = append(lines, line{constant, v})
		}
	}
	for i, left := range lines {
		for _, right := range lines[i+1:] {
			if left.v.Cmp(right.v) != 0 || left.c.Cmp(right.c) == 0 {
				continue
			}
			if r.CandidatesTested >= o.MaxCandidates {
				return r, ErrConvergence
			}
			r.CandidatesTested++
			constants := [2]*big.Rat{left.c, right.c}
			if constants[0].Cmp(constants[1]) < 0 {
				constants[0], constants[1] = constants[1], constants[0]
			}
			sign := 0
			proved := true
			for _, cell := range cells {
				origin, width := normalizedCell(cell)
				q := cell
				for _, constant := range constants {
					localC := new(big.Rat).Add(constant, new(big.Rat).Add(origin[0], new(big.Rat).Mul(left.v, origin[1])))
					var e error
					q, e = divideObliqueGraphBernstein(ctx, q, localC, width[0], new(big.Rat).Mul(left.v, width[1]), b)
					if e != nil {
						return r, e
					}
					if q == nil {
						proved = false
						break
					}
				}
				if !proved {
					break
				}
				for _, h := range q.net {
					current := h[2].Sign()
					if current == 0 || (sign != 0 && sign != current) {
						proved = false
						break
					}
					sign = current
				}
				if !proved {
					break
				}
			}
			if !proved || sign == 0 {
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
			for k, constant := range constants {
				r.Coefficients[k][1] = new(big.Rat).Inv(width[0])
				r.Coefficients[k][2] = new(big.Rat).Quo(left.v, width[1])
				r.Coefficients[k][0] = new(big.Rat).Sub(constant, new(big.Rat).Add(new(big.Rat).Mul(r.Coefficients[k][1], origin[0]), new(big.Rat).Mul(r.Coefficients[k][2], origin[1])))
				var hits [][2]*big.Rat
				for _, point := range points {
					value := new(big.Rat).Add(constant, new(big.Rat).Add(point[0], new(big.Rat).Mul(left.v, point[1])))
					if value.Sign() == 0 {
						hits = append(hits, point)
					}
				}
				if len(hits) != 2 {
					return r, ErrConvergence
				}
				for j, point := range hits {
					r.Terminals[k][j] = physical(point)
				}
			}
			square := [][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(1, 1)}}
			for band := 0; band < 3; band++ {
				polygon := square
				for k, constant := range constants {
					side := -1
					if band > k {
						side = 1
					}
					polygon = clipGraphLinePolygon(polygon, constant, big.NewRat(1, 1), left.v, side)
				}
				if len(polygon) < 3 {
					return r, ErrConvergence
				}
				r.Regions[band].Side = sign
				if band == 1 {
					r.Regions[band].Side = -sign
				}
				for _, point := range polygon {
					uv := physical(point)
					for _, value := range uv {
						if e := b.check(value); e != nil {
							return r, e
						}
					}
					r.Regions[band].Vertices = append(r.Regions[band].Vertices, uv)
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
func clipGraphLinePolygon(polygon [][2]*big.Rat, c, u, v *big.Rat, side int) [][2]*big.Rat {
	var out [][2]*big.Rat
	value := func(p [2]*big.Rat) *big.Rat {
		return new(big.Rat).Add(c, new(big.Rat).Add(new(big.Rat).Mul(u, p[0]), new(big.Rat).Mul(v, p[1])))
	}
	for i, p := range polygon {
		q := polygon[(i+1)%len(polygon)]
		fp, fq := value(p), value(q)
		if fp.Sign()*side >= 0 {
			out = append(out, p)
		}
		if fp.Sign()*fq.Sign() < 0 {
			t := new(big.Rat).Quo(fp, new(big.Rat).Sub(fp, fq))
			point := [2]*big.Rat{}
			for axis := 0; axis < 2; axis++ {
				point[axis] = new(big.Rat).Add(p[axis], new(big.Rat).Mul(t, new(big.Rat).Sub(q[axis], p[axis])))
			}
			out = append(out, point)
		}
	}
	return out
}
