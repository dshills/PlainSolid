package trimmed

import (
	"math"
	"plainsolid/geom"
	"sort"
)

// compactCrossedStrip recognizes a quarter-cylinder chart with one cosine rim
// and one constant-height rim. The quarter domain makes the rim monotone.
func compactCrossedStrip(f Face) bool {
	if compactCosineBand(f) {
		return true
	}
	if compactSplitAxialRectangle(f) || compactTangentCrossedTriangle(f) {
		return true
	}
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
	q := math.Floor(a.UVStart.X / (math.Pi / 2))
	if a.UVEnd.X > (q+1)*(math.Pi/2) || a.UVEnd.X-a.UVStart.X > math.Pi/2 {
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
func compactSplitAxialRectangle(f Face) bool {
	if _, ok := f.Surface.(geom.CylinderSurface); !ok || len(f.Holes) != 0 || len(f.Outer) < 5 || len(f.Outer) > 8 {
		return false
	}
	a := f.Outer[0]
	if a.Curve != nil || a.UVStart.Y != a.UVEnd.Y || a.UVStart.X >= a.UVEnd.X || a.UVEnd.X-a.UVStart.X > math.Pi/2 {
		return false
	}
	left, right, low := a.UVStart.X, a.UVEnd.X, a.UVStart.Y
	high := low
	stage := 0
	for _, u := range f.Outer[1:] {
		if u.Curve != nil {
			return false
		}
		p, q := u.UVStart, u.UVEnd
		switch stage {
		case 0:
			if p.X == right && q.X == right && q.Y > p.Y {
				high = q.Y
				continue
			}
			if p.Y != high || q.Y != high || p.X != right || q.X != left || high <= low {
				return false
			}
			stage = 1
		case 1:
			if p.X != left || q.X != left || q.Y >= p.Y || q.Y < low {
				return false
			}
		default:
			return false
		}
	}
	return stage == 1 && f.Outer[len(f.Outer)-1].UVEnd == a.UVStart
}
func compactCrossedStripTriangles(outer [][]geom.Vec2) ([][3]geom.Vec2, error) {
	var points []geom.Vec2
	for _, edge := range outer {
		if len(edge) < 2 {
			return nil, geom.ErrDomain
		}
		points = append(points, edge[:len(edge)-1]...)
	}
	sort.Slice(points, func(i, j int) bool {
		if points[i].X != points[j].X {
			return points[i].X < points[j].X
		}
		return points[i].Y < points[j].Y
	})
	var columns [][]geom.Vec2
	for _, p := range points {
		if len(columns) == 0 || math.Abs(p.X-columns[len(columns)-1][0].X) > 64*0x1p-52*max(1, math.Abs(p.X)) {
			columns = append(columns, []geom.Vec2{p})
		} else {
			columns[len(columns)-1] = append(columns[len(columns)-1], p)
		}
	}
	if len(columns) < 2 {
		return nil, geom.ErrDomain
	}
	for k, c := range columns {
		sort.Slice(c, func(i, j int) bool { return c[i].Y < c[j].Y })
		if len(c) < 1 || len(c) == 1 && k != 0 && k != len(columns)-1 {
			return nil, geom.ErrDomain
		}
	}
	tris := make([][3]geom.Vec2, 0, len(points)-2)
	for k := 0; k < len(columns)-1; k++ {
		a, b := columns[k], columns[k+1]
		i, j := 0, 0
		for i < len(a)-1 || j < len(b)-1 {
			if j == len(b)-1 || (i < len(a)-1 && a[i+1].Y < b[j+1].Y) {
				tris = append(tris, [3]geom.Vec2{a[i], b[j], a[i+1]})
				i++
			} else {
				tris = append(tris, [3]geom.Vec2{a[i], b[j], b[j+1]})
				j++
			}
		}
	}
	return tris, nil
}

// compactTangentCrossedTriangle has one collapsed axial endpoint where the
// cosine intersection rim meets a constant floor rim tangentially.
func compactTangentCrossedTriangle(f Face) bool {
	if _, ok := f.Surface.(geom.CylinderSurface); !ok || len(f.Outer) != 3 || len(f.Holes) != 0 {
		return false
	}
	curved := f.Outer[0]
	p, ok := curved.Curve.(unionArcUV)
	if !ok {
		return false
	}
	e := p.EllipseUV
	if curved.UVStart.X >= curved.UVEnd.X || e.Center != (geom.Vec2{}) || e.Linear != (geom.Vec2{X: 1}) || e.Cos.X != 0 || e.Sin != (geom.Vec2{}) || !finite(e.Cos.Y) || e.From != curved.UVStart.X || e.To != curved.UVEnd.X {
		return false
	}
	q := math.Floor(curved.UVStart.X / (math.Pi / 2))
	if curved.UVEnd.X > (q+1)*(math.Pi/2) {
		return false
	}
	var top Use
	found := false
	for _, u := range f.Outer[1:] {
		if u.Curve != nil {
			return false
		}
		if u.UVStart.Y == u.UVEnd.Y && u.UVStart.X > u.UVEnd.X {
			if found {
				return false
			}
			top = u
			found = true
		} else if u.UVStart.X != u.UVEnd.X {
			return false
		}
	}
	if !found || top.UVStart.X != curved.UVEnd.X || top.UVEnd.X != curved.UVStart.X {
		return false
	}
	left, right := top.UVStart.Y-curved.UVStart.Y, top.UVStart.Y-curved.UVEnd.Y
	return left == 0 && right > 0 || right == 0 && left > 0
}
