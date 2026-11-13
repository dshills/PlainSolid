package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestSewSteppedBoreNative(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, blind := range []bool{false, true} {
		stock, e := ExtrudeRoundedRectangle(ctx, frame, RoundedRectangle{Min: geom.Vec2{X: -20, Y: -20}, Max: geom.Vec2{X: 20, Y: 20}, Radii: [4]float64{2, 2, 2, 2}}, nil, nil, 10)
		if e != nil {
			t.Fatal(e)
		}
		low := 0
		depth := 0.
		removed := math.Pi * (3*3*10 + (6*6-3*3)*2)
		if blind {
			low = -1
			depth = 6
			removed = math.Pi * (3*3*6 + (6*6-3*3)*2)
		}
		out, e := SewPlanarSteppedBore(ctx, stock, low, 1, geom.Vec2{}, []AxialBoreStep{{3, depth}, {6, 2}})
		if e != nil {
			t.Fatal(blind, e)
		}
		a, e := stock.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		if _, e := out.Bounds(); e != nil {
			t.Fatal(e)
		}
		b, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(a.Volume-b.Volume-removed) > 1e-6 {
			t.Fatal(a.Volume, b.Volume, removed)
		}
		wallDepth := 10.
		areaDelta := 2*math.Pi*(3*(wallDepth-2)+6*2) - 2*math.Pi*9
		if blind {
			wallDepth = 6
			areaDelta = 2 * math.Pi * (3*(wallDepth-2) + 6*2)
		}
		if math.Abs(b.Area-a.Area-areaDelta) > 1e-6 {
			t.Fatal("area", b.Area-a.Area, areaDelta)
		}
		m, e := out.Tessellate(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
}

func TestSewSteppedBoreSignedFramesAndDecimal(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		frame, e := geom.FrameFromNormal(geom.Vec3{X: 16, Y: -32, Z: 48}, axis)
		if e != nil {
			t.Fatal(e)
		}
		stock, e := ExtrudeRoundedRectangle(ctx, frame, RoundedRectangle{Min: geom.Vec2{X: -20, Y: -20}, Max: geom.Vec2{X: 20, Y: 20}, Radii: [4]float64{2, 2, 2, 2}}, nil, nil, 10)
		if e != nil {
			t.Fatal(e)
		}
		out, e := SewPlanarSteppedBore(ctx, stock, 0, 1, geom.Vec2{X: .2, Y: -.2}, []AxialBoreStep{{2.2, 0}, {3.5, 2}})
		if e != nil {
			t.Fatal(axis, e)
		}
		p, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		want := (1600-4*(4-math.Pi))*10 - math.Pi*(2.2*2.2*10+(3.5*3.5-2.2*2.2)*2)
		if math.Abs(p.Volume-want) > 1e-6 {
			t.Fatal(p.Volume, want)
		}
	}
}

func TestSewSteppedBoreRejectsAndOwns(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	stock, e := ExtrudeRoundedRectangle(ctx, frame, RoundedRectangle{Min: geom.Vec2{X: -20, Y: -20}, Max: geom.Vec2{X: 20, Y: 20}, Radii: [4]float64{2, 2, 2, 2}}, nil, nil, 10)
	if e != nil {
		t.Fatal(e)
	}
	original := stock.Faces()
	bored, e := SewPlanarSteppedBore(ctx, stock, 0, 1, geom.Vec2{}, []AxialBoreStep{{3, 0}, {6, 2}})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(original, stock.Faces()) {
		t.Fatal("mutated stock")
	}
	if _, e = SewPlanarSteppedBore(ctx, bored, 0, 1, geom.Vec2{X: 7}, []AxialBoreStep{{2, 0}}); e == nil {
		t.Fatal("accepted touching/overlapping cavity envelope")
	}
	for _, steps := range [][]AxialBoreStep{{{3, -1}}, {{3, 11}}, {{3, 0}, {math.Inf(1), 2}}} {
		if _, e = SewPlanarSteppedBore(ctx, stock, 0, 1, geom.Vec2{}, steps); e == nil {
			t.Fatal("invalid", steps)
		}
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = SewPlanarSteppedBore(cancelCtx, stock, 0, 1, geom.Vec2{}, []AxialBoreStep{{3, 0}}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestSewSteppedBoreOrbitDecimalCenters(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	stock, e := ExtrudeRoundedRectangle(ctx, f, RoundedRectangle{Min: geom.Vec2{X: -70, Y: -54}, Max: geom.Vec2{X: 70, Y: 54}, Radii: [4]float64{10, 10, 10, 10}}, nil, nil, 10)
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range []float64{-58, 58} {
		for _, y := range []float64{-42, 42} {
			stock, e = SewPlanarSteppedBore(ctx, stock, 0, 1, geom.Vec2{X: x, Y: y}, []AxialBoreStep{{2.2, 0}, {3.5, 2}})
			if e != nil {
				t.Fatal(x, y, e)
			}
		}
	}
	if _, e = stock.Bounds(); e != nil {
		t.Fatal(e)
	}
	m, e := stock.Tessellate(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
}

func TestSewSteppedBoreOrbitTreatedBase(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	stock, e := TopFilletedBottomChamferedRoundedRectangle(ctx, f, RoundedRectangle{Min: geom.Vec2{X: -70, Y: -54}, Max: geom.Vec2{X: 70, Y: 54}, Radii: [4]float64{10, 10, 10, 10}}, 10, 1, 1, 1e-7)
	if e != nil {
		t.Fatal(e)
	}
	stock, e = SewPlanarSteppedBore(ctx, stock, 0, 1, geom.Vec2{}, []AxialBoreStep{{20, 0}})
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range []float64{-58, 58} {
		for _, y := range []float64{-42, 42} {
			stock, e = SewPlanarSteppedBore(ctx, stock, 0, 1, geom.Vec2{X: x, Y: y}, []AxialBoreStep{{3.5, 0}, {6, 4}})
			if e != nil {
				t.Fatal(x, y, e)
			}
		}
	}
	o := parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128}
	m, handled, e := stock.compactCylinderMesh(ctx, o)
	if !handled || e != nil {
		t.Fatal("compact", handled, e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	t.Log("triangles", len(m.Triangles))
}
