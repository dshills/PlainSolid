package geom

import (
	"context"
	"math"
	"testing"
)

func TestObliqueSkewCylinderMap(t *testing.T) {
	ou, u, ov, v := Vec3{X: 4}, Vec3{X: 2}, Vec3{X: 3, Y: 4, Z: 1}, Vec3{X: 3, Y: 4}
	p, e := NewObliqueSkewCylinderMap(context.Background(), ou, u, 2, ov, v, 3, SurfacePatchEvaluationOptions{})
	if e != nil || !p.Complete || p.ProjectionRelation != 1 || p.SeparationSquared.RatString() != "1" || p.SeparationSign != 1 {
		t.Fatal(p, e)
	}
	m, e := p.RepresentedAffine()
	if e != nil {
		t.Fatal(e)
	}
	if m != (Affine{{1.25, .75, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}) {
		t.Fatal(m)
	}
	for _, a := range []float64{-.4, .2, 2} {
		for _, b := range []float64{-.7, .4} {
			for _, y := range []float64{-.5, .3, 1.2} {
				q, e := p.Eval(a, b, y)
				if e != nil {
					t.Fatal(e)
				}
				x := q.Sub(ou)
				n := x.Sub(u.Scale(x.Dot(u) / u.Dot(u)))
				x = q.Sub(ov)
				other := x.Sub(v.Scale(x.Dot(v) / v.Dot(v)))
				if math.Abs(n.Dot(n)-(b*b+y*y)) > 1e-11 || math.Abs(other.Dot(other)-(a*a+(y-1)*(y-1))) > 1e-11 {
					t.Fatal("support identity", q)
				}
			}
		}
	}
	// Public diagnostics are owned copies, not mutable evaluation provenance.
	p.ClosestFeet[0][0].SetInt64(100)
	q, _ := p.Eval(0, 0, 0)
	if q != (Vec3{}) {
		t.Fatal("mutation", q)
	}
	for _, offset := range []float64{5, 6, -5} {
		p, e = NewObliqueSkewCylinderMap(context.Background(), Vec3{}, u, 2, Vec3{Z: offset}, v, 3, SurfacePatchEvaluationOptions{})
		want := 0
		if math.Abs(offset) > 5 {
			want = -1
		}
		if e != nil || p.ProjectionRelation != want {
			t.Fatal(p, e)
		}
	}
	p, e = NewObliqueSkewCylinderMap(context.Background(), Vec3{}, Vec3{X: 1}, 1, Vec3{Z: 1}, Vec3{X: 1, Y: 1}, 1, SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.RepresentedAffine(); e != ErrUnrepresentable {
		t.Fatal("irrational map", e)
	}
	if p, e = NewObliqueSkewCylinderMap(context.Background(), ou, u, 2, ov, v, 3, SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil || p.Complete {
		t.Fatal("budget", p, e)
	}
}
