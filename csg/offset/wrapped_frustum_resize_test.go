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

func TestResizeEccentricFrustumTube(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
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
	inner, _, e = geom.DraftWrappedFrustum(ctx, inner, 0, 1, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	faces := source.Faces()
	faces = append(faces[1:], faces[:1]...)
	source, e = trimmed.NewSolid(source.Vertices(), source.Edges(), faces, source.Tolerance())
	if e != nil {
		t.Fatal(e)
	}
	before, e := source.ValidateCanonicalWrappedFrustumBore(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	for _, span := range [][2]float64{{-1, 6}, {.5, 3}, {0, 4}} {
		out, h, e := ResizeWrappedCircularFrustumTube(ctx, source, span[0], span[1], o)
		if e != nil {
			t.Fatal(span, e)
		}
		a, b := 4+.75*span[0], 4+.75*span[1]
		c, d := 1+.25*span[0], 1+.25*span[1]
		height := span[1] - span[0]
		want := [2][2]float64{{a, b}, {c, d}}
		for i := 0; i < 2; i++ {
			if h.OuterRadii[i].Cmp(new(big.Rat).SetFloat64(want[0][i])) != 0 || h.InnerRadii[i].Cmp(new(big.Rat).SetFloat64(want[1][i])) != 0 {
				t.Fatal("original support extrapolation", h)
			}
		}
		for i := 0; i < 3; i++ {
			if h.SourceFaces[h.OuterFaces[i]][0] != before.OuterFaces[i] || h.SourceFaces[h.InnerFaces[i]][0] != before.InnerFaces[i] {
				t.Fatal("original ancestry", h)
			}
		}
		volume := math.Pi * height * (a*a + a*b + b*b - c*c - c*d - d*d) / 3
		boreVolume := math.Pi * height * (c*c + c*d + d*d) / 3
		area := math.Pi * ((a+b)*math.Hypot(height, b-a) + (c+d)*math.Hypot(height, d-c) + a*a + b*b - c*c - d*d)
		mass, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil || math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 || math.Abs(mass.Centroid.X+.5*boreVolume/volume) > 1e-7 || math.Abs(mass.Centroid.Y+.25*boreVolume/volume) > 1e-7 {
			t.Fatal(mass, volume, area, e)
		}
		if out.Topology().EulerCharacteristic != 0 {
			t.Fatal(out.Topology())
		}
		agreement, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !agreement.Complete {
			t.Fatal(agreement, e)
		}
		var native bytes.Buffer
		if e = trimmed.WriteJSON(&native, out); e != nil {
			t.Fatal(e)
		}
		back, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		proof, e := back.ValidateCanonicalWrappedFrustumBore(ctx, o)
		if e != nil || !proof.Complete {
			t.Fatal(e)
		}
	}
	for _, span := range [][2]float64{{-4, 4}, {2, 2}, {3, 2}, {math.NaN(), 4}, {math.Nextafter(-1, 0), 6}} {
		out, h, e := ResizeWrappedCircularFrustumTube(ctx, source, span[0], span[1], o)
		if e == nil || out.Topology().Faces != 0 || h.Heights[0] != nil {
			t.Fatal("atomic invalid/nonrepresentable span", span, e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if out, _, e := ResizeWrappedCircularFrustumTube(cancelled, source, -1, 6, o); e != context.Canceled || out.Topology().Faces != 0 {
		t.Fatal(e)
	}
	if out, _, e := ResizeWrappedCircularFrustumTube(ctx, source, -1, 6, geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || out.Topology().Faces != 0 {
		t.Fatal("work limit", e)
	}
}

func TestResizeTubeRejectsExtendedContact(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: .5}, 1, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	inner, _, e = geom.DraftWrappedFrustum(ctx, inner, 0, 2, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	for _, upper := range []float64{5, 6} {
		out, h, e := ResizeWrappedCircularFrustumTube(ctx, source, 0, upper, o)
		if e == nil || out.Topology().Faces != 0 || h.Heights[0] != nil {
			t.Fatal("extension reaches contact", upper, e)
		}
	}
	if out, _, e := ResizeWrappedCircularFrustumTube(ctx, source, 0, 4.5, o); e != nil || out.Topology().Faces != 4 {
		t.Fatal("valid shorter extension", e)
	}
}
