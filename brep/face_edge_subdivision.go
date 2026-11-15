package brep

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// triangulateFaceWires keeps every original boundary vertex. A strictly
// interior straight-edge station is suppressed only while triangulating the
// polygon, then reinserted with triangles incident to both child edges.
func triangulateFaceWires(rings [][]geom.Vec3) ([][3]int, error) {
	total := 0
	for _, r := range rings {
		total += len(r)
	}
	if len(rings) > 1 && total > MaxHoledFaceVertices {
		return nil, ErrTriangulationLimit
	}
	if len(rings) == 1 && total > 10000 {
		return nil, fmt.Errorf("too many face vertices")
	}
	var compressed [][]geom.Vec3
	var original []int
	chains := make(map[[2]int][]int)
	base := 0
	for _, r := range rings {
		var keep []int
		for i, b := range r {
			a, c := r[(i+len(r)-1)%len(r)], r[(i+1)%len(r)]
			collinear, err := collinearPlanarStation(a, b, c)
			if err != nil {
				return nil, err
			}
			if collinear {
				if !strictSegmentStation(a, b, c) {
					return nil, fmt.Errorf("collinear wire retraces an original edge")
				}
				if keep == nil {
					keep = make([]int, 0, len(r)-1)
					for j := 0; j < i; j++ {
						keep = append(keep, j)
					}
				}
			} else if keep != nil {
				keep = append(keep, i)
			}
		}
		if keep == nil {
			compressed = append(compressed, r)
			for i := range r {
				original = append(original, base+i)
			}
			base += len(r)
			continue
		}
		if len(keep) < 3 {
			return nil, fmt.Errorf("degenerate straight face wire")
		}
		wire := make([]geom.Vec3, len(keep))
		for i, from := range keep {
			to := keep[(i+1)%len(keep)]
			wire[i] = r[from]
			original = append(original, base+from)
			if j := (from + 1) % len(r); j != to {
				key := [2]int{base + from, base + to}
				for ; j != to; j = (j + 1) % len(r) {
					chains[key] = append(chains[key], base+j)
				}
			}
		}
		compressed = append(compressed, wire)
		base += len(r)
	}
	var triangles [][3]int
	var err error
	if len(compressed) == 1 {
		triangles, err = triangulate(compressed[0])
	} else {
		triangles, err = triangulateRings(compressed[0], compressed[1:])
	}
	if err != nil || len(chains) == 0 {
		return triangles, err
	}
	for i, t := range triangles {
		triangles[i] = [3]int{original[t[0]], original[t[1]], original[t[2]]}
	}
	for i := 0; i < len(triangles) && len(chains) > 0; {
		t := triangles[i]
		split := false
		for j, start := range t {
			end, opposite := t[(j+1)%3], t[(j+2)%3]
			key := [2]int{start, end}
			inside, ok := chains[key]
			if !ok {
				continue
			}
			delete(chains, key)
			triangles[i] = [3]int{start, inside[0], opposite}
			for k := 1; k < len(inside); k++ {
				triangles = append(triangles, [3]int{inside[k-1], inside[k], opposite})
			}
			triangles = append(triangles, [3]int{inside[len(inside)-1], end, opposite})
			split = true
			break
		}
		if !split {
			i++
		}
	}
	if len(chains) != 0 || len(triangles) != total-2+2*(len(rings)-1) {
		return nil, fmt.Errorf("could not retain original planar edge subdivision")
	}
	return triangles, nil
}

func strictSegmentStation(a, b, c geom.Vec3) bool {
	return a != b && b != c &&
		b.X >= min(a.X, c.X) && b.X <= max(a.X, c.X) &&
		b.Y >= min(a.Y, c.Y) && b.Y <= max(a.Y, c.Y) &&
		b.Z >= min(a.Z, c.Z) && b.Z <= max(a.Z, c.Z)
}

func collinearPlanarStation(a, b, c geom.Vec3) (bool, error) {
	// The largest approximate cross component only orders the projection
	// checks. Every zero decision still requires an exact orient2D sign.
	q := a.Sub(b).Cross(c.Sub(b))
	m := [3]float64{math.Abs(q.X), math.Abs(q.Y), math.Abs(q.Z)}
	first := 0
	for d := 1; d < 3; d++ {
		if m[d] > m[first] {
			first = d
		}
	}
	project := func(v geom.Vec3, drop int) geom.Vec2 {
		switch drop {
		case 0:
			return geom.Vec2{X: v.Y, Y: v.Z}
		case 1:
			return geom.Vec2{X: v.X, Y: v.Z}
		default:
			return geom.Vec2{X: v.X, Y: v.Y}
		}
	}
	for i := 0; i < 3; i++ {
		d := (i + first) % 3
		s, err := geom.Orient2D(project(a, d), project(b, d), project(c, d))
		if err != nil || s != 0 {
			return false, err
		}
	}
	return true, nil
}
