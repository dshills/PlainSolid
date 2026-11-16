package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestProjectiveGraphBoundaryEventsRetainSimpleAndTangentOriginalRoots(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	poly := []*big.Rat{big.NewRat(-2, 9), big.NewRat(10, 9), big.NewRat(-11, 6), big.NewRat(1, 1)} // (y-1/2)(y-2/3)^2
	b := projectiveBoundaryEventChart(t, [2][]*big.Rat{poly, poly})
	o := ProjectiveGraphBoundaryEventOptions{}
	r, err := CertifyProjectiveGraphBoundaryEvents(ctx, a, b, [2]int{0, 1}, o)
	if err != nil || !r.Complete || len(r.Coincident) != 0 || len(r.Isolated) != 2 {
		t.Fatal("missing complete simple and double roots", r, err)
	}
	for i, e := range r.Isolated {
		wantA := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(2, 3)}[i]
		wantB := [2]*big.Rat{big.NewRat(3, 5), big.NewRat(3, 4)}[i]
		wantRaw := [2]*big.Rat{big.NewRat(32, 5), big.NewRat(7, 1)}[i]
		if e.OrderBefore != i+1 || e.OrderAfter != i+1 {
			t.Fatal("lost one-sided order at an original knot", i, e)
		}
		for _, check := range []struct {
			root *ExactAlgebraicRoot
			want *big.Rat
		}{{e.Stations[0], wantA}, {e.Stations[1], wantB}, {e.OriginalUV[0][1], wantA}, {e.OriginalUV[1][1], wantRaw}} {
			assertProjectiveBoundaryRoot(t, check.root, check.want)
		}
	}
	short := o
	short.MaxOperations = r.ExactOperations - 1
	if r, err := CertifyProjectiveGraphBoundaryEvents(ctx, a, b, [2]int{0, 1}, short); !errors.Is(err, ErrConvergence) ||
		r.Complete || len(r.Isolated)+len(r.Coincident) != 0 || !r.Contact.Owners[0].Complete || !r.Contact.Owners[1].Complete {
		t.Fatal("partial algebraic events cannot replace original pending contact", r, err)
	}
}

func TestProjectiveGraphBoundaryEventsKeepAnIrrationalRootOnBothOwners(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	p := []*big.Rat{big.NewRat(-1, 2), new(big.Rat), big.NewRat(1, 1)}
	b := projectiveBoundaryEventChart(t, [2][]*big.Rat{p, p})
	r, err := CertifyProjectiveGraphBoundaryEvents(ctx, a, b, [2]int{0, 1}, ProjectiveGraphBoundaryEventOptions{})
	if err != nil || !r.Complete || len(r.Isolated) != 1 || r.Isolated[0].OrderBefore != 1 || r.Isolated[0].OrderAfter != 1 {
		t.Fatal("unqualified irrational shared-edge contact", r, err)
	}
	for i, coefficients := range [2][]*big.Rat{p, {big.NewRat(-9, 1), big.NewRat(6, 1), big.NewRat(7, 1)}} {
		station := r.Isolated[0].Stations[i]
		sign, err := station.SignPolynomial(ctx, coefficients, ExactCurvePlaneEventOptions{})
		if err != nil || sign.Sign != 0 || station.Interval().Lower.Cmp(station.Interval().Upper) == 0 {
			t.Fatal("lost an irrational exact station after transport", i, station.Interval(), sign, err)
		}
	}
}

func TestProjectiveGraphBoundaryEventsSeparateCoincidentRunAndIsolatedPoint(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	p := []*big.Rat{big.NewRat(5, 9), big.NewRat(-3, 2), big.NewRat(1, 1)} // (y-2/3)(y-5/6)
	b := projectiveBoundaryEventChart(t, [2][]*big.Rat{nil, p})
	r, err := CertifyProjectiveGraphBoundaryEvents(ctx, a, b, [2]int{0, 1}, ProjectiveGraphBoundaryEventOptions{})
	if err != nil || !r.Complete || len(r.Coincident) != 1 || r.Coincident[0].Lower.Sign() != 0 ||
		r.Coincident[0].Upper.Cmp(big.NewRat(2, 3)) != 0 || len(r.Isolated) != 1 {
		t.Fatal("coincident interval and its continuation could not be distinguished", r, err)
	}
	assertProjectiveBoundaryRoot(t, r.Isolated[0].Stations[0], big.NewRat(5, 6))
	assertProjectiveBoundaryRoot(t, r.Isolated[0].Stations[1], big.NewRat(15, 17))
}

func assertProjectiveBoundaryRoot(t *testing.T, root *ExactAlgebraicRoot, want *big.Rat) {
	t.Helper()
	constant := new(big.Rat).Neg(want)
	proof, err := root.SignPolynomial(context.Background(), []*big.Rat{constant, big.NewRat(1, 1)}, ExactCurvePlaneEventOptions{})
	if err != nil || proof.Sign != 0 {
		t.Fatal("wrong exact owner event", root.Interval(), want, proof, err)
	}
}

// Independently author z=x*y+P(y) over each of two original knot spans,
// with y = (2/3)v/(1-v/3). The cut v=3/4 has physical y=2/3.
func projectiveBoundaryEventChart(t *testing.T, residuals [2][]*big.Rat) *SurfacePatch {
	t.Helper()
	const degree = 3
	p := &SurfacePatch{degreeU: 1, degreeV: degree, countU: 2, countV: 7,
		knotsU: []float64{2, 2, 3, 3}, knotsV: []float64{4, 4, 4, 4, 7, 7, 7, 8, 8, 8, 8}}
	p.options, _ = patchOptions(SurfacePatchOptions{})
	b := &polynomialBudget{maxOperations: p.options.MaxOperations, maxBits: p.options.MaxCoefficientBits}
	one := rationalPolynomial{big.NewRat(1, 1)}
	y, w := rationalPolynomial{new(big.Rat), big.NewRat(2, 3)}, rationalPolynomial{big.NewRat(1, 1), big.NewRat(-1, 3)}
	multiply := func(a, c rationalPolynomial) rationalPolynomial {
		v, err := polynomialProduct(a, c, b)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	yw, ww := [degree + 1]rationalPolynomial{one}, [degree + 1]rationalPolynomial{one}
	for i := 1; i <= degree; i++ {
		yw[i], ww[i] = multiply(yw[i-1], y), multiply(ww[i-1], w)
	}
	for x := int64(1); x <= 2; x++ {
		for side, residual := range residuals {
			weight := ww[degree]
			modelX := multiply(rationalPolynomial{big.NewRat(x, 1)}, weight)
			modelY := multiply(y, ww[degree-1])
			modelZ := multiply(rationalPolynomial{big.NewRat(x, 1)}, modelY)
			for i, q := range residual {
				term := multiply(rationalPolynomial{q}, multiply(yw[i], ww[degree-i]))
				for j, c := range term {
					modelZ[j].Add(modelZ[j], c)
				}
			}
			net := make([]exactHomogeneous, degree+1)
			for i := range net {
				for axis, f := range [4]rationalPolynomial{modelX, modelY, modelZ, weight} {
					net[i][axis] = new(big.Rat)
					for j := 0; j <= i && j < len(f); j++ {
						ratio := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(degree, int64(j)))
						net[i][axis].Add(net[i][axis], ratio.Mul(ratio, f[j]))
					}
				}
			}
			l, h, err := splitHomogeneousBezier(net, big.NewRat(3, 4), b)
			if err != nil {
				t.Fatal(err)
			}
			if side == 0 {
				p.net = append(p.net, l...)
			} else {
				for axis, q := range h[0] {
					if q.Cmp(p.net[len(p.net)-1][axis]) != 0 {
						t.Fatal("bad independently authored original knot join", x, axis)
					}
				}
				p.net = append(p.net, h[1:]...)
			}
		}
	}
	if err := p.computeBounds(b); err != nil {
		t.Fatal(err)
	}
	return p
}
