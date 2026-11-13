package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSphereLensLift(t *testing.T) {
	// Section radius sqrt(3); tool radius1.5, offset2: both centers lie
	// outside the opposing disk, but the chord-midpoint fan remains valid.
	q, e := NewSphereLensLifts(context.Background(), Vec3{}, 2, Vec3{X: 2}, Vec3{Z: 1}, 1.5, big.NewRat(1, 1), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range []float64{0, .2, .7, 1} {
		a, _ := q[0].Eval(0, v)
		b, _ := q[1].Eval(1, v)
		c, _ := q[0].Eval(1, v)
		d, _ := q[1].Eval(0, v)
		if a != b || c != d {
			t.Fatal("seam", a, b, c, d)
		}
	}
	for _, p := range q {
		bound, _ := p.HessianBounds()
		uu, _ := bound.UU.Float64()
		uv, _ := bound.UV.Float64()
		vv, _ := bound.VV.Float64()
		for _, u := range []float64{.1, .4, .8} {
			for _, v := range []float64{.2, .6, 1} {
				x, e := p.Eval(u, v)
				if e != nil || math.Abs(x.Dot(x)-4) > 1e-12 || x.Z < 1-1e-12 || (x.X-2)*(x.X-2)+x.Y*x.Y > 2.25+1e-12 {
					t.Fatal(x, e)
				}
				a, b, _ := p.Partials(u, v)
				aa, ab, bb, _ := p.SecondPartials(u, v)
				if a.Cross(b).Dot(x) >= 0 || aa.Length() > uu+1e-10 || ab.Length() > uv+1e-10 || bb.Length() > vv+1e-10 {
					t.Fatal(x, a, b, aa, ab, bb)
				}
				if v < 1 {
					h := 1e-5
					am, bm, _ := p.Partials(u-h, v)
					ap, bp, _ := p.Partials(u+h, v)
					_, vm, _ := p.Partials(u, v-h)
					_, vp, _ := p.Partials(u, v+h)
					if ap.Sub(am).Scale(1/(2*h)).Sub(aa).Length() > 1e-6 || bp.Sub(bm).Scale(1/(2*h)).Sub(ab).Length() > 1e-6 || vp.Sub(vm).Scale(1/(2*h)).Sub(bb).Length() > 1e-6 {
						t.Fatal("jet")
					}
				}
			}
		}
	}
	if _, e := NewSphereLensLifts(context.Background(), Vec3{}, 2, Vec3{X: 2}, Vec3{Z: 1}, 1.5, big.NewRat(0, 1), SurfacePatchEvaluationOptions{}); e != ErrDomain {
		t.Fatal(e)
	}
}
