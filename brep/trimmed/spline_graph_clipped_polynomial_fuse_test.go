package trimmed

import (
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func checkClippedFuseMass(t *testing.T, r *ClippedPolynomialFuse, layout, axis, profile int) {
	t.Helper()
	// In the original unit physical parameters, B=3+uv/64 and
	// A-B=(105/128)(v-f)(v-g). The physical Jacobian is six.
	// Integrate B-base on the full footprint, then add A-B on the
	// retained A strips and the two positive overlap sectors. At fixed
	// u their dependent antiderivative is v³/3-(f+g)v²/2+fgv.
	volumes := [2][2]string{
		{"677452138343329261127/41216943789694779392", "677376236977816168007/41216943789694779392"},
		{"680001999132163925575/41216943789694779392", "679897637611624031815/41216943789694779392"},
	}
	want, _ := new(big.Rat).SetString(volumes[layout][profile])
	if r.ExactVolume().Cmp(want) != 0 {
		t.Fatal("independent original material antiderivatives", layout, axis, profile, r.ExactVolume(), want)
	}
	// Independent 32-point Gauss-Legendre per slab integrates the
	// original roof derivatives on the selected strips and sectors.
	// Add the planar base, outer wall fibers (roof-base), and interior
	// riser fibers (selected-roof minus continuing-roof).
	areas := [2][2][2]float64{
		{{40.144998232704417, 40.443802639426487}, {40.130544012507457, 40.432947134870815}},
		{{40.273027597410483, 40.637944013171207}, {40.253281989598470, 40.623031457871193}},
	}
	value, _ := want.Float64()
	mass, e := r.Boundary().MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	area := areas[layout][profile][axis]
	if e != nil || math.Abs(mass.Volume-value) > 1e-8 || math.Abs(mass.Area-area) > 1e-8 {
		t.Fatal("native boundary matches independently integrated physical volume and area", layout, axis, profile, mass.Volume, value, mass.Area, area, e)
	}
}

func TestNativeClippedPolynomialFuseDependentFootprint(t *testing.T) {
	ctx := context.Background()
	for layout := range 2 {
		base, a, b := rectangularPolynomialStock(t, 0, 0)
		d := geom.ParameterRectangle{UMin: 2, UMax: 5, VMin: -.75, VMax: 1}
		if layout == 0 {
			a = restrictOriginalClipStock(t, a, geom.ParameterRectangle{UMin: -2, UMax: 2, VMin: 8.25, VMax: 10})
		} else {
			d.VMax = .75
		}
		b = restrictOriginalClipStock(t, b, d)
		r, e := FuseClippedPolynomialEnvelope(ctx, base, a, b, 0, 1e-9, PolynomialCubicCrossingTangencyEnvelopeOptions{})
		if e != nil {
			t.Fatal("original footprint can also differ on the dependent axis", layout, e)
		}
		// Integrate B-base over the full unit footprint. Add the two
		// positive A sectors over dependent range [0,7/8] for partial
		// overlap and [0,1] for containment, with physical Jacobian six.
		volumes := [2]string{"10238301406447463/628920651087872", "10376007315048295/628920651087872"}
		if r.ExactVolume().RatString() != volumes[layout] {
			t.Fatal("independent dependent-strip union volume", r.ExactVolume())
		}
		risers := 0
		for _, w := range r.Walls() {
			if w.Low == geom.ClippedPolynomialRoofBHeight && w.High == geom.ClippedPolynomialRoofAHeight {
				risers++
				if w.Points[0].Y != 1.625 || w.Points[1].Y != 1.625 || len(w.Stations[0]) != 1 || len(w.Stations[1]) != 1 {
					t.Fatal("one whole interior step on the dependent side", w)
				}
			}
		}
		if risers != 1-layout {
			t.Fatal("only an ending higher roof creates a step", layout, risers)
		}
	}
}
