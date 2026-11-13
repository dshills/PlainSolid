package geom

import (
	"context"
	"math"
	"testing"
)

func TestCylinderCapRadialJunctions(t *testing.T) {
	a := RawCylinder{Axis: Vec3{Z: 1}, Radius: 2, Min: -1, Max: 0}
	b := RawCylinder{Axis: Vec3{X: 1}, Radius: 1, Min: -4, Max: 4}
	o := ExactCurveSegmentOptions{ParameterTolerance: 1e-10}
	r, e := IntersectCylinderCapRadialJunctions(context.Background(), a, b, true, o)
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	for i, c := range r.Contacts {
		if len(c.Hits) != 1 || len(c.Coincident) != 0 || c.Hits[0].Multiplicity != 1 {
			t.Fatal(i, c)
		}
		h := c.Hits[0]
		lo, _ := h.Lower.Float64()
		hi, _ := h.Upper.Float64()
		p, e := r.Curves[i].Eval((lo + hi) / 2)
		if e != nil || math.Abs(p.Y*p.Y-1) > 1e-8 || math.Abs(p.X*p.X+p.Y*p.Y-4) > 1e-10 || p.Z != 0 {
			t.Fatal(p, e)
		}
	}
	// A genuinely oblique radial quadric yields quartic chart residuals.
	b.Axis, b.Radius = Vec3{X: 1, Y: 1, Z: 1}, 1.5
	r, e = IntersectCylinderCapRadialJunctions(context.Background(), a, b, true, o)
	if e != nil {
		t.Fatal(e)
	}
	total := 0
	for i, c := range r.Contacts {
		for _, h := range c.Hits {
			total++
			lo, _ := h.Lower.Float64()
			hi, _ := h.Upper.Float64()
			p, _ := r.Curves[i].Eval((lo + hi) / 2)
			n := p.Sub(b.Axis.Scale(p.Dot(b.Axis) / 3))
			if math.Abs(n.Dot(n)-2.25) > 1e-8 {
				t.Fatal(p)
			}
		}
	}
	if total != 4 {
		t.Fatal(total)
	}
	b.Axis, b.Radius = Vec3{X: 1}, 2
	r, e = IntersectCylinderCapRadialJunctions(context.Background(), a, b, true, o)
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, c := range r.Contacts {
		for _, h := range c.Hits {
			count++
			if h.Multiplicity != 2 || h.Parameter == nil {
				t.Fatal(h)
			}
		}
	}
	if count != 4 {
		t.Fatal("two cardinal tangencies with double ancestry", count)
	}
	b.Axis = Vec3{Z: 3}
	r, e = IntersectCylinderCapRadialJunctions(context.Background(), a, b, true, o)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range r.Contacts {
		if len(c.Coincident) != 1 || len(c.Hits) != 0 {
			t.Fatal(c)
		}
	}
	a.Axis = Vec3{X: 1, Y: 1, Z: 1}
	if _, e = IntersectCylinderCapRadialJunctions(context.Background(), a, b, true, o); e != ErrUnrepresentable {
		t.Fatal("irrational frame", e)
	}
	a.Axis = Vec3{Z: 1}
	if r, e = IntersectCylinderCapRadialJunctions(context.Background(), a, b, true, ExactCurveSegmentOptions{MaxOperations: 1}); e == nil || r.Complete {
		t.Fatal("budget", r, e)
	}
}
