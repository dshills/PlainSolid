package plaincadbridge

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"plainsolid/brep/parametric"
)

// Exercise the normal bridge entry points with a translated, cyclically placed
// chain. Using different world coordinates checks that selected endpoints and
// tool footprints are mapped through the actual stock frame.
func TestPlacedEdgeChainAnalyticGeometry(t *testing.T) {
	b := New()
	point := func(x, y, z float64) vector { return vector{30 + z, 40 + x, 50 + y} }
	stock := box(t, b, point(0, 0, 0), point(20, 10, 15))
	treat := func(base Shape, op string, z, size float64) Shape {
		return shape(t, b, map[string]any{"op": op, "handle": base.Handle, "edgeStart": point(20, 0, z), "edgeEnd": point(0, 0, z), "normal": vector{1, 0, 0}, "radius": size, "distance": size})
	}
	rounded := treat(stock, "fillet", 15, 2)
	treated := treat(rounded, "chamfer", 0, 1)
	treatedVolume := 3000 - 80*(1-math.Pi/4) - 10
	near(t, treated.GeometryAssertions.Volume, treatedVolume)
	tool := circle(t, b, point(5, 5, 0), vector{1, 0, 0}, 1, 15)
	before := len(b.shapes)
	var overlap struct{ Volume float64 }
	if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "overlapVolume", "a": treated.Handle, "b": tool.Handle}), &overlap); err != nil {
		t.Fatal(err)
	}
	near(t, overlap.Volume, 15*math.Pi)
	if len(b.shapes) != before {
		t.Fatal("scope proof published a shape")
	}
	bored := shape(t, b, map[string]any{"op": "cutMany", "a": treated.Handle, "tools": []string{tool.Handle}})
	pocket := box(t, b, point(10, 5, 0), point(13, 8, 15))
	pocketed := shape(t, b, map[string]any{"op": "cut", "a": bored.Handle, "b": pocket.Handle})
	boss := box(t, b, point(15, 5, 15), point(17, 7, 18))
	final := shape(t, b, map[string]any{"op": "fuse", "a": pocketed.Handle, "b": boss.Handle})
	suppressed := shape(t, b, map[string]any{"op": "fuse", "a": bored.Handle, "b": boss.Handle})
	want := treatedVolume - 15*math.Pi - 135 + 12
	near(t, final.GeometryAssertions.Volume, want)
	near(t, suppressed.GeometryAssertions.Volume, want+135)
	near(t, shape(t, b, map[string]any{"op": "inspect", "handle": treated.Handle}).GeometryAssertions.Volume, treatedVolume)
	if final.Bounds != (Bounds{point(0, 0, 0), point(20, 10, 18)}) {
		t.Fatal(final.Bounds)
	}
	retained := b.shapes[final.Handle]
	props, err := retained.treated.chain.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	near(t, props.Volume, want)
	near(t, props.Area, final.GeometryAssertions.SurfaceArea)
	m, err := retained.tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	if math.Abs(m.SignedVolume()/want-1) >= .001 {
		t.Fatalf("mesh volume %g, analytic %g", m.SignedVolume(), want)
	}
}
