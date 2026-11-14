package geom

import (
	"context"
	"math"
	"math/big"
	"reflect"
	"testing"
)

func TestTorusCylinderVolume(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, c := range []float64{5, 6} {
		previous := math.Inf(1)
		for _, tolerance := range []*big.Rat{r(1, 10), r(1, 100)} {
			for _, op := range []TorusCylinderOperation{TorusCylinderCommon, TorusCylinderDifference, TorusCylinderUnion} {
				s, _ := NewTorusCylinderMaterial(5, 2, c, -3, 4, op)
				v, e := s.BoundVolume(ctx, TorusCylinderVolumeOptions{VolumeTolerance: tolerance})
				if e != nil {
					t.Fatal(c, op, e)
				}
				a := c - 5
				common := 4 * math.Pi * (5*(a*math.Sqrt(4-a*a)/2+2*math.Asin(a/2)+math.Pi) - math.Pow(4-a*a, 1.5)/3)
				truth := common
				if op == TorusCylinderDifference {
					truth = 40*math.Pi*math.Pi - common
				}
				if op == TorusCylinderUnion {
					truth = 40*math.Pi*math.Pi + 7*c*c*math.Pi - common
				}
				lo, _ := v.Lower.Float64()
				hi, _ := v.Upper.Float64()
				if lo > truth || hi < truth || new(big.Rat).Sub(v.Upper, v.Lower).Cmp(tolerance) > 0 {
					t.Fatal("analytic integral", c, op, lo, truth, hi)
				}
				if op == TorusCylinderCommon {
					if hi-lo >= previous {
						t.Fatal("refinement not tighter")
					}
					previous = hi - lo
				}
				t.Log(c, op, v.Cells, v.ExactOperations)
			}
		}
	}
	s, _ := NewTorusCylinderMaterial(5, 2, 6, -3, 4, TorusCylinderUnion)
	for _, o := range []TorusCylinderVolumeOptions{{VolumeTolerance: r(1, 100), MaxCells: 1}, {VolumeTolerance: r(1, 100), Arithmetic: SurfacePatchOptions{MaxOperations: 1}}, {VolumeTolerance: r(0, 1)}, {VolumeTolerance: r(1, 100), SqrtBits: 1}} {
		v, e := s.BoundVolume(ctx, o)
		if e == nil || !reflect.DeepEqual(v, TorusCylinderVolume{}) {
			t.Fatal("atomic limit", v, e)
		}
	}
	good, e := s.BoundVolume(ctx, TorusCylinderVolumeOptions{VolumeTolerance: r(1, 10)})
	if e != nil {
		t.Fatal(e)
	}
	failed, e := s.BoundVolume(ctx, TorusCylinderVolumeOptions{VolumeTolerance: r(1, 10), Arithmetic: SurfacePatchOptions{MaxOperations: good.ExactOperations - 1}})
	if e == nil || !reflect.DeepEqual(failed, TorusCylinderVolume{}) {
		t.Fatal("aggregate operation limit", failed, e)
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	v, e := s.BoundVolume(c, TorusCylinderVolumeOptions{VolumeTolerance: r(1, 100)})
	if e == nil || !reflect.DeepEqual(v, TorusCylinderVolume{}) {
		t.Fatal("cancel")
	}
}
