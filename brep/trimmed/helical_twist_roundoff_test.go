package trimmed

import (
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBoundedTwistActualAcme(t *testing.T) {
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	pitch, major, clearance := 2.032, 9.525/2, .1016
	minor := major - pitch/2
	lead := 2 * pitch
	slope := math.Tan(14.5 * math.Pi / 180)
	for _, nut := range []bool{false, true} {
		center, endMargin := pitch*.65, pitch*.85
		height, stock, inner, outer, bore, phase := 21.59, major, minor, major, 0., .25
		half := func(r float64) float64 { return pitch/4 + slope*(r-major) }
		relief := minor - pitch*.025
		maxHalf := half(major + pitch*.05)
		if nut {
			height, stock, inner, outer, bore, phase = 10.668, major+pitch, minor+clearance, major+clearance, minor+clearance, 0
			half = func(r float64) float64 { return pitch/4 + slope*(major-r) + clearance*(1+slope) }
			relief = outer + pitch*.025
			maxHalf = half(bore - pitch*.05)
		}
		r := HelicalGrooveRecipe{Frame: frame, OuterRadius: stock, BoreRadius: bore, Height: height, Profile: []geom.Vec2{{X: inner, Y: center - half(inner)}, {X: outer, Y: center - half(outer)}, {X: outer, Y: center + half(outer)}, {X: inner, Y: center + half(inner)}}, Lead: lead, End: (height - center - endMargin) / lead, Hand: geom.LeftHanded, Starts: 2}
		var result HelicalOverlapRelief
		var e error
		budget := min(1e-10, max(stock, height)*1e-12)
		if nut {
			ends, err := GrooveWithPhasedEndReliefs(context.Background(), r, phase, relief, center+maxHalf+pitch*.01, endMargin+maxHalf+pitch*.01, budget)
			result, e = ends.HelicalOverlapRelief, err
		} else {
			result, e = GrooveWithPhasedOverlappingEntryRelief(context.Background(), r, phase, relief, center+maxHalf+pitch*.01, budget)
		}
		if e != nil {
			t.Fatal(nut, e)
		}
		twistExact := new(big.Rat).Quo(big.NewRat(-1, 1), ratInset(lead))
		twist, _ := twistExact.Float64()
		drift := new(big.Rat).Mul(new(big.Rat).Abs(new(big.Rat).Sub(twistExact, ratInset(twist))), ratInset(height))
		if drift.Sign() == 0 || ratInset(result.ChartRoundoff.MaxTurnError).Cmp(drift) < 0 {
			t.Fatal("missing full-height reciprocal drift", result.ChartRoundoff, drift)
		}
		if result.ChartRoundoff.MaxAxialError+4*math.Pi*stock*result.ChartRoundoff.MaxTurnError > budget {
			t.Fatal("chart budget exceeded", result.ChartRoundoff)
		}
		removed, _ := new(big.Rat).Add(result.GrooveVolumePiCoefficient, result.EntryVolumePiCoefficient).Float64()
		want := math.Pi * ((stock*stock-bore*bore)*height - removed)
		mass, e := result.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{MaxSubdivisions: 256, MaxSamples: 16000000})
		if e != nil || math.Abs(mass.Volume-want) > want*1e-8 {
			t.Fatal(nut, mass.Volume, want, e)
		}
		m, e := result.Solid.TessellateHelical(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal("display", nut, e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		t.Log("actual Acme", nut, "triangles", len(m.Triangles), "chart", result.ChartRoundoff)
	}
}

func TestBoundedTwistUsesFullStockHeight(t *testing.T) {
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	r := HelicalGrooveRecipe{Frame: frame, OuterRadius: 2, Height: 24, Profile: []geom.Vec2{{X: 1, Y: .4}, {X: 2, Y: .3}, {X: 2, Y: .6}, {X: 1, Y: .5}}, Lead: 3, End: 4, Hand: geom.RightHanded, Starts: 1}
	_, proof, e := GrooveWithBoundedChart(context.Background(), r, 1e-12)
	if e != nil {
		t.Fatal(e)
	}
	exact := big.NewRat(1, 3)
	rounded, _ := exact.Float64()
	drift := new(big.Rat).Mul(new(big.Rat).Abs(new(big.Rat).Sub(exact, ratInset(rounded))), big.NewRat(24, 1))
	if ratInset(proof.MaxTurnError).Cmp(drift) < 0 {
		t.Fatal("full stock height drift omitted", proof, drift)
	}
}

func TestBoundedTwistStrictAndBudget(t *testing.T) {
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := []geom.Vec2{{X: 1, Y: .4}, {X: 2, Y: .3}, {X: 2, Y: .6}, {X: 1, Y: .5}}
	for _, pitch := range []float64{1, 1.27} {
		r := HelicalGrooveRecipe{Frame: frame, OuterRadius: 2, Height: 8, Profile: p, Lead: pitch, End: 4, Hand: geom.RightHanded, Starts: 1}
		if pitch == 1.27 {
			if _, e := GrooveCylinderWithHelicalStarts(context.Background(), frame, 2, 8, p, pitch, 0, 4, geom.RightHanded, 1); e == nil {
				t.Fatal("strict reciprocal requirement relaxed")
			}
		}
		if _, _, e := GrooveWithBoundedChart(context.Background(), r, 1e-12); e != nil {
			t.Fatal(pitch, e)
		}
		if pitch == 1.27 {
			if _, _, e := GrooveWithBoundedChart(context.Background(), r, 1e-20); e == nil {
				t.Fatal("unbounded reciprocal drift admitted")
			}
		}
	}
}
