package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestFuseAnnularPedestal(t *testing.T) {
	for _, axis := range []geom.Vec3{{Z: 1}, {X: -1}, {Y: 1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 3, Y: 5, Z: 7}, axis)
		stock, e := TopFilletedRoundedRectangle(context.Background(), f, RoundedRectangle{Min: geom.Vec2{X: -90, Y: -80}, Max: geom.Vec2{X: 90, Y: 80}, Radii: [4]float64{6, 6, 6, 6}}, 12, 2, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		before, e := stock.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		var original bytes.Buffer
		if e = WriteJSON(&original, stock); e != nil {
			t.Fatal(e)
		}
		joined, center, e := FuseAnnularPedestal(context.Background(), stock, 1, f, 32, 62, 12, 32)
		if e != nil {
			t.Fatal(axis, e)
		}
		after, e := joined.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		dv, da := math.Pi*(62*62-32*32)*20, 2*math.Pi*(62+32)*20
		if math.Abs(after.Volume-before.Volume-dv) > 1e-6 || math.Abs(after.Area-before.Area-da) > 1e-6 {
			t.Fatal(before, after, dv, da)
		}
		area, e := joined.PlanarFaceArea(center)
		if e != nil || math.Abs(area-math.Pi*32*32) > 1e-8 {
			t.Fatal(area, e)
		}
		area, e = joined.PlanarFaceArea(center + 1)
		if e != nil || math.Abs(area-math.Pi*(62*62-32*32)) > 1e-8 {
			t.Fatal(area, e)
		}
		if len(joined.Faces()[1].Holes) != 1 {
			t.Fatal("original top did not retain annular opening")
		}
		m, e := joined.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal("mesh", e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		t.Log("pedestal triangles", len(m.Triangles))
		var untouched bytes.Buffer
		_ = WriteJSON(&untouched, stock)
		if !bytes.Equal(original.Bytes(), untouched.Bytes()) {
			t.Fatal("stock mutated")
		}
		var saved bytes.Buffer
		if e = WriteJSON(&saved, joined); e != nil {
			t.Fatal(e)
		}
		restored, e := ReadJSON(&saved, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		mass, e := restored.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil || math.Abs(mass.Volume-after.Volume) > 1e-6 {
			t.Fatal(mass, e)
		}
	}
}

func TestFuseAnnularPedestalRejects(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	s, e := ExtrudeRoundedRectangle(context.Background(), f, RoundedRectangle{Min: geom.Vec2{X: -10, Y: -10}, Max: geom.Vec2{X: 10, Y: 10}, Radii: [4]float64{2, 2, 2, 2}}, nil, nil, 8)
	if e != nil {
		t.Fatal(e)
	}
	for _, q := range [][4]float64{{2, 11, 8, 12}, {2, 4, 7, 12}, {4, 4, 8, 12}, {2, 4, 8, 8}} {
		if _, _, e = FuseAnnularPedestal(context.Background(), s, 1, f, q[0], q[1], q[2], q[3]); e == nil {
			t.Fatal(q)
		}
	}
	pocket, e := ExtrudeBoxWithPocketsAndBore(context.Background(), f, geom.Vec3{X: 20, Y: 20, Z: 8}, []BoxBlindPocket{{Footprint: ExtrusionRectangle{Min: geom.Vec2{X: 5, Y: 5}, Max: geom.Vec2{X: 15, Y: 15}}, Floor: 5}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	centered, _ := geom.FrameFromNormal(geom.Vec3{X: 10, Y: 10}, geom.Vec3{Z: 1})
	if _, _, e = FuseAnnularPedestal(context.Background(), pocket, 10, centered, 1, 2, 5, 10); e == nil {
		t.Fatal("stock above selected cap accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e = FuseAnnularPedestal(ctx, s, 1, f, 2, 4, 8, 12); e != context.Canceled {
		t.Fatal(e)
	}
}
