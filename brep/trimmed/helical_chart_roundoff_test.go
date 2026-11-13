package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBoundedChartM3Profiles(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, internal := range []bool{false, true} {
		for _, hand := range []geom.HelixHandedness{geom.RightHanded, geom.LeftHanded} {
			pitch, major := .5, 1.5
			inner := major - math.Sqrt(3)*17/48*pitch
			a := pitch / 12
			b := a + (major-inner)/math.Sqrt(3)
			outerRadius, boreRadius := major, 0.
			if internal {
				inner = 1.25
				a = pitch/16 + (major-inner)/math.Sqrt(3)
				b = pitch / 16
				outerRadius, boreRadius = 3, inner
			}
			margin := pitch * .55
			p := []geom.Vec2{{X: inner, Y: margin - a}, {X: major, Y: margin - b}, {X: major, Y: margin + b}, {X: inner, Y: margin + a}}
			if !internal && hand == geom.RightHanded {
				if _, e := GrooveCylinderWithHelicalStarts(context.Background(), f, outerRadius, 4, p, pitch, 0, 4, hand, 1); e == nil {
					t.Fatal("strict constructor changed")
				}
			}
			s, proof, e := GrooveWithBoundedChart(context.Background(), HelicalGrooveRecipe{Frame: f, OuterRadius: outerRadius, BoreRadius: boreRadius, Height: 4, Profile: p, Lead: pitch, Start: 0, End: 4, Hand: hand, Starts: 1}, 1e-12)
			if e != nil {
				t.Fatal(internal, hand, e)
			}
			if !internal && hand == geom.RightHanded {
				mesh, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .1, AnalyticBound: true, MaxSubdivisions: 128, MaxTriangles: 1000000})
				if e != nil {
					t.Fatal("M3 mesh", e)
				}
				if e = mesh.ValidateSolid(); e != nil {
					t.Fatal(e)
				}
			}
			m, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{MaxSubdivisions: 256, MaxSamples: 16000000})
			if e != nil {
				t.Fatal(e)
			}
			radialIntegral := a*(major*major-inner*inner) + 2*(b-a)/(major-inner)*((major*major*major-inner*inner*inner)/3-inner*(major*major-inner*inner)/2)
			want := math.Pi*(outerRadius*outerRadius-boreRadius*boreRadius)*4 - 2*math.Pi*4*radialIntegral
			if math.Abs(m.Volume-want) > 1e-7 {
				t.Fatal("independent M3 volume", m.Volume, want)
			}
			if proof.MaxAxialError+4*math.Pi*outerRadius*proof.MaxTurnError > 1e-12 {
				t.Fatal("chart bound", proof)
			}
		}
	}
}

func TestBoundedChartRejects(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	r := HelicalGrooveRecipe{Frame: f, OuterRadius: 2, Height: 4, Profile: []geom.Vec2{{X: 1, Y: .3}, {X: 2, Y: .2}, {X: 2, Y: .4}, {X: 1, Y: .3}}, Lead: .5, End: 4, Hand: geom.RightHanded, Starts: 1}
	for _, limit := range []float64{0, -1, math.NaN(), math.Inf(1), 1e-3} {
		if _, _, e := GrooveWithBoundedChart(context.Background(), r, limit); e == nil {
			t.Fatal(limit)
		}
	}
	r.BoreRadius = math.NaN()
	if _, _, e := GrooveWithBoundedChart(context.Background(), r, 1e-12); e == nil {
		t.Fatal("nonfinite bore")
	}
}
