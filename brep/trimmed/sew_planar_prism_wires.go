package trimmed

import (
	"math"
	"math/big"

	"plainsolid/geom"
)

func prismMissesPlanarWires(f Face, frame geom.Frame, world []geom.Vec3, disk *prismDisk, pad float64) bool {
	if !prismCardinal(frame) {
		return false
	}
	profile := disk.inFrame(frame)
	if profile != nil && prismDiskMissesWire(*profile, f.Outer, pad) {
		return true
	}
	corners := make([]geom.Vec2, 0, len(world))
	for _, p := range world {
		local, err := prismExactLocal(frame, p)
		if err != nil {
			return false
		}
		corners = append(corners, geom.Vec2{X: local.X, Y: local.Y})
	}
	for _, hole := range f.Holes {
		if profile != nil {
			if c, ok := prismWholeCircle(hole); ok && profile.inside(c.Center, ratInset(c.Cos.X), pad) {
				return true
			}
		}
		if prismInsideConvexHole(hole, corners, pad) {
			return true
		}
	}
	return false
}

func prismInsideConvexHole(hole []Use, corners []geom.Vec2, pad float64) bool {
	if len(hole) < 3 || len(hole) > 64 {
		return false
	}
	for _, u := range hole {
		if u.Curve != nil {
			arc, ok := prismCircleCurve(u.Curve)
			if !ok || arc.Linear != (geom.Vec2{}) || arc.Cos.X <= 0 || arc.Cos.Y != 0 || arc.Sin.X != 0 ||
				math.Abs(arc.Sin.Y) != arc.Cos.X || (arc.To-arc.From)*arc.Sin.Y >= 0 || math.Abs(arc.To-arc.From) > math.Pi/2 {
				return false
			}
		}
		for _, v := range hole {
			turn, err := geom.Orient2D(u.UVEnd, u.UVStart, v.UVStart)
			if err != nil || turn < 0 {
				return false
			}
		}
		for _, p := range corners {
			point := prismCircle{center: p}
			if !point.leftOf(u.UVEnd, u.UVStart, pad) {
				return false
			}
		}
	}
	return true
}

// Separate the disk from the convex hull of the boundary segments and the
// full support disks of its arcs. The tested directions are sufficient for
// axis-aligned rounded corners and are conservative for other trims.
func prismDiskMissesWire(disk prismCircle, wire []Use, pad float64) bool {
	type part struct {
		center geom.Vec2
		radius float64
	}
	parts := make([]part, 0, 2*len(wire))
	for _, use := range wire {
		parts = append(parts, part{center: use.UVStart}, part{center: use.UVEnd})
		if use.Curve == nil {
			continue
		}
		c, ok := prismCircleCurve(use.Curve)
		if !ok || c.Linear != (geom.Vec2{}) || c.Cos.X <= 0 || c.Cos.Y != 0 || c.Sin.X != 0 || math.Abs(c.Sin.Y) != c.Cos.X {
			return false
		}
		parts = append(parts, part{center: c.Center, radius: c.Cos.X})
	}
	for _, n := range []geom.Vec2{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1}, {X: 1, Y: -1}, {X: -1, Y: -1}} {
		norm2 := ratInset(n.X*n.X + n.Y*n.Y)
		separate := true
		for _, p := range parts {
			dx := new(big.Rat).Sub(ratInset(disk.center.X), ratInset(p.center.X))
			dy := new(big.Rat).Sub(ratInset(disk.center.Y), ratInset(p.center.Y))
			dot := new(big.Rat).Add(dx.Mul(dx, ratInset(n.X)), dy.Mul(dy, ratInset(n.Y)))
			r := new(big.Rat).Add(ratInset(disk.radius), ratInset(p.radius))
			r.Add(r, ratInset(pad))
			if dot.Sign() <= 0 || dot.Mul(dot, dot).Cmp(r.Mul(r, r).Mul(r, norm2)) <= 0 {
				separate = false
				break
			}
		}
		if separate {
			return true
		}
	}
	return false
}
