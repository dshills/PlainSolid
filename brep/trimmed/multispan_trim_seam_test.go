//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func partitionedSeamPatch(t *testing.T, reversed bool) *geom.SurfacePatch {
	t.Helper()
	return partitionedSeamPatchY(t, reversed, false)
}

func partitionedSeamPatchY(t *testing.T, reversed, flipY bool) *geom.SurfacePatch {
	t.Helper()
	cuts := []float64{0, .5, 1}
	if reversed {
		cuts = []float64{0, .25, .5, .75, 1}
	}
	var poles []geom.Vec3
	knots := []float64{0, 0, 0}
	for span := 0; span+1 < len(cuts); span++ {
		x0, x2 := cuts[span], cuts[span+1]
		if reversed {
			x0, x2 = 1-x0, 1-x2
		}
		xs := [3]float64{x0, (x0 + x2) / 2, x2}
		fs := [3]float64{x0 * x0, x0 * x2, x2 * x2}
		if (x0+x2)/2 > .5 {
			fs = [3]float64{.25 + (x0-.5)*(x0-.5), .25 + (x0-.5)*(x2-.5), .25 + (x2-.5)*(x2-.5)}
		}
		for i := 0; i < 3; i++ {
			if span > 0 && i == 0 {
				continue
			}
			for j := 0; j < 2; j++ {
				v := float64(j)
				if flipY {
					v = .5 - v
				}
				poles = append(poles, geom.Vec3{X: xs[i], Y: v, Z: fs[i] + xs[i]*v})
			}
		}
		if span+2 < len(cuts) {
			knots = append(knots, cuts[span+1], cuts[span+1])
		}
	}
	knots = append(knots, 1, 1, 1)
	s, e := geom.NewBSplineSurface(2, 1, len(poles)/2, 2, poles, knots, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestMultiSpanTrimSeamDifferentPartitions(t *testing.T) {
	a, b := partitionedSeamPatch(t, false), partitionedSeamPatch(t, true)
	ca, e := geom.NewBSpline(1, []geom.Vec3{{Y: .25}, {X: 1, Y: .25}}, []float64{-2, -2, 6, 6}, []float64{1, .5})
	if e != nil {
		t.Fatal(e)
	}
	cb, e := geom.NewBSpline(1, []geom.Vec3{{X: 1, Y: .25}, {Y: .25}}, []float64{10, 10, 26, 26}, []float64{1, .5})
	if e != nil {
		t.Fatal(e)
	}
	ta, tb := SplineUV{Curve: ca, Range: UVInterval{Min: -2, Max: 6}}, SplineUV{Curve: cb, Range: UVInterval{Min: 10, Max: 26}}
	r, e := CertifyMultiSpanTrimSeam(context.Background(), a, b, ta, tb, MultiSpanTrimSeamOptions{})
	if e != nil {
		t.Fatal(e)
	}
	want := []*big.Rat{big.NewRat(0, 1), big.NewRat(2, 5), big.NewRat(2, 3), big.NewRat(6, 7), big.NewRat(1, 1)}
	if len(r.Pieces) != 4 || r.IdentityChecks != 68 {
		t.Fatalf("%+v", r)
	}
	for i, p := range r.Pieces {
		if p.Min.Cmp(want[i]) != 0 || p.Max.Cmp(want[i+1]) != 0 {
			t.Fatalf("piece %d: %s..%s", i, p.Min, p.Max)
		}
		for end, q := range []*big.Rat{p.Min, p.Max} {
			point := p.ExactStart
			if end == 1 {
				point = p.ExactEnd
			}
			x := new(big.Rat).Quo(q, new(big.Rat).Sub(big.NewRat(2, 1), q))
			f := new(big.Rat).Mul(x, x)
			if x.Cmp(big.NewRat(1, 2)) > 0 {
				d := new(big.Rat).Sub(x, big.NewRat(1, 2))
				f.Add(big.NewRat(1, 4), new(big.Rat).Mul(d, d))
			}
			z := new(big.Rat).Add(f, new(big.Rat).Quo(x, big.NewRat(4, 1)))
			if point[0].Cmp(x) != 0 || point[1].Cmp(big.NewRat(1, 4)) != 0 || point[2].Cmp(z) != 0 {
				t.Fatal("independent model mismatch")
			}
			for chart, m := range r.ParameterMaps {
				expected := new(big.Rat).Add(m.Offset, new(big.Rat).Mul(m.Scale, q))
				if p.SourceRanges[chart][end].Cmp(expected) != 0 {
					t.Fatal("source range")
				}
			}
		}
	}
	for _, o := range []MultiSpanTrimSeamOptions{{MaxPieces: 3}, {MaxIdentityChecks: 67}, {MaxKnotTests: 1}} {
		got, e := CertifyMultiSpanTrimSeam(context.Background(), a, b, ta, tb, o)
		if !errors.Is(e, geom.ErrConvergence) || len(got.Pieces) != 0 {
			t.Fatalf("partial/budget: %+v %v", got, e)
		}
	}
	bad := tb
	bad.Reversed = true
	if got, e := CertifyMultiSpanTrimSeam(context.Background(), a, b, ta, bad, MultiSpanTrimSeamOptions{}); e == nil || len(got.Pieces) != 0 {
		t.Fatal("mismatched geometry certified")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := CertifyMultiSpanTrimSeam(ctx, a, b, ta, tb, MultiSpanTrimSeamOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestMultiSpanTrimSeamIrrationalAndCoincidentKnot(t *testing.T) {
	p := partitionedSeamPatch(t, false)
	c, e := geom.NewBSpline(2, []geom.Vec3{{Y: .25}, {Y: .25}, {X: 1, Y: .25}}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	trim := SplineUV{Curve: c, Range: UVInterval{Max: 1}}
	got, e := CertifyMultiSpanTrimSeam(context.Background(), p, p, trim, trim, MultiSpanTrimSeamOptions{})
	if !errors.Is(e, geom.ErrConvergence) || len(got.Pieces) != 0 {
		t.Fatalf("irrational crossing: %+v %v", got, e)
	}
	c, e = geom.NewBSpline(1, []geom.Vec3{{X: .5}, {X: .5, Y: 1}}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	trim = SplineUV{Curve: c, Range: UVInterval{Max: 1}}
	got, e = CertifyMultiSpanTrimSeam(context.Background(), p, p, trim, trim, MultiSpanTrimSeamOptions{})
	if e != nil || len(got.Pieces) != 1 {
		t.Fatalf("coincident knot: %+v %v", got, e)
	}
}
