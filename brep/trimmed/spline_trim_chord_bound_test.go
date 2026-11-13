package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestSplineTrimChordBoundIndependent(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	curve, e := geom.NewBSpline(2, []geom.Vec3{{X: .125, Y: .25}, {X: .5, Y: .75}, {X: .875, Y: .25}}, []float64{0, 0, 0, 1, 1, 1}, []float64{1, .5, 1})
	if e != nil {
		t.Fatal(e)
	}
	trim := SplineUV{Curve: curve, Range: UVInterval{Min: 0, Max: 1}}
	a, b := geom.Vec3{X: .125, Y: .25, Z: .03125}, geom.Vec3{X: .875, Y: .25, Z: .21875}
	bound, e := BoundSplineTrimChord(ctx, patch, trim, a, b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if bound.SquaredDistance == nil || bound.SquaredDistance.Sign() <= 0 {
		t.Fatal(bound)
	}
	upper, _ := bound.SquaredDistance.Float64()
	for i := 0; i <= 100; i++ {
		// Direct rational quadratic and independent support S(u,v)=(u,v,uv).
		q := float64(i) / 100
		w := (1-q)*(1-q) + q*(1-q) + q*q
		u := (.125*(1-q)*(1-q) + .5*q*(1-q) + .875*q*q) / w
		v := (.25*(1-q)*(1-q) + .75*q*(1-q) + .25*q*q) / w
		p := geom.Vec3{X: u, Y: v, Z: u * v}
		d := b.Sub(a)
		s := p.Sub(a).Dot(d) / d.Dot(d)
		if s < 0 {
			s = 0
		}
		if s > 1 {
			s = 1
		}
		error := p.Sub(a.Add(d.Scale(s)))
		if error.Dot(error) > upper {
			t.Fatal("sample outside proved upper bound", i)
		}
	}
	trim.Range = UVInterval{Min: .375, Max: .625}
	uv0, _ := trim.EvalUV(0)
	uv1, _ := trim.EvalUV(1)
	small, e := BoundSplineTrimChord(ctx, patch, trim, geom.Vec3{X: uv0.X, Y: uv0.Y, Z: uv0.X * uv0.Y}, geom.Vec3{X: uv1.X, Y: uv1.Y, Z: uv1.X * uv1.Y}, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if small.SquaredDistance.Cmp(bound.SquaredDistance) >= 0 {
		t.Fatal("restriction did not improve enclosure")
	}
	failed, e := BoundSplineTrimChord(ctx, patch, trim, a, b, geom.SurfacePatchOptions{MaxOperations: 1})
	if e == nil || failed.SquaredDistance != nil {
		t.Fatal("budget false certificate", failed, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	failed, e = BoundSplineTrimChord(canceled, patch, trim, a, b, geom.SurfacePatchOptions{})
	if !errors.Is(e, context.Canceled) || failed.SquaredDistance != nil {
		t.Fatal(e)
	}
	trim.Range = UVInterval{Min: 0, Max: 1}
	trim.Reversed = true
	poly, e := TessellateSplineTrimCertified(ctx, patch, trim, .05, 256, geom.SurfacePatchOptions{})
	if e != nil || len(poly.Points) < 3 || poly.Parameters[0] != 1 || poly.Parameters[len(poly.Parameters)-1] != 0 {
		t.Fatal("certified reversed polyline", e)
	}
	tol := new(big.Rat).SetFloat64(.05)
	tol.Mul(tol, tol)
	for _, squared := range poly.SquaredErrors {
		if squared.Cmp(tol) > 0 {
			t.Fatal("false accepted tolerance")
		}
	}
	denied, e := TessellateSplineTrimCertified(ctx, patch, trim, .00001, 1, geom.SurfacePatchOptions{})
	if !errors.Is(e, geom.ErrConvergence) || len(denied.Points) != 0 {
		t.Fatal("partial budget polyline", e)
	}

	// Caller-owned output does not mutate subsequent certificates.
	small.SquaredDistance.SetInt64(-1)
	if bound.SquaredDistance.Cmp(big.NewRat(0, 1)) <= 0 {
		t.Fatal("aliased certificate")
	}
}

func TestCertifiedWrappedSplineTrimBoundary(t *testing.T) {
	ctx := context.Background()
	w := independentWrappedToroid(t)
	// This interval crosses an interior knot of the wrapped support.
	curve, e := geom.NewBSpline(1, []geom.Vec3{{X: .5, Y: .5}, {X: 1.5, Y: .5}}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	trim := SplineUV{Curve: curve, Range: UVInterval{Min: 0, Max: 1}}
	line, e := TessellateSplineTrimCertified(ctx, w.FundamentalPatch(), trim, .25, 128, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(line.Points) < 3 || len(line.SquaredErrors) != len(line.Points)-1 {
		t.Fatal("wrapped certified boundary")
	}
	for _, bound := range line.SquaredErrors {
		if bound.Cmp(big.NewRat(1, 16)) > 0 {
			t.Fatal("wrapped false certificate")
		}
	}
}
