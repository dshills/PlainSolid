package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestOffsetCircleExitAndCapStripBounds(t *testing.T) {
	a2 := big.NewRat(7, 3)
	d := .5
	q, e := BoundOffsetCircleExit(context.Background(), a2, d, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Proven {
		t.Fatal(q, e)
	}
	low, _ := q.Value[0].Float64()
	high, _ := q.Value[1].Float64()
	b1, _ := q.FirstAbsolute.Float64()
	b2, _ := q.SecondAbsolute.Float64()
	for i := 0; i < 64; i++ {
		u := 2 * math.Pi * float64(i) / 64
		s, c := math.Sincos(u)
		w := math.Sqrt(7.0/3 - d*d*s*s)
		wp := -d * d * s * c / w
		wpp := -d*d*(c*c-s*s)/w - d*d*d*d*s*s*c*c/(w*w*w)
		A := -d*c + w
		Ap := d*s + wp
		App := d*c + wpp
		if A < low-1e-13 || A > high+1e-13 || math.Abs(Ap) > b1+1e-13 || math.Abs(App) > b2+1e-13 {
			t.Fatal(u, A, Ap, App)
		}
	}
	strip, e := BoundSphereToolCapStrip(context.Background(), 5, 2, 1, big.NewRat(5, 1), SurfacePatchEvaluationOptions{})
	if e != nil || !strip.Proven || strip.Empty || strip.CosineMinimum.Sign() != 0 {
		t.Fatal(strip, e)
	}
	UU, _ := strip.UU.Float64()
	UV, _ := strip.UV.Float64()
	VV, _ := strip.VV.Float64()
	point := func(u, v float64) Vec3 {
		s, c := math.Sincos(u)
		A := -c + math.Sqrt(5-s*s)
		rho := (1-v)*A + 2*v
		return Vec3{X: 1 + rho*c, Y: rho * s, Z: math.Sqrt(25 - 1 - rho*rho - 2*rho*c)}
	}
	for _, u := range []float64{-.9, 0, .8} {
		for _, v := range []float64{.2, .6} {
			h := 1e-4
			p := point(u, v)
			uu := point(u+h, v).Add(point(u-h, v)).Sub(p.Scale(2)).Scale(1 / (h * h))
			vv := point(u, v+h).Add(point(u, v-h)).Sub(p.Scale(2)).Scale(1 / (h * h))
			uv := point(u+h, v+h).Sub(point(u+h, v-h)).Sub(point(u-h, v+h)).Add(point(u-h, v-h)).Scale(1 / (4 * h * h))
			if uu.Length() > UU || uv.Length() > UV || vv.Length() > VV {
				t.Fatal(u, v, uu, uv, vv)
			}
		}
	}
	empty, e := BoundSphereToolCapStrip(context.Background(), 5, 1, .5, big.NewRat(9, 1), SurfacePatchEvaluationOptions{})
	if e != nil || !empty.Empty {
		t.Fatal(empty, e)
	}
	if _, e = BoundOffsetCircleExit(context.Background(), big.NewRat(1, 1), 1, SurfacePatchEvaluationOptions{}); e != ErrDomain {
		t.Fatal(e)
	}
	if _, e = BoundSphereToolCapStrip(context.Background(), 5, 2, 1, big.NewRat(5, 1), SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
}
