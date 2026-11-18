//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestWrappedRationalDraftFrustum(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w := makeCanonicalWrappedCylinder(t, true, 0)
	d, proof, e := geom.DraftWrappedFrustum(ctx, w, 0, -1, o)
	if e != nil || !proof.Complete || proof.LowerRadius.Cmp(big.NewRat(2, 1)) != 0 ||
		proof.UpperRadius.Cmp(big.NewRat(1, 1)) != 0 || d.Domain() != w.Domain() {
		t.Fatal(proof, e)
	}
	s, proof, e := NewWrappedFrustum(ctx, d, 1e-9, o)
	if e != nil || !proof.Circular || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(proof, e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
	if e != nil || math.Abs(p.Volume-35*math.Pi/3) > 1e-5 ||
		math.Abs(p.Area-(5+3*math.Sqrt(26))*math.Pi) > 1e-5 ||
		math.Abs(p.Centroid.Z-(5.*11./28.-2)) > 1e-6 {
		t.Fatal(p, e)
	}
	if _, _, e = NewWrappedCylinder(ctx, d, 1e-9, o); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("drafted support accepted as constant-radius cylinder", e)
	}
	if r, _, e := geom.DraftWrappedFrustum(ctx, d, 0, -1, o); r != nil || !errors.Is(e, geom.ErrDegenerate) {
		t.Fatal("collapsed endpoint accepted", e)
	}
	a, b, split, e := SplitWrappedFrustumAtZ(ctx, d, .5, 1e-9, o)
	if e != nil || split.Lower.CutCap != 2 || split.Upper.CutCap != 1 {
		t.Fatal(split, e)
	}
	for i, q := range []Solid{a, b} {
		m, e := q.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
		want := [2]float64{185 * math.Pi / 24, 95 * math.Pi / 24}[i]
		if e != nil || math.Abs(m.Volume-want) > 1e-5 {
			t.Fatal(i, m, want, e)
		}
	}
	a, b, _, e = SplitWrappedFrustumAtZ(ctx, d, 0, 1e-9, o) // radius 8/5
	if !errors.Is(e, geom.ErrUnrepresentable) || len(a.faces) != 0 || len(b.faces) != 0 {
		t.Fatal("nonrepresentable cut geometry was returned", e)
	}
}
