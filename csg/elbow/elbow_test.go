package elbow

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestSignedElbowAndContinuation(t *testing.T) {
	frame, e := geom.NewFrame(geom.Vec3{X: .1, Y: -2, Z: 3}, geom.Vec3{X: 1, Y: 2}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, sign := range []float64{-1, 1} {
		s, e := NewSigned(context.Background(), frame, 4, 1, sign*1.25)
		if e != nil {
			t.Fatal(e)
		}
		join, e := Continue(context.Background(), s, sign*.75)
		if e != nil {
			t.Fatal(e)
		}
		coeff, e := join.Union.VolumePiCoefficient()
		if e != nil {
			t.Fatal(e)
		}
		det := join.Union.determinant()
		want := mul(big.NewRat(8, 1), det)
		if coeff.Cmp(want) != 0 {
			t.Fatal("exact affine volume", coeff, want)
		}
		mass, e := join.Union.Solid().MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		v, _ := coeff.Float64()
		if math.Abs(mass.Volume-math.Pi*v) > 1e-7 {
			t.Fatal("native ideal comparison", mass.Volume, math.Pi*v)
		}
		b, e := join.Union.IdealBounds()
		if e != nil {
			t.Fatal(e)
		}
		f := join.Union.Frame()
		for i := 0; i <= 20; i++ {
			u := 2 * float64(i) / 20
			for j := 0; j < 32; j++ {
				v := 2 * math.Pi * float64(j) / 32
				p := f.ToWorld(geom.Vec3{X: (4 + math.Cos(v)) * math.Cos(u), Y: (4 + math.Cos(v)) * math.Sin(u), Z: math.Sin(v)})
				if p.X < b.Min.X || p.Y < b.Min.Y || p.Z < b.Min.Z || p.X > b.Max.X || p.Y > b.Max.Y || p.Z > b.Max.Z {
					t.Fatal("bounds", p, b)
				}
			}
		}
		if join.SharedAngle.Cmp(big.NewRat(5, 4)) != 0 || join.Intervals[1][1].Cmp(big.NewRat(2, 1)) != 0 || join.Union.SignedAngle() != sign*2 {
			t.Fatal("ancestry")
		}
		coeff.SetInt64(0)
		join.SharedAngle.SetInt64(0)
		again, _ := join.Union.VolumePiCoefficient()
		if again.Sign() == 0 {
			t.Fatal("mutable proof")
		}
	}
}
func TestElbowLimits(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, a := range []float64{0, math.Copysign(0, -1), 1e-15, 2 * math.Pi, -2 * math.Pi, math.NaN()} {
		if _, e := NewSigned(context.Background(), f, 4, 1, a); e == nil {
			t.Fatal("accepted", a)
		}
	}
	s, e := NewSigned(context.Background(), f, 4, 1, 5.5)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := Continue(context.Background(), s, 1); e == nil {
		t.Fatal("wrap")
	}
	if _, e := Continue(context.Background(), s, -1); e == nil {
		t.Fatal("retrace")
	}
	s, _ = NewSigned(context.Background(), f, 4, 1, .1)
	if _, e := Continue(context.Background(), s, .2); e == nil {
		t.Fatal("rounded sum")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Continue(ctx, s, .1); e == nil {
		t.Fatal("cancel")
	}
}
