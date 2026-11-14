package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// ExtrudeSemicircle sweeps the upper half of a disk (local Y>=center.Y)
// and its diameter. The cap retains two analytic quarter-circle trims.
// Height is signed along frame Z; all cardinal rim points must be represented
// exactly. This bounded constructor accepts no holes or arbitrary arc chains.
func ExtrudeSemicircle(ctx context.Context, frame geom.Frame, center geom.Vec2, radius, height float64) (Solid, error) {
	if _, e := validateExtrusionHoles(ctx, frame, nil, nil, height); e != nil {
		return Solid{}, e
	}
	if !uvFinite(center) || !finite(radius) || radius <= 0 {
		return Solid{}, geom.ErrInvalid
	}
	points, e := mixedCardinalPoints(center, radius)
	if e != nil {
		return Solid{}, e
	}
	circle := ExtrusionCircle{Center: center, Radius: radius}
	outer := []geom.Vec2{points[0], points[1], points[2]}
	arcs := []extrusionQuarterArc{{circle: circle, quadrant: 0}, {circle: circle, quadrant: 1}, {}}
	return extrudeCircularTopology(ctx, frame, outer, nil, arcs, nil, nil, nil, nil, min(0, height), max(0, height))
}

// ExtrudeCapsule sweeps a horizontal stadium profile: equal-radius semicircles
// centered at left/right, joined by their common upper and lower tangents.
// Centers must share represented Y and have left.X<right.X. Arbitrary world
// orientation is supplied by frame. The exact cardinal rim coordinates are
// retained; no chord polygon substitutes for the circular boundary.
func ExtrudeCapsule(ctx context.Context, frame geom.Frame, left, right geom.Vec2, radius, height float64) (Solid, error) {
	if _, e := validateExtrusionHoles(ctx, frame, nil, nil, height); e != nil {
		return Solid{}, e
	}
	if !uvFinite(left) || !uvFinite(right) || left.Y != right.Y || left.X >= right.X || !finite(radius) || radius <= 0 {
		return Solid{}, geom.ErrInvalid
	}
	a, e := mixedCardinalPoints(left, radius)
	if e != nil {
		return Solid{}, e
	}
	b, e := mixedCardinalPoints(right, radius)
	if e != nil {
		return Solid{}, e
	}
	l, r := ExtrusionCircle{Center: left, Radius: radius}, ExtrusionCircle{Center: right, Radius: radius}
	outer := []geom.Vec2{a[3], b[3], b[0], b[1], a[1], a[2]}
	arcs := []extrusionQuarterArc{{}, {circle: r, quadrant: 3}, {circle: r, quadrant: 0}, {}, {circle: l, quadrant: 1}, {circle: l, quadrant: 2}}
	return extrudeCircularTopology(ctx, frame, outer, nil, arcs, nil, nil, nil, nil, min(0, height), max(0, height))
}

func mixedCardinalPoints(center geom.Vec2, radius float64) ([4]geom.Vec2, error) {
	var points [4]geom.Vec2
	for i, d := range [4]geom.Vec2{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}} {
		x, e := geom.RepresentedExactScalar(new(big.Rat).Add(ratInset(center.X), new(big.Rat).Mul(ratInset(d.X), ratInset(radius))))
		if e != nil {
			return points, e
		}
		y, e := geom.RepresentedExactScalar(new(big.Rat).Add(ratInset(center.Y), new(big.Rat).Mul(ratInset(d.Y), ratInset(radius))))
		if e != nil {
			return points, e
		}
		points[i] = geom.Vec2{X: x, Y: y}
	}
	return points, nil
}
