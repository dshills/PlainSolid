package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestExactBezierNumericalQuadratureJet(t *testing.T) {
	// Nonbinary positive weights ensure the approximate homogeneous evaluator
	// is exercised even though a lossless Euclidean export is unavailable.
	var h [][4]*big.Rat
	for i := range 4 {
		for j := range 3 {
			w := big.NewRat(int64(11+i+j), 17)
			h = append(h, [4]*big.Rat{big.NewRat(int64(i*i-j), 7), big.NewRat(int64(j*j+i), 11), big.NewRat(int64(i*j-2), 13), w})
		}
	}
	p, err := NewExactHomogeneousBezierSurface(context.Background(), 3, 2, h, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p, err = p.Trim(ParameterRectangle{UMin: .125, UMax: .875, VMin: .25, VMax: .9375})
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewExactBezierSurface(p)
	if err != nil {
		t.Fatal(err)
	}
	b := a.NumericalSurface()
	if b == a {
		t.Fatal("ordinary positive-weight rational chart should use the approximate jet")
	}
	for _, w := range []Vec2{{.125, .25}, {.875, .9375}, {.5, .6}, {.17, .912}} {
		want, err := a.Eval(w.X, w.Y)
		if err != nil {
			t.Fatal(err)
		}
		u, v, err := a.Partials(w.X, w.Y)
		if err != nil {
			t.Fatal(err)
		}
		got, err := b.Eval(w.X, w.Y)
		if err != nil {
			t.Fatal(err)
		}
		bu, bv, err := b.Partials(w.X, w.Y)
		if err != nil || want.Distance(got) > 1e-14 || u.Distance(bu) > 1e-14 || v.Distance(bv) > 1e-14 {
			t.Fatal("homogeneous numerical jet lost original chart units", w, want, got, u, bu, v, bv, err)
		}
	}
	if _, err := b.Eval(.1, .4); err == nil {
		t.Fatal("approximate view widened the original parameter domain")
	}
}
