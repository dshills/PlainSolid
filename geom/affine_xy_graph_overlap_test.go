package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestAffineXYPartialGraphOverlap(t *testing.T) {
	ctx := context.Background()
	s, e := NewBSplineSurface(2, 1, 3, 2, make([]Vec3, 6), []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		w := big.NewRat(int64(2+i), 2)
		x := []int64{0, 1, 4}[i]
		for j := 0; j < 2; j++ {
			a.net[2*i+j] = exactHomogeneous{big.NewRat(x, 1), new(big.Rat).Mul(w, big.NewRat(int64(j), 1)), big.NewRat(x*int64(j), 1), new(big.Rat).Set(w)}
		}
	}
	if e = a.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	poles := make([]Vec3, 4)
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			x, y := float64(1+2*j), float64(i)
			poles[2*i+j] = Vec3{x, y, x * y}
		}
	}
	s, e = NewBSplineSurface(1, 1, 2, 2, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, e = a.InsertUKnot(.75, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.InsertVKnot(.125, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := CertifyAffineXYGraphOverlap(ctx, a, b, SurfacePatchOptions{})
	if e != nil || !proof.Coincident || proof.NoArea || proof.Orientation != -1 || proof.ComparedCells < 3 {
		t.Fatal(proof, e)
	}
	if proof.A[0][0].Cmp(big.NewRat(1, 2)) != 0 || proof.A[0][1].Cmp(big.NewRat(1, 1)) != 0 || proof.B[1][0].Sign() != 0 || proof.B[1][1].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(proof)
	}
	reverse, e := CertifyAffineXYGraphOverlap(ctx, b, a, SurfacePatchOptions{})
	if e != nil || !reverse.Coincident || reverse.Orientation != -1 {
		t.Fatal(reverse, e)
	}
	if _, e := CertifyAffineXYGraphOverlap(ctx, a, b, SurfacePatchOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}

	// Source x=3u, target x=1+2v: exact partial overlap starts at u=1/3
	// and crosses both owners' distinct internal knots.
	rationalA, e := permuteGraphPatch(a, [3]int{0, 1, 2}, &polynomialBudget{maxOperations: 100000, maxBits: 16384})
	if e != nil {
		t.Fatal(e)
	}
	for i := range rationalA.net {
		for _, axis := range []int{0, 2} {
			rationalA.net[i][axis] = new(big.Rat).Mul(rationalA.net[i][axis], big.NewRat(3, 2))
		}
	}
	rationalProof, e := CertifyAffineXYGraphOverlap(ctx, rationalA, b, SurfacePatchOptions{})
	if e != nil || !rationalProof.Coincident || rationalProof.A[0][0].Cmp(big.NewRat(1, 3)) != 0 || rationalProof.ComparedCells < 3 {
		t.Fatal(rationalProof, e)
	}
	rationalA.net[0][2] = new(big.Rat).Add(rationalA.net[0][2], big.NewRat(1, 10000))
	if bad, e := CertifyAffineXYGraphOverlap(ctx, rationalA, b, SurfacePatchOptions{}); e == nil || bad.Coincident {
		t.Fatal("nonbinary perturbed overlap", bad, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CertifyAffineXYGraphOverlap(cancelled, a, b, SurfacePatchOptions{}); e == nil {
		t.Fatal("cancel")
	}
	far, e := permuteGraphPatch(b, [3]int{0, 1, 2}, &polynomialBudget{maxOperations: 100000, maxBits: 16384})
	if e != nil {
		t.Fatal(e)
	}
	for i := range far.net {
		far.net[i][0] = new(big.Rat).Add(far.net[i][0], new(big.Rat).Mul(big.NewRat(10, 1), far.net[i][3]))
	}
	none, e := CertifyAffineXYGraphOverlap(ctx, a, far, SurfacePatchOptions{})
	if e != nil || !none.NoArea || none.Coincident {
		t.Fatal(none, e)
	}
	// Perturbation does not imply separation: overlap remains unresolved.
	b.net[0][2].Add(b.net[0][2], big.NewRat(1, 1000))
	if r, e := CertifyAffineXYGraphOverlap(ctx, a, b, SurfacePatchOptions{}); e == nil || r.Coincident {
		t.Fatal("mixed contact accepted", r, e)
	}
}
