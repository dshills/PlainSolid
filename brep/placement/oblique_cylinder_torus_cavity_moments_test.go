package placement

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestObliqueCavityWorldMoments(t *testing.T) {
	r := big.NewRat
	ctx := context.Background()
	local := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	s, e := geom.NewObliqueCylinderTorusCavityMaterial(ctx, 3, 1, 8, -6, 6, local, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pose, _ := NewRationalRigid([3][4]*big.Rat{{r(1, 1), r(0, 1), r(0, 1), r(2, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(0, 1), r(1, 1), r(3, 1)}}, 4096)
	tol := r(1, 1000000)
	o := TorusCylinderOccurrenceMomentOptions{Source: geom.TorusCylinderMomentOptions{VolumeTolerance: tol, SecondMomentTolerance: tol, CentroidTolerance: tol, InertiaTolerance: tol}}
	v, e := BoundObliqueCylinderTorusCavityMoments(ctx, s, pose, o)
	if e != nil {
		t.Fatal(e)
	}
	want := (333./25)*math.Pi*math.Pi - 18*math.Pi*math.Pi + 6*(768*math.Pi-6*math.Pi*math.Pi)
	lo, _ := v.Second[0][2][0].Float64()
	hi, _ := v.Second[0][2][1].Float64()
	if want < lo-1e-8 || want > hi+1e-8 {
		t.Fatal(want, lo, hi)
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 2; k++ {
				if v.InertiaCentroid[i][j][k].Cmp(v.Source.InertiaCentroid[i][j][k]) != 0 {
					t.Fatal("translation invariant")
				}
			}
		}
	}
	o.MaxTransformOperations = 1
	if _, e = BoundObliqueCylinderTorusCavityMoments(ctx, s, pose, o); e == nil {
		t.Fatal("budget")
	}
}
