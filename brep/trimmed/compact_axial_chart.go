package trimmed

import (
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// compactAxialStart qualifies a rectangular cylinder or straight-meridian
// revolution chart. Quarter-rim coedges may subdivide its horizontal sides;
// opposite chains must use the same exact angular partition.
func compactAxialStart(f Face) (int, bool) {
	if compactCrossedStrip(f) {
		return 0, true
	}
	if len(f.Holes) != 0 || len(f.Outer) < 4 {
		return 0, false
	}
	base, _ := parametric.SurfaceMapping(f.Surface)
	switch base.(type) {
	case geom.CylinderSurface:
	default:
		_, a, b, ok := parametric.RevolutionSideParameters(base)
		if !ok || a.Y == b.Y || a.X <= 0 || b.X <= 0 {
			return 0, false
		}
	}
	start := -1
	for i, u := range f.Outer {
		if u.Curve != nil {
			return 0, false
		}
		previous := f.Outer[(i+len(f.Outer)-1)%len(f.Outer)]
		if u.UVStart.Y == u.UVEnd.Y && u.UVStart.X < u.UVEnd.X && previous.UVStart.X == previous.UVEnd.X {
			if start >= 0 {
				return 0, false
			}
			start = i
		}
	}
	if start < 0 {
		return 0, false
	}
	var lower, upper []float64
	stage := 0
	low, high := 0., 0.
	left, right := 0., 0.
	for j := range f.Outer {
		u := f.Outer[(start+j)%len(f.Outer)]
		a, b := u.UVStart, u.UVEnd
		switch stage {
		case 0:
			if j == 0 {
				low = a.Y
				left = a.X
				lower = append(lower, a.X)
			}
			if a.Y == b.Y {
				if a.Y != low || b.X <= a.X || b.X-a.X > 2*math.Pi {
					return 0, false
				}
				lower = append(lower, b.X)
			} else {
				if a.X != b.X || b.Y <= a.Y {
					return 0, false
				}
				right = a.X
				high = b.Y
				upper = append(upper, b.X)
				stage = 1
			}
		case 1:
			if a.Y == b.Y {
				if a.Y != high || b.X >= a.X || a.X-b.X > 2*math.Pi {
					return 0, false
				}
				upper = append(upper, b.X)
			} else {
				if j != len(f.Outer)-1 || a.X != left || b.X != left || b.Y != low || a.Y != high {
					return 0, false
				}
				stage = 2
			}
		default:
			return 0, false
		}
	}
	if stage != 2 || len(lower) < 2 || len(upper) < 2 || lower[len(lower)-1] != right || upper[0] != right || upper[len(upper)-1] != left {
		return 0, false
	}
	return start, true
}

func compactAxialTriangles(outer [][]geom.Vec2) ([][3]geom.Vec2, error) {
	var low, high []geom.Vec2
	bottom := outer[0][0].Y
	for _, edge := range outer {
		if len(edge) < 2 {
			return nil, geom.ErrDomain
		}
		a, b := edge[0], edge[len(edge)-1]
		if a.Y != b.Y {
			if len(edge) != 2 {
				return nil, geom.ErrDomain
			}
			continue
		}
		target := &high
		if a.Y == bottom {
			target = &low
		}
		*target = append(*target, edge[:len(edge)-1]...)
	}
	// Horizontal chains omit their final endpoint above; recover them from the
	// immediately following vertical edge without introducing new model samples.
	for _, edge := range outer {
		a, b := edge[0], edge[len(edge)-1]
		if a.Y == b.Y {
			continue
		}
		if a.Y == bottom {
			low = append(low, a)
		} else {
			high = append(high, a)
		}
	}

	if len(low) < 2 || len(high) < 2 {
		return nil, geom.ErrDomain
	}
	for i, j := 0, len(high)-1; i < j; i, j = i+1, j-1 {
		high[i], high[j] = high[j], high[i]
	}
	triangles := make([][3]geom.Vec2, 0, len(low)+len(high)-2)
	i, j := 0, 0
	for i < len(low)-1 || j < len(high)-1 {
		if i < len(low)-1 && j < len(high)-1 && math.Abs(low[i+1].X-high[j+1].X) <= 64*0x1p-52*max(1, math.Abs(low[i+1].X), math.Abs(high[j+1].X)) {
			triangles = append(triangles, [3]geom.Vec2{low[i], low[i+1], high[j+1]}, [3]geom.Vec2{low[i], high[j+1], high[j]})
			i++
			j++
			continue
		}
		if j == len(high)-1 || i < len(low)-1 && low[i+1].X <= high[j+1].X {
			triangles = append(triangles, [3]geom.Vec2{low[i], low[i+1], high[j]})
			i++
		} else {
			triangles = append(triangles, [3]geom.Vec2{low[i], high[j+1], high[j]})
			j++
		}
	}
	return triangles, nil
}

// Roundoff in a split chart endpoint must not add an entire extra angular
// strip. This chooses only sample count; all geometry and error checks remain.
func compactAngularCount(n int, span float64) int {
	ideal := float64(n) * span / (math.Pi / 2)
	nearest := math.Round(ideal)
	if math.Abs(ideal-nearest) <= 64*0x1p-52*max(1, ideal) {
		ideal = nearest
	}
	return max(1, int(math.Ceil(ideal)))
}
