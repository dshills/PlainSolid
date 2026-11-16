package parametric

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestExactNonuniformPeriodicCurveCodec(t *testing.T) {
	ctx := context.Background()
	s, err := geom.NewPeriodicBSplineWithExactWeights(2, []geom.Vec3{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}}, []float64{1, 3, 2, 4}, -2, 4)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.ExactCyclicSpline(ctx, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c, err = c.InsertKnot(ctx, big.NewRat(-15, 11), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeCurve(c)
	if err != nil || !bytes.Contains(raw, []byte(`"-15/11"`)) {
		t.Fatal("curve codec rounded the inserted knot", string(raw), err)
	}
	curve, err := DecodeCurve(raw, 5)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, ok := curve.(*geom.ExactPeriodicSplineCurve)
	if !ok {
		t.Fatalf("decoded %T", curve)
	}
	bytesAgain, err := EncodeCurve(curve)
	if err != nil || !bytes.Equal(raw, bytesAgain) {
		t.Fatal("native curve replay changed its recipe", err)
	}
	for _, t0 := range []*big.Rat{big.NewRat(-15, 11), big.NewRat(-1, 2), big.NewRat(2, 1)} {
		far := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(-1), 100))
		x := new(big.Rat).Add(t0, far)
		a, err := c.EvaluateRational(ctx, x, geom.SurfacePatchOptions{})
		b, e2 := reloaded.EvaluateRational(ctx, x, geom.SurfacePatchOptions{})
		if err != nil || e2 != nil || a.Span != b.Span || a.Cycle.Cmp(b.Cycle) != 0 {
			t.Fatal("wrong replayed curve owners", a, b, err, e2)
		}
		for i := range a.Point {
			if a.Point[i].Cmp(b.Point[i]) != 0 || a.Derivative[i].Cmp(b.Derivative[i]) != 0 {
				t.Fatal("replayed exact curve changed position or derivative", x, i)
			}
		}
	}
	for _, tc := range [][2]string{
		{`"-15/11"`, `"-30/22"`}, {`"-15/11"`, `"1e100000000"`},
		{`"-15/11"`, `"-2"`}, {`"Version":1`, `"Version":2`},
		{`"Kind":`, `"Start":1,"Kind":`},
	} {
		changed := bytes.Replace(raw, []byte(tc[0]), []byte(tc[1]), 1)
		if bytes.Equal(raw, changed) {
			t.Fatal("bad negative fixture", tc)
		}
		if out, err := DecodeCurve(changed, 5); err == nil || out != nil {
			t.Fatal("accepted a forged exact curve recipe", string(changed), err)
		}
	}
	if out, err := DecodeCurve(raw, 4); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("short curve control budget published geometry", err)
	}
}

func TestExactNonuniformPeriodicTensorCodec(t *testing.T) {
	ctx := context.Background()
	source := geom.BSplineSurfaceData{
		DegreeU: 1, DegreeV: 1, CountU: 3, CountV: 4,
		KnotsV: []float64{0, 0, .5, .5, 1, 1},
	}
	for u := range source.CountU {
		for v := range source.CountV {
			source.Poles = append(source.Poles, geom.Vec3{X: float64(u), Y: float64((u + v) % 3), Z: float64(v)})
			source.Weights = append(source.Weights, float64(1+(u+v)%4))
		}
	}
	base, err := geom.NewPeriodicBSplineSurface(source, &geom.PeriodicSplineAxis{Period: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s, err := base.ExactCyclicSurface(ctx, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.InsertPeriodicKnot(ctx, 0, big.NewRat(1, 7), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeSurface(s)
	if err != nil || !bytes.Contains(raw, []byte(`"1/7"`)) {
		t.Fatal("tensor codec rounded the inserted knot", string(raw), err)
	}
	reloaded, err := DecodeSurface(raw, 16)
	if err != nil {
		t.Fatal(err)
	}
	copy, ok := reloaded.(*geom.ExactPeriodicSplineSurface)
	if !ok {
		t.Fatalf("decoded %T", reloaded)
	}
	bytesAgain, err := EncodeSurface(reloaded)
	if err != nil || !bytes.Equal(raw, bytesAgain) {
		t.Fatal("native exact tensor replay changed bytes", err)
	}
	u, v := big.NewRat(1, 7), big.NewRat(1, 2)
	jet, err := copy.EvaluateRational(ctx, u, v, geom.SurfacePatchOptions{})
	if err != nil || jet.U.Span != 1 || jet.V.Span != 2 || jet.V.Cycle != nil || jet.Point[2].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("right owner of original ordinary discontinuity changed", jet, err)
	}
	for _, u := range []float64{-.375, .2, 1, 8.5} {
		a, err := s.Eval(u, .5)
		b, e2 := reloaded.Eval(u, .5)
		du, dv, e3 := s.Partials(u, .5)
		x, y, e4 := reloaded.Partials(u, .5)
		if err != nil || e2 != nil || e3 != nil || e4 != nil || a != b || du != x || dv != y {
			t.Fatal("numerical rotation of native tensor changes exact results", u, err, e2, e3, e4)
		}
	}
	for _, tc := range [][2]string{
		{`"Periodic":true`, `"Periodic":false`}, {`"Periodic":false`, `"Periodic":true`},
		{`"1/7"`, `"2/14"`}, {`"Kind":`, `"A":1,"Kind":`}, {`"Version":1`, `"Version":1,"Unexpected":true`},
	} {
		changed := bytes.Replace(raw, []byte(tc[0]), []byte(tc[1]), 1)
		if bytes.Equal(raw, changed) {
			t.Fatal("bad negative fixture", tc)
		}
		if out, err := DecodeSurface(changed, 16); err == nil || out != nil {
			t.Fatal("accepted a forged exact tensor recipe", string(changed), err)
		}
	}
	if out, err := DecodeSurface(raw, 15); out != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("short tensor control budget published geometry", err)
	}
}
