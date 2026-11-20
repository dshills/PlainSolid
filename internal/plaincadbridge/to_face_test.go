package plaincadbridge

import (
	"context"
	"encoding/json"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
	"testing"
)

func rejectToFace(t *testing.T, b *Bridge, q any) {
	t.Helper()
	before := len(b.shapes)
	data, _ := json.Marshal(q)
	reply := b.Call(string(data))
	if !strings.Contains(reply, `"ok":false`) || len(b.shapes) != before {
		t.Fatalf("failed operation published result: %s", reply)
	}
}
func TestToFaceFiniteCoverageAndHoles(t *testing.T) {
	b := New()
	target := shape(t, b, map[string]any{"op": "prism", "outer": []vector{{-5, -5, 10}, {5, -5, 10}, {5, 5, 10}, {-5, 5, 10}}, "holes": [][]vector{{{-1, -1, 10}, {-1, 1, 10}, {1, 1, 10}, {1, -1, 10}}}, "offset": vector{0, 0, 1}})
	q := map[string]any{"op": "circleToFace", "b": target.Handle, "origin": vector{0, 0, 10}, "normal": vector{0, 0, 1}, "direction": vector{0, 0, 1}, "center": vector{}, "radius": 2}
	rejectToFace(t, b, q) // Entire target hole lies inside the circle; checking only its rim is insufficient.
	q["center"] = vector{3, 0, 0}
	q["radius"] = 1
	s := shape(t, b, q)
	near(t, s.GeometryAssertions.Volume, 10*math.Pi)
	q["center"] = vector{5, 0, 0}
	rejectToFace(t, b, q)
	q["center"] = vector{3, 0, 11}
	rejectToFace(t, b, q)
	q["center"] = vector{3, 0, 10}
	rejectToFace(t, b, q)
	q["center"] = vector{3, 0, 0}
	q["direction"] = vector{1, 0, 0}
	rejectToFace(t, b, q)
	p := map[string]any{"op": "prismToFace", "b": target.Handle, "origin": vector{0, 0, 10}, "normal": vector{0, 0, 1}, "direction": vector{0, 0, 1}, "outer": []vector{{-2, -2, 0}, {2, -2, 0}, {2, 2, 0}, {-2, 2, 0}}, "holes": [][]vector{{{-1, -1, 0}, {-1, 1, 0}, {1, 1, 0}, {1, -1, 0}}}}
	s = shape(t, b, p)
	near(t, s.GeometryAssertions.Volume, 120)
	near(t, s.GeometryAssertions.SurfaceArea, 264)
	delete(p, "holes")
	rejectToFace(t, b, p)
	near(t, shape(t, b, map[string]any{"op": "inspect", "handle": target.Handle}).GeometryAssertions.Volume, 96)
}
func TestToFaceCircleSignedAxesRetainsAnalyticSolid(t *testing.T) {
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			b := New()
			lo, hi := vector{-5, -5, -5}, vector{5, 5, 5}
			lo[axis], hi[axis] = 10*sign, 11*sign
			if sign < 0 {
				lo[axis], hi[axis] = hi[axis], lo[axis]
			}
			target := box(t, b, lo, hi)
			n, origin := vector{}, vector{}
			n[axis] = sign
			origin[axis] = 10 * sign
			s := shape(t, b, map[string]any{"op": "circleToFace", "b": target.Handle, "origin": origin, "normal": n, "direction": n, "center": vector{}, "radius": 2})
			near(t, s.GeometryAssertions.Volume, 40*math.Pi)
			near(t, s.GeometryAssertions.SurfaceArea, 48*math.Pi)
			solid := boundary(t, b, s.Handle)
			curved := false
			for _, f := range solid.Faces() {
				base, _ := parametric.SurfaceMapping(f.Surface)
				if _, ok := base.(geom.CylinderSurface); ok {
					curved = true
				}
			}
			if !curved {
				t.Fatal("the wall of the cylinder was faceted")
			}
			mass, err := solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if err != nil {
				t.Fatal(err)
			}
			near(t, mass.Volume, 40*math.Pi)
		}
	}
}
func TestToFacePolygonObliqueAndBoundary(t *testing.T) {
	b := New()
	target := box(t, b, vector{-5, -5, 10}, vector{5, 5, 11})
	q := map[string]any{"op": "prismToFace", "b": target.Handle, "origin": vector{0, 0, 10}, "normal": vector{0, 0, 1}, "direction": vector{0, 0, 1}, "outer": []vector{{-5, -5, 0}, {5, -5, 0}, {5, 5, 0}, {-5, 5, 0}}}
	near(t, shape(t, b, q).GeometryAssertions.Volume, 1000)
	target = shape(t, b, map[string]any{"op": "transform", "handle": target.Handle, "matrix": [][]float64{{1, 0, 0, 0}, {0, 1, 0, 0}, {0.5, 0, 1, 0}}})
	q["b"] = target.Handle
	q["normal"] = vector{-.5, 0, 1}
	q["outer"] = []vector{{-1, -1, 0}, {1, -1, 0}, {1, 1, 0}, {-1, 1, 0}}
	s := shape(t, b, q)
	near(t, s.GeometryAssertions.Volume, 40)
	near(t, s.Bounds.Max[2], 10.5)
	q["origin"] = vector{0, 0, 12}
	rejectToFace(t, b, q)
}
