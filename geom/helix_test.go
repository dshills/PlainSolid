package geom

import (
	"math"
	"testing"
)

func TestAnalyticHelix(t *testing.T) {
	f, e := NewFrame(Vec3{2, -3, 5}, Vec3{0, 1, 0}, Vec3{1, 0, 0})
	if e != nil {
		t.Fatal(e)
	}
	for _, hand := range []HelixHandedness{RightHanded, LeftHanded} {
		h, e := NewHelix(f, 2, 3, hand)
		if e != nil {
			t.Fatal(e)
		}
		for _, u := range []float64{-1.75, -.125, 0, .25, 1, 1048576.25} {
			p, e := h.Eval(u)
			if e != nil {
				t.Fatal(e)
			}
			q := f.ToLocal(p)
			if math.Abs(math.Hypot(q.X, q.Y)-2) > 1e-9 || math.Abs(q.Z-3*u) > 1e-9 {
				t.Fatal(u, q)
			}
			if u == .25 && math.Abs(q.Y-2*float64(hand)) > 1e-12 {
				t.Fatal("handedness", q)
			}
			d, e := h.Derivative(u)
			if e != nil {
				t.Fatal(e)
			}
			dd, e := h.SecondDerivative(u)
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(d.Length()-math.Hypot(4*math.Pi, 3)) > 1e-10 || math.Abs(dd.Length()-8*math.Pi*math.Pi) > 1e-10 {
				t.Fatal(d, dd)
			}
			if math.Abs(u) < 2 {
				eps := 1e-6
				p0, _ := h.Eval(u - eps)
				p1, _ := h.Eval(u + eps)
				if p1.Sub(p0).Scale(.5/eps).Sub(d).Length() > 1e-7 {
					t.Fatal("first derivative")
				}
				d0, _ := h.Derivative(u - eps)
				d1, _ := h.Derivative(u + eps)
				if d1.Sub(d0).Scale(.5/eps).Sub(dd).Length() > 1e-7 {
					t.Fatal("second derivative")
				}
			}
		}
		box, e := h.Bounds(-.3, .7)
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i <= 100; i++ {
			p, _ := h.Eval(-.3 + float64(i)/100)
			if p.X < box.Min.X || p.X > box.Max.X || p.Y < box.Min.Y || p.Y > box.Max.Y || p.Z < box.Min.Z || p.Z > box.Max.Z {
				t.Fatal(p, box)
			}
		}
		for _, step := range []float64{.01, .1, .5, 2} {
			bound, e := h.ChordDeviationBound(-.2, -.2+step)
			if e != nil {
				t.Fatal(e)
			}
			a, _ := h.Eval(-.2)
			b, _ := h.Eval(-.2 + step)
			for i := 0; i <= 100; i++ {
				s := float64(i) / 100
				p, _ := h.Eval(-.2 + step*s)
				chord := a.Scale(1 - s).Add(b.Scale(s))
				if p.Sub(chord).Length() > bound+1e-12 {
					t.Fatal(step, bound)
				}
			}
		}
	}
}
func TestHelixLimitsAndTinyBounds(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{1, 0, 0}, Vec3{0, 0, 1})
	for _, bad := range []struct {
		r, l float64
		h    HelixHandedness
	}{{0, 1, RightHanded}, {1, 0, RightHanded}, {1, -1, RightHanded}, {1, 1, 0}, {math.Inf(1), 1, RightHanded}} {
		if _, e := NewHelix(f, bad.r, bad.l, bad.h); e == nil {
			t.Fatal(bad)
		}
	}
	if _, e := NewHelix(Frame{}, 1, 1, RightHanded); e == nil {
		t.Fatal("frame")
	}
	h, _ := NewHelix(f, math.SmallestNonzeroFloat64, 1, RightHanded)
	b, e := h.ChordDeviationBound(0, .125)
	if e != nil || b <= 0 {
		t.Fatal("underflow bound", b, e)
	}
	if z, e := h.ChordDeviationBound(2, 2); e != nil || z != 0 {
		t.Fatal(z, e)
	}
	if _, e := h.Bounds(2, 1); e == nil {
		t.Fatal("reversed interval")
	}
	if _, e := h.Eval(math.NaN()); e == nil {
		t.Fatal("nan")
	}
	if _, e := (Helix{}).Derivative(0); e == nil {
		t.Fatal("zero")
	}
	huge, _ := NewHelix(f, 1, math.MaxFloat64, RightHanded)
	if _, e := huge.Eval(2); e == nil {
		t.Fatal("overflow")
	}
	if _, e := huge.Bounds(0, 2); e == nil {
		t.Fatal("bound overflow")
	}
}
