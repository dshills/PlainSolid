package trimmed

import (
	"math"
	"plainsolid/geom"
)

// compactCosineBand qualifies a cylinder quarter chart between two affine
// cosine-height rims. Their difference is monotone on this angular interval,
// so positive endpoint gaps prove the entire strip remains noncollapsed.
func compactCosineBand(f Face) bool {
	if _, ok := f.Surface.(geom.CylinderSurface); !ok || len(f.Holes) != 0 || len(f.Outer) != 4 {
		return false
	}
	a, b, c, d := f.Outer[0], f.Outer[1], f.Outer[2], f.Outer[3]
	if b.Curve != nil || d.Curve != nil || b.UVStart.X != b.UVEnd.X || d.UVStart.X != d.UVEnd.X || a.UVStart.X >= a.UVEnd.X || a.UVStart.X != c.UVEnd.X || a.UVEnd.X != c.UVStart.X {
		return false
	}
	q := math.Floor(a.UVStart.X / (math.Pi / 2))
	if a.UVEnd.X > (q+1)*(math.Pi/2) {
		return false
	}
	var e [2]EllipseUV
	for i, u := range []Use{a, c} {
		p, ok := u.Curve.(unionArcUV)
		if !ok {
			return false
		}
		v := p.EllipseUV
		if v.Center.X != 0 || !finite(v.Center.Y) || v.Linear != (geom.Vec2{X: 1}) || v.Cos.X != 0 || !finite(v.Cos.Y) || v.Sin != (geom.Vec2{}) || v.From != u.UVStart.X || v.To != u.UVEnd.X {
			return false
		}
		e[i] = v
	}
	if a.UVStart.Y >= c.UVEnd.Y || a.UVEnd.Y >= c.UVStart.Y {
		return false
	}
	for _, u := range []float64{a.UVStart.X, a.UVEnd.X} {
		gap := e[1].Center.Y - e[0].Center.Y + (e[1].Cos.Y-e[0].Cos.Y)*math.Cos(u)
		if !finite(gap) || gap <= 0 {
			return false
		}
	}
	return true
}
