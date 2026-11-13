package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSameSideSphereCylinderSlab(t *testing.T) {
	r := big.NewRat
	for _, c := range []struct {
		axis       Vec3
		caps       [2]*big.Rat
		near, side int
	}{
		{Vec3{Z: 3}, [2]*big.Rat{r(1, 3), r(1, 2)}, 0, 1},
		{Vec3{Z: 3}, [2]*big.Rat{r(-1, 2), r(-1, 3)}, 1, -1},
		{Vec3{Z: -3}, [2]*big.Rat{r(1, 3), r(1, 2)}, 0, 1},
	} {
		q, e := ClassifySameSideSphereCylinderSlab(context.Background(), Vec3{X: 1}, 2, Vec3{}, c.axis, 1, c.caps, SurfacePatchEvaluationOptions{})
		if e != nil || !q.Complete || q.NearCap != c.near || q.Side != c.side || len(q.Windows) != 4 || len(q.SphereWall.Components) != 2 {
			t.Fatal(c, q, e)
		}
		count := [3]int{}
		for _, w := range q.Windows {
			count[w.Kind]++
		}
		if count != [3]int{1, 1, 2} {
			t.Fatal(count)
		}
		near, far := 0, 0
		for _, s := range q.Stations {
			if s.NearCapEvent {
				near++
			}
			if s.FarCapEvent {
				far++
			}
		}
		if near != 2 || far != 2 {
			t.Fatal(q.Stations)
		}
		// At ray +X, sphere height 2 exceeds both cap heights1,1.5,
		// so full wall remains. At -X it is zero: no wall above either cap. In between,
		// projection0 yields height sqrt(2), strictly between the two caps.
		for _, s := range q.Arrangement.Sectors {
			if s.InsideCap == [2]bool{true, true} && q.Windows[s.Start].Kind != SphereCylinderWallCapToCap {
				t.Fatal(q)
			}
		}
	}
	q, e := ClassifySameSideSphereCylinderSlab(context.Background(), Vec3{X: 1}, 2, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(-1, 1), r(1, 1)}, SurfacePatchEvaluationOptions{})
	if e != ErrDomain || q.Complete {
		t.Fatal(q, e)
	}
	q, e = ClassifySameSideSphereCylinderSlab(context.Background(), Vec3{X: 1}, 2, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(1, 1), r(3, 2)}, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete || len(q.Windows) != 0 {
		t.Fatal(q, e)
	}
}
