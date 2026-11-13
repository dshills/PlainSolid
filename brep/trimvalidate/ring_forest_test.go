package trimvalidate

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestClassifyRingForest(t *testing.T) {
	box := func(x, y, size float64, cw bool) Loop {
		p := [4]geom.Vec2{{X: x, Y: y}, {X: x + size, Y: y}, {X: x + size, Y: y + size}, {X: x, Y: y + size}}
		if cw {
			p[1], p[3] = p[3], p[1]
		}
		ring := make(Loop, len(p))
		for i, a := range p {
			ring[i] = Segment{Start: a, End: p[(i+1)%len(p)]}
		}
		return ring
	}
	ctx := context.Background()
	// Input order gives no hint about enclosure or orientation.
	rings := []Loop{box(3, 3, 2, false), box(12, 0, 1, true), box(2, 2, 6, true), box(0, 0, 10, false)}
	want := []RingPlacement{{2, 2, 1}, {-1, 0, -1}, {3, 1, -1}, {-1, 0, 1}}
	certificate, got, proof, e := CertifyRingForest(ctx, rings, Options{})
	if e != nil || proof.Status != Certified || len(got) != len(want) {
		t.Fatal(got, proof, e)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("ring %d: %+v, want %+v", i, got[i], want[i])
		}
	}
	for _, c := range []struct {
		x, y int64
		want Location
	}{{1, 1, Inside}, {2, 3, Boundary}, {3, 6, Outside}, {4, 4, Inside}, {13, 2, Outside}, {12, 0, Boundary}} {
		loc, proof, e := certificate.Locate(ctx, [2]*big.Rat{big.NewRat(c.x, 1), big.NewRat(c.y, 1)})
		if e != nil || proof.Status != Certified || loc != c.want {
			t.Fatal("nonzero signed winding", c, loc, proof, e)
		}
	}
	if bad, proof, e := ClassifyRingForest(ctx, rings, Options{MaxWork: 1}); e != nil || proof.Status != Unresolved || bad != nil {
		t.Fatal("work exhausted after exposing a partial forest", bad, proof, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if area, e := certificate.BoundSignedArea(canceled, big.NewRat(1, 100)); e != context.Canceled || area.Lower != nil || area.Upper != nil {
		t.Fatal("cancelled area query returned bounds", area, e)
	}
	rings[1] = box(10, 0, 1, true)
	if bad, proof, e := ClassifyRingForest(ctx, rings, Options{}); e != nil || proof.Status != Invalid || bad != nil {
		t.Fatal("contacting rings admitted", bad, proof, e)
	}
}

func TestCertifiedExactCubicGreenArea(t *testing.T) {
	ctx := context.Background()
	// Degree-three version of y=1−x²/2, traveled right to left.
	h := [][3]*big.Rat{
		{big.NewRat(1, 1), big.NewRat(1, 2), big.NewRat(1, 1)},
		{big.NewRat(2, 3), big.NewRat(5, 6), big.NewRat(1, 1)},
		{big.NewRat(1, 3), big.NewRat(1, 1), big.NewRat(1, 1)},
		{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(1, 1)},
	}
	arc, e := ExactBezier(h)
	if e != nil {
		t.Fatal(e)
	}
	r := Loop{{Start: geom.Vec2{}, End: geom.Vec2{X: 1}}, {Start: geom.Vec2{X: 1}, End: geom.Vec2{X: 1, Y: .5}}, arc,
		{Start: geom.Vec2{Y: 1}, End: geom.Vec2{}}}
	c, report, e := CertifyLoops(ctx, r, nil, Options{})
	if e != nil || report.Status != Certified {
		t.Fatal(report, e)
	}
	a, e := c.BoundSignedArea(ctx, big.NewRat(1, 1000000))
	if e != nil || a.Lower.Cmp(big.NewRat(5, 6)) != 0 || a.Upper.Cmp(a.Lower) != 0 || a.Subdivisions != 0 {
		t.Fatal("exact cubic Green integral", a, e)
	}
}
