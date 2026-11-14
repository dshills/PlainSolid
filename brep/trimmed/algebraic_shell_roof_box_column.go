package trimmed

import (
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Primitive coefficients describe a fixed column as a signed sum of
// antiderivatives at the two original roofs and floors. All coefficients are
// -1, 0 or +1; zero may cancel a buried roof or an internal floor.
func (q *MixedRoofMaterialQuery) fixedColumnPrimitives(chart [2]bool, height [2]geom.ExactParameterInterval) (roof, floor [2]int, proven bool) {
	return q.columnPrimitivesWithOrder(chart, height, geom.ExactParameterInterval{})
}

func (q *MixedRoofMaterialQuery) columnPrimitivesWithOrder(chart [2]bool, height [2]geom.ExactParameterInterval, order geom.ExactParameterInterval) (roof, floor [2]int, proven bool) {
	if chart[0] && q.op != parametric.Intersection {
		roof[0], floor[0] = 1, -1
	}
	if chart[1] && q.op == parametric.Union {
		roof[1], floor[1] = 1, -1
	}
	if chart[0] && chart[1] {
		maxFloor := 0
		if q.box[0].floor.Cmp(q.box[1].floor) < 0 {
			maxFloor = 1
		}
		f := q.box[maxFloor].floor
		if height[0].Upper.Cmp(f) > 0 && height[1].Upper.Cmp(f) > 0 {
			if height[0].Lower.Cmp(f) < 0 || height[1].Lower.Cmp(f) < 0 {
				return roof, floor, false
			}
			lower := 0
			if height[1].Upper.Cmp(height[0].Lower) <= 0 || order.Lower != nil && order.Lower.Sign() >= 0 {
				lower = 1
			} else if height[0].Upper.Cmp(height[1].Lower) > 0 && (order.Upper == nil || order.Upper.Sign() > 0) {
				return roof, floor, false
			}
			m := -1
			if q.op == parametric.Intersection {
				m = 1
			}
			roof[lower] += m
			floor[maxFloor] -= m
		}
	}
	return roof, floor, true
}

// Try the exact relative difference only when the absolute ranges are too
// broad to order two roofs above the higher floor. This also lets a rational
// support cancel before deciding whether a cell has polynomial primitives.
func (q *MixedRoofMaterialQuery) correlatedColumnPrimitives(c mixedRoofVolumeCell, height [2]geom.ExactParameterInterval, cache roofOrderCache, w *mixedRoofLineWork) (roof, floor [2]int, proven bool, err error) {
	roof, floor, proven = q.fixedColumnPrimitives(c.chart, height)
	if proven || !c.chart[0] || !c.chart[1] {
		return
	}
	top := maxRat(q.box[0].floor, q.box[1].floor)
	if height[0].Lower.Cmp(top) < 0 || height[1].Lower.Cmp(top) < 0 {
		return
	}
	order, err := cache.order(c.roof, c.xy, w)
	if err != nil {
		return roof, floor, false, err
	}
	roof, floor, proven = q.columnPrimitivesWithOrder(c.chart, height, order)
	return
}

// A lower column uses the lowest allowed roofs of the selected sources and
// the highest subtracting roof. The upper column swaps those choices. This
// is a pointwise set sandwich, including at all ties and empty columns.
func mixedRoofColumnSandwich(chart [2]bool, op parametric.BooleanOp, height [2]geom.ExactParameterInterval, floor [2]*big.Rat) (low, high []geom.ExactParameterInterval) {
	column := func(upper bool) []geom.ExactParameterInterval {
		a, b := height[0].Lower, height[1].Lower
		if upper {
			a, b = height[0].Upper, height[1].Upper
		}
		if op == parametric.Difference {
			b = height[1].Upper
			if upper {
				b = height[1].Lower
			}
		}
		var out []geom.ExactParameterInterval
		add := func(l, u *big.Rat) {
			if l.Cmp(u) < 0 {
				out = append(out, geom.ExactParameterInterval{Lower: l, Upper: u})
			}
		}
		switch op {
		case parametric.Union:
			if chart[0] && chart[1] && minRat(a, b).Cmp(maxRat(floor[0], floor[1])) >= 0 {
				add(minRat(floor[0], floor[1]), maxRat(a, b))
				break
			}
			if chart[0] {
				add(floor[0], a)
			}
			if chart[1] {
				add(floor[1], b)
			}
		case parametric.Intersection:
			if chart[0] && chart[1] {
				add(maxRat(floor[0], floor[1]), minRat(a, b))
			}
		case parametric.Difference:
			if chart[0] && chart[1] && floor[1].Cmp(b) < 0 {
				add(floor[0], minRat(a, floor[1]))
				add(maxRat(floor[0], b), a)
			} else if chart[0] {
				add(floor[0], a)
			}
		}
		return out
	}
	return column(false), column(true)
}
