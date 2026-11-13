package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func graphRootPiece(t *testing.T, poly rationalPolynomial, start, end *big.Rat, rational bool) SurfaceAffineCurvePiece {
	budget := &polynomialBudget{maxOperations: 1000000, maxBits: 4096}
	local, e := composeProjectionPolynomial(poly, rationalPolynomial{start, new(big.Rat).Sub(end, start)}, budget)
	if e != nil {
		t.Fatal(e)
	}
	weights := rationalPolynomial{big.NewRat(1, 1)}
	if rational {
		weights = append(weights, big.NewRat(1, 1))
		local, e = polynomialProduct(local, weights, budget)
		if e != nil {
			t.Fatal(e)
		}
	}
	degree := max(len(local), len(weights)) - 1
	if degree < 1 {
		degree = 1
	}
	bern := func(power rationalPolynomial, i int) *big.Rat {
		v := new(big.Rat)
		for k, c := range power {
			if k > i {
				break
			}
			f := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(int64(degree), int64(k)))
			v.Add(v, new(big.Rat).Mul(c, f))
		}
		return v
	}
	net := make([]exactHomogeneous, degree+1)
	for i := range net {
		net[i] = exactHomogeneous{new(big.Rat), new(big.Rat), bern(local, i), bern(weights, i)}
	}
	curve, e := exactCurveFromHomogeneousBezier(net, TubeAffineLineOptions{MaxPoles: 100, MaxOperations: 1000000, MaxCoefficientBits: 4096}, budget)
	if e != nil {
		t.Fatal(e)
	}
	return SurfaceAffineCurvePiece{Start: new(big.Rat).Set(start), End: new(big.Rat).Set(end), Curve: curve}
}
func TestExactGraphLineRootIndependentPieceEquality(t *testing.T) {
	// Z=t²+t+1/2, height=1+t: the event is sqrt(1/2).
	polynomial := rationalPolynomial{big.NewRat(1, 2), big.NewRat(1, 1), big.NewRat(1, 1)}
	pieces := []SurfaceAffineCurvePiece{graphRootPiece(t, polynomial, big.NewRat(0, 1), big.NewRat(1, 1), false), graphRootPiece(t, polynomial, big.NewRat(1, 3), big.NewRat(1, 1), true)}
	var roots [2]*ExactGraphLineRoot
	for i, piece := range pieces {
		var e error
		roots[i], e = NewExactGraphLineRoot(context.Background(), piece, big.NewRat(1, 1), big.NewRat(2, 1), ExactParameterInterval{Lower: big.NewRat(2, 3), Upper: big.NewRat(3, 4)}, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	// Mutate external input and returned enclosures; proof provenance is owned.
	pieces[1].Start.SetInt64(9)
	bounds := roots[0].Interval()
	bounds.Lower.SetInt64(8)
	r, e := CompareExactGraphLineRoots(context.Background(), roots[0], roots[1], ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Order != 0 {
		t.Fatal(r, e)
	}
}
func TestExactGraphLineRootNearIrrationalOrder(t *testing.T) {
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	var roots [2]*ExactGraphLineRoot
	for i := 0; i < 2; i++ {
		constant := big.NewRat(-1, 2)
		if i == 1 {
			constant.Sub(constant, tiny)
		}
		piece := graphRootPiece(t, rationalPolynomial{constant, new(big.Rat), big.NewRat(1, 1)}, big.NewRat(1, 3), big.NewRat(1, 1), i == 1)
		var e error
		roots[i], e = NewExactGraphLineRoot(context.Background(), piece, new(big.Rat), new(big.Rat), ExactParameterInterval{Lower: big.NewRat(2, 3), Upper: big.NewRat(3, 4)}, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	for i, pair := range [][2]*ExactGraphLineRoot{{roots[0], roots[1]}, {roots[1], roots[0]}} {
		r, e := CompareExactGraphLineRoots(context.Background(), pair[0], pair[1], ExactCurvePlaneEventOptions{})
		want := -1
		if i == 1 {
			want = 1
		}
		if e != nil || !r.Proven || r.Order != want {
			t.Fatal(r, e)
		}
	}
	r, e := CompareExactGraphLineRoots(context.Background(), roots[0], roots[1], ExactCurvePlaneEventOptions{MaxSubdivisions: 2})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
}
func TestExactGraphLineRootEndpointAndAmbiguity(t *testing.T) {
	piece := graphRootPiece(t, rationalPolynomial{big.NewRat(1, 4), big.NewRat(-1, 1), big.NewRat(1, 1)}, big.NewRat(1, 2), big.NewRat(1, 1), false)
	root, e := NewExactGraphLineRoot(context.Background(), piece, new(big.Rat), new(big.Rat), ExactParameterInterval{Lower: big.NewRat(0, 1), Upper: big.NewRat(3, 4)}, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := CompareExactGraphLineRoots(context.Background(), root, root, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Order != 0 {
		t.Fatal(r, e)
	}
	piece = graphRootPiece(t, rationalPolynomial{big.NewRat(2, 9), big.NewRat(-1, 1), big.NewRat(1, 1)}, big.NewRat(0, 1), big.NewRat(1, 1), false)
	_, e = NewExactGraphLineRoot(context.Background(), piece, new(big.Rat), new(big.Rat), ExactParameterInterval{Lower: big.NewRat(0, 1), Upper: big.NewRat(1, 1)}, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = CompareExactGraphLineRoots(ctx, root, root, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
