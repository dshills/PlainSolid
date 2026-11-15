package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func mappedChainFixture(t *testing.T, other bool) (geom.Surface, MappedSplineChain) {
	t.Helper()
	ctx := context.Background()
	makeCurve := func(p [][2]float64) ExactBezierUV {
		h := make([][3]*big.Rat, len(p))
		for i, v := range p {
			h[i] = [3]*big.Rat{ratInset(v[0]), ratInset(v[1]), big.NewRat(1, 1)}
		}
		q, e := NewExactBezierUV(h)
		if e != nil {
			t.Fatal(e)
		}
		return q
	}
	a := makeCurve([][2]float64{{0, .25}, {.125, .125}, {.375, .125}, {.5, .25}})
	b := makeCurve([][2]float64{{.5, .25}, {.625, .5}, {.875, .5}, {1, .25}})
	pieces := []ExactBezierUV{a, b}
	stations := []*big.Rat{big.NewRat(1, 2)}
	speeds := []*big.Rat{big.NewRat(2, 1), big.NewRat(1, 2)}
	raw := []float64{2, 5, 9}
	if other {
		left, right, e := a.Split(big.NewRat(1, 2))
		if e != nil {
			t.Fatal(e)
		}
		pieces = []ExactBezierUV{left, right, b}
		stations = []*big.Rat{big.NewRat(1, 4), big.NewRat(1, 2)}
		speeds = []*big.Rat{big.NewRat(4, 1), big.NewRat(2, 1), big.NewRat(1, 4)}
		raw = []float64{10, 15, 21, 28}
	}
	var h [][3]*big.Rat
	for i, p := range pieces {
		c := p.HomogeneousControls()
		power := big.NewRat(1, 1)
		for j := range c {
			if j > 0 {
				power.Mul(power, speeds[i])
			}
			if other {
				c[j][0], c[j][1] = c[j][1], c[j][0]
			}
			for k := range c[j] {
				c[j][k].Mul(c[j][k], power)
			}
		}
		if len(h) > 0 {
			scale := new(big.Rat).Quo(h[len(h)-1][2], c[0][2])
			for j := range c {
				for k := range c[j] {
					c[j][k].Mul(c[j][k], scale)
				}
			}
			h = append(h, c[1:]...)
		} else {
			h = append(h, c...)
		}
	}
	var poles []geom.Vec3
	var weights, knots []float64
	for _, p := range h {
		x, e := geom.RepresentedExactScalar(new(big.Rat).Quo(p[0], p[2]))
		if e != nil {
			t.Fatal(e)
		}
		y, e := geom.RepresentedExactScalar(new(big.Rat).Quo(p[1], p[2]))
		if e != nil {
			t.Fatal(e)
		}
		w, e := geom.RepresentedExactScalar(p[2])
		if e != nil {
			t.Fatal(e)
		}
		poles = append(poles, geom.Vec3{X: x, Y: y})
		weights = append(weights, w)
	}
	for i, k := range raw {
		n := 3
		if i == 0 || i == len(raw)-1 {
			n++
		}
		for j := 0; j < n; j++ {
			knots = append(knots, k)
		}
	}
	if other {
		for i, j := 0, len(poles)-1; i < j; i, j = i+1, j-1 {
			poles[i], poles[j] = poles[j], poles[i]
			weights[i], weights[j] = weights[j], weights[i]
		}
		sum := raw[0] + raw[len(raw)-1]
		for i, j := 0, len(knots)-1; i <= j; i, j = i+1, j-1 {
			x, y := knots[i], knots[j]
			knots[i], knots[j] = sum-y, sum-x
		}
	}
	curve, e := geom.NewBSpline(3, poles, knots, weights)
	if e != nil {
		t.Fatal(e)
	}
	var parts []MappedSplineUV
	for i := range pieces {
		lo, hi := raw[i], raw[i+1]
		if other {
			sum := raw[0] + raw[len(raw)-1]
			lo, hi = sum-hi, sum-lo
		}
		q, e := NewMappedSplineUV(ctx, SplineUV{Curve: curve, Range: UVInterval{lo, hi}, Reversed: other}, new(big.Rat).Inv(speeds[i]), geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		parts = append(parts, q)
	}
	chain, e := NewMappedSplineChain(ctx, parts, stations, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var net []geom.Vec3
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			x, y := float64(i), float64(j)
			if other {
				x, y = y, x
			}
			net = append(net, geom.Vec3{X: x, Y: y, Z: x * y})
		}
	}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, net, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return surface, chain
}
func TestMappedSplineChainAgreement(t *testing.T) {
	ctx := context.Background()
	a, ca := mappedChainFixture(t, false)
	b, cb := mappedChainFixture(t, true)
	// A genuine derivative jump at t=1/2 rules out one global polynomial.
	da, _ := ca.parts[0].curve.DerivativeUV(1)
	db, _ := ca.parts[1].curve.DerivativeUV(0)
	if da.Y == db.Y {
		t.Fatal("fixture is not C0 crease")
	}
	if _, e := NewPiecewiseMappedSplineUV(ctx, SplineUV{Curve: ca.parts[0].source.Curve, Range: UVInterval{2, 9}}, big.NewRat(1, 2), [2]*big.Rat{big.NewRat(1, 2), big.NewRat(2, 1)}, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("old global polynomial accepted crease")
	}
	r, e := CertifyMappedSplineChainAgreement(ctx, a, ca, b, cb, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	cuts, points := r.ModelCuts(), r.ModelPoints()
	if len(cuts) != 4 || cuts[1].Cmp(big.NewRat(1, 4)) != 0 || cuts[2].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("common partition")
	}
	for owner, c := range []MappedSplineChain{ca, cb} {
		raw, e := r.AuthoredParameters(owner)
		if e != nil {
			t.Fatal(e)
		}
		for i, tau := range raw {
			uv := authoredMixedSplinePoint(t, c.parts[0].source, tau)
			x, y := uv[0], uv[1]
			if owner == 1 {
				x, y = y, x
			}
			if x.Cmp(points[i][0]) != 0 || y.Cmp(points[i][1]) != 0 || new(big.Rat).Mul(x, y).Cmp(points[i][2]) != 0 {
				t.Fatal("independent original support witness")
			}
		}
	}
	cuts[1].SetInt64(99)
	points[0][0].SetInt64(99)
	if r.ModelCuts()[1].Cmp(big.NewRat(1, 4)) != 0 || r.ModelPoints()[0][0].Sign() != 0 {
		t.Fatal("mutable evidence")
	}
	for _, o := range []edgecheck.Options{{MaxWork: 1}, {MaxIntervals: 2}} {
		if x, e := CertifyMappedSplineChainAgreement(ctx, a, ca, b, cb, o); e == nil || len(x.ModelCuts()) != 0 {
			t.Fatal("quota")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if x, e := CertifyMappedSplineChainAgreement(canceled, a, ca, b, cb, edgecheck.Options{}); e == nil || len(x.ModelCuts()) != 0 {
		t.Fatal("canceled")
	}
	wrong := cb
	wrong.stations = []*big.Rat{big.NewRat(1, 8), big.NewRat(1, 2)}
	if x, e := CertifyMappedSplineChainAgreement(ctx, a, ca, b, wrong, edgecheck.Options{}); e == nil || len(x.ModelCuts()) != 0 {
		t.Fatal("same endpoint different interior accepted")
	}
	if _, e := NewMappedSplineChain(ctx, []MappedSplineUV{ca.parts[1], ca.parts[0]}, ca.stations, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("reordered source")
	}
}
