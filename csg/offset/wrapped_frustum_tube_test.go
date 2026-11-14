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

func TestFrustumTubeIndependentWallInsets(t *testing.T) {
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
		inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 1, 4, o)
		if e != nil {
			t.Fatal(e)
		}
		inner, _, e = geom.DraftWrappedFrustum(ctx, inner, 0, innerRise, o)
		if e != nil {
			t.Fatal(e)
		}
		source, _, e := trimmed.NewWrappedCircularFrustumBore(ctx, outer, inner, 1e-9, o)
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
		out, h, e := InsetWrappedCircularFrustumTube(ctx, source, FrustumTubeInsets{.5, .25, .5, 1}, o)
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
			rejected, rh, e := InsetWrappedCircularFrustumTube(ctx, source, bad, o)
			if e == nil || rejected.Topology().Faces != 0 || rh.Heights[0] != nil {
				t.Fatal("atomic invalid clearance/coordinates", bad, e)
			}
		}
		if innerRise == 0 {
			// Exact lower-rim contact: outer r=4, expanded inner r=4.
			rejected, _, e := InsetWrappedCircularFrustumTube(ctx, source, FrustumTubeInsets{0, 3, 0, 0}, o)
			if e == nil || rejected.Topology().Faces != 0 {
				t.Fatal("tangent wall accepted", e)
			}
		}
	}
}

func TestFrustumTubeInsetScope(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"eccentric", "blind", "limited", "cancelled"} {
		base, height := geom.Vec3{}, 4.0
		if kind == "eccentric" {
			base.X = .25
		}
		if kind == "blind" {
			height = 3
		}
		inner, e := geom.NewCanonicalWrappedCylinder(ctx, base, 1, height, o)
		if e != nil {
			t.Fatal(e)
		}
		source, _, e := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		options := o
		if kind == "limited" {
			options.MaxOperations = 1
		}
		callCtx := ctx
		if kind == "cancelled" {
			var cancel context.CancelFunc
			callCtx, cancel = context.WithCancel(ctx)
			cancel()
		}
		out, h, e := InsetWrappedCircularFrustumTube(callCtx, source, FrustumTubeInsets{.25, .25, .25, .25}, options)
		if e == nil || out.Topology().Faces != 0 || h.Heights[0] != nil {
			t.Fatal("scope failure", kind, e)
		}
	}
}
