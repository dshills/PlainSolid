package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestRectangularIslandPocketAnalytic(t *testing.T) {
	for _, axis := range []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		frame, _ := geom.FrameFromNormal(geom.Vec3{X: 3, Y: 5, Z: 7}, axis)
		p := RectangularIslandPocket{Frame: frame, OuterMin: geom.Vec2{X: -10, Y: -10}, OuterMax: geom.Vec2{X: 10, Y: 10}, PocketMin: geom.Vec2{X: -7, Y: -7}, PocketMax: geom.Vec2{X: 7, Y: 7}, IslandRadius: 3, Height: 8, Depth: 3}
		result, e := p.Build(context.Background())
		if e != nil {
			t.Fatal(axis, e)
		}
		wantV := 3200 - 588 + 27*math.Pi
		wantA := 1440 + 168 + 18*math.Pi
		v, _ := p.NominalVolume()
		area, _ := p.SurfaceArea()
		if v != wantV || area != wantA {
			t.Fatal(v, area)
		}
		mass, e := result.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil || math.Abs(mass.Volume-wantV) > 1e-8 || math.Abs(mass.Area-wantA) > 1e-8 {
			t.Fatal(mass, e)
		}
		expected := frame.ToWorld(geom.Vec3{Z: (3200*4 - (588-27*math.Pi)*6.5) / wantV})
		if mass.Centroid.Distance(expected) > 1e-8 {
			t.Fatal(mass.Centroid, expected)
		}
		faces := result.Solid.Faces()
		if len(faces[result.FloorFace].Holes) != 1 || len(faces[result.IslandTopFace].Outer) != 4 {
			t.Fatal("missing retained floor/island cap")
		}
		m, e := result.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		mm, e := m.MassProperties()
		if e != nil || math.Abs(mm.Volume-wantV) > wantV*.005 {
			t.Fatal(mm, e)
		}
		if len(m.Triangles) > 2000 {
			t.Fatal("excessive display", len(m.Triangles))
		}
	}
}

func TestRectangularIslandPocketPersistenceAndRejects(t *testing.T) {
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := RectangularIslandPocket{Frame: frame, OuterMax: geom.Vec2{X: 20, Y: 20}, PocketMin: geom.Vec2{X: 3, Y: 3}, PocketMax: geom.Vec2{X: 17, Y: 17}, IslandCenter: geom.Vec2{X: 11, Y: 9}, IslandRadius: 3, Height: 8, Depth: 3}
	s, e := p.Solid(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, s); e != nil {
		t.Fatal(e)
	}
	restored, e := ReadJSON(&buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	mass, e := restored.MassProperties(context.Background(), parametric.IntegrationOptions{})
	want, _ := p.NominalVolume()
	if e != nil || math.Abs(mass.Volume-want) > 1e-8 {
		t.Fatal(mass, e)
	}
	for _, bad := range []RectangularIslandPocket{func() RectangularIslandPocket { q := p; q.Depth = 8; return q }(), func() RectangularIslandPocket { q := p; q.IslandRadius = 6; return q }(), func() RectangularIslandPocket { q := p; q.PocketMin.X = 0; return q }()} {
		if _, e = bad.Solid(context.Background()); e == nil {
			t.Fatal("invalid pocket accepted", bad)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = p.Solid(ctx); e != context.Canceled {
		t.Fatal(e)
	}
}
