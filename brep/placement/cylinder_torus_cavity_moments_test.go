package placement

import (
	"context"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/geom"
	"strconv"
	"strings"
	"testing"
)

func TestCylinderTorusCavityMoments(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	tol := r(1, 1000000)
	o := geom.TorusCylinderMomentOptions{VolumeTolerance: tol, SecondMomentTolerance: tol, CentroidTolerance: tol, InertiaTolerance: tol}
	pose, _ := NewRationalRigid([3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}, 4096)
	for reflected := 0; reflected < 2; reflected++ {
		lo, hi := -3., 4.
		if reflected == 1 {
			lo, hi = -4, 3
		}
		q, _ := geom.NewCylinderTorusCavityMaterial(5, 2, 8, lo, hi)
		local, e := q.BoundMoments(ctx, o)
		if e != nil {
			t.Fatal(e)
		}
		want := int64(224)
		if reflected == 1 {
			want = -224
		}
		if local.Coefficients[0][0].Cmp(r(448, 1)) != 0 || local.Coefficients[0][1].Cmp(r(-40, 1)) != 0 || local.Coefficients[1][0].Cmp(r(want, 1)) != 0 || local.Coefficients[2][0].Cmp(r(7168, 1)) != 0 || local.Coefficients[2][1].Cmp(r(-560, 1)) != 0 || local.Coefficients[3][1].Cmp(r(-40, 1)) != 0 {
			t.Fatal(local.Coefficients)
		}
		world, e := BoundCylinderTorusCavityMoments(ctx, q, pose, TorusCylinderOccurrenceMomentOptions{Source: o})
		if e != nil {
			t.Fatal(e)
		}
		if oracle := os.Getenv("PLAINSOLID_OCCT_CAVITY_MOMENTS"); oracle != "" {
			raw, e := exec.Command(oracle, strconv.Itoa(reflected)).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(raw))
			}
			tokens := strings.Fields(string(raw))
			if len(tokens) != 13 {
				t.Fatal(tokens)
			}
			bounds := [][2]*big.Rat{world.Volume}
			for _, v := range world.Centroid {
				bounds = append(bounds, v)
			}
			for _, row := range world.InertiaCentroid {
				for _, v := range row {
					bounds = append(bounds, v)
				}
			}
			for i, b := range bounds {
				v, e := strconv.ParseFloat(tokens[i], 64)
				lo, _ := b[0].Float64()
				hi, _ := b[1].Float64()
				if e != nil || v < lo-1e-6 || v > hi+1e-6 {
					t.Fatal(i, v, lo, hi, e)
				}
			}
		}
		bad := o
		bad.Arithmetic.MaxOperations = 1
		if _, e = q.BoundMoments(ctx, bad); e == nil {
			t.Fatal("budget")
		}
		if _, e = BoundCylinderTorusCavityMoments(ctx, q, pose, TorusCylinderOccurrenceMomentOptions{Source: o, MaxTransformOperations: 1}); e == nil {
			t.Fatal("mapping budget")
		}
	}
}
