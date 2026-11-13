package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

// ExtrusionRectangle is an axis-aligned filled rectangular footprint in
// sketch coordinates, used as a through pocket.
type ExtrusionRectangle struct{ Min, Max geom.Vec2 }

func extrusionRectangleRing(r ExtrusionRectangle) []geom.Vec2 {
	return []geom.Vec2{r.Min, {X: r.Min.X, Y: r.Max.Y}, r.Max, {X: r.Max.X, Y: r.Min.Y}}
}

func proveRectangularExtrusionHoles(outer []geom.Vec2, rectangles []ExtrusionRectangle, circles []ExtrusionCircle) ([][]geom.Vec2, error) {
	if len(rectangles) > 16 {
		return nil, geom.ErrInvalid
	}
	var rings [][]geom.Vec2
	for i, r := range rectangles {
		if !uvFinite(r.Min) || !uvFinite(r.Max) || r.Min.X >= r.Max.X || r.Min.Y >= r.Max.Y {
			return nil, geom.ErrInvalid
		}
		// Inner cap loops are clockwise.
		ring := extrusionRectangleRing(r)
		for _, p := range ring {
			for j, a := range outer {
				sign, e := geom.Orient2D(a, outer[(j+1)%len(outer)], p)
				if e != nil {
					return nil, e
				}
				if sign <= 0 {
					return nil, geom.ErrDomain
				}
			}
		}
		for _, q := range rectangles[:i] {
			if r.Min.X <= q.Max.X && q.Min.X <= r.Max.X && r.Min.Y <= q.Max.Y && q.Min.Y <= r.Max.Y {
				return nil, geom.ErrDomain
			}
		}
		for _, c := range circles {
			closest := geom.Vec2{X: max(r.Min.X, min(r.Max.X, c.Center.X)), Y: max(r.Min.Y, min(r.Max.Y, c.Center.Y))}
			radius := ratInset(c.Radius)
			if extrusionCenterDistanceSquared(c, ExtrusionCircle{Center: closest}).Cmp(new(big.Rat).Mul(radius, radius)) <= 0 {
				return nil, geom.ErrDomain
			}
		}
		rings = append(rings, ring)
	}
	return rings, nil
}
