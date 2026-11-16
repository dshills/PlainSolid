package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

func doubledNormalTangent(q *big.Rat) *big.Rat {
	// q is the squared tangent of a half-angle less than pi/4.
	d := new(big.Rat).Sub(big.NewRat(1, 1), q)
	return new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(4, 1), q), d.Mul(d, d))
}

func mappedNormalTurnLimit(angle float64) *big.Rat {
	direct := mappedAngleTangentLimit(angle)
	doubled := doubledNormalTangent(mappedAngleTangentLimit(angle / 2))
	if doubled.Cmp(direct) > 0 {
		return doubled
	}
	return direct
}

type roofNormalSlopeRange struct {
	orientation int
	lo, hi      [2]*big.Rat
}

func (r *roofNormalSlopeRange) extend(orientation int, x, y *big.Rat) bool {
	if orientation == 0 || r.orientation != 0 && r.orientation != orientation {
		return false
	}
	r.orientation = orientation
	for i, q := range [2]*big.Rat{x, y} {
		if r.lo[i] == nil || q.Cmp(r.lo[i]) < 0 {
			r.lo[i] = q
		}
		if r.hi[i] == nil || q.Cmp(r.hi[i]) > 0 {
			r.hi[i] = q
		}
	}
	return true
}

// Each original normal divided by its absolute z component lies in the
// plane z=±1 and inside this slope rectangle. Circular cones of angle less than pi/2
// are convex. The pairwise bound for the four corner rays therefore bounds
// any pair of original rays, without committing to one cone center.
func (r roofNormalSlopeRange) turnBound(limit *big.Rat) *big.Rat {
	if r.orientation == 0 {
		return nil
	}
	z := big.NewRat(int64(r.orientation), 1)
	corners := [4][3]*big.Rat{
		{r.lo[0], r.lo[1], z}, {r.lo[0], r.hi[1], z}, {r.hi[0], r.lo[1], z}, {r.hi[0], r.hi[1], z},
	}
	maximum := new(big.Rat)
	for i, a := range corners {
		for _, b := range corners[:i] {
			q, ok := ddTanSquared(a, b)
			if !ok || q.Cmp(limit) > 0 {
				return nil
			}
			if q.Cmp(maximum) > 0 {
				maximum.Set(q)
			}
		}
	}
	return maximum
}

func (r *mappedRoofNormals) boundPairwiseTurn(ctx context.Context, p *geom.SurfacePatch, uv [3][2]*big.Rat) (*big.Rat, error) {
	boxes := []geom.ParameterRectangle{r.normalRectangle(p, uv)}
	for depth := 0; depth <= 5; depth++ {
		var slopes roofNormalSlopeRange
		complete := true
		for _, box := range boxes {
			present, err := r.extendBernsteinGraphSlopes(ctx, p, box, &slopes)
			if err != nil {
				return nil, err
			}
			if !present {
				complete = false
				break
			}
		}
		if complete {
			if q := slopes.turnBound(r.turnLimit); q != nil {
				return q, nil
			}
		}
		if depth == 5 {
			break
		}
		next := make([]geom.ParameterRectangle, 0, len(boxes)*2)
		for _, b := range boxes {
			a, c := b, b
			if b.UMax-b.UMin >= b.VMax-b.VMin {
				a.UMax = b.UMin/2 + b.UMax/2
				c.UMin = a.UMax
				if a.UMax <= b.UMin || c.UMin >= b.UMax {
					return nil, nil
				}
			} else {
				a.VMax = b.VMin/2 + b.VMax/2
				c.VMin = a.VMax
				if a.VMax <= b.VMin || c.VMin >= b.VMax {
					return nil, nil
				}
			}
			next = append(next, a, c)
		}
		boxes = next
	}
	return nil, nil
}

// The physical XY projection is exactly affine. Each normalized normal is
// sign*(-H_x,-H_y,1), with H the original rational height. Represent both
// derivative numerators and W² in the same tensor Bernstein basis. Positive
// W² controls make each actual slope a convex combination of their exact
// coefficient ratios, without interval dependency between W and its gradient.
func (r *mappedRoofNormals) extendBernsteinGraphSlopes(ctx context.Context, p *geom.SurfacePatch, rect geom.ParameterRectangle, rangeOut *roofNormalSlopeRange) (bool, error) {
	for i := 0; i < 16; i++ {
		if err := r.charge(ctx); err != nil {
			return false, err
		}
	}
	child, err := p.Trim(rect)
	if err != nil {
		return false, err
	}
	c, _, ok := child.RationalBezierCoefficients(4)
	if !ok {
		return false, nil
	}
	u, v := 2*(len(c)-1), 2*(len(c[0])-1)
	power := make([][][3]*big.Rat, u+1)
	for i := range power {
		power[i] = make([][3]*big.Rat, v+1)
		for j := range power[i] {
			power[i][j] = [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		}
	}
	for a, row := range c {
		for b, h := range row {
			for i, other := range c {
				for j, w := range other {
					power[a+i][b+j][2].Add(power[a+i][b+j][2], new(big.Rat).Mul(h[3], w[3]))
					if a != i {
						n := new(big.Rat).Mul(h[2], w[3])
						n.Mul(n, big.NewRat(int64(a-i), 1))
						power[a+i-1][b+j][0].Add(power[a+i-1][b+j][0], n)
					}
					if b != j {
						n := new(big.Rat).Mul(h[2], w[3])
						n.Mul(n, big.NewRat(int64(b-j), 1))
						power[a+i][b+j-1][1].Add(power[a+i][b+j-1][1], n)
					}
				}
			}
		}
	}
	weights := func(n int) [][]*big.Rat {
		out := make([][]*big.Rat, n+1)
		for i := range out {
			out[i] = make([]*big.Rat, i+1)
			for k := range out[i] {
				out[i][k] = new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(int64(n), int64(k)))
			}
		}
		return out
	}
	wu, wv := weights(u), weights(v)
	inverse, err := r.turnChart.InverseCoefficients()
	if err != nil {
		return false, err
	}
	orientation := r.turnChart.Orientation()
	sign := big.NewRat(-int64(orientation), 1)
	span := [2]*big.Rat{
		new(big.Rat).Sub(ratInset(rect.UMax), ratInset(rect.UMin)),
		new(big.Rat).Sub(ratInset(rect.VMax), ratInset(rect.VMin)),
	}
	for _, wx := range wu {
		for _, wy := range wv {
			b := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
			for k, x := range wx {
				for l, y := range wy {
					factor := new(big.Rat).Mul(x, y)
					for axis := range b {
						b[axis].Add(b[axis], new(big.Rat).Mul(power[k][l][axis], factor))
					}
				}
			}
			if b[2].Sign() <= 0 {
				return false, nil
			}
			for k := 0; k < 2; k++ {
				b[k].Quo(b[k], new(big.Rat).Mul(span[k], b[2]))
			}
			var slope [2]*big.Rat
			for k := range slope {
				n := new(big.Rat).Add(new(big.Rat).Mul(inverse[0][k], b[0]), new(big.Rat).Mul(inverse[1][k], b[1]))
				slope[k] = n.Mul(n, sign)
				if slope[k].Num().BitLen() > 16384 || slope[k].Denom().BitLen() > 16384 {
					return false, geom.ErrConvergence
				}
			}
			if !rangeOut.extend(orientation, slope[0], slope[1]) {
				return false, nil
			}
		}
	}
	return true, nil
}
