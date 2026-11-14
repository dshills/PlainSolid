package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestExactQuarticRootSigns(t *testing.T) {
	ctx := context.Background()
	r := func(n int64) *big.Rat { return big.NewRat(n, 1) }
	p := []*big.Rat{r(6), r(0), r(-5), r(0), r(1)}
	roots, e := IsolateExactPolynomialRoots(ctx, p, r(-2), r(2), ExactCurvePlaneEventOptions{})
	if e != nil || len(roots.Roots) != 4 {
		t.Fatal(roots, e)
	}
	p[0].SetInt64(100)
	for i, v := range roots.Roots {
		if v.Multiplicity != 1 {
			t.Fatal(v)
		}
		sign, e := v.Root.SignPolynomial(ctx, []*big.Rat{r(-2), r(0), r(1)}, ExactCurvePlaneEventOptions{})
		want := 0
		if i == 0 || i == 3 {
			want = 1
		}
		if e != nil || sign.Sign != want {
			t.Fatal(i, sign, e)
		}
		if i > 0 {
			o, e := roots.Roots[i-1].Root.Compare(ctx, v.Root, ExactCurvePlaneEventOptions{})
			if e != nil || !o.Proven || o.Order != -1 {
				t.Fatal(o, e)
			}
		}
	}
	repeated, e := IsolateExactPolynomialRoots(ctx, []*big.Rat{r(1), r(-2), r(1)}, r(0), r(2), ExactCurvePlaneEventOptions{})
	if e != nil || len(repeated.Roots) != 1 || repeated.Roots[0].Multiplicity != 2 {
		t.Fatal(repeated, e)
	}
	if v, e := IsolateExactPolynomialRoots(ctx, []*big.Rat{r(0)}, r(0), r(2), ExactCurvePlaneEventOptions{}); e != ErrDomain || v.Roots != nil {
		t.Fatal(e)
	}
	if v, e := IsolateExactPolynomialRoots(ctx, []*big.Rat{r(6), r(0), r(-5), r(0), r(1)}, r(-2), r(2), ExactCurvePlaneEventOptions{MaxOperations: 1}); e == nil || v.Roots != nil {
		t.Fatal("budget", e)
	}
}

func TestExactQuarticToroidBranchAndTrimSigns(t *testing.T) {
	ctx := context.Background()
	r := func(n, d int64) *big.Rat { return big.NewRat(n, d) }
	// Independent circular torus R=3,r=1, horizontal line(x=t,y=0,z=1/2):
	// (t²+33/4)²-36t². The exact roots are ±3±sqrt(3)/2.
	// The outer radial branch requires t²-9>=0; an independent retained
	// halfspace t>=0 then leaves only the positive outer root.
	poly := []*big.Rat{r(1089, 16), r(0, 1), r(-39, 2), r(0, 1), r(1, 1)}
	remaining := 200000
	initial := remaining
	o := ExactCurvePlaneEventOptions{MaxOperations: remaining}
	roots, e := IsolateExactPolynomialRoots(ctx, poly, r(-5, 1), r(5, 1), o)
	if e != nil || len(roots.Roots) != 4 {
		t.Fatal(roots, e)
	}
	remaining -= roots.Operations
	retained, inner, trimmed := 0, 0, 0
	for i, event := range roots.Roots {
		if event.Multiplicity != 1 {
			t.Fatal(event)
		}
		opts := ExactCurvePlaneEventOptions{MaxOperations: remaining}
		branch, e := event.Root.SignPolynomial(ctx, []*big.Rat{r(-9, 1), r(0, 1), r(1, 1)}, opts)
		if e != nil || branch.Operations <= 0 {
			t.Fatal(branch, e)
		}
		remaining -= branch.Operations
		halfspace, e := event.Root.SignPolynomial(ctx, []*big.Rat{r(0, 1), r(1, 1)}, ExactCurvePlaneEventOptions{MaxOperations: remaining})
		if e != nil {
			t.Fatal(e)
		}
		remaining -= halfspace.Operations
		identity, e := event.Root.SignPolynomial(ctx, poly, ExactCurvePlaneEventOptions{MaxOperations: remaining})
		if e != nil || identity.Sign != 0 {
			t.Fatal("exact common-factor identity", identity, e)
		}
		remaining -= identity.Operations
		if branch.Sign < 0 {
			inner++
		} else if halfspace.Sign < 0 {
			trimmed++
		} else {
			retained++
			if i != 3 {
				t.Fatal("wrong conjugate")
			}
		}
	}
	if retained != 1 || inner != 2 || trimmed != 1 || remaining <= 0 || remaining >= initial {
		t.Fatal(retained, inner, trimmed, remaining)
	}
	// Work is reported even on atomic failure; no partial root ledger escapes.
	exhausted, e := IsolateExactPolynomialRoots(ctx, poly, r(-5, 1), r(5, 1), ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || exhausted.Roots != nil || exhausted.Operations == 0 {
		t.Fatal(exhausted, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := roots.Roots[0].Root.SignPolynomial(cancelled, poly, ExactCurvePlaneEventOptions{})
	if e != context.Canceled || bad.Sign != 0 {
		t.Fatal(bad, e)
	}
}

func TestExactPolynomialSignTrailingZero(t *testing.T) {
	ctx := context.Background()
	root, _, e := NewExactRationalRoot(ctx, big.NewRat(1, 1), ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, n := range []int64{-1, 0, 1} {
		s, e := root.SignPolynomial(ctx, []*big.Rat{big.NewRat(n, 1), new(big.Rat), new(big.Rat)}, ExactCurvePlaneEventOptions{})
		if e != nil || s.Sign != int(n) {
			t.Fatal(n, s, e)
		}
	}
}
