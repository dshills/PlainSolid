package thread

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func pairFixture(t *testing.T, hand geom.HelixHandedness) PairRecipe {
	t.Helper()
	frame, e := geom.NewFrame(geom.Vec3{X: 2, Y: -3, Z: 1}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return PairRecipe{
		Shaft: trimmed.HelicalGrooveRecipe{
			Frame: frame, OuterRadius: 2, Height: 8, Lead: 2, Start: .5, End: 2.5, Hand: hand, Starts: 2,
			Profile: []geom.Vec2{{X: 1.5, Y: -.125}, {X: 2, Y: -.125}, {X: 2, Y: .625}, {X: 1.5, Y: .625}},
		},
		Nut: trimmed.HelicalGrooveRecipe{
			Frame: frame, OuterRadius: 3, BoreRadius: 1.75, Height: 8, Lead: 2, Start: .5, End: 2.5, Hand: hand, Starts: 2,
			Profile: []geom.Vec2{{X: 1.75, Y: .5}, {X: 2.25, Y: .5}, {X: 2.25, Y: 1}, {X: 1.75, Y: 1}},
		},
	}
}

func TestBuildPairWholeBandAndNativeVolumes(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, hand := range []geom.HelixHandedness{geom.RightHanded, geom.LeftHanded} {
		recipe := pairFixture(t, hand)
		pair, e := BuildPair(ctx, recipe, geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(hand, e)
		}
		proof := pair.Fit
		if proof.Minimum.Cmp(big.NewRat(1, 8)) != 0 || proof.Spacing.Cmp(big.NewRat(1, 1)) != 0 || len(proof.Flanks) != 1 ||
			proof.RadialBand[0].Cmp(big.NewRat(7, 4)) != 0 || proof.RadialBand[1].Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal("wrong two-flank proof", proof)
		}
		for _, cap := range []int{0, 1} {
			if proof.Flanks[0].Lower[cap].Cmp(big.NewRat(1, 8)) != 0 || proof.Flanks[0].Upper[cap].Cmp(big.NewRat(1, 8)) != 0 {
				t.Fatal("wrong exact flank gaps", proof.Flanks)
			}
		}
		if pair.ShaftRemovedPi.Cmp(big.NewRat(21, 4)) != 0 || pair.NutRemovedPi.Cmp(big.NewRat(4, 1)) != 0 {
			t.Fatal(pair.ShaftRemovedPi, pair.NutRemovedPi)
		}
		for i, body := range []trimmed.Solid{pair.Shaft.Solid(), pair.Nut.Solid()} {
			want := []float64{26.75 * math.Pi, 43.5 * math.Pi}[i]
			volume, e := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
			if e != nil || math.Abs(volume.Volume-want) > 1e-6 {
				t.Fatal(i, hand, volume.Volume, want, e)
			}
			var native bytes.Buffer
			if e := trimmed.WriteJSON(&native, body); e != nil {
				t.Fatal(e)
			}
			got, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
			if e != nil || got.Topology().InnerWires != 2 || got.Topology().EulerCharacteristic != []int{2, 0}[i] {
				t.Fatal(i, got.Topology(), e)
			}
		}
		// On the +X ray at r=15/8, shaft and nut materials alternate
		// strictly through the proved engagement window.
		for _, w := range []struct {
			z          int64
			shaft, nut parametric.Location
		}{{16, parametric.Outside, parametric.Inside}, {23, parametric.Boundary, parametric.Outside}, {22, parametric.Inside, parametric.Outside}} {
			r, z := big.NewRat(15, 8), big.NewRat(w.z, 8)
			phase := [2]*big.Rat{big.NewRat(1, 1), new(big.Rat)}
			s, _, e := pair.Shaft.ClassifyAxialProbe(ctx, r, z, phase, geom.HelicalStartsLineOptions{})
			n, _, e2 := pair.Nut.ClassifyAxialProbe(ctx, r, z, phase, geom.HelicalStartsLineOptions{})
			if e != nil || e2 != nil || s != w.shaft || n != w.nut {
				t.Fatal(hand, z, s, n, w, e, e2)
			}
		}
	}
}

func TestBuildPairRejectsUnfittedFlanks(t *testing.T) {
	ctx := context.Background()
	for _, delta := range []float64{-.125, .125} {
		r := pairFixture(t, geom.RightHanded)
		for i := range r.Nut.Profile {
			r.Nut.Profile[i].Y += delta
		}
		if p, e := BuildPair(ctx, r, geom.SurfacePatchEvaluationOptions{}); !errors.Is(e, geom.ErrDomain) || len(p.Shaft.Solid().Faces()) != 0 {
			t.Fatal("zero flank clearance accepted", delta, e)
		}
	}
	r := pairFixture(t, geom.RightHanded)
	r.Nut.Hand = geom.LeftHanded
	if _, e := BuildPair(ctx, r, geom.SurfacePatchEvaluationOptions{}); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("opposite screw hand accepted", e)
	}
}
