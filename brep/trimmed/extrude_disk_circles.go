package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// ExtrudeDiskWithHoles sweeps a circular outer sketch with optional eccentric
// circular bores and blind pockets. Height is signed along the sketch frame Z
// axis; each pocket starts on the original sketch plane. The caps retain
// circular trims and the swept walls remain analytic cylinders. The outer disk
// and all inner disks must be strictly nested and mutually disjoint as
// appropriate, with no tangencies, in represented sketch coordinates.
// The constructor uses the same model tolerance as ExtrudePolygonWithHoles.
func ExtrudeDiskWithHoles(ctx context.Context, frame geom.Frame, outer ExtrusionCircle, through []ExtrusionCircle, blind []ExtrusionBlindHole, height float64) (Solid, error) {
	if !uvFinite(outer.Center) || !finite(outer.Radius) || outer.Radius <= 0 {
		return Solid{}, geom.ErrInvalid
	}
	holes, e := validateExtrusionHoles(ctx, frame, through, blind, height)
	if e != nil {
		return Solid{}, e
	}
	for _, hole := range holes {
		clear := new(big.Rat).Sub(ratInset(outer.Radius), ratInset(hole.Radius))
		if clear.Sign() <= 0 || extrusionCenterDistanceSquared(outer, hole).Cmp(clear.Mul(clear, clear)) >= 0 {
			return Solid{}, geom.ErrDomain
		}
	}
	return extrudeCircularTopology(ctx, frame, nil, &outer, nil, nil, nil, through, blind, min(0, height), max(0, height))
}

func validateExtrusionHoles(ctx context.Context, frame geom.Frame, through []ExtrusionCircle, blind []ExtrusionBlindHole, height float64) ([]ExtrusionCircle, error) {
	if ctx == nil || frame.ZAxis().Length() == 0 || !finite(height) || height == 0 || len(through) > 16 || len(blind) > 16-len(through) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	holes := append([]ExtrusionCircle(nil), through...)
	for _, h := range blind {
		if !finite(h.Depth) || h.Depth <= 0 || h.Depth >= math.Abs(height) {
			return nil, geom.ErrDomain
		}
		holes = append(holes, h.ExtrusionCircle)
	}
	for i, h := range holes {
		if !uvFinite(h.Center) || !finite(h.Radius) || h.Radius <= 0 {
			return nil, geom.ErrInvalid
		}
		for _, g := range holes[:i] {
			r := new(big.Rat).Add(ratInset(h.Radius), ratInset(g.Radius))
			if extrusionCenterDistanceSquared(h, g).Cmp(r.Mul(r, r)) <= 0 {
				return nil, geom.ErrDomain
			}
		}
	}
	return holes, nil
}

func extrusionCenterDistanceSquared(h, g ExtrusionCircle) *big.Rat {
	x := new(big.Rat).Sub(ratInset(h.Center.X), ratInset(g.Center.X))
	y := new(big.Rat).Sub(ratInset(h.Center.Y), ratInset(g.Center.Y))
	return x.Mul(x, x).Add(x, y.Mul(y, y))
}

func extrusionArc(h ExtrusionCircle, i int, a, b geom.Vec2, clockwise bool) (anchoredCircularUV, error) {
	sine := h.Radius
	if clockwise {
		sine = -sine
	}
	return newAnchoredCircularUV(EllipseUV{Center: h.Center, Cos: geom.Vec2{X: h.Radius}, Sin: geom.Vec2{Y: sine},
		From: float64(i) * math.Pi / 2, To: float64(i+1) * math.Pi / 2}, a, b)
}
