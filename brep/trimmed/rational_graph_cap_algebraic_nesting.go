package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// MaterialRegions proves exact nesting of the retained boundary cycles.
// It locates independently chosen rational points of every ring against all
// other rings using transverse exact rays. A small display hole or a noisy
// representative cannot affect the result. Indices refer to the freshly
// reconstructed private wire order.
func (c AlgebraicGraphCapClip) MaterialRegions(ctx context.Context, options geom.ExactCurvePlaneEventOptions) ([]RationalGraphShellRegion, error) {
	if ctx == nil || c.regions == nil || options.MaxOperations < 0 || options.MaxSubdivisions < 0 || options.MaxCoefficientBits < 0 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if options.MaxOperations == 0 {
		options.MaxOperations = 2000000
	}
	if options.MaxSubdivisions == 0 {
		options.MaxSubdivisions = 8192
	}
	if options.MaxCoefficientBits == 0 {
		options.MaxCoefficientBits = 16384
	}
	if options.MaxCoefficientBits < 64 {
		return nil, geom.ErrInvalid
	}
	b := capClipBudget{ctx, options.MaxOperations, options.MaxSubdivisions, options.MaxCoefficientBits}
	return c.materialRegions(&b)
}

func (c AlgebraicGraphCapClip) materialRegions(b *capClipBudget) ([]RationalGraphShellRegion, error) {
	if c.separated {
		// The retained forest is the entire original certified section.
		// Its topology and order already match the preserved wires.
		regions := make([]RationalGraphShellRegion, len(c.regions.material))
		for i, r := range c.regions.material {
			if e := b.charge(1+len(r.Holes), 0); e != nil {
				return nil, e
			}
			regions[i] = RationalGraphShellRegion{Outer: r.Outer, Holes: append([]int(nil), r.Holes...)}
		}
		return regions, nil
	}

	axes := orientedRationalGraphProjection(c.regions.plane)
	seen := make([][]bool, len(c.wires))
	depth := make([]int, len(c.wires))
	for i, ring := range c.wires {
		seen[i] = make([]bool, len(c.wires))
		if len(c.wires) == 1 {
			break
		}
		if len(ring) < 2 {
			return nil, geom.ErrConvergence
		}
		sample, e := graphCapRationalWirePoint(ring[0].Cell, b)
		if e != nil {
			return nil, e
		}
		for j, other := range c.wires {
			if i == j {
				continue
			}
			in, e := graphCapWireContains(other, sample, axes, b)
			if e != nil {
				return nil, e
			}
			seen[i][j] = in
			if in {
				depth[i]++
			}
		}
	}
	outer := make(map[int]int)
	var out []RationalGraphShellRegion
	for i, d := range depth {
		if d%2 == 0 {
			outer[i] = len(out)
			out = append(out, RationalGraphShellRegion{Outer: i})
		}
	}
	for i, d := range depth {
		if d%2 == 0 {
			continue
		}
		p := -1
		for j, in := range seen[i] {
			if in && depth[j] == d-1 {
				if p >= 0 {
					return nil, geom.ErrConvergence
				}
				p = j
			}
		}
		id, ok := outer[p]
		if !ok {
			return nil, geom.ErrConvergence
		}
		out[id].Holes = append(out[id].Holes, i)
	}
	return out, b.ctx.Err()
}

func graphCapRationalWirePoint(cell *geom.ExactComposedCurveCell, b *capClipBudget) ([3]*big.Rat, error) {
	var zero [3]*big.Rat
	roots := cell.EndpointIntervals()
	left, right := roots[0].Upper, roots[1].Lower
	if left.Cmp(right) >= 0 {
		lo, _ := cell.EndpointRoot(0)
		hi, _ := cell.EndpointRoot(1)
		a, e := lo.Compare(b.ctx, hi, b.limits())
		if e != nil {
			return zero, e
		}
		if e := b.charge(a.Operations, a.Subdivisions); e != nil {
			return zero, e
		}
		if !a.Proven || a.Order != -1 {
			return zero, geom.ErrConvergence
		}
		left, right = a.Intervals[0][1], a.Intervals[1][0]
		if left.Cmp(right) > 0 {
			return zero, geom.ErrConvergence
		}
	}
	t := new(big.Rat).Quo(new(big.Rat).Add(left, right), big.NewRat(2, 1))
	p, e := cell.EvaluateRational(b.ctx, t, b.limits())
	if e != nil {
		return zero, e
	}
	if e := b.charge(p.ExactOperations, p.Subdivisions); e != nil {
		return zero, e
	}
	return p.Point, nil
}

func graphCapWireContains(wire []AlgebraicGraphCapUse, point [3]*big.Rat, axes [2]int, b *capClipBudget) (bool, error) {
	x, y := axes[0], axes[1]
	p, used, e := geom.NewExactRationalRoot(b.ctx, point[x], b.limits())
	if e != nil {
		return false, e
	}
	if e := b.charge(used, 0); e != nil {
		return false, e
	}
	for attempt := 0; attempt < 32; attempt++ {
		var plane [4]*big.Rat
		// y-y0 = slope*(x-x0), traversed in increasing x.
		slope := big.NewRat(int64(attempt*(1-2*(attempt%2))), 7)
		plane[x], plane[y] = new(big.Rat).Neg(slope), big.NewRat(1, 1)
		plane[3] = new(big.Rat).Sub(new(big.Rat).Mul(slope, point[x]), point[y])
		next := false
		crossings := 0
		for _, u := range wire {
			h, e := u.Cell.IntersectRawPlane(b.ctx, plane, b.limits())
			if e != nil {
				return false, e
			}
			if e := b.charge(h.Operations, h.Subdivisions); e != nil {
				return false, e
			}
			if !h.Complete || h.Coincident {
				next = true
				break
			}
			for _, event := range h.Events {
				if event.Multiplicity() != 1 || event.EndpointMask() != 0 {
					next = true
					break
				}
				hit, used, e := event.CoordinateRoot(b.ctx, x, b.limits())
				if e != nil {
					return false, e
				}
				if e := b.charge(used, 0); e != nil {
					return false, e
				}
				order, e := hit.Compare(b.ctx, p, b.limits())
				if e != nil {
					return false, e
				}
				if e := b.charge(order.Operations, order.Subdivisions); e != nil {
					return false, e
				}
				if !order.Proven || order.Order == 0 {
					return false, geom.ErrConvergence
				}
				if order.Order > 0 {
					crossings++
				}
			}
			if next {
				break
			}
		}
		if !next {
			return crossings%2 != 0, nil
		}
	}
	return false, geom.ErrConvergence
}
