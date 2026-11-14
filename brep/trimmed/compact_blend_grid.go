package trimmed

import (
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func compactTorus(f Face) bool {
	base, _ := parametric.SurfaceMapping(f.Surface)
	if _, ok := base.(geom.TorusSurface); !ok || len(f.Outer) != 4 || len(f.Holes) != 0 {
		return false
	}
	for _, u := range f.Outer {
		if u.Curve != nil {
			return false
		}
	}
	a, b, c, d := f.Outer[0].UVStart, f.Outer[1].UVStart, f.Outer[2].UVStart, f.Outer[3].UVStart
	return a.X < b.X && a.Y < d.Y && a.Y == b.Y && b.X == c.X && c.Y == d.Y && d.X == a.X &&
		b.X-a.X <= math.Pi/2 && d.Y-a.Y <= math.Pi/2
}

func compactTorusGrid(outer [][]geom.Vec2, vertex func(geom.Vec2) error) ([][3]geom.Vec2, error) {
	u, v := len(outer[0])-1, len(outer[1])-1
	if len(outer[2]) != u+1 || len(outer[3]) != v+1 {
		return nil, geom.ErrDomain
	}
	grid := make([][]geom.Vec2, v+1)
	for j := range grid {
		grid[j] = make([]geom.Vec2, u+1)
		for i := range grid[j] {
			switch {
			case j == 0:
				grid[j][i] = outer[0][i]
			case j == v:
				grid[j][i] = outer[2][u-i]
			case i == 0:
				grid[j][i] = outer[3][v-j]
			case i == u:
				grid[j][i] = outer[1][j]
			default:
				p := geom.Vec2{X: outer[0][i].X, Y: outer[1][j].Y}
				if err := vertex(p); err != nil {
					return nil, err
				}
				grid[j][i] = p
			}
		}
	}
	triangles := make([][3]geom.Vec2, 0, 2*u*v)
	for j := range v {
		for i := range u {
			a, b, c, d := grid[j][i], grid[j][i+1], grid[j+1][i+1], grid[j+1][i]
			triangles = append(triangles, [3]geom.Vec2{a, b, c}, [3]geom.Vec2{a, c, d})
		}
	}
	return triangles, nil
}
