package trimmed

import (
	"math"
	"plainsolid/geom"
)

// compactCrossedStrip recognizes a quarter-cylinder chart with one cosine rim
// and one constant-height rim. The quarter domain makes the rim monotone.
func compactCrossedStrip(f Face) bool {
	if _, ok := f.Surface.(geom.CylinderSurface); !ok || len(f.Outer) != 4 || len(f.Holes) != 0 {
		return false
	}
	a, b, c, d := f.Outer[0], f.Outer[1], f.Outer[2], f.Outer[3]
	if b.Curve != nil || d.Curve != nil || b.UVStart.X != b.UVEnd.X || d.UVStart.X != d.UVEnd.X || a.UVStart.X >= a.UVEnd.X || a.UVEnd.X != c.UVStart.X || a.UVStart.X != c.UVEnd.X {
		return false
	}
	if a.UVStart.Y >= c.UVEnd.Y || a.UVEnd.Y >= c.UVStart.Y {
		return false
	}
	q := a.UVStart.X / (math.Pi / 2)
	if q != math.Trunc(q) || a.UVEnd.X != (q+1)*(math.Pi/2) {
		return false
	}
	curved := 0
	for _, u := range []Use{a, c} {
		if u.Curve == nil {
			if u.UVStart.Y != u.UVEnd.Y {
				return false
			}
			continue
		}
		p, ok := u.Curve.(unionArcUV)
		if !ok {
			return false
		}
		e := p.EllipseUV
		if e.Center != (geom.Vec2{}) || e.Linear != (geom.Vec2{X: 1}) || e.Cos.X != 0 || e.Sin != (geom.Vec2{}) || !finite(e.Cos.Y) || e.From != u.UVStart.X || e.To != u.UVEnd.X {
			return false
		}
		curved++
	}
	return curved == 1
}
func compactCrossedStripTriangles(outer [][]geom.Vec2) ([][3]geom.Vec2, error) {
	if len(outer) != 4 || len(outer[0]) != len(outer[2]) || len(outer[1]) != 2 || len(outer[3]) != 2 {
		return nil, geom.ErrDomain
	}
	lo, hi := outer[0], outer[2]
	n := len(lo) - 1
	tris := make([][3]geom.Vec2, 0, 2*n)
	for i := 0; i < n; i++ {
		j := n - i
		if math.Abs(lo[i].X-hi[j].X) > 64*0x1p-52*max(1, math.Abs(lo[i].X)) {
			return nil, geom.ErrUnrepresentable
		}
		tris = append(tris, [3]geom.Vec2{lo[i], lo[i+1], hi[j-1]}, [3]geom.Vec2{lo[i], hi[j-1], hi[j]})
	}
	return tris, nil
}
