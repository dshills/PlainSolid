package trimmed

import (
	"math"
	"math/big"

	"plainsolid/geom"
)

// The chord polygon of a complete analytic circle may exclude a hole
// strictly inside the actual circle. Prove the native disk arrangement
// before selecting any sampled triangulation.
func certifyAnalyticCircularCap(f Face) bool {
	if _, ok := f.Surface.(geom.PlaneSurface); !ok {
		return false
	}
	outer, ok := capSeedCircle(f.Outer)
	if !ok || len(f.Holes) > 16 {
		return false
	}
	holes := make([]ExtrusionCircle, 0, len(f.Holes))
	for _, wire := range f.Holes {
		hole, ok := capSeedCircle(wire)
		if !ok {
			return false
		}
		gap := new(big.Rat).Sub(ratInset(outer.Radius), ratInset(hole.Radius))
		if gap.Sign() <= 0 || extrusionCenterDistanceSquared(outer, hole).Cmp(gap.Mul(gap, gap)) >= 0 {
			return false
		}
		for _, prior := range holes {
			sum := new(big.Rat).Add(ratInset(prior.Radius), ratInset(hole.Radius))
			if extrusionCenterDistanceSquared(prior, hole).Cmp(sum.Mul(sum, sum)) <= 0 {
				return false
			}
		}
		holes = append(holes, hole)
	}
	return true
}

// Authenticates the four original, consecutively directed analytic
// quadrants and their represented cardinal endpoints.
func capSeedCircle(wire []Use) (ExtrusionCircle, bool) {
	if len(wire) != 4 {
		return ExtrusionCircle{}, false
	}
	var first anchoredCircularUV
	for i, u := range wire {
		arc, ok := u.Curve.(anchoredCircularUV)
		if !ok || arc.Start != u.UVStart || arc.End != u.UVEnd {
			return ExtrusionCircle{}, false
		}
		if _, err := newAnchoredCircularUV(arc.EllipseUV, arc.Start, arc.End); err != nil {
			return ExtrusionCircle{}, false
		}
		if i == 0 {
			first = arc
		}
		if arc.Center != first.Center || arc.Cos != first.Cos || arc.Sin != first.Sin {
			return ExtrusionCircle{}, false
		}
		next, ok := wire[(i+1)%4].Curve.(anchoredCircularUV)
		if !ok || arc.To != next.From && !(arc.To == 2*math.Pi && next.From == 0) {
			return ExtrusionCircle{}, false
		}
	}
	return ExtrusionCircle{Center: first.Center, Radius: first.Cos.X}, true
}
