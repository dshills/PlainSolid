package geom

import (
	"context"
	"math/big"
)

func projectiveHoleCuts(cuts ProjectiveSurfaceOriginalCuts, holes [2][]ExactSurfaceRectangle) (ProjectiveSurfaceOriginalCuts, error) {
	for owner, h := range holes {
		if len(h) > 32 {
			return ProjectiveSurfaceOriginalCuts{}, ErrConvergence
		}
		if len(h) == 0 {
			continue
		}
		for axis := range cuts[owner] {
			cuts[owner][axis] = append([]*big.Rat(nil), cuts[owner][axis]...)
		}
		for _, rect := range h {
			for axis, u := range [2]ExactParameterInterval{rect.U, rect.V} {
				if u.Lower == nil || u.Upper == nil || u.Lower.Cmp(u.Upper) >= 0 {
					return ProjectiveSurfaceOriginalCuts{}, ErrInvalid
				}
				cuts[owner][axis] = append(cuts[owner][axis], u.Lower, u.Upper)
			}
		}
	}
	return cuts, nil
}

func projectiveRectanglesSeparated(a, c ExactSurfaceRectangle, allowTouch bool) bool {
	for i, x := range [2]ExactParameterInterval{a.U, a.V} {
		y := [2]ExactParameterInterval{c.U, c.V}[i]
		q, s := x.Upper.Cmp(y.Lower), y.Upper.Cmp(x.Lower)
		if q < 0 || s < 0 || allowTouch && (q == 0 || s == 0) {
			return true
		}
	}
	return false
}

func projectiveContactHoleMask(ctx context.Context, atlas ProjectiveSurfaceHeightDifference, sources [2]*SurfacePatch,
	holes [2][]ExactSurfaceRectangle, b *polynomialBudget) ([]ExactSurfaceRectangle, []bool, []int, error) {
	var common []ExactSurfaceRectangle
	for owner, original := range holes {
		overlap := atlas.Overlap.Areas[owner]
		for _, hole := range original {
			if err := ctx.Err(); err != nil {
				return nil, nil, nil, err
			}
			if projectiveRectanglesSeparated(overlap, hole, true) {
				continue
			}
			var input ProjectiveSurfaceOriginalCuts
			for i, d := range [2]ExactParameterInterval{hole.U, hole.V} {
				x := [2]ExactParameterInterval{overlap.U, overlap.V}[i]
				if d.Lower.Cmp(x.Lower) <= 0 || d.Upper.Cmp(x.Upper) >= 0 {
					return nil, nil, nil, ErrConvergence
				}
				input[owner][i] = []*big.Rat{d.Lower, d.Upper}
			}
			var bounds [2][]*big.Rat
			if err := appendProjectiveSurfaceOriginalCuts(ctx, &bounds, sources, atlas.Overlap, input, 2, b); err != nil {
				return nil, nil, nil, err
			}
			for _, q := range bounds {
				if len(q) != 2 {
					return nil, nil, nil, ErrConvergence
				}
				if q[0].Cmp(q[1]) > 0 {
					q[0], q[1] = q[1], q[0]
				}
			}
			projected := ExactSurfaceRectangle{ExactParameterInterval{bounds[0][0], bounds[0][1]}, ExactParameterInterval{bounds[1][0], bounds[1][1]}}
			for _, prior := range common {
				if err := b.check(projected.U.Lower); err != nil {
					return nil, nil, nil, err
				}
				if !projectiveRectanglesSeparated(projected, prior, false) {
					return nil, nil, nil, ErrConvergence
				}
			}
			common = append(common, projected)
		}
	}
	if len(common) == 0 {
		return nil, nil, nil, nil
	}
	excluded := make([]bool, len(atlas.Cells))
	var dropped []int
	for i, p := range atlas.Cells {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		for _, h := range common {
			if err := b.check(p.Common.U.Lower); err != nil {
				return nil, nil, nil, err
			}
			if projectiveRectanglesSeparated(p.Common, h, true) {
				continue
			}
			if p.Common.U.Lower.Cmp(h.U.Lower) < 0 || p.Common.U.Upper.Cmp(h.U.Upper) > 0 ||
				p.Common.V.Lower.Cmp(h.V.Lower) < 0 || p.Common.V.Upper.Cmp(h.V.Upper) > 0 {
				return nil, nil, nil, ErrConvergence
			}
			excluded[i] = true
			dropped = append(dropped, i)
			break
		}
	}
	return common, excluded, dropped, nil
}

func projectiveContactHoleBoundary(ctx context.Context, p [2]*ExactAlgebraicRoot, holes []ExactSurfaceRectangle,
	b *polynomialBudget) (bool, error) {
	one := big.NewRat(1, 1)
	order := func(root *ExactAlgebraicRoot, q *big.Rat) (int, error) {
		x := root.root
		if x.bounds[0].Cmp(q) > 0 {
			return 1, nil
		}
		if x.bounds[1].Cmp(q) < 0 {
			return -1, nil
		}
		if x.bounds[0].Cmp(x.bounds[1]) == 0 {
			return x.bounds[0].Cmp(q), nil
		}
		sign, _, err := composedPolynomialRootSign(ctx, x, rationalPolynomial{new(big.Rat).Neg(q), one}, b, 8192)
		return sign, err
	}
	for _, h := range holes {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		found, outside := false, false
		for i, d := range [2]ExactParameterInterval{h.U, h.V} {
			lo, err := order(p[i], d.Lower)
			if err != nil {
				return false, err
			}
			hi, err := order(p[i], d.Upper)
			if err != nil {
				return false, err
			}
			found = found || lo == 0 || hi == 0
			if lo < 0 || hi > 0 {
				outside = true
				break
			}
		}
		if found && !outside {
			return true, nil
		}
	}
	return false, nil
}
