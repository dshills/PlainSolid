package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestAffineSurfaceChartInverseAndExtrapolation(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{1 + 3*u + v, 2 + u + 2*v, -1 + u - v} })
	p, e := p.Reparameterize(ParameterRectangle{UMin: -2, UMax: 4, VMin: 3, VMax: 7}, false, false, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	chart, e := NewAffineSurfaceChart(context.Background(), p, AffineSurfaceChartOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, uv := range [][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(4, 1)}, {big.NewRat(10, 1), big.NewRat(-3, 1)}} {
		point, e := chart.EvalExact(uv[0], uv[1])
		if e != nil {
			t.Fatal(e)
		}
		inverse, e := chart.InvertExact(point)
		if e != nil || !inverse.OnPlane || inverse.U.Cmp(uv[0]) != 0 || inverse.V.Cmp(uv[1]) != 0 {
			t.Fatal(inverse, e)
		}
		inside := uv[0].Cmp(big.NewRat(0, 1)) == 0
		if inverse.Inside != inside {
			t.Fatal(inverse)
		}
		point[2].Add(point[2], big.NewRat(1, 1000))
		inverse, e = chart.InvertExact(point)
		if e != nil || inverse.OnPlane || inverse.Inside {
			t.Fatal(inverse, e)
		}
	}
}
func TestAffineSurfaceChartNonrepresentableAndOwnedRationals(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	for i := range p.net {
		p.net[i][2] = big.NewRat(1, 3)
	}
	budget := &polynomialBudget{maxOperations: 1000000, maxBits: 16384}
	if e := p.computeBounds(budget); e != nil {
		t.Fatal(e)
	}
	c, e := NewAffineSurfaceChart(context.Background(), p, AffineSurfaceChartOptions{})
	if e != nil {
		t.Fatal(e)
	}
	point := [3]*big.Rat{big.NewRat(1, 7), big.NewRat(2, 5), big.NewRat(1, 3)}
	r, e := c.InvertExact(point)
	if e != nil || !r.Inside || r.U.Cmp(big.NewRat(1, 7)) != 0 || r.V.Cmp(big.NewRat(2, 5)) != 0 {
		t.Fatal(r, e)
	}
	if new(big.Rat).SetFloat64(r.UBounds.Min).Cmp(r.U) > 0 || new(big.Rat).SetFloat64(r.UBounds.Max).Cmp(r.U) < 0 {
		t.Fatal(r)
	}
	r.U.SetInt64(99)
	r.V.SetInt64(88)
	again, e := c.InvertExact(point)
	if e != nil || again.U.Cmp(point[0]) != 0 || again.V.Cmp(point[1]) != 0 {
		t.Fatal(again, e)
	}
	output, e := c.EvalExact(point[0], point[1])
	if e != nil {
		t.Fatal(e)
	}
	output[2].SetInt64(0)
	output, e = c.EvalExact(point[0], point[1])
	if e != nil || output[2].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(output, e)
	}
}
func TestAffineSurfaceChartRejectsAndCancellation(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	_, e := NewAffineSurfaceChart(context.Background(), p, AffineSurfaceChartOptions{})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	p = arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	_, e = NewAffineSurfaceChart(context.Background(), p, AffineSurfaceChartOptions{ProjectionAxes: [2]int{0, 2}})
	if !errors.Is(e, ErrDegenerate) {
		t.Fatal(e)
	}
	_, e = NewAffineSurfaceChart(context.Background(), p, AffineSurfaceChartOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	_, e = NewAffineSurfaceChart(distanceCancelContext(3), p, AffineSurfaceChartOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	c, e := NewAffineSurfaceChart(context.Background(), p, AffineSurfaceChartOptions{})
	if e != nil {
		t.Fatal(e)
	}
	_, e = c.Invert(Vec3{math.NaN(), 0, 0})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	_, e = c.InvertExact([3]*big.Rat{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	_, e = c.EvalExact(nil, big.NewRat(0, 1))
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	_, e = NewAffineSurfaceChart(nil, p, AffineSurfaceChartOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
