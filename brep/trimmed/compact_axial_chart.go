package trimmed

import (
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// compactAxialStart qualifies a chart that the compact mesher triangulates in
// angular strips, and gives the use its outer loop is read from: one of the
// charts of compactRectangularStart, or else a band (compactBand).
func compactAxialStart(f Face) (int, bool) {
	if start, ok := compactRectangularStart(f); ok {
		return start, true
	}
	start, _, ok := compactBand(f)
	return start, ok
}

// compactRectangularStart qualifies a rectangular cylinder or
// straight-meridian revolution chart. Quarter-rim coedges may subdivide its
// horizontal sides; opposite chains must use the same exact angular partition.
func compactRectangularStart(f Face) (int, bool) {
	if compactCrossedStrip(f) {
		return 0, true
	}
	if len(f.Holes) != 0 || len(f.Outer) < 4 {
		return 0, false
	}
	base, _ := parametric.SurfaceMapping(f.Surface)
	switch base.(type) {
	case geom.CylinderSurface:
	case *geom.ExactBezierSurface:
		if !compactExactVerticalChart(base) {
			return 0, false
		}
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

// compactRim reports whether a use spans an angle. A use along a generator
// does not, although the two ends of one that lies on the seam of a chart can
// differ in the last bits of the angle.
func compactRim(u Use) bool {
	return math.Abs(u.UVEnd.X-u.UVStart.X) > 64*0x1p-52*max(1, math.Abs(u.UVStart.X), math.Abs(u.UVEnd.X))
}

// compactBand qualifies a cylinder chart without holes whose boundary is two
// chains, each a graph of axial height over the angle: every use either runs
// along a generator or moves one way in the angle, as a straight chart
// segment does and as the trim of a plane does, and going around the loop
// the angle increases along one chain and decreases along the other. Any
// number of uses may make up a chain, a generator may join the two chains at
// either end or step a chain up or down, and the chains may meet at a point.
// This is every face a plane or a parallel cylinder cuts from a cylinder
// wall, unless the face has a hole or winds back on itself.
//
// start is the first use of the lower chain, read from the generators that
// close the band on the left, and lower is the number of uses from there to
// the last generator on the right; the rest is the upper chain. A chart that
// compactRectangularStart qualifies is not a band: it keeps its own strips.
func compactBand(f Face) (start, lower int, ok bool) {
	if _, ok := compactRectangularStart(f); ok || len(f.Holes) != 0 {
		return 0, 0, false
	}
	if base, _ := parametric.SurfaceMapping(f.Surface); base == nil {
		return 0, 0, false
	} else if _, ok := base.(geom.CylinderSurface); !ok {
		return 0, 0, false
	}
	n := len(f.Outer)
	turn := make([]int, n) // +1 up the angle, -1 down it, 0 along a generator
	rises, falls := 0, 0
	for i, u := range f.Outer {
		if !compactRim(u) {
			if u.Curve != nil {
				return 0, 0, false
			}
			continue
		}
		turn[i] = 1
		if u.UVEnd.X < u.UVStart.X {
			turn[i] = -1
		}
		if u.Curve != nil {
			// The angle along the trim of a plane is linear in the
			// parameter. No other curved trim is known to be a graph.
			// Tessellate reads a trim through its memo.
			curve := u.Curve
			if memo, ok := curve.(tessellationMemoCurve); ok {
				curve = memo.source
			}
			c, ok := curve.(interface{ EllipseParameters() EllipseUV })
			if !ok {
				return 0, 0, false
			}
			e := c.EllipseParameters()
			if e.Cos.X != 0 || e.Sin.X != 0 || !(e.Linear.X*(e.To-e.From)*float64(turn[i]) > 0) {
				return 0, 0, false
			}
		}
		if turn[i] > 0 {
			rises++
		} else {
			falls++
		}
	}
	if rises == 0 || falls == 0 {
		return 0, 0, false
	}
	// The uses that go down the angle must be one stretch of the loop, which
	// generators may interrupt. The lower chain starts after its last use.
	start = -1
	for i := range f.Outer {
		if turn[i] >= 0 {
			continue
		}
		j := (i + 1) % n
		for turn[j] == 0 {
			j = (j + 1) % n
		}
		if turn[j] > 0 {
			if start >= 0 {
				return 0, 0, false
			}
			start = (i + 1) % n
		}
	}
	if start < 0 {
		return 0, 0, false
	}
	for lower < n && turn[(start+lower)%n] >= 0 {
		lower++
	}
	return start, lower, true
}

// compactBandTriangles triangulates a band from the samples of its two
// chains, each use with both of its ends. It walks the chains together from
// the left and joins the next sample in the angle to the two it has reached,
// so that every triangle has a side on one chain and its third corner on the
// other. Three samples of one rim lie in the plane that cut the rim, and a
// triangle on them would stand across the wall instead of along it.
//
// A triangle that does not keep the orientation of the chart is not made;
// the walk then takes the sample of the other chain. Where neither is
// possible the samples are too coarse for the band and errRefine is returned.
func compactBandTriangles(lower, upper [][]geom.Vec2) ([][3]geom.Vec2, error) {
	chain := func(uses [][]geom.Vec2, end geom.Vec2) ([]geom.Vec2, error) {
		var points []geom.Vec2
		for _, use := range uses {
			if len(use) < 2 {
				return nil, geom.ErrDomain
			}
			points = append(points, use[:len(use)-1]...)
		}
		return append(points, end), nil
	}
	if len(lower) == 0 || len(upper) == 0 || len(lower[0]) == 0 || len(upper[0]) == 0 {
		return nil, geom.ErrDomain
	}
	low, err := chain(lower, upper[0][0])
	if err != nil {
		return nil, err
	}
	high, err := chain(upper, lower[0][0])
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(high)-1; i < j; i, j = i+1, j-1 {
		high[i], high[j] = high[j], high[i]
	}
	// Both chains now run from the same first point to the same last point.
	m, n := len(low)-1, len(high)-1
	if m+n < 3 {
		return nil, errRefine
	}
	triangles := make([][3]geom.Vec2, 0, m+n-2)
	for i, j := 1, 0; !(i == m && j == n-1) && !(j == n && i == m-1); {
		// The shared last point closes the band, so a chain takes it
		// only when the other has reached the sample before it.
		a := [3]geom.Vec2{low[i], low[min(i+1, m)], high[j]}
		b := [3]geom.Vec2{low[i], high[min(j+1, n)], high[j]}
		canLow, canHigh := i+1 < m || i+1 == m && j == n-1, j+1 < n || j+1 == n && i == m-1
		if canLow {
			turn, err := geom.Orient2D(a[0], a[1], a[2])
			canLow = err == nil && turn == 1
		}
		if canHigh {
			turn, err := geom.Orient2D(b[0], b[1], b[2])
			canHigh = err == nil && turn == 1
		}
		switch {
		case canLow && (!canHigh || a[1].X < b[1].X-64*0x1p-52*max(1, math.Abs(a[1].X), math.Abs(b[1].X))):
			triangles = append(triangles, a)
			i++
		case canHigh:
			triangles = append(triangles, b)
			j++
		default:
			return nil, errRefine
		}
	}
	return triangles, nil
}
