package plaincadbridge

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestSideTangentBlindBoreDisplayAndFabrication(t *testing.T) {
	for _, radius := range []float64{1, .1} {
		for _, reverse := range []bool{false, true} {
			b := New()
			stock := box(t, b, vector{}, vector{20, 10, 10})
			origin, normal := vector{radius, 5, 7}, vector{0, 0, 1}
			if reverse {
				origin[2], normal[2] = 10, -1
			}
			tool := shape(t, b, map[string]any{"op": "circle", "origin": origin, "normal": normal, "radius": radius, "height": 3})
			var overlap struct{ Volume float64 }
			if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "overlapVolume", "a": tool.Handle, "b": stock.Handle}), &overlap); err != nil {
				t.Fatal(err)
			}
			near(t, overlap.Volume, 3*math.Pi*radius*radius)
			hole := shape(t, b, map[string]any{"op": "cutMany", "a": stock.Handle, "tools": []string{tool.Handle}})
			s := b.shapes[hole.Handle]
			if s.tangent == nil || !hole.GeometryAssertions.Valid || hole.GeometryAssertions.SolidCount != 1 || hole.Bounds != stock.Bounds {
				t.Fatal(hole)
			}
			near(t, hole.GeometryAssertions.Volume, 2000-overlap.Volume)
			near(t, hole.GeometryAssertions.SurfaceArea, 1000+6*math.Pi*radius)
			m, err := s.tessellate(request{})
			if err != nil {
				t.Fatal(err)
			}
			if err := m.ValidateSolid(); err == nil {
				t.Fatal("contact must fail manifold validation")
			}
			var signedVolume float64
			for _, tri := range m.Triangles {
				a, c, d := m.Vertices[tri[0]], m.Vertices[tri[1]], m.Vertices[tri[2]]
				signedVolume += a.Dot(c.Cross(d)) / 6
			}
			if math.Abs(signedVolume-hole.GeometryAssertions.Volume) > .12*radius*radius {
				t.Fatalf("display volume is %g", signedVolume)
			}
			floor, err := s.tangent.planarFace(geom.Vec3{Z: 7}, geom.Vec3{Z: 1})
			if err != nil {
				t.Fatal(err)
			}
			near(t, floor.Area, math.Pi*radius*radius)
			invoke(t, b, map[string]any{"op": "displayEdges", "handle": hole.Handle, "maxEdgeUses": 100, "maxEdgeSegments": 1000})
			reflected := shape(t, b, map[string]any{"op": "transform", "handle": hole.Handle, "matrix": geom.Affine{{-1, 0, 0, 20}, {0, 1, 0, 0}, {0, 0, 1, 5}}})
			if reflected.Bounds.Min != (vector{0, 0, 5}) || reflected.Bounds.Max != (vector{20, 10, 15}) {
				t.Fatal(reflected)
			}
			for _, q := range []map[string]any{
				{"op": "stl", "handle": reflected.Handle},
				{"op": "exportStep", "handles": []string{reflected.Handle}},
			} {
				input, err := json.Marshal(q)
				if err != nil {
					t.Fatal(err)
				}
				if got := b.Call(string(input)); !strings.Contains(got, "non-manifold contact edge") {
					t.Fatal(got)
				}
			}
		}
	}
}

func TestSideTangentBlindBoreRecipeRejectsNearbyAndThroughTools(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{20, 10, 10})
	for _, candidate := range []struct {
		x, z, height float64
	}{
		{math.Nextafter(1, 0), 7, 3},
		{math.Nextafter(1, 2), 7, 3},
		{1, 0, 10},
		{1, 6, 3},
	} {
		tool := shape(t, b, map[string]any{"op": "circle", "origin": vector{candidate.x, 5, candidate.z}, "normal": vector{0, 0, 1}, "radius": 1, "height": candidate.height})
		if _, ok := tangentBlindBoreRecipe(b.shapes[stock.Handle], b.shapes[tool.Handle]); ok {
			t.Fatalf("out-of-domain tool was qualified: %+v", candidate)
		}
	}
}
