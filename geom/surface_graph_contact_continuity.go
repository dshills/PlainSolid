package geom

import (
	"context"
	"math/big"
)

func graphContactBezierCells(ctx context.Context, p *SurfacePatch, maxCells int, b *polynomialBudget) ([]*SurfacePatch, error) {
	stack := []*SurfacePatch{p}
	var cells []*SurfacePatch
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if knot, axis, ok := patchSplitChoice(current); ok {
			if len(stack)+len(cells)+2 > maxCells {
				return nil, ErrConvergence
			}
			l, h, e := current.split(knot, axis, b)
			if e != nil {
				return nil, e
			}
			stack = append(stack, h, l)
		} else {
			cells = append(cells, current)
		}
	}
	return cells, nil
}

// Each pair of adjacent cells in the common tensor grid has identical varying
// interval and degree. Equality of edge Bernstein coefficients proves F and its
// normal derivative match identically. Tangential equality follows from F.
func graphContactC1(ctx context.Context, cells []*SurfacePatch, b *polynomialBudget) (int, error) {
	count := 0
	for i, a := range cells {
		for _, c := range cells[i+1:] {
			if e := ctx.Err(); e != nil {
				return count, e
			}
			if e := b.check(big.NewRat(0, 1)); e != nil {
				return count, e
			}
			ad, cd := a.Domain(), c.Domain()
			axis := -1
			left, right := a, c
			if ad.VMin == cd.VMin && ad.VMax == cd.VMax {
				if ad.UMax == cd.UMin {
					axis = 0
				} else if cd.UMax == ad.UMin {
					axis = 0
					left, right = c, a
				}
			}
			if ad.UMin == cd.UMin && ad.UMax == cd.UMax {
				if ad.VMax == cd.VMin {
					axis = 1
				} else if cd.VMax == ad.VMin {
					axis = 1
					left, right = c, a
				}
			}
			if axis < 0 {
				continue
			}
			count++
			var edge [2][2][]*big.Rat
			for side, p := range []*SurfacePatch{left, right} {
				f := make([]*big.Rat, len(p.net))
				for k, h := range p.net {
					f[k] = h[2]
				}
				df, e := scalarPartial(f, p.countU, p.countV, axis == 0, p.Domain(), b)
				if e != nil {
					return count, e
				}
				if axis == 0 {
					row := 0
					if side == 0 {
						row = p.countU - 1
					}
					edge[side][0] = f[row*p.countV : (row+1)*p.countV]
					row = 0
					if side == 0 {
						row = p.countU - 2
					}
					edge[side][1] = df[row*p.countV : (row+1)*p.countV]
				} else {
					column := 0
					if side == 0 {
						column = p.countV - 1
					}
					for row := 0; row < p.countU; row++ {
						edge[side][0] = append(edge[side][0], f[row*p.countV+column])
					}
					column = 0
					if side == 0 {
						column = p.countV - 2
					}
					for row := 0; row < p.countU; row++ {
						edge[side][1] = append(edge[side][1], df[row*(p.countV-1)+column])
					}
				}
			}
			for derivative := 0; derivative < 2; derivative++ {
				if len(edge[0][derivative]) != len(edge[1][derivative]) {
					return count, ErrConvergence
				}
				for k, x := range edge[0][derivative] {
					delta := new(big.Rat).Sub(x, edge[1][derivative][k])
					if e := b.check(delta); e != nil {
						return count, e
					}
					if delta.Sign() != 0 {
						return count, ErrConvergence
					}
				}
			}
		}
	}
	return count, nil
}
