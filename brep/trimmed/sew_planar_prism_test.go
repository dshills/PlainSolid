package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestSewPlanarThroughRoundedPrism(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	stock, e := ExtrudeRoundedRectangle(ctx, f, RoundedRectangle{Min: geom.Vec2{X: -17, Y: -8}, Max: geom.Vec2{X: 17, Y: 8}, Radii: [4]float64{2, 2, 2, 2}}, nil, nil, 8)
	if e != nil {
		t.Fatal(e)
	}
	tool, e := ExtrudeRoundedRectangle(ctx, f, RoundedRectangle{Min: geom.Vec2{X: -12, Y: -2.5}, Max: geom.Vec2{X: 12, Y: 2.5}, Radii: [4]float64{1, 1, 1, 1}}, nil, nil, 8)
	if e != nil {
		t.Fatal(e)
	}
	result, e := SewPlanarThroughPrism(ctx, stock, tool, 0, 1, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	checkPrismDifference(t, stock, tool, result)
	checkPrismMesh(t, result)
}
func TestSewPlanarThroughChamferedAnnulus(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	stock, e := (ChamferedAnnularPrism{Frame: f, OuterRadius: 78, InnerRadius: 26, Height: 12, UpperChamfer: .6}).Build(ctx)
	if e != nil {
		t.Fatal(e)
	}
	tool, e := ExtrudeRoundedRectangle(ctx, f, RoundedRectangle{Min: geom.Vec2{X: 45, Y: -4}, Max: geom.Vec2{X: 71, Y: 4}, Radii: [4]float64{3, 3, 3, 3}}, nil, nil, 12)
	if e != nil {
		t.Fatal(e)
	}
	result, e := SewPlanarThroughPrism(ctx, stock.Solid, tool, stock.LowerFace, stock.UpperFace, 0, 1)
	if e != nil {
		t.Fatal("slot", e)
	}
	checkPrismDifference(t, stock.Solid, tool, result)
	hole, e := ExtrudeDiskWithHoles(ctx, f, ExtrusionCircle{Center: geom.Vec2{X: 63.75, Y: 26.4}, Radius: 3.2}, nil, nil, 12)
	if e != nil {
		t.Fatal(e)
	}
	next, e := SewPlanarThroughPrism(ctx, result, hole, stock.LowerFace, stock.UpperFace, 0, 1)
	if e != nil {
		t.Fatal("diagonal hole", e)
	}
	checkPrismDifference(t, result, hole, next)
	checkPrismMesh(t, next)
}
func checkPrismDifference(t *testing.T, stock, tool, result Solid) {
	t.Helper()
	ctx := context.Background()
	a, e := stock.MassProperties(ctx, parametric.IntegrationOptions{MaxSubdivisions: 256, MaxSamples: 16000000})
	if e != nil {
		t.Fatal(e)
	}
	b, e := tool.MassProperties(ctx, parametric.IntegrationOptions{MaxSubdivisions: 256, MaxSamples: 16000000})
	if e != nil {
		t.Fatal(e)
	}
	p, e := result.MassProperties(ctx, parametric.IntegrationOptions{MaxSubdivisions: 256, MaxSamples: 16000000})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(p.Volume-(a.Volume-b.Volume)) > 1e-7 {
		t.Fatal("mass", a.Volume, b.Volume, p.Volume)
	}
}
func checkPrismMesh(t *testing.T, result Solid) {
	t.Helper()
	ctx := context.Background()
	m, e := result.Tessellate(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 1000000})
	if e != nil {
		t.Fatal("mesh", e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
}

func TestSewPlanarPrismOwnershipAndRejects(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 3, Y: 5, Z: 7}, axis)
		stock, e := ExtrudeRoundedRectangle(ctx, f, RoundedRectangle{Min: geom.Vec2{X: -17, Y: -8}, Max: geom.Vec2{X: 17, Y: 8}, Radii: [4]float64{2, 2, 2, 2}}, nil, nil, 8)
		if e != nil {
			t.Fatal(e)
		}
		moved, e := geom.NewFrame(f.ToWorld(geom.Vec3{X: 1, Y: 1}), f.XAxis(), f.ZAxis())
		if e != nil {
			t.Fatal(e)
		}
		profile := RoundedRectangle{Min: geom.Vec2{X: -5, Y: -2}, Max: geom.Vec2{X: 5, Y: 2}, Radii: [4]float64{1, 1, 1, 1}}
		tool, e := ExtrudeRoundedRectangle(ctx, moved, profile, nil, nil, 8)
		if e != nil {
			t.Fatal(e)
		}
		result, e := SewPlanarThroughPrism(ctx, stock, tool, 0, 1, 0, 1)
		if e != nil {
			t.Fatal(axis, e)
		}
		checkPrismDifference(t, stock, tool, result)
		checkPrismMesh(t, result)
		if len(stock.faces[0].Holes) != 0 || len(tool.faces[0].Holes) != 0 {
			t.Fatal("mutated input")
		}
		if _, e = SewPlanarThroughPrism(ctx, result, tool, 0, 1, 0, 1); e == nil {
			t.Fatal("accepted overlapping cut")
		}
		short, e := ExtrudeRoundedRectangle(ctx, moved, profile, nil, nil, 7)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = SewPlanarThroughPrism(ctx, stock, short, 0, 1, 0, 1); e == nil {
			t.Fatal("accepted finite short tool")
		}
		wide, e := ExtrudeRoundedRectangle(ctx, f, RoundedRectangle{Min: geom.Vec2{X: -16, Y: -7}, Max: geom.Vec2{X: 16, Y: 7}, Radii: [4]float64{1, 1, 1, 1}}, nil, nil, 8)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = SewPlanarThroughPrism(ctx, stock, wide, 0, 1, 0, 1); e == nil {
			t.Fatal("accepted unproved corner containment")
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, e = SewPlanarThroughPrism(cancelled, stock, tool, 0, 1, 0, 1); !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
		var data bytes.Buffer
		if e = WriteJSON(&data, result); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		checkPrismDifference(t, stock, tool, loaded)
	}
}
