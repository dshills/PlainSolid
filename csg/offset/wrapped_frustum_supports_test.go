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

func TestIndependentFrustumSupportInsets(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	for _, descending := range []bool{false, true} {
		base, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 4, o)
		if e != nil {
			t.Fatal(e)
		}
		changes := [2]float64{0, 3}
		if descending {
			changes = [2]float64{3, 0}
		}
		support, _, e := geom.DraftWrappedFrustum(ctx, base, changes[0], changes[1], o)
		if e != nil {
			t.Fatal(e)
		}
		source, _, e := trimmed.NewWrappedFrustum(ctx, support, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		faces := source.Faces()
		faces = []trimmed.Face{faces[2], faces[0], faces[1]}
		source, e = trimmed.NewSolid(source.Vertices(), source.Edges(), faces, source.Tolerance())
		if e != nil {
			t.Fatal(e)
		}
		out, h, e := InsetWrappedCircularFrustumSupports(ctx, source, FrustumSupportInsets{Lateral: .5, LowerCap: .5, UpperCap: 1}, o)
		if e != nil {
			t.Fatal(e)
		}
		want := [2]*big.Rat{big.NewRat(15, 4), big.NewRat(45, 8)}
		if descending {
			want = [2]*big.Rat{big.NewRat(6, 1), big.NewRat(33, 8)}
		}
		for i := range want {
			if h.Radii[i].Cmp(want[i]) != 0 || len(h.RimEdges[i]) != 4 {
				t.Fatal(h)
			}
		}
		if h.Heights[0].Cmp(big.NewRat(1, 2)) != 0 || h.Heights[1].Cmp(big.NewRat(3, 1)) != 0 || h.NormalLength.Cmp(big.NewRat(5, 4)) != 0 {
			t.Fatal(h)
		}
		if h.SourceFaces[h.Lateral][0] != 1 || h.SourceFaces[h.Caps[0]][0] != 2 || h.SourceFaces[h.Caps[1]][0] != 0 {
			t.Fatal("original ownership", h)
		}
		a, _ := want[0].Float64()
		b, _ := want[1].Float64()
		height := 2.5
		volume := math.Pi * height * (a*a + a*b + b*b) / 3
		area := math.Pi * (a*a + b*b + (a+b)*math.Hypot(height, b-a))
		mass, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil || math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 {
			t.Fatal(mass, volume, area, e)
		}
		agreement, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !agreement.Complete {
			t.Fatal(agreement, e)
		}
		var buf bytes.Buffer
		if e = trimmed.WriteJSON(&buf, out); e != nil {
			t.Fatal(e)
		}
		back, e := trimmed.ReadJSONContext(ctx, &buf, trimmed.JSONLimits{})
		if e != nil || back.Topology().Faces != 3 {
			t.Fatal(e)
		}
		uniform, uh, e := InsetWrappedCircularFrustumSupports(ctx, source, FrustumSupportInsets{.5, .5, .5}, o)
		if e != nil {
			t.Fatal(e)
		}
		erosion, ep, e := ErodeWrappedCircularFrustum(ctx, support, .5, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		if uh.Radii[0].Cmp(ep.LowerRadius) != 0 || uh.Radii[1].Cmp(ep.UpperRadius) != 0 || uniform.Topology().Faces != erosion.Topology().Faces {
			t.Fatal("uniform reduction")
		}
		for _, bad := range []FrustumSupportInsets{{-1, 0, 0}, {0, 2, 2}, {10, 0, 0}, {math.NaN(), 0, 0}, {.1, .5, 1}} {
			rejected, rh, e := InsetWrappedCircularFrustumSupports(ctx, source, bad, o)
			if e == nil || rejected.Topology().Faces != 0 || rh.Radii[0] != nil {
				t.Fatal("atomic rejection", bad, e)
			}
		}
	}
}

func TestFrustumSupportInsetProofLimits(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	base, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	for _, rise := range []float64{3, 4} {
		w, _, e := geom.DraftWrappedFrustum(ctx, base, 0, rise, o)
		if e != nil {
			t.Fatal(e)
		}
		s, _, e := trimmed.NewWrappedFrustum(ctx, w, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		options := o
		if rise == 3 {
			options.MaxOperations = 1
		}
		out, h, e := InsetWrappedCircularFrustumSupports(ctx, s, FrustumSupportInsets{.5, .5, 1}, options)
		if e == nil || out.Topology().Faces != 0 || h.Radii[0] != nil {
			t.Fatal("irrational normal or work limit", rise, e)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		out, h, e = InsetWrappedCircularFrustumSupports(cancelled, s, FrustumSupportInsets{.5, .5, 1}, o)
		if e != context.Canceled || out.Topology().Faces != 0 || h.Radii[0] != nil {
			t.Fatal("cancellation", e)
		}
	}
}
