package plaincadbridge

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"testing"
)

func TestRetainedPlanarBoreFillRoofAndRedrill(t *testing.T) {
	for _, kind := range []string{"fill", "roof", "plug"} {
		t.Run(kind, func(t *testing.T) {
			b := New()
			stock := box(t, b, vector{}, vector{60, 40, 20})
			pocket := box(t, b, vector{12, 12, 14}, vector{24, 20, 20})
			stock = shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": pocket.Handle})
			hole := circle(t, b, vector{45, 25, 20}, vector{0, 0, -1}, 2, 20)
			stock = shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": hole.Handle})
			var filler Shape
			want := 48000 - 80*math.Pi
			switch kind {
			case "fill":
				filler = box(t, b, vector{11, 11, 13}, vector{25, 21, 20})
			case "roof":
				filler = box(t, b, vector{11, 11, 18}, vector{25, 21, 20})
				want -= 384
			case "plug":
				filler = circle(t, b, vector{45, 25, 0}, vector{0, 0, 1}, 3, 20)
				want = 48000 - 576
			}
			joined := shape(t, b, map[string]any{"op": "fuse", "a": stock.Handle, "b": filler.Handle})
			near(t, joined.GeometryAssertions.Volume, want)
			if kind == "roof" && len(b.shapes[joined.Handle].analytic.solid.Topology().Shells) != 2 {
				t.Fatal("missing cavity")
			}
			verifyRetainedPlanarBore(t, b, joined)
			if kind == "roof" {
				matrix := [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}
				moved := shape(t, b, map[string]any{"op": "transform", "handle": joined.Handle, "matrix": matrix})
				movedTool := shape(t, b, map[string]any{"op": "transform", "handle": pocket.Handle, "matrix": matrix})
				movedOut := shape(t, b, map[string]any{"op": "cut", "a": moved.Handle, "b": movedTool.Handle})
				near(t, movedOut.GeometryAssertions.Volume, 48000-576-80*math.Pi)
				verifyRetainedPlanarBore(t, b, movedOut)
			}
			tool := pocket
			if kind == "plug" {
				tool = hole
			}
			out := shape(t, b, map[string]any{"op": "cutMany", "a": joined.Handle, "tools": []string{tool.Handle}})
			near(t, out.GeometryAssertions.Volume, 48000-576-80*math.Pi)
			verifyRetainedPlanarBore(t, b, out)
		})
	}
}
func verifyRetainedPlanarBore(t *testing.T, b *Bridge, s Shape) {
	t.Helper()
	native := b.shapes[s.Handle].analytic.solid
	p, e := native.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	near(t, p.Volume, s.GeometryAssertions.Volume)
	near(t, p.Area, s.GeometryAssertions.SurfaceArea)
	m, e := native.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
}

func TestRetainedPlanarBoreProofRejectsUnsafeDomains(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{60, 40, 20})
	pocket := box(t, b, vector{12, 12, 14}, vector{24, 20, 20})
	stock = shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": pocket.Handle})
	hole := circle(t, b, vector{45, 25, 0}, vector{0, 0, 1}, 2, 20)
	stock = shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": hole.Handle})
	plug := circle(t, b, vector{45, 25, -1}, vector{0, 0, 1}, 3, 22)
	_, _, handled, e := retainedPlanarBoolean(b.shapes[stock.Handle], b.shapes[plug.Handle], "fuse")
	if !handled || e == nil {
		t.Fatal("extending plug accepted")
	}
	roof := box(t, b, vector{11, 11, 18}, vector{25, 21, 20})
	joined := shape(t, b, map[string]any{"op": "fuse", "a": stock.Handle, "b": roof.Handle})
	body := b.shapes[joined.Handle].analytic
	if _, e = b.putAnalyticBody(&analyticBody{solid: body.solid, volume: body.volume, area: body.area}); e == nil {
		t.Fatal("uncertified cavity accepted")
	}
	cross := circle(t, b, vector{18, 16, 0}, vector{0, 0, 1}, 1, 20)
	_, _, handled, e = retainedPlanarBoolean(b.shapes[joined.Handle], b.shapes[cross.Handle], "cut")
	if !handled || e == nil {
		t.Fatal("cavity crossing bore accepted")
	}
	far := box(t, b, vector{70, 0, 0}, vector{80, 10, 20})
	_, _, handled, e = retainedPlanarBoolean(b.shapes[joined.Handle], b.shapes[far.Handle], "fuse")
	if !handled || e == nil {
		t.Fatal("disconnected material accepted")
	}
}
