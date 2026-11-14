package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphPolynomialWindowOptions bounds exact integration of retained
// cells after ordinary refinement. Each cell must fit one original knot span
// per owner, with degree<=6, constant positive homogeneous denominator and a
// signed/scaled coordinate-permutation projection. Other supports fail closed.
// MaxPolynomialCells defaults to64 (maximum4096); CellOperations defaults to
// 65536 (maximum16M) and includes conservative coefficient-extraction charges.
// Each original knot vector is limited to256 entries for this optional path.
type AlgebraicGraphPolynomialWindowOptions struct {
	Refinement                         AlgebraicGraphWindowRefineOptions
	MaxPolynomialCells, CellOperations int
}
type AlgebraicGraphExactCellVolume struct {
	Leaf   int
	Volume *big.Rat
}
type AlgebraicGraphPolynomialWindowIntegral struct {
	Bounds                 AlgebraicGraphWindowAggregate
	ExactCells             []AlgebraicGraphExactCellVolume
	RetainedCellVolume     *big.Rat
	PolynomialQuotaCharged int
}

// IntegratePolynomialWindow replaces only proved retained cells' volume
// intervals with exact polynomial integrals. Unresolved rim cells retain their
// previous enclosures and explicit unresolved area. This does not classify
// unknown cells or integrate a rational denominator by polynomial approximation.
func (q *AlgebraicGraphMaterialQuery) IntegratePolynomialWindow(ctx context.Context, w AlgebraicGraphProjectionWindow, o AlgebraicGraphPolynomialWindowOptions) (AlgebraicGraphPolynomialWindowIntegral, error) {
	return q.integratePolynomialWindowPrepared(ctx, w, o, nil)
}

// prepared is private same-call evidence, never a caller-supplied partition.
func (q *AlgebraicGraphMaterialQuery) integratePolynomialWindowPrepared(ctx context.Context, w AlgebraicGraphProjectionWindow, o AlgebraicGraphPolynomialWindowOptions, prepared *AlgebraicGraphWindowAggregate) (out AlgebraicGraphPolynomialWindowIntegral, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphPolynomialWindowIntegral{PolynomialQuotaCharged: out.PolynomialQuotaCharged}
		}
	}()
	if o.MaxPolynomialCells == 0 {
		o.MaxPolynomialCells = 64
	}
	if o.CellOperations == 0 {
		o.CellOperations = 65536
	}
	if o.MaxPolynomialCells < 1 || o.MaxPolynomialCells > 4096 || o.CellOperations < 1 || o.CellOperations > 16000000 {
		return out, geom.ErrInvalid
	}
	exact, e := graphMaterialLimits(o.Refinement.CellOptions.Exact)
	if e != nil {
		return out, e
	}
	var base AlgebraicGraphWindowAggregate
	if prepared == nil {
		base, e = q.RefineWindowDepth(ctx, w, o.Refinement)
		if e != nil {
			return out, e
		}
	} else {
		if ctx == nil || q == nil || !prepared.HasBounds {
			return out, geom.ErrInvalid
		}
		if e := ctx.Err(); e != nil {
			return out, e
		}
		base = *prepared
		for i := 0; i < 2; i++ {
			base.Volume[i] = new(big.Rat).Set(prepared.Volume[i])
		}
	}
	out.Bounds = base
	out.RetainedCellVolume = new(big.Rat)
	for i, leaf := range base.Leaves {
		if !leaf.FullMaterial {
			continue
		}
		if len(out.ExactCells) >= o.MaxPolynomialCells || o.CellOperations > int(^uint(0)>>1)-out.PolynomialQuotaCharged {
			return out, geom.ErrConvergence
		}
		out.PolynomialQuotaCharged += o.CellOperations
		value, e := q.integrateGraphPolynomialCell(ctx, leaf, o.CellOperations, exact.MaxCoefficientBits)
		if e != nil {
			return out, e
		}
		area := new(big.Rat).Mul(new(big.Rat).Sub(leaf.Window.Max[0], leaf.Window.Min[0]), new(big.Rat).Sub(leaf.Window.Max[1], leaf.Window.Min[1]))
		old := [2]*big.Rat{new(big.Rat).Mul(area, leaf.Depth[0]), new(big.Rat).Mul(area, leaf.Depth[1])}
		if value.Sign() < 0 || value.Cmp(old[0]) < 0 || value.Cmp(old[1]) > 0 {
			return out, geom.ErrConvergence
		}
		for j := 0; j < 2; j++ {
			out.Bounds.Volume[j].Sub(out.Bounds.Volume[j], old[j])
			if !graphPolynomialBits(out.Bounds.Volume[j], exact.MaxCoefficientBits) {
				return out, geom.ErrConvergence
			}
			out.Bounds.Volume[j].Add(out.Bounds.Volume[j], value)
			if !graphPolynomialBits(out.Bounds.Volume[j], exact.MaxCoefficientBits) {
				return out, geom.ErrConvergence
			}
		}
		out.RetainedCellVolume.Add(out.RetainedCellVolume, value)
		if !graphPolynomialBits(out.RetainedCellVolume, exact.MaxCoefficientBits) {
			return out, geom.ErrConvergence
		}
		out.ExactCells = append(out.ExactCells, AlgebraicGraphExactCellVolume{Leaf: i, Volume: value})
	}
	return out, nil
}
func graphPolynomialBits(x *big.Rat, bits int) bool {
	return x != nil && x.Num().BitLen() <= bits && x.Denom().BitLen() <= bits
}
func (q *AlgebraicGraphMaterialQuery) integrateGraphPolynomialCell(ctx context.Context, leaf AlgebraicGraphWindowLeaf, limit, bits int) (*big.Rat, error) {
	r, e := q.integrateGraphPolynomialCellMode(ctx, leaf, limit, bits, 0)
	return r[0], e
}
func (q *AlgebraicGraphMaterialQuery) integrateGraphPolynomialCellMode(ctx context.Context, leaf AlgebraicGraphWindowLeaf, limit, bits int, mode int) ([10]*big.Rat, error) {
	firstMoments := mode > 0
	secondMoments := mode > 1
	reserve := 4096
	if firstMoments {
		reserve = 65536
	}
	if secondMoments {
		reserve = 262144
	}
	if limit < reserve+4096 {
		return [10]*big.Rat{}, geom.ErrConvergence
	}
	step := (limit - reserve) / 6
	var failure error
	operations := 0
	calc := func(op byte, a, b *big.Rat) *big.Rat {
		r := new(big.Rat)
		if failure != nil {
			return r
		}
		operations++
		if operations > reserve || !graphPolynomialBits(a, bits) || !graphPolynomialBits(b, bits) {
			failure = geom.ErrConvergence
			return r
		}
		switch op {
		case '+':
			r.Add(a, b)
		case '-':
			r.Sub(a, b)
		case '*':
			r.Mul(a, b)
		case '/':
			if b.Sign() == 0 {
				failure = geom.ErrConvergence
				return r
			}
			r.Quo(a, b)
		}
		if !graphPolynomialBits(r, bits) {
			failure = geom.ErrConvergence
		}
		return r
	}
	heights := [2]*big.Rat{}
	var chartMoments [2][3]*big.Rat
	var chartSecond [2][6]*big.Rat
	for chart, p := range q.patches {
		if e := ctx.Err(); e != nil {
			return [10]*big.Rat{}, e
		}
		inv, e := q.graphs[chart].InverseCoefficients()
		if e != nil {
			return [10]*big.Rat{}, e
		}
		var ranges [2][2]*big.Rat
		seen := [2]bool{}
		var columns [2]int
		jac := big.NewRat(1, 1)
		for axis, row := range inv {
			column := -1
			for i := 0; i < 2; i++ {
				if row[i].Sign() != 0 {
					if column >= 0 {
						return [10]*big.Rat{}, geom.ErrConvergence
					}
					column = i
				}
			}
			if column < 0 || seen[column] {
				return [10]*big.Rat{}, geom.ErrConvergence
			}
			seen[column] = true
			columns[axis] = column
			jac = calc('*', jac, row[column])
			ranges[axis] = [2]*big.Rat{calc('+', calc('*', row[column], leaf.Window.Min[column]), row[2]), calc('+', calc('*', row[column], leaf.Window.Max[column]), row[2])}
			if ranges[axis][0].Cmp(ranges[axis][1]) > 0 {
				ranges[axis][0], ranges[axis][1] = ranges[axis][1], ranges[axis][0]
			}
		}
		if failure != nil {
			return [10]*big.Rat{}, failure
		}
		jac.Abs(jac)
		u, v := p.KnotBreaks()
		if len(u) > 256 || len(v) > 256 {
			return [10]*big.Rat{}, geom.ErrConvergence
		}
		var span [2][2]float64
		for axis, knots := range [][]float64{u, v} {
			found := false
			for k := 0; k+1 < len(knots); k++ {
				if ranges[axis][0].Cmp(ratInset(knots[k])) >= 0 && ranges[axis][1].Cmp(ratInset(knots[k+1])) <= 0 {
					span[axis] = [2]float64{knots[k], knots[k+1]}
					found = true
					break
				}
			}
			if !found {
				return [10]*big.Rat{}, geom.ErrConvergence
			}
		}
		limited, e := p.WithLimits(geom.SurfacePatchOptions{MaxOperations: step, MaxCoefficientBits: bits})
		if e != nil {
			return [10]*big.Rat{}, e
		}
		patch, _, e := limited.TrimWithWorkLimit(geom.ParameterRectangle{UMin: span[0][0], UMax: span[0][1], VMin: span[1][0], VMax: span[1][1]}, step)
		if e != nil {
			return [10]*big.Rat{}, e
		}
		degreeLimit := 6
		if secondMoments {
			degreeLimit = 3
		}
		coeff, _, ok := patch.RationalBezierCoefficients(degreeLimit)
		if !ok {
			return [10]*big.Rat{}, geom.ErrConvergence
		}
		weight := coeff[0][0][3]
		if weight.Sign() <= 0 {
			return [10]*big.Rat{}, geom.ErrConvergence
		}
		for i, row := range coeff {
			for j, c := range row {
				if (i != 0 || j != 0) && c[3].Sign() != 0 {
					return [10]*big.Rat{}, geom.ErrConvergence
				}
			}
		}
		moments := [2][]*big.Rat{}
		scale := big.NewRat(1, 1)
		for axis, n := range []int{len(coeff), len(coeff[0])} {
			if secondMoments {
				n = 3*n - 2
			} else if firstMoments {
				n = 2*n - 1
			}
			width := calc('-', ratInset(span[axis][1]), ratInset(span[axis][0]))
			scale = calc('*', scale, width)
			lo := calc('/', calc('-', ranges[axis][0], ratInset(span[axis][0])), width)
			hi := calc('/', calc('-', ranges[axis][1], ratInset(span[axis][0])), width)
			a, b := big.NewRat(1, 1), big.NewRat(1, 1)
			for power := 1; power <= n; power++ {
				a = calc('*', a, lo)
				b = calc('*', b, hi)
				moments[axis] = append(moments[axis], calc('/', calc('-', b, a), big.NewRat(int64(power), 1)))
			}
		}
		sum := new(big.Rat)
		for i, row := range coeff {
			for j, c := range row {
				term := calc('*', calc('*', c[q.residual], moments[0][i]), moments[1][j])
				sum = calc('+', sum, term)
			}
		}
		heights[chart] = calc('/', calc('*', sum, scale), calc('*', weight, jac))
		if firstMoments {
			for uvAxis, column := range columns {
				offset := calc('/', calc('-', ratInset(span[uvAxis][0]), inv[uvAxis][2]), inv[uvAxis][column])
				slope := calc('/', calc('-', ratInset(span[uvAxis][1]), ratInset(span[uvAxis][0])), inv[uvAxis][column])
				weighted := new(big.Rat)
				for i, row := range coeff {
					for j, c := range row {
						if c[q.residual].Sign() == 0 {
							continue
						}
						a, b := moments[0][i], moments[1][j]
						if uvAxis == 0 {
							a = calc('+', calc('*', offset, a), calc('*', slope, moments[0][i+1]))
						} else {
							b = calc('+', calc('*', offset, b), calc('*', slope, moments[1][j+1]))
						}
						weighted = calc('+', weighted, calc('*', calc('*', c[q.residual], a), b))
					}
				}
				chartMoments[chart][q.axes[column]] = calc('/', calc('*', weighted, scale), calc('*', weight, jac))
			}
			squared := new(big.Rat)
			for i, row := range coeff {
				for j, a := range row {
					if a[q.residual].Sign() == 0 {
						continue
					}
					for k, other := range coeff {
						for l, b := range other {
							if b[q.residual].Sign() == 0 {
								continue
							}
							term := calc('*', calc('*', calc('*', a[q.residual], b[q.residual]), moments[0][i+k]), moments[1][j+l])
							squared = calc('+', squared, term)
						}
					}
				}
			}
			chartMoments[chart][q.residual] = calc('/', calc('*', squared, scale), calc('*', big.NewRat(2, 1), calc('*', calc('*', weight, weight), jac)))
		}

		if secondMoments {
			var offsets, slopes [2]*big.Rat
			var uvForColumn [2]int
			for axis, column := range columns {
				uvForColumn[column] = axis
				offsets[axis] = calc('/', calc('-', ratInset(span[axis][0]), inv[axis][2]), inv[axis][column])
				slopes[axis] = calc('/', calc('-', ratInset(span[axis][1]), ratInset(span[axis][0])), inv[axis][column])
			}
			weighted := func(axis, power, count int) *big.Rat {
				m := moments[axis]
				a, b := offsets[axis], slopes[axis]
				switch count {
				case 0:
					return m[power]
				case 1:
					return calc('+', calc('*', a, m[power]), calc('*', b, m[power+1]))
				default:
					return calc('+', calc('+', calc('*', calc('*', a, a), m[power]), calc('*', calc('*', big.NewRat(2, 1), calc('*', a, b)), m[power+1])), calc('*', calc('*', b, b), m[power+2]))
				}
			}
			for first := 0; first < 2; first++ {
				for second := first; second < 2; second++ {
					counts := [2]int{}
					counts[uvForColumn[first]]++
					counts[uvForColumn[second]]++
					sum := new(big.Rat)
					for i, row := range coeff {
						for j, c := range row {
							if c[q.residual].Sign() == 0 {
								continue
							}
							term := calc('*', calc('*', c[q.residual], weighted(0, i, counts[0])), weighted(1, j, counts[1]))
							sum = calc('+', sum, term)
						}
					}
					chartSecond[chart][graphSecondIndex(q.axes[first], q.axes[second])] = calc('/', calc('*', sum, scale), calc('*', weight, jac))
				}
			}
			squared := make([][]*big.Rat, 2*len(coeff)-1)
			for i := range squared {
				squared[i] = make([]*big.Rat, 2*len(coeff[0])-1)
				for j := range squared[i] {
					squared[i][j] = new(big.Rat)
				}
			}
			for i, row := range coeff {
				for j, a := range row {
					if a[q.residual].Sign() == 0 {
						continue
					}
					for k, other := range coeff {
						for l, b := range other {
							if b[q.residual].Sign() != 0 {
								squared[i+k][j+l] = calc('+', squared[i+k][j+l], calc('*', a[q.residual], b[q.residual]))
							}
						}
					}
				}
			}
			for column := 0; column < 2; column++ {
				counts := [2]int{}
				counts[uvForColumn[column]] = 1
				sum := new(big.Rat)
				for i, row := range squared {
					for j, c := range row {
						if c.Sign() != 0 {
							sum = calc('+', sum, calc('*', calc('*', c, weighted(0, i, counts[0])), weighted(1, j, counts[1])))
						}
					}
				}
				chartSecond[chart][graphSecondIndex(q.axes[column], q.residual)] = calc('/', calc('*', sum, scale), calc('*', big.NewRat(2, 1), calc('*', calc('*', weight, weight), jac)))
			}
			cubed := new(big.Rat)
			for i, row := range squared {
				for j, a := range row {
					if a.Sign() == 0 {
						continue
					}
					for k, other := range coeff {
						for l, b := range other {
							if b[q.residual].Sign() != 0 {
								cubed = calc('+', cubed, calc('*', calc('*', calc('*', a, b[q.residual]), moments[0][i+k]), moments[1][j+l]))
							}
						}
					}
				}
			}
			chartSecond[chart][graphSecondIndex(q.residual, q.residual)] = calc('/', calc('*', cubed, scale), calc('*', big.NewRat(3, 1), calc('*', calc('*', calc('*', weight, weight), weight), jac)))
		}
		if failure != nil {
			return [10]*big.Rat{}, failure
		}
	}
	upper := -1
	for _, r := range q.regions {
		if r.faces == leaf.Faces {
			upper = r.upper
			break
		}
	}
	if upper < 0 {
		return [10]*big.Rat{}, geom.ErrConvergence
	}
	value := calc('-', heights[upper], heights[upper^1])
	if failure != nil {
		return [10]*big.Rat{}, failure
	}
	result := [10]*big.Rat{value}
	if firstMoments {
		for axis := 0; axis < 3; axis++ {
			result[axis+1] = calc('-', chartMoments[upper][axis], chartMoments[upper^1][axis])
		}
	}
	if secondMoments {
		for i := 0; i < 6; i++ {
			result[i+4] = calc('-', chartSecond[upper][i], chartSecond[upper^1][i])
		}
	}
	if failure != nil {
		return [10]*big.Rat{}, failure
	}
	return result, nil
}

// Packed symmetric tensor order is XX, XY, XZ, YY, YZ, ZZ.
func graphSecondIndex(a, b int) int {
	if a > b {
		a, b = b, a
	}
	if a == 0 {
		return b
	}
	if a == 1 {
		return b + 2
	}
	return 5
}
