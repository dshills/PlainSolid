package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestM3OverlappingEntryRelief(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, internal := range []bool{false, true} {
		for _, hand := range []geom.HelixHandedness{geom.RightHanded, geom.LeftHanded} {
			pitch, major := .5, 1.5
			inner := major - math.Sqrt(3)*17/48*pitch
			a := pitch / 12
			b := a + (major-inner)/math.Sqrt(3)
			outer, bore, relief := major, 0., inner-pitch*.02
			if internal {
				inner = 1.25
				a = pitch/16 + (major-inner)/math.Sqrt(3)
				b = pitch / 16
				outer, bore, relief = 3, inner, major+pitch*.02
			}
			margin := pitch * .55
			r := HelicalGrooveRecipe{Frame: f, OuterRadius: outer, BoreRadius: bore, Height: 4, Profile: []geom.Vec2{{X: inner, Y: margin - a}, {X: major, Y: margin - b}, {X: major, Y: margin + b}, {X: inner, Y: margin + a}}, Lead: pitch, End: 4, Hand: hand, Starts: 1}
			result, e := GrooveWithOverlappingEntryRelief(context.Background(), r, relief, pitch*.74, 1e-12)
			if e != nil {
				t.Fatal(internal, hand, e)
			}
			if !internal && hand == geom.RightHanded {
				mesh, e := result.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .2, AnalyticBound: true, MaxSubdivisions: 128, MaxTriangles: 1000000})
				if e != nil {
					t.Fatal("mesh", e)
				}
				if e = mesh.ValidateSolid(); e != nil {
					t.Fatal(e)
				}
			}
			m, e := result.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-7, MaxSubdivisions: 256, MaxSamples: 16000000})
			if e != nil {
				t.Fatal("mass", internal, hand, e)
			}
			area, err := result.Solid.HelicalSurfaceArea(context.Background())
			if err != nil || math.Abs(area-m.Area) > 1e-6 {
				t.Fatal("independent retained surface area", internal, hand, area, m.Area, err)
			}
			gc, _ := result.GrooveVolumePiCoefficient.Float64()
			ec, _ := result.EntryVolumePiCoefficient.Float64()
			want := (4*(outer*outer-bore*bore) - gc - ec) * math.Pi
			if math.Abs(m.Volume-want) > 1e-6 {
				t.Fatal("independent retained integration", internal, hand, m.Volume, want)
			}
			stockRadius := major
			if internal {
				stockRadius = bore
			}
			fullEntry := math.Abs(relief*relief-stockRadius*stockRadius) * pitch * .74
			if ec <= 0 || ec >= fullEntry {
				t.Fatal("overlap must remove less than full relief", ec, fullEntry)
			}
			var data bytes.Buffer
			if e = WriteJSON(&data, result.Solid); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadJSON(&data, JSONLimits{})
			if e != nil {
				t.Fatal("reload", e)
			}
			if loaded.Topology().EulerCharacteristic != result.Solid.Topology().EulerCharacteristic {
				t.Fatal("topology reload")
			}
		}
	}
}

func TestOverlapReliefSignedFramesAndRejects(t *testing.T) {
	for _, axis := range []geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}, {X: -1}, {Y: -1}, {Z: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 3, Y: 5, Z: 7}, axis)
		r := HelicalGrooveRecipe{Frame: f, OuterRadius: 2, Height: 4, Profile: []geom.Vec2{{X: 1.5, Y: .25}, {X: 2, Y: .125}, {X: 2, Y: .375}, {X: 1.5, Y: .25 + .0625}}, Lead: 1, End: 2, Hand: geom.LeftHanded, Starts: 1}
		result, e := GrooveWithOverlappingEntryRelief(context.Background(), r, 1.4, .3, 1e-12)
		if e != nil {
			t.Fatal(axis, e)
		}
		if result.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("shaft topology")
		}
	}
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	r := HelicalGrooveRecipe{Frame: f, OuterRadius: 2, Height: 4, Profile: []geom.Vec2{{X: 1.5, Y: .25}, {X: 2, Y: .125}, {X: 2, Y: .375}, {X: 1.5, Y: .3125}}, Lead: 1, End: 2, Hand: geom.RightHanded, Starts: 1}
	for _, p := range [][2]float64{{1.4, .375}, {1.4, .1}, {1.6, .3}, {0, .3}, {1.4, math.NaN()}} {
		if _, e := GrooveWithOverlappingEntryRelief(context.Background(), r, p[0], p[1], 1e-12); e == nil {
			t.Fatal("unsupported geometry accepted", p)
		}
	}
	r.Starts = 2
	if _, e := GrooveWithOverlappingEntryRelief(context.Background(), r, 1.4, .3, 1e-12); e == nil {
		t.Fatal("unsupported starts")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.Starts = 1
	if _, e := GrooveWithOverlappingEntryRelief(ctx, r, 1.4, .3, 1e-12); e != context.Canceled {
		t.Fatal(e)
	}
}
