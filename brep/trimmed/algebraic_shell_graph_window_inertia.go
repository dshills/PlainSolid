package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphWindowInertiaOptions leaves existing volume/first-moment
// budgets unchanged. SecondCellOperations defaults to1048576, maximum16M per
// retained cell. Second moments require constant-weight degree<=3 per original
// knot span. Unknown cells need no polynomial assumption beyond their existing
// certified volume and coordinate enclosures.
type AlgebraicGraphWindowInertiaOptions struct {
	Moments              AlgebraicGraphWindowMomentOptions
	SecondCellOperations int
}
type AlgebraicGraphExactCellSecondMoments struct {
	Leaf          int
	SecondMoments [6]*big.Rat
}

// AlgebraicGraphWindowInertia reports unit-density world-coordinate moments.
// All packed symmetric tensors use XX,XY,XZ,YY,YZ,ZZ order. RawSecondMoments
// integrate x_i*x_j; OriginInertia has diagonal sums and negative off-diagonals.
// CentroidalInertia uses exact interval parallel-axis subtraction, only when
// volume is proved positive. Bounds may be conservative and correlated.
type AlgebraicGraphWindowInertia struct {
	Moments                                            AlgebraicGraphWindowMoments
	RawSecondMoments, OriginInertia, CentroidalInertia [6][2]*big.Rat
	HasCentroidal                                      bool
	ExactCells                                         []AlgebraicGraphExactCellSecondMoments
	SecondQuotaCharged                                 int
}

func (q *AlgebraicGraphMaterialQuery) IntegrateWindowInertia(ctx context.Context, w AlgebraicGraphProjectionWindow, o AlgebraicGraphWindowInertiaOptions) (AlgebraicGraphWindowInertia, error) {
	return q.integrateWindowInertiaPrepared(ctx, w, o, nil)
}
func (q *AlgebraicGraphMaterialQuery) integrateWindowInertiaPrepared(ctx context.Context, w AlgebraicGraphProjectionWindow, o AlgebraicGraphWindowInertiaOptions, prepared *AlgebraicGraphWindowMoments) (out AlgebraicGraphWindowInertia, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphWindowInertia{SecondQuotaCharged: out.SecondQuotaCharged}
		}
	}()
	if o.SecondCellOperations == 0 {
		o.SecondCellOperations = 1048576
	}
	if o.SecondCellOperations < 1 || o.SecondCellOperations > 16000000 {
		return out, geom.ErrInvalid
	}
	exact, e := graphMaterialLimits(o.Moments.Polynomial.Refinement.CellOptions.Exact)
	if e != nil {
		return out, e
	}
	check := func(x *big.Rat) error {
		if !graphPolynomialBits(x, exact.MaxCoefficientBits) {
			return geom.ErrConvergence
		}
		return nil
	}
	if prepared == nil {
		out.Moments, err = q.IntegrateWindowMoments(ctx, w, o.Moments)
		if err != nil {
			return out, err
		}
	} else {
		if ctx == nil || q == nil || !prepared.Volume.Bounds.HasBounds {
			return out, geom.ErrInvalid
		}
		if e := ctx.Err(); e != nil {
			return out, e
		}
		out.Moments = *prepared
	}
	var residual [2]*big.Rat
	for _, p := range q.patches {
		b, e := p.Bounds()
		if e != nil {
			return out, e
		}
		lo := [3]float64{b.Min.X, b.Min.Y, b.Min.Z}
		hi := [3]float64{b.Max.X, b.Max.Y, b.Max.Z}
		if !finite(lo[q.residual]) || !finite(hi[q.residual]) {
			return out, geom.ErrConvergence
		}
		a, z := ratInset(lo[q.residual]), ratInset(hi[q.residual])
		if e := check(a); e != nil {
			return out, e
		}
		if e := check(z); e != nil {
			return out, e
		}
		if residual[0] == nil || a.Cmp(residual[0]) < 0 {
			residual[0] = a
		}
		if residual[1] == nil || z.Cmp(residual[1]) > 0 {
			residual[1] = z
		}
	}
	for i := range out.RawSecondMoments {
		out.RawSecondMoments[i] = [2]*big.Rat{new(big.Rat), new(big.Rat)}
	}
	volumes := map[int]*big.Rat{}
	for _, c := range out.Moments.Volume.ExactCells {
		volumes[c.Leaf] = c.Volume
	}
	first := map[int][3]*big.Rat{}
	for _, c := range out.Moments.ExactCells {
		first[c.Leaf] = c.FirstMoments
	}
	// Exact interval products, including the sharper square for diagonal terms.
	product := func(a, b [2]*big.Rat, square bool) (r [2]*big.Rat, e error) {
		for _, x := range a {
			for _, y := range b {
				v := new(big.Rat).Mul(x, y)
				if e := check(v); e != nil {
					return r, e
				}
				if r[0] == nil || v.Cmp(r[0]) < 0 {
					r[0] = new(big.Rat).Set(v)
				}
				if r[1] == nil || v.Cmp(r[1]) > 0 {
					r[1] = new(big.Rat).Set(v)
				}
			}
		}
		if square && a[0].Sign() <= 0 && a[1].Sign() >= 0 {
			r[0].SetInt64(0)
		}
		return r, nil
	}
	for leafIndex, leaf := range out.Moments.Volume.Bounds.Leaves {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if leaf.NoFootprint {
			continue
		}
		var contributions [6][2]*big.Rat
		if leaf.FullMaterial {
			if o.SecondCellOperations > int(^uint(0)>>1)-out.SecondQuotaCharged {
				return out, geom.ErrConvergence
			}
			out.SecondQuotaCharged += o.SecondCellOperations
			value, e := q.integrateGraphPolynomialCellMode(ctx, leaf, o.SecondCellOperations, exact.MaxCoefficientBits, 2)
			if e != nil {
				return out, e
			}
			v, ok := volumes[leafIndex]
			if !ok || v.Cmp(value[0]) != 0 {
				return out, geom.ErrConvergence
			}
			m, ok := first[leafIndex]
			if !ok {
				return out, geom.ErrConvergence
			}
			for i := 0; i < 3; i++ {
				if m[i].Cmp(value[i+1]) != 0 {
					return out, geom.ErrConvergence
				}
			}
			record := AlgebraicGraphExactCellSecondMoments{Leaf: leafIndex}
			for i := 0; i < 6; i++ {
				if (i == 0 || i == 3 || i == 5) && value[4+i].Sign() < 0 {
					return out, geom.ErrConvergence
				}
				record.SecondMoments[i] = new(big.Rat).Set(value[4+i])
				contributions[i] = [2]*big.Rat{value[4+i], value[4+i]}
			}
			out.ExactCells = append(out.ExactCells, record)
		} else if leaf.Unresolved {
			area := new(big.Rat).Mul(new(big.Rat).Sub(leaf.Window.Max[0], leaf.Window.Min[0]), new(big.Rat).Sub(leaf.Window.Max[1], leaf.Window.Min[1]))
			if e := check(area); e != nil {
				return out, e
			}
			v := new(big.Rat).Mul(area, leaf.Depth[1])
			if e := check(v); e != nil {
				return out, e
			}
			var coordinates [3][2]*big.Rat
			coordinates[q.residual] = residual
			for projection, axis := range q.axes {
				coordinates[axis] = [2]*big.Rat{leaf.Window.Min[projection], leaf.Window.Max[projection]}
			}
			for a := 0; a < 3; a++ {
				for b := a; b < 3; b++ {
					r, e := product(coordinates[a], coordinates[b], a == b)
					if e != nil {
						return out, e
					}
					r[0].Mul(r[0], v)
					r[1].Mul(r[1], v)
					for _, x := range r {
						if e := check(x); e != nil {
							return out, e
						}
					}
					if r[0].Sign() > 0 {
						r[0].SetInt64(0)
					}
					if r[1].Sign() < 0 {
						r[1].SetInt64(0)
					}
					contributions[graphSecondIndex(a, b)] = r
				}
			}
		} else {
			return out, geom.ErrConvergence
		}
		for i := 0; i < 6; i++ {
			for end := 0; end < 2; end++ {
				out.RawSecondMoments[i][end].Add(out.RawSecondMoments[i][end], contributions[i][end])
				if e := check(out.RawSecondMoments[i][end]); e != nil {
					return out, e
				}
			}
		}
	}
	inertia := func(second [6][2]*big.Rat) (r [6][2]*big.Rat, e error) {
		for a := 0; a < 3; a++ {
			for b := a; b < 3; b++ {
				index := graphSecondIndex(a, b)
				if a != b {
					r[index] = [2]*big.Rat{new(big.Rat).Neg(second[index][1]), new(big.Rat).Neg(second[index][0])}
				} else {
					j, k := (a+1)%3, (a+2)%3
					r[index] = [2]*big.Rat{new(big.Rat).Add(second[graphSecondIndex(j, j)][0], second[graphSecondIndex(k, k)][0]), new(big.Rat).Add(second[graphSecondIndex(j, j)][1], second[graphSecondIndex(k, k)][1])}
				}
				for _, x := range r[index] {
					if e := check(x); e != nil {
						return r, e
					}
				}
			}
		}
		return r, nil
	}
	out.OriginInertia, err = inertia(out.RawSecondMoments)
	if err != nil {
		return out, err
	}
	volume := out.Moments.Volume.Bounds.Volume
	if volume[0].Sign() > 0 {
		var central [6][2]*big.Rat
		for a := 0; a < 3; a++ {
			for b := a; b < 3; b++ {
				index := graphSecondIndex(a, b)
				p, e := product(out.Moments.FirstMoments[a], out.Moments.FirstMoments[b], a == b)
				if e != nil {
					return out, e
				}
				var quotient [2]*big.Rat
				for _, m := range p {
					for _, v := range volume {
						x := new(big.Rat).Quo(m, v)
						if e := check(x); e != nil {
							return out, e
						}
						if quotient[0] == nil || x.Cmp(quotient[0]) < 0 {
							quotient[0] = new(big.Rat).Set(x)
						}
						if quotient[1] == nil || x.Cmp(quotient[1]) > 0 {
							quotient[1] = new(big.Rat).Set(x)
						}
					}
				}
				central[index] = [2]*big.Rat{new(big.Rat).Sub(out.RawSecondMoments[index][0], quotient[1]), new(big.Rat).Sub(out.RawSecondMoments[index][1], quotient[0])}
				for _, x := range central[index] {
					if e := check(x); e != nil {
						return out, e
					}
				}
				if a == b {
					if central[index][1].Sign() < 0 {
						return out, geom.ErrConvergence
					}
					if central[index][0].Sign() < 0 {
						central[index][0].SetInt64(0)
					}
				}
			}
		}
		out.CentroidalInertia, err = inertia(central)
		if err != nil {
			return out, err
		}
		out.HasCentroidal = true
	}
	return out, nil
}
