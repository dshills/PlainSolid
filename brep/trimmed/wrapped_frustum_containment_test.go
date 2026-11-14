package trimmed

import (
	"context"
	"errors"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCircularFrustumEccentricCavity(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	var stock Solid
	makeFrustum := func(c geom.Vec3, r, h, rise float64) Solid {
		t.Helper()
		w, e := geom.NewCanonicalWrappedCylinder(ctx, c, r, h, o)
		if e != nil {
			t.Fatal(e)
		}
		w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, rise, o)
		if e != nil {
			t.Fatal(e)
		}
		s, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	stock = makeFrustum(geom.Vec3{}, 2, 4, 2)
	// Inside heights [1,3] the stock radii are 5/2 and 7/2.
	// The cutter radii 5/4 and 9/4 leave a 5/4 clearance. Its
	// eccentric axis distance sqrt((1/2)^2+(1/2)^2) is irrational.
	inner := makeFrustum(geom.Vec3{X: .5, Y: .5, Z: 1}, 1.25, 2, 1)
	cut, history, e := CombineNestedSplineComponents(ctx, stock, inner, parametric.Difference, SplineShellBooleanOptions{})
	if e != nil || cut.Topology().Faces != 6 || len(cut.Topology().Shells) != 2 || len(history.FacesB) != 3 {
		t.Fatal("strict eccentric cavity", cut.Topology(), history, e)
	}
	// Increasing only that eccentric axis offset to the exact clearance
	// creates tangency on the two lateral surfaces, and must not sew.
	tangent := makeFrustum(geom.Vec3{X: 1.25, Z: 1}, 1.25, 2, 1)
	cut, _, e = CombineNestedSplineComponents(ctx, stock, tangent, parametric.Difference, SplineShellBooleanOptions{})
	if !errors.Is(e, geom.ErrConvergence) || len(cut.Faces()) != 0 {
		t.Fatal("tangent shell was accepted as a strict cavity", cut.Topology(), e)
	}
}
