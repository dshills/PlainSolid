package geom

import (
	"context"
	"math/big"
	"os"
	"os/exec"
	"testing"
)

func TestObliqueCylinderTorusCavity(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	pose := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	s, e := NewObliqueCylinderTorusCavityMaterial(ctx, 3, 1, 8, -6, 6, pose, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		t        int64
		location BorePointLocation
	}{{0, BoreInside}, {3, BoreOutside}, {4, BoreBoundary}} {
		p := [3]*big.Rat{r(5+3*tc.t, 5), r(0, 1), r(-4*tc.t, 5)}
		l, m, e := s.ClassifyRational(ctx, p, SurfacePatchOptions{})
		if e != nil || l != tc.location || (l == BoreBoundary && m != TorusCylinderTorus) {
			t.Fatal(tc, l, m, e)
		}
	}
	p := [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}
	d := [3]*big.Rat{r(3, 5), r(0, 1), r(-4, 5)}
	line, e := s.ClipRationalLine(ctx, p, d, r(-9, 1), r(9, 1), ToroidLineOptions{ParameterTolerance: 1e-9})
	if e != nil || len(line.Intervals) != 3 {
		t.Fatal(line, e)
	}
	if line.Intervals[0].StartBoundary != TorusCylinderUpperCap || line.Intervals[2].EndBoundary != TorusCylinderLowerCap {
		t.Fatal("original cap owners")
	}
	roots := []*ExactAlgebraicRoot{line.Intervals[0].Start, line.Intervals[0].End, line.Intervals[1].Start, line.Intervals[1].End, line.Intervals[2].Start, line.Intervals[2].End}
	for i, want := range []*big.Rat{r(-15, 2), r(-4, 1), r(-2, 1), r(2, 1), r(4, 1), r(15, 2)} {
		b := roots[i].Interval()
		if b.Lower.Cmp(want) > 0 || b.Upper.Cmp(want) < 0 {
			t.Fatal("section", i)
		}
	}
	owned, _ := s.TorusFrame()
	owned[0][3].SetInt64(999)
	again, _ := s.TorusFrame()
	if again[0][3].Cmp(r(1, 1)) != 0 {
		t.Fatal("ownership")
	}
	pose[0][0] = r(-3, 5)
	pose[2][0] = r(4, 5)
	if _, e = NewObliqueCylinderTorusCavityMaterial(ctx, 3, 1, 8, -6, 6, pose, SurfacePatchOptions{}); e == nil {
		t.Fatal("reflection")
	}
	pose[0][0] = r(3, 5)
	pose[2][0] = r(-4, 5)
	pose[0][3] = r(4, 1)
	if _, e = NewObliqueCylinderTorusCavityMaterial(ctx, 3, 1, 8, -6, 6, pose, SurfacePatchOptions{}); e == nil {
		t.Fatal("envelope contact")
	}
	if _, _, e = s.ClassifyRational(ctx, p, SurfacePatchOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_OBLIQUE_CAVITY"); oracle != "" {
		if log, e := exec.Command(oracle).CombinedOutput(); e != nil {
			t.Fatal(e, string(log))
		}
	}
}
