package geom

import (
	"context"
	"math/big"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestObliqueCylinderTorusCavityMoments(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	pose := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	s, e := NewObliqueCylinderTorusCavityMaterial(ctx, 3, 1, 8, -6, 6, pose, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	tol := r(1, 1000000)
	o := TorusCylinderMomentOptions{VolumeTolerance: tol, SecondMomentTolerance: tol, CentroidTolerance: tol, InertiaTolerance: tol}
	v, e := s.BoundMoments(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if v.VolumeCoefficients[0].Cmp(r(768, 1)) != 0 || v.VolumeCoefficients[1].Cmp(r(-6, 1)) != 0 || v.FirstCoefficients[0][1].Cmp(r(-6, 1)) != 0 || v.SecondCoefficients[0][2][1].Cmp(r(333, 25)) != 0 {
		t.Fatal("exact rotated cross moment", v.SecondCoefficients)
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if v.SecondCoefficients[i][j][1].Cmp(v.SecondCoefficients[j][i][1]) != 0 {
				t.Fatal("symmetry")
			}
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_OBLIQUE_CAVITY"); oracle != "" {
		raw, e := exec.Command(oracle).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(raw))
		}
		fields := strings.Fields(string(raw))
		if len(fields) != 14 {
			t.Fatal(fields)
		}
		compare := func(index int, b [2]*big.Rat) {
			t.Helper()
			x, e := strconv.ParseFloat(fields[index], 64)
			lo, _ := b[0].Float64()
			hi, _ := b[1].Float64()
			if e != nil || x < lo-1e-6 || x > hi+1e-6 {
				t.Fatal(index, x, lo, hi, e)
			}
		}
		compare(0, v.Volume)
		for i := 0; i < 3; i++ {
			compare(i+2, v.Centroid[i])
			for j := 0; j < 3; j++ {
				compare(5+3*i+j, v.InertiaCentroid[i][j])
			}
		}
	}
	affine := [3][4]*big.Rat{{r(1, 1), r(1, 1), r(1, 2), r(1, 3)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(0, 1), r(1, 1), r(0, 1)}}
	bounds, e := s.BoundAffineRange(ctx, affine, 64, SurfacePatchOptions{})
	if e != nil || bounds.Extrema[0][1].RadialSquared.Cmp(r(2, 1)) != 0 || bounds.Extrema[0][1].Height[0].Cmp(r(6, 1)) != 0 {
		t.Fatal(bounds, e)
	}
	bad := o
	bad.Arithmetic.MaxOperations = 1
	if _, e = s.BoundMoments(ctx, bad); e == nil {
		t.Fatal("budget")
	}
}
