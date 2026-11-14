package trimmed

import (
	"math"

	"plainsolid/geom"
)

// compactObliqueBand recognizes a cylinder chart whose two rims are graphs
// of axial height over the same angular interval. A shared angular partition
// keeps the strip edges parallel to the cylinder axis.
func compactObliqueBand(f Face) bool {
	if _, ok := f.Surface.(geom.CylinderSurface); !ok || len(f.Holes) != 0 || len(f.Outer) != 4 {
		return false
	}
	a, b, c, d := f.Outer[0], f.Outer[1], f.Outer[2], f.Outer[3]
	if b.Curve != nil || d.Curve != nil ||
		a.UVStart.X >= a.UVEnd.X || a.UVEnd.X-a.UVStart.X > math.Pi/2 ||
		a.UVStart.X != c.UVEnd.X || a.UVEnd.X != c.UVStart.X ||
		b.UVStart.X != b.UVEnd.X || d.UVStart.X != d.UVEnd.X ||
		a.UVStart.Y >= c.UVEnd.Y || a.UVEnd.Y >= c.UVStart.Y {
		return false
	}
	var rims [2]EllipseUV
	for i, use := range [2]Use{a, c} {
		rim, ok := use.Curve.(EllipseUV)
		if !ok || rim.Linear != (geom.Vec2{X: 1}) || rim.Center.X != 0 ||
			rim.Cos.X != 0 || rim.Sin.X != 0 || rim.From != use.UVStart.X || rim.To != use.UVEnd.X ||
			!finite(rim.Center.Y) || !finite(rim.Cos.Y) || !finite(rim.Sin.Y) {
			return false
		}
		rims[i] = rim
	}
	lower, upper := rims[0], rims[1]
	gap := upper.Center.Y - lower.Center.Y
	reach := math.Hypot(upper.Cos.Y-lower.Cos.Y, upper.Sin.Y-lower.Sin.Y)
	scale := max(1, math.Abs(lower.Center.Y), math.Abs(upper.Center.Y),
		math.Hypot(lower.Cos.Y, lower.Sin.Y), math.Hypot(upper.Cos.Y, upper.Sin.Y))
	return finite(gap) && finite(reach) && gap-reach > 64*0x1p-52*scale
}
