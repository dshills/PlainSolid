package offset

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestEccentricFrustumIndependentInsets(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	for _, innerRise := range []float64{0, 3} {
		outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 4, o)
		if e != nil {
			t.Fatal(e)
		}
		outer, _, e = geom.DraftWrappedFrustum(ctx, outer, 0, 3, o)
		if e != nil {
			t.Fatal(e)
		}
		inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: .5, Y: .25}, 1, 4, o)
		if e != nil {
			t.Fatal(e)
		}
		inner, _, e = geom.DraftWrappedFrustum(ctx, inner, 0, innerRise, o)
		if e != nil {
			t.Fatal(e)
		}
		source, _, e := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		faces := source.Faces()
		faces = append(faces[2:], faces[:2]...)
		source, e = trimmed.NewSolid(source.Vertices(), source.Edges(), faces, source.Tolerance())
		if e != nil {
			t.Fatal(e)
		}
		before, e := source.ValidateCanonicalWrappedFrustumBore(ctx, o)
		if e != nil {
			t.Fatal(e)
		}
		out, h, e := InsetWrappedEccentricCircularFrustumTube(ctx, source, FrustumTubeInsets{.5, .25, .5, 1}, o)
		if e != nil {
			t.Fatal(e)
		}
		wantInner := [2]*big.Rat{big.NewRat(5, 4), big.NewRat(5, 4)}
		if innerRise == 3 {
			wantInner = [2]*big.Rat{big.NewRat(27, 16), big.NewRat(57, 16)}
		}
		wantOuter := [2]*big.Rat{big.NewRat(15, 4), big.NewRat(45, 8)}
		for i := 0; i < 2; i++ {
			if h.InnerRadii[i].Cmp(wantInner[i]) != 0 || h.OuterRadii[i].Cmp(wantOuter[i]) != 0 || h.RadialClearance[i].Cmp(new(big.Rat).Sub(wantOuter[i], wantInner[i])) != 0 {
				t.Fatal(h)
			}
		}
		for i := 0; i < 3; i++ {
			if h.SourceFaces[h.OuterFaces[i]][0] != before.OuterFaces[i] || h.SourceFaces[h.InnerFaces[i]][0] != before.InnerFaces[i] {
				t.Fatal("original ancestry", h)
			}
		}
		if out.Topology().Faces != 4 || out.Topology().EulerCharacteristic != 0 {
			t.Fatal(out.Topology())
		}
		R0, _ := wantOuter[0].Float64()
		R1, _ := wantOuter[1].Float64()
		r0, _ := wantInner[0].Float64()
		r1, _ := wantInner[1].Float64()
		height := 2.5
		volume := math.Pi * height * (R0*R0 + R0*R1 + R1*R1 - r0*r0 - r0*r1 - r1*r1) / 3
		area := math.Pi * ((R0+R1)*math.Hypot(height, R1-R0) + (r0+r1)*math.Hypot(height, r1-r0) + R0*R0 + R1*R1 - r0*r0 - r1*r1)
		mass, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil || math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 {
			t.Fatal(mass, volume, area, e)
		}
		innerVolume := math.Pi * height * (r0*r0 + r0*r1 + r1*r1) / 3
		if math.Abs(mass.Centroid.X+.5*innerVolume/volume) > 1e-7 || math.Abs(mass.Centroid.Y+.25*innerVolume/volume) > 1e-7 {
			t.Fatal("independent eccentric first moments", mass.Centroid)
		}
		agreement, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !agreement.Complete {
			t.Fatal(agreement, e)
		}
		var b bytes.Buffer
		if e = trimmed.WriteJSON(&b, out); e != nil {
			t.Fatal(e)
		}
		back, e := trimmed.ReadJSONContext(ctx, &b, trimmed.JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		proof, e := back.ValidateCanonicalWrappedFrustumBore(ctx, o)
		if e != nil || !proof.Complete || proof.Kind != "tube" {
			t.Fatal(proof, e)
		}
		for _, bad := range []FrustumTubeInsets{{0, 4, 0, 0}, {.5, .25, 2, 2}, {-.5, 0, 0, 0}, {.1, .25, .5, 1}} {
			rejected, rh, e := InsetWrappedEccentricCircularFrustumTube(ctx, source, bad, o)
			if e == nil || rejected.Topology().Faces != 0 || rh.Heights[0] != nil {
				t.Fatal("atomic invalid clearance/coordinates", bad, e)
			}
		}
		if innerRise == 0 {
			// Endpoint radius gap .5 is less than irrational axis distance sqrt(5)/4.
			rejected, _, e := InsetWrappedEccentricCircularFrustumTube(ctx, source, FrustumTubeInsets{0, 2.5, 0, 0}, o)
			if e == nil || rejected.Topology().Faces != 0 {
				t.Fatal("tangent wall accepted", e)
			}
		}
	}
}

func TestEccentricFrustumInsetTangency(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: 1}, 1, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	for _, amount := range []float64{2, 2.25} {
		out, h, e := InsetWrappedEccentricCircularFrustumTube(ctx, source, FrustumTubeInsets{InnerWall: amount}, o)
		if e == nil || out.Topology().Faces != 0 || h.InnerRadii[0] != nil {
			t.Fatal("tangent/crossing accepted", amount, e)
		}
	}
	out, _, e := InsetWrappedEccentricCircularFrustumTube(ctx, source, FrustumTubeInsets{InnerWall: 1.75}, o)
	if e != nil || out.Topology().Faces != 4 {
		t.Fatal("positive exact clearance", e)
	}
	// The previous coaxial-only entry point still rejects this source.
	out, _, e = InsetWrappedCircularFrustumTube(ctx, source, FrustumTubeInsets{}, o)
	if e == nil || out.Topology().Faces != 0 {
		t.Fatal("legacy scope changed", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	out, _, e = InsetWrappedEccentricCircularFrustumTube(cancelled, source, FrustumTubeInsets{}, o)
	if e != context.Canceled || out.Topology().Faces != 0 {
		t.Fatal(e)
	}
}
