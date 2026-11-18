//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func rationalCutTrimFixture(t *testing.T) (*geom.SurfacePatch, SplineUV) {
	t.Helper()
	p := partitionedSeamPatch(t, true)
	// u=q/(2-q), with a genuine curved quadratic v trace.
	c, e := geom.NewBSpline(2, []geom.Vec3{{Y: .125}, {X: .5, Y: .5}, {X: 1, Y: .125}}, []float64{-2, -2, -2, 6, 6, 6}, []float64{1, .5, .25})
	if e != nil {
		t.Fatal(e)
	}
	return p, SplineUV{Curve: c, Range: UVInterval{Min: -2, Max: 6}}
}
func TestRationalCutQuadraticTrimMultiSpan(t *testing.T) {
	p, trim := rationalCutTrimFixture(t)
	opts := RationalCutTrimOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .005}}
	got, e := TessellateRationalCutSplineTrimCertified(context.Background(), p, trim, []*big.Rat{big.NewRat(1, 3)}, opts)
	if e != nil {
		t.Fatal(e)
	}
	required := map[string]bool{"0": false, "1/3": false, "2/5": false, "2/3": false, "6/7": false, "1": false}
	for i, q := range got.NormalizedParameters {
		if _, ok := required[q.RatString()]; ok {
			required[q.RatString()] = true
		}
		// Independent authored rational Bernstein evaluation.
		a := new(big.Rat).Sub(big.NewRat(1, 1), q)
		b0 := new(big.Rat).Mul(a, a)
		b1 := new(big.Rat).Mul(a, q)
		b2 := new(big.Rat).Quo(new(big.Rat).Mul(q, q), big.NewRat(4, 1))
		w := new(big.Rat).Add(new(big.Rat).Add(b0, b1), b2)
		u := new(big.Rat).Quo(q, new(big.Rat).Sub(big.NewRat(2, 1), q))
		v := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Quo(new(big.Rat).Add(b0, b2), big.NewRat(8, 1)), new(big.Rat).Quo(b1, big.NewRat(2, 1))), w)
		x := new(big.Rat).Sub(big.NewRat(1, 1), u)
		f := new(big.Rat).Mul(x, x)
		if x.Cmp(big.NewRat(1, 2)) > 0 {
			d := new(big.Rat).Sub(x, big.NewRat(1, 2))
			f.Add(big.NewRat(1, 4), new(big.Rat).Mul(d, d))
		}
		z := new(big.Rat).Add(f, new(big.Rat).Mul(x, v))
		want := [3]*big.Rat{x, v, z}
		for k := range want {
			if got.ExactPoints[i][k].Cmp(want[k]) != 0 {
				t.Fatalf("independent point %d,%d", i, k)
			}
		}
		source := new(big.Rat).Add(big.NewRat(-2, 1), new(big.Rat).Mul(big.NewRat(8, 1), q))
		if got.Parameters[i].Cmp(source) != 0 {
			t.Fatal("original parameter")
		}
		if got.ExactUV[i][0].Cmp(u) != 0 || got.ExactUV[i][1].Cmp(v) != 0 {
			t.Fatal("source UV")
		}
	}
	for key, present := range required {
		if !present {
			t.Fatal("omitted fixed cut", key)
		}
	}
	if len(got.FixedCuts) != 6 {
		t.Fatal("mandatory cuts", got.FixedCuts)
	}
	for i, bound := range got.SquaredErrors {
		if bound.Sign() < 0 || bound.Cmp(new(big.Rat).Mul(ratInset(.005), ratInset(.005))) > 0 {
			t.Fatal("bound")
		}
		q := new(big.Rat).Quo(new(big.Rat).Add(got.NormalizedParameters[i], got.NormalizedParameters[i+1]), big.NewRat(2, 1))
		controls, e := quadraticTrimSourceControls(trim)
		if e != nil {
			t.Fatal(e)
		}
		one := new(big.Rat).Sub(big.NewRat(1, 1), q)
		weights := [3]*big.Rat{new(big.Rat).Mul(one, one), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(one, q)), new(big.Rat).Mul(q, q)}
		h := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		for j, w := range weights {
			for k := range h {
				h[k].Add(h[k], new(big.Rat).Mul(w, controls[j][k]))
			}
		}
		jet, e := p.EvaluateRational(context.Background(), new(big.Rat).Quo(h[0], h[2]), new(big.Rat).Quo(h[1], h[2]), geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		norm := new(big.Rat)
		aa, bb := exactBoundPoint(got.Points[i]), exactBoundPoint(got.Points[i+1])
		for k := range aa {
			mid := new(big.Rat).Quo(new(big.Rat).Add(aa[k], bb[k]), big.NewRat(2, 1))
			d := new(big.Rat).Sub(jet.Point[k], mid)
			norm.Add(norm, new(big.Rat).Mul(d, d))
		}
		if norm.Cmp(bound) > 0 {
			t.Fatal("independent residual witness exceeds certificate")
		}
	}
	t.Logf("segments=%d checks=%d", len(got.SquaredErrors), got.Checks)
	for _, limits := range []QuadraticTrimTessellationOptions{{Deflection: .005, MaxChecks: got.Checks - 1}, {Deflection: .005, MaxSegments: len(got.SquaredErrors) - 1}, {Deflection: 1e-8, MaxDepth: 1}} {
		r, e := TessellateRationalCutSplineTrimCertified(context.Background(), p, trim, []*big.Rat{big.NewRat(1, 3)}, RationalCutTrimOptions{Tessellation: limits})
		if !errors.Is(e, geom.ErrConvergence) || len(r.Points) != 0 {
			t.Fatal("partial budget", e)
		}
	}
}
func TestRationalCutQuadraticTrimReversedAndInvalid(t *testing.T) {
	p, trim := rationalCutTrimFixture(t)
	trim.Reversed = true
	o := RationalCutTrimOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .02}}
	r, e := TessellateRationalCutSplineTrimCertified(context.Background(), p, trim, []*big.Rat{big.NewRat(1, 3)}, o)
	if e != nil {
		t.Fatal(e)
	}
	for i, q := range r.NormalizedParameters {
		want := new(big.Rat).Sub(big.NewRat(6, 1), new(big.Rat).Mul(big.NewRat(8, 1), q))
		if r.Parameters[i].Cmp(want) != 0 {
			t.Fatal("reversed original ancestry")
		}
	}
	for _, cuts := range [][]*big.Rat{{nil}, {big.NewRat(-1, 3)}, {big.NewRat(4, 3)}} {
		if _, e := TessellateRationalCutSplineTrimCertified(context.Background(), p, trim, cuts, o); !errors.Is(e, geom.ErrInvalid) {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := TessellateRationalCutSplineTrimCertified(ctx, p, trim, nil, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
