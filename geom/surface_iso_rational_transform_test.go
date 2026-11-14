package geom

import (
	"context"
	"math/big"
	"reflect"
	"testing"
)

func TestExactCurveRationalTransform(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	controls := [][4]*big.Rat{{r(0, 1), r(0, 1), r(1, 3), r(1, 1)}, {r(1, 2), r(3, 4), r(1, 4), r(3, 4)}, {r(1, 1), r(1, 1), r(1, 3), r(1, 1)}, {r(6, 7), r(3, 7), r(1, 7), r(3, 7)}, {r(3, 1), r(0, 1), r(1, 3), r(1, 1)}}
	source, e := NewExactHomogeneousSpline(ctx, 2, controls, []float64{2, 2, 2, 5, 5, 9, 9, 9}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 3)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 7)}, {r(-4, 5), r(0, 1), r(3, 5), r(5, 11)}}
	mapped, e := source.TransformRational(ctx, m, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(mapped.patch.knotsU, source.patch.knotsU) || mapped.patch.degreeU != 2 {
		t.Fatal("changed original basis")
	}
	for _, u := range []*big.Rat{r(2, 1), r(11, 3), r(5, 1), r(17, 3), r(9, 1)} {
		a, e := source.patch.EvaluateRational(ctx, u, r(0, 1), SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := mapped.patch.EvaluateRational(ctx, u, r(0, 1), SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for axis := 0; axis < 3; axis++ {
			p, d := new(big.Rat).Set(m[axis][3]), new(big.Rat)
			for j := 0; j < 3; j++ {
				p.Add(p, new(big.Rat).Mul(m[axis][j], a.Point[j]))
				d.Add(d, new(big.Rat).Mul(m[axis][j], a.Du[j]))
			}
			if p.Cmp(b.Point[axis]) != 0 || d.Cmp(b.Du[axis]) != 0 {
				t.Fatal("exact nonbinary point/tangent", u, axis)
			}
		}
	}
	var inverse [3][4]*big.Rat
	for i := range inverse {
		inverse[i][3] = new(big.Rat)
		for j := 0; j < 3; j++ {
			inverse[i][j] = new(big.Rat).Set(m[j][i])
			inverse[i][3].Sub(inverse[i][3], new(big.Rat).Mul(m[j][i], m[j][3]))
		}
	}
	restored, e := mapped.TransformRational(ctx, inverse, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	same, e := CompareExactCurvesContext(ctx, source, restored, ExactCurveComparisonOptions{})
	if e != nil || !same.Complete || !same.Equal {
		t.Fatal("full-domain exact inverse", same, e)
	}
	var collapse [3][4]*big.Rat
	for i := range collapse {
		for j := range collapse[i] {
			collapse[i][j] = new(big.Rat)
		}
		collapse[i][3] = r(int64(i+1), 7)
	}
	constant, e := source.TransformRational(ctx, collapse, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	jet, e := constant.patch.EvaluateRational(ctx, r(11, 3), r(0, 1), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := range jet.Point {
		if jet.Point[i].Cmp(collapse[i][3]) != 0 || jet.Du[i].Sign() != 0 {
			t.Fatal("exact singular image")
		}
	}
	collapse[0][3] = new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	if got, e := source.TransformRational(ctx, collapse, SurfacePatchOptions{MaxCoefficientBits: 64}); e == nil || got != nil {
		t.Fatal("coefficient quota")
	}
	saved := new(big.Rat).Set(mapped.patch.net[0][0])
	m[0][3].SetInt64(99)
	controls[0][0].SetInt64(99)
	if mapped.patch.net[0][0].Cmp(saved) != 0 {
		t.Fatal("matrix/source alias")
	}
	for _, limits := range []SurfacePatchOptions{{MaxOperations: 1}, {MaxPoles: 4}} {
		if got, e := source.TransformRational(ctx, m, limits); e == nil || got != nil {
			t.Fatal("atomic quota", e)
		}
	}
	m[0][0] = nil
	if got, e := source.TransformRational(ctx, m, SurfacePatchOptions{}); e == nil || got != nil {
		t.Fatal("nil coefficient")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := source.TransformRational(cancelled, inverse, SurfacePatchOptions{}); e == nil || got != nil {
		t.Fatal("cancel")
	}
}
