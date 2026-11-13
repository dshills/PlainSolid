package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func nativeNonbinaryTrim(t *testing.T) ExactQuadraticUV {
	t.Helper()
	h := [3][3]*big.Rat{{new(big.Rat), big.NewRat(1, 8), big.NewRat(1, 1)}, {big.NewRat(1, 4), big.NewRat(1, 6), big.NewRat(1, 2)}, {big.NewRat(1, 4), big.NewRat(1, 32), big.NewRat(1, 4)}}
	q, e := NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	h[1][1].SetInt64(99) // Constructor must retain an independent exact copy.
	return q
}
func TestNativeExactQuadraticTrimCertifiedPolyline(t *testing.T) {
	p := partitionedSeamPatch(t, true)
	trim := nativeNonbinaryTrim(t)
	o := ExactQuadraticTrimMeshOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .005}}
	got, e := TessellateExactQuadraticTrimCertified(context.Background(), p, trim, []*big.Rat{big.NewRat(1, 3)}, o)
	if e != nil {
		t.Fatal(e)
	}
	if got.Source.HomogeneousControls()[1][1].Cmp(big.NewRat(1, 6)) != 0 || got.CertifiedRationalCutTrimPolyline.Source.Curve != nil {
		t.Fatal("native source rounded or converted")
	}
	if !got.Composition.Complete || !got.Composition.Continuous || len(got.Composition.Pieces) != 4 {
		t.Fatal("composition")
	}
	required := map[string]bool{"1/3": false, "2/5": false, "2/3": false, "6/7": false}
	for i, q := range got.NormalizedParameters {
		if _, ok := required[q.RatString()]; ok {
			required[q.RatString()] = true
		}
		// Independent represented formula; the nonbinary middle UV pole is1/3.
		m := new(big.Rat).Sub(big.NewRat(1, 1), q)
		b0 := new(big.Rat).Mul(m, m)
		b1 := new(big.Rat).Mul(m, q)
		b2 := new(big.Rat).Quo(new(big.Rat).Mul(q, q), big.NewRat(4, 1))
		w := new(big.Rat).Add(new(big.Rat).Add(b0, b1), b2)
		u := new(big.Rat).Quo(q, new(big.Rat).Sub(big.NewRat(2, 1), q))
		v := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Quo(new(big.Rat).Add(b0, b2), big.NewRat(8, 1)), new(big.Rat).Quo(b1, big.NewRat(3, 1))), w)
		if got.ExactUV[i][0].Cmp(u) != 0 || got.ExactUV[i][1].Cmp(v) != 0 || got.Parameters[i].Cmp(q) != 0 {
			t.Fatal("exact UV/source witness")
		}
		x := new(big.Rat).Sub(big.NewRat(1, 1), u)
		f := new(big.Rat).Mul(x, x)
		if x.Cmp(big.NewRat(1, 2)) > 0 {
			d := new(big.Rat).Sub(x, big.NewRat(1, 2))
			f.Add(big.NewRat(1, 4), new(big.Rat).Mul(d, d))
		}
		z := new(big.Rat).Add(f, new(big.Rat).Mul(x, v))
		if got.ExactPoints[i][0].Cmp(x) != 0 || got.ExactPoints[i][1].Cmp(v) != 0 || got.ExactPoints[i][2].Cmp(z) != 0 {
			t.Fatal("independent model witness")
		}
	}
	for q, found := range required {
		if !found {
			t.Fatal("missing rational cut", q)
		}
	}
	for i, bound := range got.SquaredErrors {
		if bound.Cmp(new(big.Rat).Mul(ratInset(.005), ratInset(.005))) > 0 {
			t.Fatal("bound")
		}
		q := new(big.Rat).Quo(new(big.Rat).Add(got.Parameters[i], got.Parameters[i+1]), big.NewRat(2, 1))
		uv, _, e := trim.EvaluateRational(q)
		if e != nil {
			t.Fatal(e)
		}
		model, e := p.EvaluateRational(context.Background(), uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		aa, bb := exactBoundPoint(got.Points[i]), exactBoundPoint(got.Points[i+1])
		norm := new(big.Rat)
		for k := range aa {
			mid := new(big.Rat).Quo(new(big.Rat).Add(aa[k], bb[k]), big.NewRat(2, 1))
			d := new(big.Rat).Sub(model.Point[k], mid)
			norm.Add(norm, new(big.Rat).Mul(d, d))
		}
		if norm.Cmp(bound) > 0 {
			t.Fatal("independent residual exceeds proof")
		}
	}
	t.Logf("segments=%d checks=%d exact composition operations=%d", len(got.SquaredErrors), got.Checks, got.Composition.ExactOperations)
	for _, kind := range []int{0, 1, 2} {
		limited := o
		if kind == 0 {
			limited.Tessellation.MaxChecks = got.Checks - 1
		} else if kind == 1 {
			limited.Tessellation.MaxSegments = len(got.SquaredErrors) - 1
		} else {
			limited.Composition.MaxPieces = 1
		}
		r, e := TessellateExactQuadraticTrimCertified(context.Background(), p, trim, []*big.Rat{big.NewRat(1, 3)}, limited)
		if !errors.Is(e, geom.ErrConvergence) || len(r.Points) != 0 {
			t.Fatal("partial bounded result", kind, e)
		}
	}
}
func TestNativeExactQuadraticTrimRejectsInvalid(t *testing.T) {
	p := partitionedSeamPatch(t, false)
	trim := nativeNonbinaryTrim(t)
	o := ExactQuadraticTrimMeshOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .01}}
	if _, e := TessellateExactQuadraticTrimCertified(context.Background(), p, ExactQuadraticUV{}, nil, o); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal(e)
	}
	if _, e := TessellateExactQuadraticTrimCertified(context.Background(), p, trim, []*big.Rat{nil}, o); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := TessellateExactQuadraticTrimCertified(ctx, p, trim, nil, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	o.MaxInputBits = 4
	if r, e := TessellateExactQuadraticTrimCertified(context.Background(), p, trim, nil, o); !errors.Is(e, geom.ErrConvergence) || len(r.Points) != 0 {
		t.Fatal("input bit budget", e)
	}
}

func TestNativeExactQuadraticTrimReversalAndUnresolvedKnot(t *testing.T) {
	p := partitionedSeamPatch(t, false)
	trim := nativeNonbinaryTrim(t)
	opts := ExactQuadraticTrimMeshOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .01}}
	got, e := TessellateExactQuadraticTrimCertified(context.Background(), p, trim.reversed(), nil, opts)
	if e != nil {
		t.Fatal(e)
	}
	want, _, e := trim.EvaluateRational(big.NewRat(1, 1))
	if e != nil {
		t.Fatal(e)
	}
	if got.ExactUV[0][0].Cmp(want[0]) != 0 || got.ExactUV[0][1].Cmp(want[1]) != 0 {
		t.Fatal("reversed source")
	}
	h := [3][3]*big.Rat{{new(big.Rat), big.NewRat(1, 8), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 6), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(1, 8), big.NewRat(1, 1)}}
	irrational, e := NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	opts.Composition.MaxRootSubdivisions = 8
	// u=q² reaches the support knot1/2 at sqrt(1/2). No approximate
	// parameter may enter a supposedly exact chord partition.
	r, e := TessellateExactQuadraticTrimCertified(context.Background(), p, irrational, nil, opts)
	if !errors.Is(e, geom.ErrConvergence) || len(r.Points) != 0 {
		t.Fatal("irrational cut silently rounded", e)
	}
}
