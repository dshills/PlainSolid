package trimmed

import (
	"math/big"
	"plainsolid/geom"
)

// Empty is true only for a successfully proved empty directed Difference.
// A nil pointer is not a proof and returns false.
func (r *WrappedFaceRegionTopology) Empty() bool { return r != nil && len(r.faces) == 0 }

// BoundaryCycles returns the oriented exterior first, followed by its optional
// hole. The hole direction is opposite to the exterior, including when A is
// reversed. Empty Difference has no loops. Containers are independent copies.
func (r *WrappedFaceRegionTopology) BoundaryCycles() [][]Use {
	if r == nil {
		return nil
	}
	out := make([][]Use, len(r.boundaries))
	for i, c := range r.boundaries {
		out[i] = append([]Use(nil), c...)
	}
	return out
}

// Finite orthogonal cycle validation uses exact represented-coordinate order;
// all area arithmetic is rational. No tolerance, sample fitting, or arbitrary
// winding inference is used. Callers have already proved incidence and source
// embedding. This additionally rejects self/inter-cycle crossings and touches.
func validateWrappedBoundaryCycles(cycles [][]Use, reversed bool) ([][]Use, error) {
	if len(cycles) < 1 || len(cycles) > 2 {
		return nil, geom.ErrConvergence
	}
	type segment struct {
		a, b         geom.Vec2
		cycle, index int
	}
	var segments []segment
	outer := -1
	for ci, cycle := range cycles {
		if len(cycle) < 4 || len(cycle) > 36 {
			return nil, geom.ErrConvergence
		}
		area := new(big.Rat)
		for i, u := range cycle {
			a, b := u.UVStart, u.UVEnd
			if !uvFinite(a) || !uvFinite(b) || a == b || (a.X != b.X && a.Y != b.Y) || b != cycle[(i+1)%len(cycle)].UVStart {
				return nil, geom.ErrConvergence
			}
			area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(ratInset(a.X), ratInset(b.Y)), new(big.Rat).Mul(ratInset(a.Y), ratInset(b.X))))
			segments = append(segments, segment{a, b, ci, i})
		}
		sign := area.Sign()
		if reversed {
			sign = -sign
		}
		if sign == 0 {
			return nil, geom.ErrConvergence
		}
		if sign > 0 {
			if outer >= 0 {
				return nil, geom.ErrConvergence
			}
			outer = ci
		}
	}
	if outer < 0 {
		return nil, geom.ErrConvergence
	}
	bounds := func(a, b float64) (float64, float64) {
		if a > b {
			return b, a
		}
		return a, b
	}
	for i, p := range segments {
		for _, q := range segments[i+1:] {
			px0, px1 := bounds(p.a.X, p.b.X)
			py0, py1 := bounds(p.a.Y, p.b.Y)
			qx0, qx1 := bounds(q.a.X, q.b.X)
			qy0, qy1 := bounds(q.a.Y, q.b.Y)
			x0, x1 := max(px0, qx0), min(px1, qx1)
			y0, y1 := max(py0, qy0), min(py1, qy1)
			if x0 > x1 || y0 > y1 {
				continue
			}
			// Axis-aligned segment AABBs are exact segments, so intersection is exact.
			adjacent := p.cycle == q.cycle && ((p.index+1)%len(cycles[p.cycle]) == q.index || (q.index+1)%len(cycles[p.cycle]) == p.index)
			if !adjacent || x0 != x1 || y0 != y1 {
				return nil, geom.ErrConvergence
			}
		}
	}
	// A hole vertex must lie strictly inside the exterior. All intersections
	// were excluded above, so this one exact parity witness binds the whole loop.
	if len(cycles) == 2 {
		hole := outer ^ 1
		q := cycles[hole][0].UVStart
		inside := false
		for _, u := range cycles[outer] {
			a, b := u.UVStart, u.UVEnd
			if a.X != b.X {
				continue
			}
			lo, hi := bounds(a.Y, b.Y)
			if lo <= q.Y && q.Y < hi && a.X > q.X {
				inside = !inside
			}
		}
		if !inside {
			return nil, geom.ErrConvergence
		}
	}
	out := make([][]Use, 0, len(cycles))
	out = append(out, cycles[outer])
	for i, c := range cycles {
		if i != outer {
			out = append(out, c)
		}
	}
	return out, nil
}
