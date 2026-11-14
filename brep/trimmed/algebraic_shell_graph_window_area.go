package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// AlgebraicGraphWindowAreaOptions leaves classification/refinement unchanged.
// Nonempty leaves must fit one original Bezier knot span per owner, degree<=6
// with positive weights. MaxSurfaceCells defaults to1024, maximum8192;
// CellOperations defaults to65536, maximum16M, per support/leaf. RootBits
// defaults to64 (range8..256) for outward rational square-root enclosures.
type AlgebraicGraphWindowAreaOptions struct {
	Refinement                                AlgebraicGraphWindowRefineOptions
	MaxSurfaceCells, CellOperations, RootBits int
}
type AlgebraicGraphOwnerArea struct {
	Operand     int
	NativeFaces []int
	Area        [2]*big.Rat
}

// AlgebraicGraphWindowArea encloses only retained boundary over the window,
// independently for the two original graph supports. NativeFaces lists proved or possible
// retained fragments above the window; unresolved cells conservatively include
// all selected fragments, never discarded original faces. Unknown rim cells add
// only upper area; proved hole/empty cells add none. Projection areas and native
// ownership remain available in Partition. No orientation-dependent sign is used.
type AlgebraicGraphWindowArea struct {
	Partition                         AlgebraicGraphWindowAggregate
	Owners                            [2]AlgebraicGraphOwnerArea
	Total                             [2]*big.Rat
	SurfaceCells, SurfaceQuotaCharged int
}

func (q *AlgebraicGraphMaterialQuery) BoundWindowSurfaceArea(ctx context.Context, w AlgebraicGraphProjectionWindow, o AlgebraicGraphWindowAreaOptions) (out AlgebraicGraphWindowArea, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphWindowArea{SurfaceCells: out.SurfaceCells, SurfaceQuotaCharged: out.SurfaceQuotaCharged}
		}
	}()
	if o.MaxSurfaceCells == 0 {
		o.MaxSurfaceCells = 1024
	}
	if o.CellOperations == 0 {
		o.CellOperations = 65536
	}
	if o.RootBits == 0 {
		o.RootBits = 64
	}
	if o.MaxSurfaceCells < 1 || o.MaxSurfaceCells > 8192 || o.CellOperations < 1 || o.CellOperations > 16000000 || o.RootBits < 8 || o.RootBits > 256 {
		return out, geom.ErrInvalid
	}
	exact, e := graphMaterialLimits(o.Refinement.CellOptions.Exact)
	if e != nil {
		return out, e
	}
	check := func(x *big.Rat) error {
		if !graphPolynomialBits(x, exact.MaxCoefficientBits) {
			return geom.ErrConvergence
		}
		return nil
	}
	out.Partition, err = q.RefineWindowDepth(ctx, w, o.Refinement)
	if err != nil {
		return out, err
	}
	ids := [2]map[int]bool{{}, {}}
	for owner := 0; owner < 2; owner++ {
		out.Owners[owner] = AlgebraicGraphOwnerArea{Operand: owner, Area: [2]*big.Rat{new(big.Rat), new(big.Rat)}}
	}
	for _, leaf := range out.Partition.Leaves {
		if leaf.NoFootprint {
			continue
		}
		if e := ctx.Err(); e != nil {
			return out, e
		}
		area := new(big.Rat).Mul(new(big.Rat).Sub(leaf.Window.Max[0], leaf.Window.Min[0]), new(big.Rat).Sub(leaf.Window.Max[1], leaf.Window.Min[1]))
		if e := check(area); e != nil {
			return out, e
		}
		for owner := 0; owner < 2; owner++ {
			if leaf.FullMaterial {
				ids[owner][leaf.Faces[owner]] = true
			} else if leaf.Unresolved {
				for _, r := range q.regions {
					ids[owner][r.faces[owner]] = true
				}
			} else {
				return out, geom.ErrConvergence
			}

			if out.SurfaceCells >= o.MaxSurfaceCells || o.CellOperations > int(^uint(0)>>1)-out.SurfaceQuotaCharged {
				return out, geom.ErrConvergence
			}
			out.SurfaceCells++
			out.SurfaceQuotaCharged += o.CellOperations
			density, e := q.graphCellAreaDensity(ctx, owner, leaf.Window, o.CellOperations, exact.MaxCoefficientBits, o.RootBits)
			if e != nil {
				return out, e
			}
			for end := 0; end < 2; end++ {
				if end == 0 && !leaf.FullMaterial {
					continue
				}
				v := new(big.Rat).Mul(area, density[end])
				if e := check(v); e != nil {
					return out, e
				}
				out.Owners[owner].Area[end].Add(out.Owners[owner].Area[end], v)
				if e := check(out.Owners[owner].Area[end]); e != nil {
					return out, e
				}
			}
		}
	}
	for owner := 0; owner < 2; owner++ {
		for id := range ids[owner] {
			out.Owners[owner].NativeFaces = append(out.Owners[owner].NativeFaces, id)
		}
		sort.Ints(out.Owners[owner].NativeFaces)
	}
	for end := 0; end < 2; end++ {
		out.Total[end] = new(big.Rat).Add(out.Owners[0].Area[end], out.Owners[1].Area[end])
		if e := check(out.Total[end]); e != nil {
			return out, e
		}
	}
	return out, nil
}

// Density is measured in the actual selected world-coordinate plane. Applying
// the exact inverse chart Jacobian to dH/dU,dH/dV yields graph slopes; hence the
// world area multiplier is sqrt(1+hx²+hy²), including nonunit/reversed charts.
func (q *AlgebraicGraphMaterialQuery) graphCellAreaDensity(ctx context.Context, owner int, w AlgebraicGraphProjectionWindow, limit, bits, rootBits int) ([2]*big.Rat, error) {
	return q.graphCellAffineAreaDensity(ctx, owner, w, limit, bits, rootBits, nil)
}

// A nil cofactor preserves the original graph-area path. Otherwise transform
// the area vector in original world axes; projection parity is a common sign
// and therefore has no effect on its norm.
func (q *AlgebraicGraphMaterialQuery) graphCellAffineAreaDensity(ctx context.Context, owner int, w AlgebraicGraphProjectionWindow, limit, bits, rootBits int, cofactor *[3][3]*big.Rat) ([2]*big.Rat, error) {
	if limit < 20480 {
		return [2]*big.Rat{}, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return [2]*big.Rat{}, e
	}
	step := (limit - 16384) / 3
	type interval [2]*big.Rat
	var failure error
	ops := 0
	scalar := func(op byte, a, b *big.Rat) *big.Rat {
		r := new(big.Rat)
		if failure != nil {
			return r
		}
		ops++
		if ops > 16384 || !graphPolynomialBits(a, bits) || !graphPolynomialBits(b, bits) {
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
	constant := func(x *big.Rat) interval { return interval{x, x} }
	add := func(a, b interval) interval { return interval{scalar('+', a[0], b[0]), scalar('+', a[1], b[1])} }
	subtract := func(a, b interval) interval { return interval{scalar('-', a[0], b[1]), scalar('-', a[1], b[0])} }
	multiply := func(a, b interval) interval {
		var r interval
		for _, x := range a {
			for _, y := range b {
				v := scalar('*', x, y)
				if r[0] == nil || v.Cmp(r[0]) < 0 {
					r[0] = v
				}
				if r[1] == nil || v.Cmp(r[1]) > 0 {
					r[1] = v
				}
			}
		}
		return r
	}
	divide := func(a, b interval) interval {
		if b[0].Sign() <= 0 {
			failure = geom.ErrConvergence
			return constant(new(big.Rat))
		}
		var r interval
		for _, x := range a {
			for _, y := range b {
				v := scalar('/', x, y)
				if r[0] == nil || v.Cmp(r[0]) < 0 {
					r[0] = v
				}
				if r[1] == nil || v.Cmp(r[1]) > 0 {
					r[1] = v
				}
			}
		}
		return r
	}
	square := func(a interval) interval {
		x, y := scalar('*', a[0], a[0]), scalar('*', a[1], a[1])
		if x.Cmp(y) > 0 {
			x, y = y, x
		}
		if a[0].Sign() <= 0 && a[1].Sign() >= 0 {
			x = new(big.Rat)
		}
		return interval{x, y}
	}
	inverse, e := q.graphs[owner].InverseCoefficients()
	if e != nil {
		return [2]*big.Rat{}, e
	}
	var uv [2]interval
	for axis, row := range inverse {
		uv[axis] = constant(row[2])
		for j := 0; j < 2; j++ {
			uv[axis] = add(uv[axis], multiply(constant(row[j]), interval{w.Min[j], w.Max[j]}))
		}
	}
	if failure != nil {
		return [2]*big.Rat{}, failure
	}
	patch := q.patches[owner]
	u, v := patch.KnotBreaks()
	if len(u) > 256 || len(v) > 256 {
		return [2]*big.Rat{}, geom.ErrConvergence
	}
	var span [2][2]float64
	for axis, knots := range [][]float64{u, v} {
		found := false
		for k := 0; k+1 < len(knots); k++ {
			if uv[axis][0].Cmp(ratInset(knots[k])) >= 0 && uv[axis][1].Cmp(ratInset(knots[k+1])) <= 0 {
				span[axis] = [2]float64{knots[k], knots[k+1]}
				found = true
				break
			}
		}
		if !found {
			return [2]*big.Rat{}, geom.ErrConvergence
		}
	}
	limited, e := patch.WithLimits(geom.SurfacePatchOptions{MaxOperations: step, MaxCoefficientBits: bits})
	if e != nil {
		return [2]*big.Rat{}, e
	}
	p, _, e := limited.TrimWithWorkLimit(geom.ParameterRectangle{UMin: span[0][0], UMax: span[0][1], VMin: span[1][0], VMax: span[1][1]}, step)
	if e != nil {
		return [2]*big.Rat{}, e
	}
	coeff, minWeight, ok := p.RationalBezierCoefficients(6)
	if !ok {
		return [2]*big.Rat{}, geom.ErrConvergence
	}
	var local [2]interval
	var widths [2]*big.Rat
	for axis := 0; axis < 2; axis++ {
		widths[axis] = scalar('-', ratInset(span[axis][1]), ratInset(span[axis][0]))
		local[axis] = divide(subtract(uv[axis], constant(ratInset(span[axis][0]))), constant(widths[axis]))
	}
	// Exact interval Horner evaluation of homogeneous numerators/derivatives.
	evaluate := func(coordinate, du, dv int) interval {
		sum := constant(new(big.Rat))
		for i := len(coeff) - 1 - du; i >= 0; i-- {
			row := constant(new(big.Rat))
			for j := len(coeff[0]) - 1 - dv; j >= 0; j-- {
				c := coeff[i+du][j+dv][coordinate]
				if du != 0 {
					c = scalar('*', c, big.NewRat(int64(i+1), 1))
				}
				if dv != 0 {
					c = scalar('*', c, big.NewRat(int64(j+1), 1))
				}
				row = add(multiply(row, local[1]), constant(c))
			}
			sum = add(multiply(sum, local[0]), row)
		}
		return sum
	}
	h, weight := evaluate(q.residual, 0, 0), evaluate(3, 0, 0)
	if weight[0].Cmp(minWeight) < 0 {
		weight[0] = new(big.Rat).Set(minWeight)
	}
	if weight[0].Sign() <= 0 || weight[0].Cmp(weight[1]) > 0 {
		return [2]*big.Rat{}, geom.ErrConvergence
	}
	denominator := square(weight)
	var derivative [2]interval
	for axis := 0; axis < 2; axis++ {
		du, dv := 1, 0
		if axis == 1 {
			du, dv = 0, 1
		}
		num := subtract(multiply(evaluate(q.residual, du, dv), weight), multiply(h, evaluate(3, du, dv)))
		derivative[axis] = divide(divide(num, denominator), constant(widths[axis]))
	}
	norm := constant(big.NewRat(1, 1))
	var slopes [2]interval
	for projection := 0; projection < 2; projection++ {
		slopes[projection] = add(multiply(derivative[0], constant(inverse[0][projection])), multiply(derivative[1], constant(inverse[1][projection])))
		if cofactor == nil {
			norm = add(norm, square(slopes[projection]))
		}
	}
	if cofactor != nil {
		var normal [3]interval
		normal[q.residual] = constant(big.NewRat(1, 1))
		for i, axis := range q.axes {
			normal[axis] = subtract(constant(new(big.Rat)), slopes[i])
		}
		norm = constant(new(big.Rat))
		for i := 0; i < 3; i++ {
			component := constant(new(big.Rat))
			for j := 0; j < 3; j++ {
				component = add(component, multiply(constant(cofactor[i][j]), normal[j]))
			}
			norm = add(norm, square(component))
		}
	}
	if failure != nil {
		return [2]*big.Rat{}, failure
	}
	lower, _, e := graphAreaSqrt(norm[0], rootBits, bits)
	if e != nil {
		return [2]*big.Rat{}, e
	}
	_, upper, e := graphAreaSqrt(norm[1], rootBits, bits)
	return [2]*big.Rat{lower, upper}, e
}

// Integer square root encloses the exact rational, with only a bounded extra
// 2*RootBits integer precision. No floating square-root error enters the proof.
func graphAreaSqrt(x *big.Rat, precision, bits int) (*big.Rat, *big.Rat, error) {
	if precision < 8 || precision > 256 || bits < 64 {
		return nil, nil, geom.ErrInvalid
	}
	if x == nil || x.Sign() < 0 {
		return nil, nil, geom.ErrConvergence
	}
	numerator := new(big.Int).Lsh(new(big.Int).Set(x.Num()), uint(2*precision))
	scaled := new(big.Int).Quo(numerator, x.Denom())
	root := new(big.Int).Sqrt(scaled)
	denominator := new(big.Int).Lsh(big.NewInt(1), uint(precision))
	lo := new(big.Rat).SetFrac(root, denominator)
	hi := new(big.Rat).Set(lo)
	if new(big.Rat).Mul(lo, lo).Cmp(x) != 0 {
		hi.SetFrac(new(big.Int).Add(root, big.NewInt(1)), denominator)
	}
	if !graphPolynomialBits(lo, bits) || !graphPolynomialBits(hi, bits) {
		return nil, nil, geom.ErrConvergence
	}
	return lo, hi, nil
}
