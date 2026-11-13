package geom

import (
	"context"
	"math/big"
	"sort"
)

// ExactSurfaceComparisonOptions selects B's chart map: swap its U/V axes first,
// then reverse the resulting U/V axes as requested. NormalizeDomains aligns
// both domain rectangles affinely; otherwise mapped domain endpoints must match.
// All maps and common knot partitions are rational, never rounded to float knots.
// Zero limits select 8192 common cells, 8M operations and 16384 coefficient bits.
type ExactSurfaceComparisonOptions struct {
	NormalizeDomains, SwapB, ReverseBU, ReverseBV bool
	MaxCells, MaxOperations, MaxCoefficientBits   int
}

// ExactSurfaceComparison distinguishes proven chart identity from nonidentity.
// Complete&&!Equal proves nonidentity, not geometric disjointness. DifferentCell
// is the first differing common rectangle (U-major order), DifferentAxis its
// model coordinate. Any error is unresolved. Counters include exact refinement.
type ExactSurfaceComparison struct {
	Equal, Complete                                              bool
	ComparedCells, DifferentCell, DifferentAxis, ExactOperations int
}

type comparisonTensor struct {
	u0, u1, v0, v1 *big.Rat
	nu, nv         int
	net            []exactHomogeneous
}

// CompareExactSurfaces tests exact homogeneous tensor polynomial identities on
// the rational union knot grid. Degrees, knot breaks and weights may differ.
// Positive weights make cross multiplication valid over every closed cell.
// Cancellation is checked between bounded extraction/restriction steps.
func CompareExactSurfaces(ctx context.Context, a, b *SurfacePatch, o ExactSurfaceComparisonOptions) (r ExactSurfaceComparison, err error) {
	return compareExactSurfaceRationalDomains(ctx, a, b, o, nil)
}

// domains, when nonnil, selects exact original-parameter subrectangles. Only
// original knot boundaries are extracted; all final clipping remains rational.
func compareExactSurfaceRationalDomains(ctx context.Context, a, b *SurfacePatch, o ExactSurfaceComparisonOptions, domains *[2][2][2]*big.Rat) (r ExactSurfaceComparison, err error) {
	r.DifferentCell, r.DifferentAxis = -1, -1
	if ctx == nil || a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	ad, bd := a.Domain(), b.Domain()
	if o.SwapB {
		bd = ParameterRectangle{UMin: bd.VMin, UMax: bd.VMax, VMin: bd.UMin, VMax: bd.UMax}
	}
	if domains == nil && !o.NormalizeDomains && ad != bd {
		return r, ErrDomain
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	extract := func(source *SurfacePatch, owner int, swap, reverseU, reverseV bool) ([]comparisonTensor, error) {
		d := source.Domain()
		domain := [2][2]*big.Rat{{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax)}, {new(big.Rat).SetFloat64(d.VMin), new(big.Rat).SetFloat64(d.VMax)}}
		if domains != nil {
			for axis := 0; axis < 2; axis++ {
				lo, hi := domains[owner][axis][0], domains[owner][axis][1]
				if lo == nil || hi == nil || lo.Cmp(hi) >= 0 || lo.Cmp(domain[axis][0]) < 0 || hi.Cmp(domain[axis][1]) > 0 {
					return nil, ErrDomain
				}
				for _, q := range []*big.Rat{lo, hi} {
					if e := budget.check(q); e != nil {
						return nil, e
					}
				}
				domain[axis] = [2]*big.Rat{lo, hi}
			}
		}
		if swap {
			domain[0], domain[1] = domain[1], domain[0]
		}
		normalized := func(t float64, axis int, reverse bool) *big.Rat {
			x := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(t), domain[axis][0]), new(big.Rat).Sub(domain[axis][1], domain[axis][0]))
			if reverse {
				x.Sub(big.NewRat(1, 1), x)
			}
			return x
		}
		stack := []*SurfacePatch{source}
		var cells []comparisonTensor
		for len(stack) > 0 {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if k, u, ok := patchSplitChoice(p); ok {
				if len(stack)+len(cells)+2 > o.MaxCells {
					return nil, ErrConvergence
				}
				left, right, e := p.split(k, u, budget)
				if e != nil {
					return nil, e
				}
				stack = append(stack, right, left)
				continue
			}
			d := p.Domain()
			nu, nv := p.countU, p.countV
			if swap {
				d = ParameterRectangle{UMin: d.VMin, UMax: d.VMax, VMin: d.UMin, VMax: d.UMax}
				nu, nv = nv, nu
			}
			cell := comparisonTensor{u0: normalized(d.UMin, 0, reverseU), u1: normalized(d.UMax, 0, reverseU), v0: normalized(d.VMin, 1, reverseV), v1: normalized(d.VMax, 1, reverseV), nu: nu, nv: nv, net: make([]exactHomogeneous, nu*nv)}
			if reverseU {
				cell.u0, cell.u1 = cell.u1, cell.u0
			}
			if reverseV {
				cell.v0, cell.v1 = cell.v1, cell.v0
			}
			for i := 0; i < nu; i++ {
				for j := 0; j < nv; j++ {
					u, v := i, j
					if reverseU {
						u = nu - 1 - u
					}
					if reverseV {
						v = nv - 1 - v
					}
					if swap {
						u, v = v, u
					}
					cell.net[i*nv+j] = p.net[u*p.countV+v]
				}
			}
			for _, x := range []*big.Rat{cell.u0, cell.u1, cell.v0, cell.v1} {
				if e := budget.check(x); e != nil {
					return nil, e
				}
			}
			cells = append(cells, cell)
		}
		return cells, nil
	}
	ac, e := extract(a, 0, false, false, false)
	if e != nil {
		return r, e
	}
	bc, e := extract(b, 1, o.SwapB, o.ReverseBU, o.ReverseBV)
	if e != nil {
		return r, e
	}
	union := func(u bool) []*big.Rat {
		values := []*big.Rat{new(big.Rat), big.NewRat(1, 1)}
		for _, cells := range [][]comparisonTensor{ac, bc} {
			for _, c := range cells {
				if u {
					values = append(values, c.u0, c.u1)
				} else {
					values = append(values, c.v0, c.v1)
				}
			}
		}
		sort.Slice(values, func(i, j int) bool { return values[i].Cmp(values[j]) < 0 })
		unique := values[:0]
		for _, x := range values {
			if x.Sign() < 0 || x.Cmp(big.NewRat(1, 1)) > 0 {
				continue
			}
			if len(unique) == 0 || x.Cmp(unique[len(unique)-1]) != 0 {
				unique = append(unique, x)
			}
		}
		return unique
	}
	us, vs := union(true), union(false)
	if len(us)-1 > o.MaxCells/(len(vs)-1) {
		return r, ErrConvergence
	}
	find := func(cells []comparisonTensor, u0, u1, v0, v1 *big.Rat) comparisonTensor {
		for _, c := range cells {
			if c.u0.Cmp(u0) <= 0 && c.u1.Cmp(u1) >= 0 && c.v0.Cmp(v0) <= 0 && c.v1.Cmp(v1) >= 0 {
				return c
			}
		}
		return comparisonTensor{}
	}
	for i := 0; i < len(us)-1; i++ {
		for j := 0; j < len(vs)-1; j++ {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			cells := [2]comparisonTensor{find(ac, us[i], us[i+1], vs[j], vs[j+1]), find(bc, us[i], us[i+1], vs[j], vs[j+1])}
			if cells[0].nu == 0 || cells[1].nu == 0 {
				return r, ErrInvalid
			}
			var powers [2][4][]*big.Rat
			for side, c := range cells {
				local := func(x, lo, hi *big.Rat) *big.Rat {
					return new(big.Rat).Quo(new(big.Rat).Sub(x, lo), new(big.Rat).Sub(hi, lo))
				}
				net, e := restrictComparisonTensor(c, local(us[i], c.u0, c.u1), local(us[i+1], c.u0, c.u1), local(vs[j], c.v0, c.v1), local(vs[j+1], c.v0, c.v1), budget)
				if e != nil {
					return r, e
				}
				for axis := 0; axis < 4; axis++ {
					powers[side][axis], e = tensorPowerCoefficients(net, c.nu, c.nv, axis, budget)
					if e != nil {
						return r, e
					}
				}
			}
			r.ComparedCells++
			for axis := 0; axis < 3; axis++ {
				left, e := tensorPolynomialProduct(powers[0][axis], cells[0].nu, cells[0].nv, powers[1][3], cells[1].nu, cells[1].nv, budget)
				if e != nil {
					return r, e
				}
				right, e := tensorPolynomialProduct(powers[0][3], cells[0].nu, cells[0].nv, powers[1][axis], cells[1].nu, cells[1].nv, budget)
				if e != nil {
					return r, e
				}
				for k := range left {
					if left[k].Cmp(right[k]) != 0 {
						r.Complete = true
						r.DifferentCell = i*(len(vs)-1) + j
						r.DifferentAxis = axis
						return r, nil
					}
				}
			}
		}
	}
	r.Complete, r.Equal = true, true
	return r, nil
}

func restrictComparisonTensor(c comparisonTensor, u0, u1, v0, v1 *big.Rat, b *polynomialBudget) ([]exactHomogeneous, error) {
	net := make([]exactHomogeneous, len(c.net))
	for j := 0; j < c.nv; j++ {
		column := make([]exactHomogeneous, c.nu)
		for i := range column {
			column[i] = c.net[i*c.nv+j]
		}
		restricted, e := restrictHomogeneousBezier(column, u0, u1, b)
		if e != nil {
			return nil, e
		}
		for i := range column {
			net[i*c.nv+j] = restricted[i]
		}
	}
	for i := 0; i < c.nu; i++ {
		row, e := restrictHomogeneousBezier(net[i*c.nv:(i+1)*c.nv], v0, v1, b)
		if e != nil {
			return nil, e
		}
		copy(net[i*c.nv:(i+1)*c.nv], row)
	}
	return net, nil
}
func tensorPowerCoefficients(net []exactHomogeneous, nu, nv, axis int, b *polynomialBudget) ([]*big.Rat, error) {
	intermediate := make([]*big.Rat, nu*nv)
	out := make([]*big.Rat, nu*nv)
	for i := range out {
		out[i] = new(big.Rat)
		intermediate[i] = new(big.Rat)
	}
	for j := 0; j < nv; j++ {
		values := make([]*big.Rat, nu)
		for i := range values {
			values[i] = net[i*nv+j][axis]
		}
		power, e := bernsteinToPower(values, b)
		if e != nil {
			return nil, e
		}
		for i, x := range power {
			intermediate[i*nv+j] = x
		}
	}
	for i := 0; i < nu; i++ {
		power, e := bernsteinToPower(intermediate[i*nv:(i+1)*nv], b)
		if e != nil {
			return nil, e
		}
		for j, x := range power {
			out[i*nv+j] = x
		}
	}
	return out, nil
}
func tensorPolynomialProduct(a []*big.Rat, au, av int, c []*big.Rat, cu, cv int, b *polynomialBudget) ([]*big.Rat, error) {
	width := av + cv - 1
	out := make([]*big.Rat, (au+cu-1)*width)
	for i := range out {
		out[i] = new(big.Rat)
	}
	for i := 0; i < au; i++ {
		for j := 0; j < av; j++ {
			for k := 0; k < cu; k++ {
				for l := 0; l < cv; l++ {
					index := (i+k)*width + j + l
					out[index].Add(out[index], new(big.Rat).Mul(a[i*av+j], c[k*cv+l]))
					if e := b.check(out[index]); e != nil {
						return nil, e
					}
				}
			}
		}
	}
	return out, nil
}
