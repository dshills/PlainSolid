package edgecheck

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func verifyWitness(t *testing.T, r Report, tolerance float64) {
	t.Helper()
	if r.Witness == nil {
		t.Fatal("no exact witness")
	}
	w := r.Witness
	if w.Parameter.Sign() < 0 || w.Parameter.Cmp(big.NewRat(1, 1)) > 0 {
		t.Fatal("parameter out of range")
	}
	sum := new(big.Rat)
	for i := 0; i < 3; i++ {
		d := sub(w.ExactPointA[i], w.ExactPointB[i])
		sum.Add(sum, mul(d, d))
	}
	if sum.Cmp(w.SquaredDistance) != 0 || sum.Cmp(mul(num(tolerance), num(tolerance))) <= 0 {
		t.Fatal("invalid witness")
	}
	if r.SquaredUpperBound != nil && sum.Cmp(r.SquaredUpperBound) > 0 {
		t.Fatal("witness exceeds certified upper bound")
	}
}
func TestExactIdentityAcrossDegreesAndWeights(t *testing.T) {
	a := Curve{Poles: []geom.Vec3{{}, {X: 1}}}
	c := Curve{Poles: []geom.Vec3{{}, {X: .5}, {X: 1}}, Weights: []float64{7, 7, 7}}
	r, e := Check(context.Background(), a, c, 0, Options{})
	if e != nil || r.Status != Certified || r.SquaredUpperBound.Sign() != 0 {
		t.Fatal(r, e)
	}
	a = Curve{Poles: []geom.Vec3{{X: 1}, {X: 1, Y: 1}, {Y: 1}}, Weights: []float64{1, 1, 2}}
	c = Curve{Poles: a.Poles, Weights: []float64{2, 2, 4}}
	r, e = Check(context.Background(), a, c, 0, Options{})
	if e != nil || r.Status != Certified {
		t.Fatal(r, e)
	}
	c.Reversed = true
	r, e = Check(context.Background(), a, c, .01, Options{})
	if e != nil || r.Status != Exceeded {
		t.Fatal(r, e)
	}
	verifyWitness(t, r, .01)
}
func TestContinuousQuadraticTolerance(t *testing.T) {
	a := Curve{Poles: []geom.Vec3{{}}}
	b := Curve{Poles: []geom.Vec3{{}, {Z: 2}, {}}}
	for _, c := range []struct {
		tol  float64
		want Status
	}{{1, Certified}, {.9, Exceeded}, {1.01, Certified}} {
		r, e := Check(context.Background(), a, b, c.tol, Options{})
		if e != nil || r.Status != c.want {
			t.Fatal(r, e)
		}
		if c.want == Exceeded {
			verifyWitness(t, r, c.tol)
		}
		if c.want == Certified && r.SquaredUpperBound.Cmp(mul(num(c.tol), num(c.tol))) > 0 {
			t.Fatal("uncertified bound")
		}
	}
	b.Poles = []geom.Vec3{{}, {Z: 3}, {Z: 1.5}}
	r, e := Check(context.Background(), a, b, 2, Options{MaxDepth: 8})
	if e != nil || r.Status != Unresolved {
		t.Fatal("nondyadic tolerance tangency must remain unresolved", r, e)
	}
	r, e = Check(context.Background(), a, b, 2.001, Options{})
	if e != nil || r.Status != Certified {
		t.Fatal(r, e)
	}
}
func TestAffineChartAndLineRanges(t *testing.T) {
	uv := PlanarCurve{Poles: []geom.Vec2{{X: 1}, {X: 1, Y: 1}, {Y: 1}}, Weights: []float64{1, 1, 2}}
	chart := Chart{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, U: geom.Vec3{X: 2}, V: geom.Vec3{Y: 3}}
	model := Curve{Poles: []geom.Vec3{{X: 3, Y: 2, Z: 3}, {X: 3, Y: 5, Z: 3}, {X: 1, Y: 5, Z: 3}}, Weights: []float64{1, 1, 2}}
	r, e := CheckPlanar(context.Background(), uv, chart, model, 0, Options{})
	if e != nil || r.Status != Certified {
		t.Fatal(r, e)
	}
	other := PlanarCurve{Poles: []geom.Vec2{{X: 3, Y: 2}, {X: 3, Y: 5}, {X: 1, Y: 5}}, Weights: uv.Weights}
	global := Chart{Origin: geom.Vec3{Z: 3}, U: geom.Vec3{X: 1}, V: geom.Vec3{Y: 1}}
	r, e = CheckPlanarPair(context.Background(), uv, chart, other, global, 0, Options{})
	if e != nil || r.Status != Certified {
		t.Fatal(r, e)
	}
	line, e := geom.NewLine(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1})
	if e != nil {
		t.Fatal(e)
	}
	uLine := PlanarCurve{Poles: []geom.Vec2{{X: 2}, {X: 1}}}
	r, e = CheckPlanarLine(context.Background(), uLine, chart, line, 4, 2, 0, Options{})
	if e != nil || r.Status != Certified {
		t.Fatal(r, e)
	}
}
func TestChartMappingIsNotRoundedBeforeProof(t *testing.T) {
	uv := PlanarCurve{Poles: []geom.Vec2{{}, {X: 1}}}
	chart := Chart{Origin: geom.Vec3{X: 1e16}, U: geom.Vec3{X: 1}, V: geom.Vec3{Y: 1}}
	model := Curve{Poles: []geom.Vec3{{X: 1e16}, {X: 1e16}}}
	r, e := CheckPlanar(context.Background(), uv, chart, model, .5, Options{})
	if e != nil || r.Status != Exceeded {
		t.Fatal(r, e)
	}
	verifyWitness(t, r, .5)
	if r.Witness.PointA != r.Witness.PointB {
		t.Fatal("fixture should collapse rounded witnesses")
	}
}
func TestLimitsAndErrors(t *testing.T) {
	a := Curve{Poles: []geom.Vec3{{}, {X: 2}}}
	for _, o := range []Options{{MaxWork: 1}, {MaxBits: 1}} {
		r, e := Check(context.Background(), a, a, 0, o)
		if e != nil || r.Status != Unresolved {
			t.Fatal(r, e)
		}
	}
	for _, o := range []Options{{MaxDegree: 129}, {MaxDepth: 257}, {MaxIntervals: -1}} {
		if _, e := Check(context.Background(), a, a, 0, o); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	for _, w := range []float64{0, -1, math.NaN()} {
		b := a
		b.Weights = []float64{1, w}
		if _, e := Check(context.Background(), a, b, 0, Options{}); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Check(ctx, a, a, 0, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

// Independent 256-bit de Casteljau evaluation checks witnesses and certificate
// bounds without using the rational product/subdivision implementation.
func highPoint(c Curve, t *big.Float) [3]*big.Float {
	prec := uint(256)
	var p [][4]*big.Float
	for j := range c.Poles {
		i := j
		if c.Reversed {
			i = len(c.Poles) - 1 - j
		}
		v := c.Poles[i]
		w := 1.
		if c.Weights != nil {
			w = c.Weights[i]
		}
		row := [4]*big.Float{}
		for k, x := range []float64{v.X, v.Y, v.Z, 1} {
			row[k] = new(big.Float).SetPrec(prec).Mul(new(big.Float).SetPrec(prec).SetFloat64(x), new(big.Float).SetPrec(prec).SetFloat64(w))
		}
		p = append(p, row)
	}
	one := new(big.Float).SetPrec(prec).Sub(new(big.Float).SetPrec(prec).SetInt64(1), t)
	for n := len(p) - 1; n > 0; n-- {
		for i := 0; i < n; i++ {
			for k := 0; k < 4; k++ {
				x := new(big.Float).SetPrec(prec).Mul(one, p[i][k])
				y := new(big.Float).SetPrec(prec).Mul(t, p[i+1][k])
				p[i][k] = new(big.Float).SetPrec(prec).Add(x, y)
			}
		}
	}
	var r [3]*big.Float
	for k := range r {
		r[k] = new(big.Float).SetPrec(prec).Quo(p[0][k], p[0][3])
	}
	return r
}
func TestHighPrecisionRationalSamples(t *testing.T) {
	a := Curve{Poles: []geom.Vec3{{X: 1}, {X: 2, Y: 2}, {Y: 1}, {X: -1, Y: 2}}, Weights: []float64{1, .5, 2, 1}}
	b := Curve{Poles: append([]geom.Vec3(nil), a.Poles...), Weights: a.Weights}
	for i := range b.Poles {
		b.Poles[i].Z = .125
	}
	r, e := Check(context.Background(), a, b, .126, Options{})
	if e != nil || r.Status != Certified {
		t.Fatal(r, e)
	}
	for i := 0; i <= 100; i++ {
		parameter := new(big.Float).SetPrec(256).Quo(new(big.Float).SetPrec(256).SetInt64(int64(i)), new(big.Float).SetPrec(256).SetInt64(100))
		x, y := highPoint(a, parameter), highPoint(b, parameter)
		sum := new(big.Float).SetPrec(256)
		for k := 0; k < 3; k++ {
			d := new(big.Float).SetPrec(256).Sub(x[k], y[k])
			sum.Add(sum, new(big.Float).SetPrec(256).Mul(d, d))
		}
		upper := new(big.Float).SetPrec(256).SetRat(r.SquaredUpperBound)
		if sum.Cmp(upper) > 0 {
			t.Fatal("high precision sample exceeded certificate")
		}
	}
}

func TestInteriorViolationBeyondEndpointsAndMidpoint(t *testing.T) {
	a := Curve{Poles: []geom.Vec3{{}}}
	b := Curve{Poles: []geom.Vec3{{}, {Z: 1}, {Z: -1}, {}}}
	r, e := Check(context.Background(), a, b, .1, Options{})
	if e != nil || r.Status != Exceeded {
		t.Fatal(r, e)
	}
	verifyWitness(t, r, .1)
	if r.Witness.Parameter.Sign() == 0 || r.Witness.Parameter.Cmp(big.NewRat(1, 2)) == 0 || r.Witness.Parameter.Cmp(big.NewRat(1, 1)) == 0 {
		t.Fatal("fixture vanishes at endpoint/midpoint samples")
	}
	r, e = Check(context.Background(), a, b, .3, Options{})
	if e != nil || r.Status != Certified {
		t.Fatal(r, e)
	}
}
func FuzzQuadraticAgreementOracle(f *testing.F) {
	f.Add(uint8(4), uint8(4))
	f.Add(uint8(12), uint8(3))
	f.Fuzz(func(t *testing.T, amplitude, tol uint8) {
		a := Curve{Poles: []geom.Vec3{{}}}
		height := float64(amplitude%32) / 8
		tolerance := float64(tol%32) / 16
		b := Curve{Poles: []geom.Vec3{{}, {Z: height}, {}}}
		want := Certified
		if height/2 > tolerance {
			want = Exceeded
		}
		r, e := Check(context.Background(), a, b, tolerance, Options{})
		if e != nil || r.Status != want {
			t.Fatal(r, e, want)
		}
		if want == Exceeded {
			verifyWitness(t, r, tolerance)
		}
	})
}
