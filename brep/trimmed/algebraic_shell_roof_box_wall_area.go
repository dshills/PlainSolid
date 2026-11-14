package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// A proved vertical wall fills a plane region bounded by the original roof
// edge and the floor. Integrate its height in model units; the original
// wall's parameter speeds and rational weights are immaterial.
func mixedRoofWallArea(ctx context.Context, b *mixedRoofBox, side int, gap float64, o MixedRoofAreaOptions) (geom.ExactParameterInterval, int, int, error) {
	var zero geom.ExactParameterInterval
	if !finite(gap) || gap <= 0 {
		return zero, 0, 0, geom.ErrConvergence
	}
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o.SurfacePatchEvaluationOptions}}
	domain := func(d geom.ParameterRectangle) [2][2]float64 {
		return [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	}
	axis, edge := side/2, side%2
	other := 1 - axis
	wall := domain(b.domain)
	full, err := w.rat(new(big.Rat).Sub(ratInset(wall[other][1]), ratInset(wall[other][0])))
	if err != nil {
		return zero, 0, w.operations, err
	}
	limit, err := w.rat(new(big.Rat).Quo(ratInset(gap), full))
	if err != nil {
		return zero, 0, w.operations, err
	}
	spans := b.roofSpans
	if len(spans) == 0 {
		spans = []*mixedRoofBox{b}
	}
	out := mixedRoofZeroInterval()
	n := 0
	type cell struct {
		v     geom.ExactParameterInterval
		depth int
	}
	for _, s := range spans {
		d := domain(s.domain)
		if d[axis][edge] != wall[axis][edge] {
			continue
		}
		length, err := w.rat(new(big.Rat).Sub(ratInset(d[other][1]), ratInset(d[other][0])))
		if err != nil {
			return zero, n, w.operations, err
		}
		fixed := big.NewRat(int64(edge), 1)
		stack := []cell{{v: geom.ExactParameterInterval{Lower: new(big.Rat), Upper: big.NewRat(1, 1)}}}
		for len(stack) > 0 {
			if err := ctx.Err(); err != nil {
				return zero, n, w.operations, err
			}
			if n >= o.MaxCells {
				return zero, n, w.operations, geom.ErrConvergence
			}
			n++
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			var uv [2]geom.ExactParameterInterval
			uv[axis] = geom.ExactParameterInterval{Lower: fixed, Upper: fixed}
			uv[other] = p.v
			var h geom.ExactParameterInterval
			if s.weight == nil {
				v, err := mixedRoofAveragePower(s.linePower, uv, &w)
				if err != nil {
					return zero, n, w.operations, err
				}
				h = geom.ExactParameterInterval{Lower: v, Upper: new(big.Rat).Set(v)}
			} else {
				h, err = s.roofHeightRange(uv, &w)
				if err != nil {
					return zero, n, w.operations, err
				}
			}
			v, err := w.rat(new(big.Rat).Sub(h.Upper, h.Lower))
			if err != nil {
				return zero, n, w.operations, err
			}
			if v.Cmp(limit) > 0 {
				if p.depth >= o.MaxDepth {
					return zero, n, w.operations, geom.ErrConvergence
				}
				m, err := w.rat(new(big.Rat).Add(p.v.Lower, p.v.Upper))
				if err != nil {
					return zero, n, w.operations, err
				}
				if _, err = w.rat(m.Quo(m, big.NewRat(2, 1))); err != nil {
					return zero, n, w.operations, err
				}
				p.depth++
				q := p
				p.v.Upper, q.v.Lower = m, m
				stack = append(stack, q, p)
				continue
			}
			width, err := w.rat(new(big.Rat).Sub(p.v.Upper, p.v.Lower))
			if err != nil {
				return zero, n, w.operations, err
			}
			if _, err = w.rat(width.Mul(width, length)); err != nil {
				return zero, n, w.operations, err
			}
			for _, pair := range [][2]*big.Rat{{out.Lower, h.Lower}, {out.Upper, h.Upper}} {
				a, err := w.rat(new(big.Rat).Sub(maxRat(pair[1], b.floor), b.floor))
				if err != nil {
					return zero, n, w.operations, err
				}
				if _, err = w.rat(a.Mul(a, width)); err != nil {
					return zero, n, w.operations, err
				}
				if _, err = w.rat(pair[0].Add(pair[0], a)); err != nil {
					return zero, n, w.operations, err
				}
			}
		}
	}
	return out, n, w.operations, nil
}
