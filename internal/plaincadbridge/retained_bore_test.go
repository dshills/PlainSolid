package plaincadbridge

import (
	"math"
	"testing"
)

func TestFilletRetainedEdgeAfterBoreAndJoin(t *testing.T) {
	b := New()
	a := box(t, b, vector{}, vector{20, 20, 10})
	hole := circle(t, b, vector{10, 10, 0}, vector{0, 0, 1}, 2, 10)
	bored := shape(t, b, map[string]any{"op": "cut", "a": a.Handle, "b": hole.Handle})
	boss := box(t, b, vector{15, 0, 0}, vector{25, 20, 10})
	joined := shape(t, b, map[string]any{"op": "fuse", "a": bored.Handle, "b": boss.Handle})
	near(t, joined.GeometryAssertions.Volume, 5000-40*math.Pi)
	out := shape(t, b, map[string]any{"op": "fillet", "handle": joined.Handle,
		"edgeStart": vector{0, 0, 10}, "edgeEnd": vector{0, 20, 10}, "normal": vector{0, 0, 1}, "radius": 2})
	near(t, out.GeometryAssertions.Volume, 5000-40*math.Pi-80*(1-math.Pi/4))
}

func TestChamferCompleteRetainedPerimeterWithBore(t *testing.T) {
	b := New()
	a := box(t, b, vector{}, vector{20, 20, 10})
	hole := circle(t, b, vector{10, 10, 0}, vector{0, 0, 1}, 2, 10)
	bored := shape(t, b, map[string]any{"op": "cut", "a": a.Handle, "b": hole.Handle})
	points := []vector{{0, 0, 10}, {20, 0, 10}, {20, 20, 10}, {0, 20, 10}}
	refs := make([]map[string]vector, len(points))
	for i, p := range points {
		refs[i] = map[string]vector{"start": p, "end": points[(i+1)%4]}
	}
	result := shape(t, b, map[string]any{"op": "capChamfer", "handle": bored.Handle,
		"edges": refs, "normal": vector{0, 0, 1}, "distance": 1})
	near(t, result.GeometryAssertions.Volume, 4000-40*math.Pi-(40-4./3))
	m, err := b.shapes[result.Handle].tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
}

func TestFuseCircularBossOnBoredBox(t *testing.T) {
	b := New()
	a := box(t, b, vector{}, vector{20, 20, 14})
	hole := circle(t, b, vector{10, 10, 0}, vector{0, 0, 1}, 2, 10)
	bored := shape(t, b, map[string]any{"op": "cut", "a": a.Handle, "b": hole.Handle})
	boss := circle(t, b, vector{5, 5, 14}, vector{0, 0, 1}, 1, 3)
	out := shape(t, b, map[string]any{"op": "fuse", "a": bored.Handle, "b": boss.Handle})
	near(t, out.GeometryAssertions.Volume, 5600-40*math.Pi+3*math.Pi)
}
