package geom

import (
	"context"
	"math"
	"math/big"
	"reflect"
	"testing"
)

func TestTorusCylinderArea(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, c := range []float64{3.25, 5, 6, 6.75} {
		for _, op := range []TorusCylinderOperation{TorusCylinderCommon, TorusCylinderDifference, TorusCylinderUnion} {
			s, _ := NewTorusCylinderMaterial(5, 2, c, -3, 4, op)
			v, e := s.BoundArea(ctx, TorusCylinderAreaOptions{AreaTolerance: r(1, 1000000)})
			if e != nil {
				t.Fatal(c, op, e)
			}
			h := math.Sqrt(4 - (c-5)*(c-5))
			common := 8 * math.Pi * (5*(math.Pi/2+math.Asin((c-5)/2)) - h)
			wall := 4 * math.Pi * c * h
			want := [4]float64{common, wall, 0, 0}
			if op != TorusCylinderCommon {
				want[0] = 40*math.Pi*math.Pi - common
			}
			if op == TorusCylinderUnion {
				want[1] = 14*math.Pi*c - wall
				want[2] = math.Pi * c * c
				want[3] = want[2]
			}
			sum := 0.0
			for i, x := range want {
				lo, _ := v.BySupport[i][0].Float64()
				hi, _ := v.BySupport[i][1].Float64()
				if x < lo-1e-10 || x > hi+1e-10 {
					t.Fatal("analytic owner area", c, op, i, lo, x, hi)
				}
				sum += x
			}
			lo, _ := v.Lower.Float64()
			hi, _ := v.Upper.Float64()
			if sum < lo-1e-10 || sum > hi+1e-10 || new(big.Rat).Sub(v.Upper, v.Lower).Cmp(r(1, 1000000)) > 0 {
				t.Fatal("total", lo, sum, hi)
			}
			if op == TorusCylinderDifference && v.MaterialNormalSigns[1] != -1 {
				t.Fatal("inward wall")
			}
			t.Log(c, op, v.ExactOperations)
		}
	}
	s, _ := NewTorusCylinderMaterial(5, 2, 6, -3, 4, TorusCylinderCommon)
	good, e := s.BoundArea(ctx, TorusCylinderAreaOptions{AreaTolerance: r(1, 1000000)})
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []TorusCylinderAreaOptions{{AreaTolerance: r(1, 1000000), Arithmetic: SurfacePatchOptions{MaxOperations: good.ExactOperations - 1}}, {AreaTolerance: r(1, 1000000000000), PrecisionBits: 16}, {AreaTolerance: r(0, 1)}} {
		v, e := s.BoundArea(ctx, o)
		if e == nil || !reflect.DeepEqual(v, TorusCylinderArea{}) {
			t.Fatal("atomic", v, e)
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	v, e := s.BoundArea(c, TorusCylinderAreaOptions{AreaTolerance: r(1, 100)})
	if e == nil || !reflect.DeepEqual(v, TorusCylinderArea{}) {
		t.Fatal("cancel")
	}
}
