package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func wrappedObliqueBandSupport(t *testing.T, scale float64) *geom.WrappedSurfacePatch {
	t.Helper()
	profile := []geom.Vec2{{X: 2}, {X: 2, Y: 2}, {Y: 2}, {X: -2, Y: 2}, {X: -2}, {X: -2, Y: -2}, {Y: -2}, {X: 2, Y: -2}, {X: 2}}
	var poles []geom.Vec3
	for _, p := range profile {
		for j, v := range []float64{-1, 0, 1} {
			x := .25
			if j == 1 {
				x = -.25
			}
			poles = append(poles, geom.Vec3{X: p.X + x, Y: p.Y, Z: scale*v + p.X + x})
		}
	}
	b, e := geom.NewBSplineSurface(2, 2, 9, 3, poles, []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}, []float64{-1, -1, -1, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(context.Background(), p, geom.WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	return w
}

func TestWrappedAlgebraicCovectorSeparator(t *testing.T) {
	ctx := context.Background()
	opts := WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}}
	a, e := NewWrappedAlgebraicQuadraticFace(ctx, wrappedObliqueBandSupport(t, 1), wrappedBandLoop(t, -.25, .25, -.5, 0), nil, opts)
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewWrappedAlgebraicQuadraticFace(ctx, wrappedObliqueBandSupport(t, 2), wrappedBandLoop(t, 3.75, 4.25, 0, .5), nil, opts)
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewWrappedAlgebraicOpenShell(ctx, a, b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	h := WrappedCovectorSeparator{Normal: [3]*big.Rat{big.NewRat(-1, 1), new(big.Rat), big.NewRat(1, 1)}, Level: new(big.Rat), ParameterAxes: [2]int{1, 1}, Scale: [2]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1)}, Offset: [2]*big.Rat{new(big.Rat), new(big.Rat)}}
	p, e := CertifyWrappedAlgebraicCovectorSeparatedPair(ctx, s, h, geom.SurfaceAffineCoordinateOptions{})
	if e != nil || p.Sides() != ([2]int{-1, 1}) {
		t.Fatal(p, e)
	}
	native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
	r, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	h.Normal[0].SetInt64(-2)
	if p.Separator().Normal[0].Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal("mutable normal")
	}
	if q, e := CertifyWrappedAlgebraicCovectorSeparatedPair(ctx, s, h, geom.SurfaceAffineCoordinateOptions{}); e == nil || q != nil {
		t.Fatal("wrong covector")
	}
	h = p.Separator()
	h.Level.SetInt64(1)
	if q, e := CertifyWrappedAlgebraicCovectorSeparatedPair(ctx, s, h, geom.SurfaceAffineCoordinateOptions{}); e == nil || q != nil {
		t.Fatal("mate outside plane")
	}
	for _, o := range []geom.SurfaceAffineCoordinateOptions{{MaxCells: 1, SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}}, {SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: p.ExactOperations() - 1}}} {
		if q, e := CertifyWrappedAlgebraicCovectorSeparatedPair(ctx, s, p.Separator(), o); e == nil || q != nil {
			t.Fatal("atomic budget")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := CertifyWrappedAlgebraicCovectorSeparatedPair(c, s, p.Separator(), geom.SurfaceAffineCoordinateOptions{}); e == nil || q != nil {
		t.Fatal("cancel")
	}
}
