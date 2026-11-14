package geom

import (
	"context"
	"math"
	"math/big"
	"reflect"
	"testing"
)

func TestTorusCylinderMoments(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	o := TorusCylinderMomentOptions{VolumeTolerance: r(1, 100), SecondMomentTolerance: r(1, 10), CentroidTolerance: r(1, 1000), InertiaTolerance: r(1, 5)}
	for _, ends := range [][2]float64{{-3, 4}, {-4, 3}} {
		for _, op := range []TorusCylinderOperation{TorusCylinderCommon, TorusCylinderDifference, TorusCylinderUnion} {
			s, _ := NewTorusCylinderMaterial(5, 2, 5, ends[0], ends[1], op)
			v, e := s.BoundMoments(ctx, o)
			if e != nil {
				t.Fatal(op, e)
			}
			R, rr := 5.0, 2.0
			V := 4 * math.Pi * (R*math.Pi*rr*rr/4 - math.Pow(rr, 3)/3)
			X := 2 * math.Pi * (math.Pow(R, 3)*math.Pi*rr*rr/4 - R*R*math.Pow(rr, 3) + 3*R*math.Pi*math.Pow(rr, 4)/16 - 2*math.Pow(rr, 5)/15)
			Z := 4 * math.Pi / 3 * (R*3*math.Pi*math.Pow(rr, 4)/16 - math.Pow(rr, 5)/5)
			F := 0.0
			tv := 2 * math.Pi * math.Pi * R * rr * rr
			tx := tv * (R*R/2 + 3*rr*rr/8)
			tz := tv * rr * rr / 4
			cv := math.Pi * 25 * (ends[1] - ends[0])
			cx := cv * 25 / 4
			cz := cv * (ends[1]*ends[1] + ends[1]*ends[0] + ends[0]*ends[0]) / 3
			if op == TorusCylinderDifference {
				V = tv - V
				X = tx - X
				Z = tz - Z
			}
			if op == TorusCylinderUnion {
				V = tv + cv - V
				X = tx + cx - X
				Z = tz + cz - Z
				F = cv * (ends[0] + ends[1]) / 2
			}
			check := func(bounds [2]*big.Rat, x float64) {
				t.Helper()
				lo, _ := bounds[0].Float64()
				hi, _ := bounds[1].Float64()
				if x < lo-1e-9 || x > hi+1e-9 {
					t.Fatal("analytic moment", op, lo, x, hi)
				}
			}
			check(v.Volume, V)
			check(v.SecondXY, X)
			check(v.SecondZ, Z)
			check(v.FirstZ, F)
			check(v.CentroidZ, F/V)
			for i := 0; i < 2; i++ {
				check(v.InertiaOrigin[i], X+Z)
				check(v.InertiaCentroid[i], X+Z-F*F/V)
			}
			check(v.InertiaOrigin[2], 2*X)
			check(v.InertiaCentroid[2], 2*X)
			t.Log(op, ends, v.Cells, v.ExactOperations)
		}
	}
	s, _ := NewTorusCylinderMaterial(5, 2, 6, -3, 4, TorusCylinderUnion)
	bad := o
	bad.MaxCells = 1
	if v, e := s.BoundMoments(ctx, bad); e == nil || !reflect.DeepEqual(v, TorusCylinderMoments{}) {
		t.Fatal("cell budget", v, e)
	}
	good, e := s.BoundMoments(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	bad = o
	bad.Arithmetic.MaxOperations = good.ExactOperations - 1
	if v, e := s.BoundMoments(ctx, bad); e == nil || !reflect.DeepEqual(v, TorusCylinderMoments{}) {
		t.Fatal("work budget", e)
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if v, e := s.BoundMoments(c, o); e == nil || !reflect.DeepEqual(v, TorusCylinderMoments{}) {
		t.Fatal("cancel")
	}
}
