package intersect

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func frameForTest(t *testing.T, n geom.Vec3) geom.Frame {
	t.Helper()
	f, e := geom.FrameFromNormal(geom.Vec3{}, n)
	if e != nil {
		t.Fatal(e)
	}
	return f
}

func TestCylinderPlaneTrace(t *testing.T) {
	f := frameForTest(t, geom.Vec3{Z: 1})
	c, e := geom.NewCylinderSurface(f, 2)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewPlaneSurface(f)
	if e != nil {
		t.Fatal(e)
	}
	a := Patch{c, geom.Vec2{X: -2 * math.Pi, Y: -1}, geom.Vec2{X: 2 * math.Pi, Y: 1}}
	b := Patch{p, geom.Vec2{X: -3, Y: -3}, geom.Vec2{X: 3, Y: 3}}
	for _, rev := range []bool{false, true} {
		r, e := Trace(context.Background(), a, b, Parameters{geom.Vec2{X: .15, Y: .02}, geom.Vec2{X: 1.95, Y: .29}}, Options{Tolerance: 1e-11, Step: .15, Steps: 30, Reverse: rev})
		if e != nil || len(r) != 31 {
			t.Fatal(len(r), e)
		}
		for i, x := range r {
			if math.Abs(x.Point.Z) > 1e-10 || math.Abs(x.Point.Length()-2) > 1e-10 || x.Residual > 1e-11 || math.Abs(x.Point.Dot(x.Tangent)) > 1e-10 {
				t.Fatal(x)
			}
			if i > 0 {
				d := x.Parameters.A.X - r[i-1].Parameters.A.X
				if math.Abs(math.Abs(d)-math.Asin(.15/2)) > 1e-9 || rev && d <= 0 || !rev && d >= 0 {
					t.Fatal(d)
				}
			}
		}
	}
}
func TestTraceFailure(t *testing.T) {
	f := frameForTest(t, geom.Vec3{Z: 1})
	p, e := geom.NewPlaneSurface(f)
	if e != nil {
		t.Fatal(e)
	}
	b := Patch{p, geom.Vec2{X: -1, Y: -1}, geom.Vec2{X: 1, Y: 1}}
	o := Options{Tolerance: 1e-10, Step: .1, Steps: 20}
	if _, e = Trace(context.Background(), b, b, Parameters{}, o); !errors.Is(e, ErrSingular) {
		t.Fatal(e)
	}
	y, e := geom.NewPlaneSurface(frameForTest(t, geom.Vec3{Y: 1}))
	if e != nil {
		t.Fatal(e)
	}
	a := b
	a.Surface = y
	r, e := Trace(context.Background(), b, a, Parameters{}, o)
	if !errors.Is(e, ErrDomain) || len(r) < 9 || len(r) > 11 {
		t.Fatalf("expected partial trace: %d %v", len(r), e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Trace(ctx, b, a, Parameters{}, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestFindSeedsTwoBranches(t *testing.T) {
	f := frameForTest(t, geom.Vec3{Z: 1})
	c, e := geom.NewCylinderSurface(f, 2)
	if e != nil {
		t.Fatal(e)
	}
	cut, e := geom.FrameFromNormal(geom.Vec3{X: .5}, geom.Vec3{X: 1})
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewPlaneSurface(cut)
	if e != nil {
		t.Fatal(e)
	}
	a := Patch{c, geom.Vec2{X: -3, Y: -1}, geom.Vec2{X: 3, Y: 1}}
	b := Patch{p, geom.Vec2{X: -3, Y: -2}, geom.Vec2{X: 3, Y: 2}}
	s, e := FindSeeds(context.Background(), a, b, SeedOptions{Tolerance: 1e-10, MaxSeeds: 256})
	if e != nil || s.Truncated || len(s.Samples) < 2 {
		t.Fatalf("%+v %v", s, e)
	}
	pos, neg := false, false
	for _, q := range s.Samples {
		if math.Abs(q.Point.X-.5) > 1e-9 || math.Abs(math.Abs(q.Point.Y)-math.Sqrt(3.75)) > 1e-9 || q.Residual > 1e-10 {
			t.Fatalf("wrong branch candidate %+v", q)
		}
		if q.Point.Y > 0 {
			pos = true
		} else {
			neg = true
		}
	}
	if !pos || !neg {
		t.Fatal("missed one generator")
	}
	u, e := FindSeeds(context.Background(), a, b, SeedOptions{Tolerance: 1e-10, MaxPairs: 1})
	if e != nil || !u.Truncated {
		t.Fatal(u, e)
	}
}

func TestFitBranchPairedParameters(t *testing.T) {
	f := frameForTest(t, geom.Vec3{Z: 1})
	c, e := geom.NewCylinderSurface(f, 2)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewPlaneSurface(f)
	if e != nil {
		t.Fatal(e)
	}
	a := Patch{c, geom.Vec2{X: -2, Y: -1}, geom.Vec2{X: 2, Y: 1}}
	b := Patch{p, geom.Vec2{X: -3, Y: -3}, geom.Vec2{X: 3, Y: 3}}
	r, e := Trace(context.Background(), a, b, Parameters{geom.Vec2{}, geom.Vec2{X: 2}}, Options{Tolerance: 1e-11, Step: .02, Steps: 40})
	if e != nil {
		t.Fatal(e)
	}
	c0, c1, e := FitBranch(a, b, r, 1e-4)
	if e != nil {
		t.Fatal(e)
	}
	_, end := c0.Range()
	for i := range 101 {
		x := end * float64(i) / 100
		p, e := c0.Eval(x)
		q, f := c1.Eval(x)
		if e != nil || f != nil || p.Sub(q).Length() > 1e-4 || math.Abs(p.Length()-2) > 1e-10 {
			t.Fatalf("%v %v %v %v", p, q, e, f)
		}
	}
	if _, _, e = FitBranch(a, b, r, 1e-8); !errors.Is(e, ErrUnresolved) {
		t.Fatal(e)
	}
}

func TestTraceClosedPlaneEllipse(t *testing.T) {
	f := frameForTest(t, geom.Vec3{Z: 1})
	a, e := geom.NewPlaneSurface(f)
	if e != nil {
		t.Fatal(e)
	}
	// A regular rectangular chart of z=x²/4+y²-1 gives a closed branch
	// that fits inside both UV domains without a periodic seam.
	p := Patch{a, geom.Vec2{X: -3, Y: -2}, geom.Vec2{X: 3, Y: 2}}
	q := p
	q.Surface = bowl{}
	o := ClosedOptions{Options: Options{Tolerance: 1e-10, Step: .04, Steps: 300}, ClosureTolerance: .001}
	c, d, r, e := ContinueClosed(context.Background(), p, q, Parameters{geom.Vec2{X: 2}, geom.Vec2{X: 2}}, o, .001)
	if e != nil || len(r) < 100 || r[0] != r[len(r)-1] {
		t.Fatal(len(r), e)
	}
	_, end := c.Range()
	if end < 9.6 || end > 9.8 {
		t.Fatal(end)
	}
	for _, x := range r {
		u, v := x.Point.X, x.Point.Y
		if math.Abs(x.Point.Z) > 1e-10 || math.Abs(u*u/4+v*v-1) > 1e-9 {
			t.Fatal(x)
		}
	}
	a0, e := c.Eval(end * .45)
	b0, ferr := d.Eval(end * .45)
	if e != nil || ferr != nil || a0.Sub(b0).Length() > .001 {
		t.Fatal(a0, b0, e, ferr)
	}
}

type bowl struct{}

func (bowl) Eval(u, v float64) (geom.Vec3, error) {
	return geom.Vec3{X: u, Y: v, Z: u*u/4 + v*v - 1}, nil
}
func (bowl) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	return geom.Vec3{X: 1, Z: u / 2}, geom.Vec3{Y: 1, Z: 2 * v}, nil
}
