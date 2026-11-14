package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestExactConicalLowerRollingFillet(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 8, 3, o)
	if e != nil {
		t.Fatal(e)
	}
	w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	f := source.Faces()
	source, e = NewSolid(source.Vertices(), source.Edges(), []Face{f[2], f[0], f[1]}, source.Tolerance())
	if e != nil {
		t.Fatal(e)
	}
	out, h, e := FilletWrappedFrustumLowerRim(ctx, source, 15.0/8, o)
	if e != nil {
		t.Fatal(e)
	}
	if out.Topology().Faces != 4 || out.Topology().EulerCharacteristic != 2 || h.Blends[0] < 0 || h.Blends[1] != -1 {
		t.Fatal(out.Topology(), h)
	}
	want := [6]*big.Rat{big.NewRat(59, 8), big.NewRat(15, 8), big.NewRat(3, 5), big.NewRat(-4, 5), big.NewRat(17, 2), big.NewRat(3, 8)}
	got := [6]*big.Rat{h.MeridianCenter[0], h.MeridianCenter[1], h.ConeNormal[0], h.ConeNormal[1], h.Tangency[0], h.Tangency[1]}
	for i := range want {
		if got[i].Cmp(want[i]) != 0 {
			t.Fatal("exact tangency", i, got[i])
		}
	}
	// Unit cone normal and exact line contact imply full circular tangency under
	// revolution, independently of the native edge construction.
	nx, nz := h.ConeNormal[0], h.ConeNormal[1]
	if new(big.Rat).Add(new(big.Rat).Mul(nx, nx), new(big.Rat).Mul(nz, nz)).Cmp(big.NewRat(1, 1)) != 0 || new(big.Rat).Add(nz, new(big.Rat).Mul(big.NewRat(4, 3), nx)).Sign() != 0 {
		t.Fatal("normal identity")
	}
	if new(big.Rat).Sub(h.Tangency[0], new(big.Rat).Mul(big.NewRat(4, 3), h.Tangency[1])).Cmp(big.NewRat(8, 1)) != 0 {
		t.Fatal("source cone contact")
	}
	if h.SourceFaces[h.Lateral][0] != 1 || h.SourceFaces[h.Caps[0]][0] != 2 || h.SourceFaces[h.Caps[1]][0] != 0 || len(h.SourceFaces[h.Blends[0]]) != 2 {
		t.Fatal(h.SourceFaces)
	}
	// Independent exact-circle antiderivatives, evaluated numerically only here.
	M, r := 59.0/8, 15.0/8
	theta := -math.Atan2(4, 3)
	lo := -math.Pi / 2
	primitive := func(a float64) float64 {
		s, c := math.Sin(a), math.Cos(a)
		return math.Pi * r * (M*M*s + M*r*(a+s*c) + r*r*(s-s*s*s/3))
	}
	V := primitive(theta) - primitive(lo) + math.Pi*(3-.375)*(8.5*8.5+8.5*12+144)/3
	A := 2*math.Pi*r*(M*(theta-lo)+r*(math.Sin(theta)+1)) + math.Pi*(8.5+12)*math.Hypot(3-.375, 12-8.5) + math.Pi*(M*M+144)
	mass, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || math.Abs(mass.Volume-V) > 1e-7 || math.Abs(mass.Area-A) > 1e-7 {
		t.Fatal(mass, V, A, e)
	}
	var b bytes.Buffer
	if e = WriteJSON(&b, out); e != nil {
		t.Fatal(e)
	}
	back, e := ReadJSONContext(ctx, &b, JSONLimits{})
	if e != nil || back.Topology().Faces != 4 {
		t.Fatal(e)
	}
}

func TestConicalRollingFilletSecondSlopeAndLimits(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	makeSource := func(radius, height, change float64) Solid {
		w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, radius, height, o)
		if e != nil {
			t.Fatal(e)
		}
		w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, change, o)
		if e != nil {
			t.Fatal(e)
		}
		s, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	// Different rational normal and exact torus endpoint V=13/4.
	second := makeSource(64, 56, 192)
	out, h, e := FilletWrappedFrustumLowerRim(ctx, second, 175.0/8, o)
	if e != nil || out.Topology().Faces != 4 || h.ConeNormal[0].Cmp(big.NewRat(7, 25)) != 0 || h.ConeNormal[1].Cmp(big.NewRat(-24, 25)) != 0 || h.Tangency[0].Cmp(big.NewRat(67, 1)) != 0 || h.Tangency[1].Cmp(big.NewRat(7, 8)) != 0 {
		t.Fatal(h, e)
	}
	stock := makeSource(8, 3, 4)
	for _, r := range []float64{0, -1, 1.5, 15, math.NaN()} {
		bad, h, e := FilletWrappedFrustumLowerRim(ctx, stock, r, o)
		if e == nil || bad.Topology().Faces != 0 || h.Radius != nil {
			t.Fatal("atomic radius/representation failure", r, e)
		}
	}
	for _, source := range []Solid{makeSource(8, 4, 3), makeSource(8, 3, 3), makeSource(8, 3, -4), makeSource(8, 3, 0)} {
		bad, h, e := FilletWrappedFrustumLowerRim(ctx, source, 15.0/8, o)
		if e == nil || bad.Topology().Faces != 0 || h.Radius != nil {
			t.Fatal("unsupported angle/normal", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if bad, h, e := FilletWrappedFrustumLowerRim(cancelled, stock, 15.0/8, o); e != context.Canceled || bad.Topology().Faces != 0 || h.Radius != nil {
		t.Fatal(e)
	}
	if bad, h, e := FilletWrappedFrustumLowerRim(ctx, stock, 15.0/8, geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || bad.Topology().Faces != 0 || h.Radius != nil {
		t.Fatal("budget", e)
	}
}
