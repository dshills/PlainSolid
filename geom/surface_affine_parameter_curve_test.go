package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactSurfaceAffineParameterCurveRationalReversal(t *testing.T) {
	p, _ := graphContactFixture(t, false, true)
	for i := 0; i < p.countU; i++ {
		for j := 0; j < p.countV; j++ {
			h := &p.net[i*p.countV+j]
			w := big.NewRat(int64((i+1)*(j+2)), 1)
			for axis := 0; axis < 3; axis++ {
				h[axis] = new(big.Rat).Mul(h[axis], w)
			}
			h[3] = w
		}
	}
	start, end := [2]*big.Rat{big.NewRat(2, 3), big.NewRat(1, 7)}, [2]*big.Rat{big.NewRat(1, 5), big.NewRat(5, 6)}
	for _, pair := range [2][2][2]*big.Rat{{start, end}, {end, start}} {
		c, e := p.AffineParameterCurve(context.Background(), pair[0], pair[1], TubeAffineLineOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for _, parameter := range []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(2, 7), big.NewRat(1, 1)} {
			var uv [2]*big.Rat
			for axis := 0; axis < 2; axis++ {
				uv[axis] = new(big.Rat).Add(pair[0][axis], new(big.Rat).Mul(parameter, new(big.Rat).Sub(pair[1][axis], pair[0][axis])))
			}
			want, e := p.EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			got, e := c.patch.EvaluateRational(context.Background(), parameter, new(big.Rat), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for axis := 0; axis < 3; axis++ {
				if want.Point[axis].Cmp(got.Point[axis]) != 0 {
					t.Fatal("exact tensor restriction mismatch")
				}
			}
		}
	}
	if _, e := p.AffineParameterCurve(context.Background(), start, end, TubeAffineLineOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := p.AffineParameterCurve(ctx, start, end, TubeAffineLineOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestBuildQuadraticSaddlePairedRayCurves(t *testing.T) {
	a, b := graphContactFixture(t, false, true)
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	rectangle := [2][2]*big.Rat{{new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 1)}}
	r, e := BuildQuadraticSaddleRayCurves(context.Background(), a, b, uv, rectangle, GraphPointContactOptions{})
	if e != nil || !r.Complete || len(r.Curves) != 4 || len(r.Pending) != 0 {
		t.Fatal(r, e)
	}
	for _, curve := range r.Curves {
		if !curve.Agreement.Equal || !curve.Agreement.Complete {
			t.Fatal(curve)
		}
		for _, parameter := range []float64{0, .25, .5, .75, 1} {
			p, e := curve.ParameterCurve.Eval(parameter)
			if e != nil {
				t.Fatal(e)
			}
			want, e := a.EvaluateRational(context.Background(), new(big.Rat).SetFloat64(p.X), new(big.Rat).SetFloat64(p.Y), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			got, e := curve.ModelA.patch.EvaluateRational(context.Background(), new(big.Rat).SetFloat64(parameter), new(big.Rat), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for axis := 0; axis < 3; axis++ {
				if got.Point[axis].Cmp(want.Point[axis]) != 0 {
					t.Fatal("pcurve/model mismatch")
				}
			}
		}
	}
	// Rational but nonrepresentable terminal coordinates remain exact pending.
	rectangle = [2][2]*big.Rat{{big.NewRat(1, 6), big.NewRat(5, 6)}, {big.NewRat(1, 3), big.NewRat(2, 3)}}
	r, e = BuildQuadraticSaddleRayCurves(context.Background(), a, b, uv, rectangle, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Pending) != 4 || !r.Section.Complete {
		t.Fatal(r, e)
	}
}
func TestSaddleRayCurvesRetainAlgebraicTerminals(t *testing.T) {
	a, b := graphContactFixture(t, false, true)
	n := a.degreeU
	q := func(i int) *big.Rat {
		return new(big.Rat).Add(big.NewRat(1, 4), new(big.Rat).Sub(big.NewRat(int64(i*(i-1)), int64(n*(n-1))), big.NewRat(int64(i), int64(n))))
	}
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			k := i*a.countV + j
			a.net[k][2] = new(big.Rat).Add(b.net[k][2], new(big.Rat).Sub(q(i), new(big.Rat).Mul(big.NewRat(2, 1), q(j))))
		}
	}
	r, e := BuildQuadraticSaddleRayCurves(context.Background(), a, b, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, [2][2]*big.Rat{{new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 1)}}, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Complete || !r.Section.Complete || len(r.Curves) != 0 || len(r.Pending) != 4 {
		t.Fatal(r, e)
	}
	for _, terminal := range r.Pending {
		if terminal.Exact || terminal.Lower[1].Cmp(terminal.Upper[1]) >= 0 {
			t.Fatal(terminal)
		}
	}
}
