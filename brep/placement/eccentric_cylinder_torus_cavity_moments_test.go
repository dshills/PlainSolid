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

func TestEccentricCylinderTorusCavityMoments(t *testing.T) {
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
		q, _ := geom.NewEccentricCylinderTorusCavityMaterial(5, 2, 10, lo, hi, 1, -1)
		local, e := q.BoundMoments(ctx, o)
		if e != nil {
			t.Fatal(e)
		}
		if local.VolumeCoefficients[0].Cmp(r(700, 1)) != 0 || local.VolumeCoefficients[1].Cmp(r(-40, 1)) != 0 || local.FirstCoefficients[0][1].Cmp(r(-40, 1)) != 0 || local.FirstCoefficients[1][1].Cmp(r(40, 1)) != 0 || local.SecondCoefficients[0][1][1].Cmp(r(40, 1)) != 0 || local.SecondCoefficients[0][0][1].Cmp(r(-600, 1)) != 0 {
			t.Fatal("translated coefficients", local)
		}
		world, e := BoundEccentricCylinderTorusCavityMoments(ctx, q, pose, TorusCylinderOccurrenceMomentOptions{Source: o})
		if e != nil {
			t.Fatal(e)
		}
		if oracle := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_CAVITY_MOMENTS"); oracle != "" {
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
		if _, e = BoundEccentricCylinderTorusCavityMoments(ctx, q, pose, TorusCylinderOccurrenceMomentOptions{Source: o, MaxTransformOperations: 1}); e == nil {
			t.Fatal("mapping budget")
		}
	}
}
