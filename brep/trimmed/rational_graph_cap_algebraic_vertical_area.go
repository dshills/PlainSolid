package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphVerticalArea is the total boundary area of the prepared
// stock, with cap intervals and the sum of original vertical wall intervals.
type AlgebraicGraphVerticalArea struct {
	Interval                      geom.ExactParameterInterval
	Caps                          [2]geom.ExactParameterInterval
	Walls                         geom.ExactParameterInterval
	OriginalWalls                 []AlgebraicGraphWallArea
	ExactOperations, Subdivisions int
}

// AlgebraicGraphWallArea identifies one retained lower cap trim and the
// original stock face swept from it. A face may have several disjoint trims.
type AlgebraicGraphWallArea struct {
	Face, Wire, Use int
	Area            geom.ExactParameterInterval
}

// BoundSurfaceArea measures the two exact cap masks and every retained
// original vertical wall. Each wall is the complete sweep between the cap
// heights along one independently proved lower-cap trim occurrence. No
// generated cap seam contributes wall area.
func (m AlgebraicGraphVerticalMaterial) BoundSurfaceArea(ctx context.Context, width *big.Rat,
	o geom.ExactCurvePlaneEventOptions) (AlgebraicGraphVerticalArea, error) {
	var zero AlgebraicGraphVerticalArea
	b, e := newCapClipBudget(ctx, o)
	if e != nil {
		return zero, e
	}
	if m.caps[0].recipe == nil || width == nil || width.Sign() <= 0 {
		return zero, geom.ErrInvalid
	}
	if !m.lineReady {
		return zero, geom.ErrUnrepresentable
	}
	if max(width.Num().BitLen(), width.Denom().BitLen()) > b.bits {
		return zero, geom.ErrConvergence
	}
	work, splits := b.work, b.subdiv
	blank := func() geom.ExactParameterInterval {
		return geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
	}
	out := AlgebraicGraphVerticalArea{Interval: blank(), Walls: blank()}
	part := new(big.Rat).Quo(width, big.NewRat(3, 1))
	for i, cap := range m.caps {
		axes := orientedRationalGraphProjection(cap.regions.plane)
		count := 0
		hull := new(big.Rat)
		for _, row := range cap.regions.wires {
			for _, edge := range row {
				for _, h := range edge.control {
					for k := 0; k < 2; k++ {
						value := new(big.Rat).Abs(new(big.Rat).Quo(h[k], h[2]))
						if value.Cmp(hull) > 0 {
							hull.Set(value)
						}
					}
					if e := b.charge(8, 0); e != nil {
						return zero, e
					}
				}
			}
		}
		for _, row := range cap.wires {
			count += len(row)
		}
		if count == 0 {
			return zero, geom.ErrConvergence
		}
		g := new(big.Rat).Mul(hull, hull)
		g.Mul(g, big.NewRat(4, 1)).Add(g, big.NewRat(1, 1))
		norm := new(big.Rat)
		for _, q := range cap.regions.plane.normal {
			norm.Add(norm, new(big.Rat).Mul(q, q))
		}
		slo, shi, e := stockAreaProjectionScale(norm, g, part, &b)
		if e != nil {
			return zero, e
		}
		l := new(big.Rat).Quo(part, new(big.Rat).Mul(shi, big.NewRat(int64(count*4), 1)))
		a := blank()
		for _, row := range cap.wires {
			for _, edge := range row {
				v, e := edge.Cell.BoundProjectedGreen(ctx, axes, l, b.limits())
				if e != nil {
					return zero, e
				}
				if e = b.charge(v.Operations, v.Subdivisions); e != nil {
					return zero, e
				}
				if edge.Reversed {
					a.Lower.Sub(a.Lower, v.Interval.Upper)
					a.Upper.Sub(a.Upper, v.Interval.Lower)
				} else {
					a.Lower.Add(a.Lower, v.Interval.Lower)
					a.Upper.Add(a.Upper, v.Interval.Upper)
				}
				if !graphStockMomentFits(a, b.bits) {
					return zero, geom.ErrConvergence
				}
			}
		}
		if a.Lower.Sign() < 0 {
			a.Lower.SetInt64(0)
		}
		a.Lower.Mul(a.Lower, slo)
		a.Upper.Mul(a.Upper, shi)
		if a.Lower.Cmp(a.Upper) > 0 || new(big.Rat).Sub(a.Upper, a.Lower).Cmp(part) > 0 || !graphStockMomentFits(a, b.bits) {
			return zero, geom.ErrConvergence
		}
		out.Caps[i] = a
	}
	base, top := m.caps[m.lower], m.caps[m.lower^1]
	l, u := base.regions.plane, top.regions.plane
	height := [3]*big.Rat{new(big.Rat).Sub(u.offset, l.offset), new(big.Rat).Sub(l.normal[0], u.normal[0]), new(big.Rat).Sub(l.normal[1], u.normal[1])}
	n := 0
	for _, row := range base.wires {
		for _, edge := range row {
			if edge.Seam < 0 {
				n++
			}
		}
	}
	if n < 1 {
		return zero, geom.ErrConvergence
	}
	w := new(big.Rat).Quo(part, big.NewRat(int64(n), 1))
	for wi, row := range base.wires {
		for ui, edge := range row {
			if edge.Seam >= 0 {
				continue
			}
			if edge.Source.Face < 0 {
				return zero, geom.ErrConvergence
			}
			v, e := edge.Cell.BoundVerticalSweepArea(ctx, [2]int{0, 1}, height, w, b.limits())
			if e != nil {
				return zero, e
			}
			if e = b.charge(v.Operations, v.Subdivisions); e != nil {
				return zero, e
			}
			out.OriginalWalls = append(out.OriginalWalls, AlgebraicGraphWallArea{Face: edge.Source.Face, Wire: wi, Use: ui, Area: v.Interval})
			out.Walls.Lower.Add(out.Walls.Lower, v.Interval.Lower)
			out.Walls.Upper.Add(out.Walls.Upper, v.Interval.Upper)
			if !graphStockMomentFits(out.Walls, b.bits) {
				return zero, geom.ErrConvergence
			}
		}
	}
	for _, v := range [3]geom.ExactParameterInterval{out.Caps[0], out.Caps[1], out.Walls} {
		out.Interval.Lower.Add(out.Interval.Lower, v.Lower)
		out.Interval.Upper.Add(out.Interval.Upper, v.Upper)
	}
	if new(big.Rat).Sub(out.Interval.Upper, out.Interval.Lower).Cmp(width) > 0 ||
		!graphStockMomentFits(out.Interval, b.bits) {
		return zero, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.ExactOperations, out.Subdivisions = work-b.work, splits-b.subdiv
	return out, nil
}
