package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicExactCurveKnotRemovalChecksReinsertion(t *testing.T) {
	ctx := context.Background()
	o := SurfacePatchOptions{}
	s, err := NewPeriodicBSplineWithExactWeights(3,
		[]Vec3{{1, 0, 0}, {3, 2, 1}, {2, -1, 3}, {-1, 1, 2}, {-2, 0, 1}}, []float64{1, 3, 2, 4, 5}, -2, 4)
	if err != nil {
		t.Fatal(err)
	}
	original, err := s.ExactCyclicSpline(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	first := big.NewRat(-13, 7)
	stations := []*big.Rat{first, big.NewRat(13, 7), big.NewRat(-6, 5), big.NewRat(2, 1), big.NewRat(-2, 1)}
	curve := original
	for _, station := range stations {
		curve, err = curve.InsertKnot(ctx, station, o)
		if err != nil {
			t.Fatal(station, err)
		}
	}
	cycles := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 102))
	for i := len(stations) - 1; i >= 0; i-- {
		curve, err = curve.RemoveKnot(ctx, new(big.Rat).Add(stations[i], cycles), o)
		if err != nil {
			t.Fatal("exact inverse of insertion", stations[i], err)
		}
	}
	assertCyclicRationalNet(t, original.Knots(), curve.Knots(), original.HomogeneousControls(), curve.HomogeneousControls())
	if out, err := curve.RemoveKnot(ctx, big.NewRat(-6, 5), o); out != nil || !errors.Is(err, ErrUnrepresentable) {
		t.Fatal("genuine source knot must not be removed by approximation", err)
	}
	if out, err := curve.RemoveKnot(ctx, big.NewRat(-2, 1), o); out != nil || !errors.Is(err, ErrDomain) {
		t.Fatal("cannot remove the only current seam", err)
	}
	if out, err := curve.RemoveKnot(ctx, first, o); out != nil || !errors.Is(err, ErrDomain) {
		t.Fatal("cannot remove a nonknot", err)
	}
	inserted, err := original.InsertKnot(ctx, first, o)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := inserted.RemoveKnot(ctx, first, SurfacePatchOptions{MaxOperations: 30}); out != nil || !errors.Is(err, ErrConvergence) {
		t.Fatal("short exact budget published a candidate net", err)
	}
	h := inserted.HomogeneousControls()
	h[2][2].Add(h[2][2], big.NewRat(1, 1024))
	changed, err := NewExactPeriodicSplineCurve(ctx, 3, inserted.Knots(), h, o)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := changed.RemoveKnot(ctx, first, o); out != nil || !errors.Is(err, ErrUnrepresentable) {
		t.Fatal("one changed homogeneous coordinate invalidates the complete inverse", err)
	}
}

func TestPeriodicExactTensorKnotRemovalIsAtomicAcrossRows(t *testing.T) {
	ctx := context.Background()
	o := SurfacePatchOptions{}
	d := BSplineSurfaceData{DegreeU: 2, DegreeV: 2, CountU: 4, CountV: 5}
	for u := range d.CountU {
		for v := range d.CountV {
			d.Poles = append(d.Poles, Vec3{float64(u), float64(v), float64((u + v*v) % 5)})
			d.Weights = append(d.Weights, float64(1+(u+v)%5))
		}
	}
	base, err := NewPeriodicBSplineSurface(d, &PeriodicSplineAxis{Start: -1, Period: 3}, &PeriodicSplineAxis{Start: 2, Period: 1})
	if err != nil {
		t.Fatal(err)
	}
	original, err := base.ExactCyclicSurface(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	ux, vy := big.NewRat(17, 11), big.NewRat(15, 7)
	uSurface, err := original.InsertPeriodicKnot(ctx, 0, ux, o)
	if err != nil {
		t.Fatal(err)
	}
	s, err := uSurface.InsertPeriodicKnot(ctx, 1, vy, o)
	if err != nil {
		t.Fatal(err)
	}
	// Removing in the opposite order tests tensor separability, including
	// a row blend that wraps after the final source U station.
	s, err = s.RemovePeriodicKnot(ctx, 0, ux, o)
	if err != nil {
		t.Fatal(err)
	}
	refinedV := s
	s, err = s.RemovePeriodicKnot(ctx, 1, vy, o)
	if err != nil {
		t.Fatal(err)
	}
	u, v, h := s.Data()
	ou, ov, oh := original.Data()
	assertCyclicRationalNet(t, ou.Knots, u.Knots, oh, h)
	assertCyclicRationalNet(t, ov.Knots, v.Knots, oh, h)
	u, v, h = refinedV.Data()
	h[(u.Count-1)*v.Count+1][0].Add(h[(u.Count-1)*v.Count+1][0], big.NewRat(1, 4096))
	changed, err := NewExactPeriodicSplineSurface(ctx, u, v, h, o)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := changed.RemovePeriodicKnot(ctx, 1, vy, o); out != nil || !errors.Is(err, ErrUnrepresentable) {
		t.Fatal("one invalid row published a partially refined surface", err)
	}
}

func assertCyclicRationalNet(t *testing.T, a, b []*big.Rat, h, g [][4]*big.Rat) {
	t.Helper()
	if len(a) != len(b) || len(h) != len(g) {
		t.Fatal("wrong knot or homogeneous control count")
	}
	for i, q := range a {
		if q.Cmp(b[i]) != 0 {
			t.Fatal("original knot changed", i, q, b[i])
		}
	}
	for i := range h {
		for j, q := range h[i] {
			if q.Cmp(g[i][j]) != 0 {
				t.Fatal("homogeneous control changed", i, j, q, g[i][j])
			}
		}
	}
}
