package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestAffineXYChartAlignment(t *testing.T) {
	ctx := context.Background()
	makePatch := func(target bool) *SurfacePatch {
		p := make([]Vec3, 4)
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				u, v := float64(i), float64(j)
				if target {
					u, v = 1-v, u
				}
				z := 2.
				if target {
					z = 0
				}
				p[2*i+j] = Vec3{u, v, z}
			}
		}
		ku, kv := []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}
		if target {
			ku = []float64{2, 2, 5, 5}
			kv = []float64{-3, -3, 3, 3}
		}
		s, e := NewBSplineSurface(1, 1, 2, 2, p, ku, kv, nil)
		if e != nil {
			t.Fatal(e)
		}
		r, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	a, b := makePatch(false), makePatch(true)
	alignment, e := AlignAffineXYCharts(ctx, a, b, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, _ := alignment.MapCoefficients()
	if m[0][1].Cmp(big.NewRat(3, 1)) != 0 || m[0][2].Cmp(big.NewRat(2, 1)) != 0 || m[1][0].Cmp(big.NewRat(-6, 1)) != 0 || m[1][2].Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal(m)
	}
	jet, e := alignment.TargetOnSource().EvaluateRational(ctx, big.NewRat(1, 3), big.NewRat(2, 5), SurfacePatchEvaluationOptions{})
	if e != nil || jet.Point[0].Cmp(big.NewRat(1, 3)) != 0 || jet.Point[1].Cmp(big.NewRat(2, 5)) != 0 {
		t.Fatal(jet, e)
	}
	r, e := alignment.HeightRange(ctx, GeneralSurfaceHeightOptions{})
	if e != nil || r.StrictSign != 1 || r.Lower.Cmp(big.NewRat(2, 1)) != 0 || r.Upper.Cmp(r.Lower) != 0 {
		t.Fatal(r, e)
	}
	// Inverse direction has nonbinary scales 1/3 and -1/6, exactly retained.
	reverse, e := AlignAffineXYCharts(ctx, b, a, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = reverse.HeightRange(ctx, GeneralSurfaceHeightOptions{})
	if e != nil || r.StrictSign != -1 {
		t.Fatal(r, e)
	}
	short, e := b.Trim(ParameterRectangle{2, 4, -3, 3})
	if e != nil {
		t.Fatal(e)
	}
	if result, work, e := AlignAffineXYChartsWithWork(ctx, a, short, SurfacePatchOptions{}); result != nil || work < 1 || !errors.Is(e, ErrDomain) {
		t.Fatal("domain mismatch", result, work, e)
	}
	if _, e = AlignAffineXYCharts(ctx, a, b, SurfacePatchOptions{MaxOperations: 1}); e == nil {
		t.Fatal("limit")
	}
	shear := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u + v, v, 0} })
	if _, e := AlignAffineXYCharts(ctx, a, shear, SurfacePatchOptions{}); e == nil {
		t.Fatal("accepted shear rectangle")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := AlignAffineXYCharts(cancelled, a, b, SurfacePatchOptions{}); e == nil {
		t.Fatal("cancel")
	}

}
