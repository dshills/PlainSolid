package trimvalidate

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func polygon(ps ...geom.Vec2) Loop {
	var loop Loop
	for i, p := range ps {
		loop = append(loop, Segment{Start: p, End: ps[(i+1)%len(ps)]})
	}
	return loop
}
func rectangle(x0, y0, x1, y1 float64) Loop {
	return polygon(geom.Vec2{X: x0, Y: y0}, geom.Vec2{X: x1, Y: y0}, geom.Vec2{X: x1, Y: y1}, geom.Vec2{X: x0, Y: y1})
}
func reverse(l Loop) Loop {
	r := make(Loop, len(l))
	for i, s := range l {
		r[len(l)-1-i] = Segment{Start: s.End, End: s.Start, Control: s.Control, Kind: s.Kind, Weights: [3]float64{s.Weights[2], s.Weights[1], s.Weights[0]}}
	}
	return r
}
func expect(t *testing.T, outer Loop, holes []Loop, status Status) Report {
	t.Helper()
	r, e := ValidateLoops(context.Background(), outer, holes, Options{})
	if e != nil || r.Status != status {
		t.Fatalf("got %+v error %v want %v", r, e, status)
	}
	return r
}
func TestLinesHolesAndContacts(t *testing.T) {
	outer := rectangle(0, 0, 6, 6)
	expect(t, outer, []Loop{reverse(rectangle(1, 1, 2, 2)), reverse(rectangle(4, 4, 5, 5))}, Certified)
	expect(t, outer, []Loop{reverse(rectangle(5, 5, 7, 7))}, Invalid)
	expect(t, outer, []Loop{reverse(rectangle(1, 1, 5, 5)), reverse(rectangle(2, 2, 3, 3))}, Invalid)
	expect(t, outer, []Loop{reverse(rectangle(0, 1, 2, 2))}, Invalid)
	expect(t, reverse(outer), nil, Invalid)
	expect(t, outer, []Loop{rectangle(1, 1, 2, 2)}, Invalid)
	expect(t, outer, []Loop{reverse(rectangle(7, 7, 8, 8))}, Invalid)
}
func TestQuadraticConvexAndSubdivision(t *testing.T) {
	outer := rectangle(0, 0, 4, 4)
	controls := []geom.Vec2{{X: 2, Y: -1}, {X: 5, Y: 2}, {X: 2, Y: 5}, {X: -1, Y: 2}}
	for i := range outer {
		outer[i].Kind = Quadratic
		outer[i].Control = controls[i]
	}
	expect(t, outer, []Loop{reverse(rectangle(1, 1, 3, 3))}, Certified)
	outer = rectangle(0, 0, 4, 2)
	outer[0].Kind = Quadratic
	outer[0].Control = geom.Vec2{X: 2, Y: 3}
	r := expect(t, outer, nil, Certified)
	if r.Subdivisions == 0 {
		t.Fatal("overlapping hull fixture should require subdivision")
	}
	outer[0].Control.Y = 7
	expect(t, outer, nil, Invalid)
}
func TestExactQuadraticContactAndRetracing(t *testing.T) {
	outer := rectangle(0, 0, 4, 2)
	outer[0].Kind = Quadratic
	outer[0].Control = geom.Vec2{X: 2, Y: 4}
	expect(t, outer, nil, Invalid) // midpoint contacts the top edge
	outer = rectangle(0, 0, 4, 4)
	outer[0].Kind = Quadratic
	outer[0].Control = geom.Vec2{X: 5}
	expect(t, outer, nil, Invalid)
	outer[0].Control = outer[0].Start
	expect(t, outer, nil, Certified) // monotone t-squared line
}
func TestNearContactAndBudgetOutcomes(t *testing.T) {
	outer := rectangle(0, 0, 4, 2)
	outer[0].Kind = Quadratic
	outer[0].Control = geom.Vec2{X: 2, Y: math.Nextafter(4, 0)}
	expect(t, outer, nil, Certified)
	for _, o := range []Options{{MaxWork: 1}, {MaxEdges: 1}, {MaxBits: 1}} {
		r, e := ValidateLoops(context.Background(), outer, nil, o)
		if e != nil || r.Status != Unresolved {
			t.Fatal(r, e)
		}
	}
	outer[0].Control.Y = 3
	r, e := ValidateLoops(context.Background(), outer, nil, Options{MaxSubdivisions: 1, MaxDepth: 1})
	if e != nil || r.Status == Invalid {
		t.Fatal(r, e)
	}
	outer[0].Kind = UnsupportedCurve
	expect(t, outer, nil, Unresolved)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = ValidateLoops(ctx, outer, nil, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e = ValidateLoops(nil, outer, nil, Options{}); !errors.Is(e, ErrOptions) {
		t.Fatal(e)
	}
}
func TestProvenQuadraticTransverseCrossing(t *testing.T) {
	a := curve{p: [3]point{exact(geom.Vec2{X: -2}), exact(geom.Vec2{Y: .2}), exact(geom.Vec2{X: 2})}}
	b := curve{p: [3]point{exact(geom.Vec2{Y: -2}), exact(geom.Vec2{X: .1}), exact(geom.Vec2{Y: 2})}}
	budget := &budget{ctx: context.Background(), o: Options{MaxWork: 10000, MaxDepth: 48, MaxSubdivisions: 1000, MaxBits: 16384}}
	if status := separate(a, b, [2]*point{}, budget); status != Invalid {
		t.Fatal(status, budget.err)
	}
}
func TestWrongEndpointAndDegenerate(t *testing.T) {
	outer := rectangle(0, 0, 4, 4)
	outer[0].End.X = 3
	expect(t, outer, nil, Invalid)
	outer = rectangle(0, 0, 4, 4)
	outer[0].Kind = Quadratic
	outer[0].Control.X = math.NaN()
	expect(t, outer, nil, Invalid)
}

func TestNondyadicTangencyAndExplicitUnresolved(t *testing.T) {
	outer := polygon(geom.Vec2{}, geom.Vec2{X: 4, Y: 1.5}, geom.Vec2{X: 4, Y: 2}, geom.Vec2{Y: 2})
	outer[0].Kind = Quadratic
	outer[0].Control = geom.Vec2{X: 2, Y: 3}
	expect(t, outer, nil, Invalid) // tangent at t=2/3
	outer[0].End.Y = 1.49
	outer[1].Start.Y = 1.49
	expect(t, outer, nil, Certified)
	r, e := ValidateLoops(context.Background(), outer, nil, Options{MaxDepth: 1})
	if e != nil || r.Status != Unresolved {
		t.Fatal(r, e)
	}
}
func FuzzParabolicLoopClassification(f *testing.F) {
	f.Add(uint8(10), uint8(3))
	f.Fuzz(func(t *testing.T, height, control uint8) {
		h := float64(height%16+1) / 4
		c := float64(control%32) / 4
		outer := rectangle(0, 0, 4, h)
		outer[0].Kind = Quadratic
		outer[0].Control = geom.Vec2{X: 2, Y: c}
		want := Certified
		if c/2 >= h {
			want = Invalid
		}
		expect(t, outer, nil, want)
	})
}

func circle(cx, cy, r float64) Loop {
	points := []geom.Vec2{{X: cx + r, Y: cy}, {X: cx, Y: cy + r}, {X: cx - r, Y: cy}, {X: cx, Y: cy - r}}
	controls := []geom.Vec2{{X: cx + r, Y: cy + r}, {X: cx - r, Y: cy + r}, {X: cx - r, Y: cy - r}, {X: cx + r, Y: cy - r}}
	var out Loop
	for i, p := range points {
		out = append(out, Segment{Kind: RationalQuadratic, Start: p, Control: controls[i], End: points[(i+1)%4], Weights: [3]float64{1, 1, 2}})
	}
	return out
}
func TestExactRationalAnnulus(t *testing.T) {
	outer := circle(0, 0, 2)
	hole := reverse(circle(0, 0, 1))
	expect(t, outer, []Loop{hole}, Certified)
	expect(t, outer, []Loop{circle(0, 0, 1)}, Invalid)
	expect(t, outer, []Loop{reverse(circle(3, 0, 2))}, Invalid)
	expect(t, outer, []Loop{reverse(circle(1, 0, 1))}, Invalid)
}
func TestExactCircleEquationAndHomogeneousSubdivision(t *testing.T) {
	segment := circle(0, 0, 1)[0]
	c := curve{p: [3]point{exact(segment.Start), exact(segment.Control), exact(segment.End)}, w: [3]q{num(1), num(1), num(2)}}
	for _, tval := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(2, 3), big.NewRat(1, 1)} {
		p := evaluate(c, tval)
		if dot(p, p).Cmp(big.NewRat(1, 1)) != 0 || !onCurve(p, c) {
			t.Fatal("not an exact unit circle", p)
		}
	}
	b := &budget{ctx: context.Background(), o: Options{MaxWork: 1000, MaxDepth: 48, MaxSubdivisions: 100, MaxBits: 16384}}
	a, z := b.split(c)
	for _, tval := range []*big.Rat{big.NewRat(1, 3), big.NewRat(2, 3)} {
		if !same(evaluate(a, tval), evaluate(c, mul(tval, big.NewRat(1, 2)))) || !same(evaluate(z, tval), evaluate(c, add(big.NewRat(1, 2), mul(tval, big.NewRat(1, 2))))) {
			t.Fatal("homogeneous subdivision changed conic")
		}
	}
}
func TestRationalNearTangencyAndInvalidWeights(t *testing.T) {
	expect(t, circle(0, 0, 2), []Loop{reverse(circle(1-math.Ldexp(1, -40), 0, 1))}, Certified)
	outer := circle(0, 0, 2)
	for _, w := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		outer[0].Weights[1] = w
		expect(t, outer, nil, Invalid)
	}
	outer = circle(0, 0, 2)
	outer[0].Weights = [3]float64{7, 7, 7}
	expect(t, outer, nil, Certified)
	outer = circle(0, 0, 2)
	r, e := ValidateLoops(context.Background(), outer, []Loop{reverse(circle(.99, 0, 1))}, Options{MaxDepth: 1})
	if e != nil || r.Status == Invalid {
		t.Fatal(r, e)
	}
}

func TestRationalOrientationEnclosureRefinement(t *testing.T) {
	arc := circle(0, 0, 1)[0]
	middle := geom.Vec2{X: .5, Y: .5}
	loop := Loop{arc, {Start: arc.End, End: middle}, {Start: middle, End: arc.Start}}
	r := expect(t, loop, nil, Certified)
	if r.Subdivisions == 0 {
		t.Fatal("zero-area chord polygon requires orientation refinement")
	}
	expect(t, reverse(loop), nil, Invalid)
}
func TestRationalNondyadicTangencyRemainsUnresolved(t *testing.T) {
	outer := circle(0, 0, 2)
	hole := reverse(circle(.625, 1.5, .375))
	r, e := ValidateLoops(context.Background(), outer, []Loop{hole}, Options{MaxDepth: 12})
	if e != nil || r.Status != Unresolved {
		t.Fatal(r, e)
	}
	expect(t, outer, []Loop{reverse(circle(.625-math.Ldexp(1, -20), 1.5, .375))}, Certified)
}
func FuzzExactRationalCircleContainment(f *testing.F) {
	f.Add(uint8(1), uint8(3))
	f.Add(uint8(8), uint8(8))
	f.Fuzz(func(t *testing.T, offset, radius uint8) {
		x := float64(offset%16) / 8
		r := float64(radius%8+1) / 8
		outer := circle(0, 0, 2)
		hole := reverse(circle(x, 0, r))
		want := Certified
		if x+r >= 2 {
			want = Invalid
		}
		expect(t, outer, []Loop{hole}, want)
	})
}
