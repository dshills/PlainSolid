package brep

import (
	"context"
	"errors"
	"fmt"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestBoundedPlanarTriangulationManyHoles(t *testing.T) {
	ring := func(cx, cy, r float64, n int, reverse bool) []geom.Vec3 {
		p := make([]geom.Vec3, n)
		for i := range p {
			a := 2 * math.Pi * float64(i) / float64(n)
			if reverse {
				a = -a
			}
			p[i] = geom.Vec3{X: cx + r*math.Cos(a), Y: cy + r*math.Sin(a)}
		}
		return p
	}
	for _, samples := range []int{32, 128} {
		t.Run(fmt.Sprint(samples), func(t *testing.T) {
			outer := ring(0, 0, 10, 128, false)
			var holes [][]geom.Vec3
			for i := range 8 {
				a := 2 * math.Pi * float64(i) / 8
				holes = append(holes, ring(6*math.Cos(a), 6*math.Sin(a), .5, samples, true))
			}
			if _, e := triangulateRings(outer, holes); e == nil {
				t.Fatal("default solid domain enlarged")
			}
			triangles, e := TriangulatePlanarRings(context.Background(), outer, holes, PlanarTriangulationOptions{MaxVertices: 2048, MaxOperations: 64000000})
			if e != nil {
				t.Fatal(e)
			}
			if len(triangles) != 128+8*samples+16-2 {
				t.Fatal(len(triangles))
			}
			points := append([]geom.Vec3(nil), outer...)
			for _, h := range holes {
				points = append(points, h...)
			}
			area := 0.
			edges := map[[2]int]int{}
			for _, tri := range triangles {
				a, b, c := points[tri[0]], points[tri[1]], points[tri[2]]
				cross := b.Sub(a).Cross(c.Sub(a)).Z
				if cross <= 0 {
					t.Fatal("nonpositive triangle")
				}
				area += cross / 2
				for i, x := range tri {
					y := tri[(i+1)%3]
					edges[[2]int{min(x, y), max(x, y)}]++
				}
			}
			want := 100*128*math.Sin(2*math.Pi/128)/2 - 8*.25*float64(samples)*math.Sin(2*math.Pi/float64(samples))/2
			if math.Abs(area-want) > 1e-9 {
				t.Fatal(area, want)
			}
			for edge, count := range edges {
				if count != 1 && count != 2 {
					t.Fatal(edge, count)
				}
			}
			o := PlanarTriangulationOptions{MaxVertices: 2048, MaxOperations: 1}
			if _, e = TriangulatePlanarRings(context.Background(), outer, holes, o); !errors.Is(e, ErrTriangulationLimit) {
				t.Fatal(e)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, e = TriangulatePlanarRings(ctx, outer, holes, PlanarTriangulationOptions{}); e != context.Canceled {
				t.Fatal(e)
			}
		})
	}
}
