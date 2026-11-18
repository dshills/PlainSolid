package trimmed

import (
	"context"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestPlainCADLeadPhasedStarts(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	frame, err := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	const major, minor, bore, clearance, pitch = 4., 3., 3.1, .1, 2.
	slope := math.Tan(math.Pi / 12)
	for _, nut := range []bool{false, true} {
		r := HelicalGrooveRecipe{Frame: frame, OuterRadius: major, Height: 18, Lead: 8, Starts: 4, Start: 0, Hand: geom.RightHanded}
		lo, hi, phase := minor, major, .125
		half := func(x float64) float64 { return pitch/4 + slope*(x-major) }
		if nut {
			r.OuterRadius, r.BoreRadius, r.Height = 6, bore, 10
			lo, hi, phase = bore, major+clearance, 0
			half = func(x float64) float64 { return pitch/4 + slope*(major-x) + clearance*(1+slope) }
		}
		start := pitch * .65
		r.End = (r.Height - pitch*.85 - start) / r.Lead
		a, b := half(lo), half(hi)
		r.Profile = []geom.Vec2{{X: lo, Y: start - a}, {X: hi, Y: start - b}, {X: hi, Y: start + b}, {X: lo, Y: start + a}}
		for _, hand := range []geom.HelixHandedness{geom.RightHanded, geom.LeftHanded} {
			r.Hand = hand
			s, _, err := GrooveWithPhasedStarts(ctx, r, phase, 1e-12)
			if err != nil {
				t.Fatal(nut, hand, err)
			}
			if len(s.Topology().Shells) != 1 {
				t.Fatal("phased grooves must retain one shell")
			}
			props, err := s.MassProperties(ctx, parametric.IntegrationOptions{MaxSubdivisions: 256, MaxSamples: 16000000})
			if err != nil {
				t.Fatal(nut, hand, err)
			}
			integral := a*(hi*hi-lo*lo) + 2*(b-a)/(hi-lo)*((hi*hi*hi-lo*lo*lo)/3-lo*(hi*hi-lo*lo)/2)
			want := math.Pi * ((r.OuterRadius*r.OuterRadius-r.BoreRadius*r.BoreRadius)*r.Height - 2*r.End*float64(r.Starts)*integral)
			if math.Abs(props.Volume-want) > 1e-6 {
				t.Fatal(nut, hand, props.Volume, want)
			}
		}
	}
}
