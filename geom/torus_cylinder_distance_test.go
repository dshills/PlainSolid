package geom

import (
	"context"
	"math"
	"math/big"
	"reflect"
	"testing"
)

func TestTorusCylinderDistance(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, tc := range []struct {
		op    TorusCylinderOperation
		p     [3]*big.Rat
		truth float64
		owner BoreBoundaryMask
	}{{TorusCylinderCommon, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, 9, 1}, {TorusCylinderDifference, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, 36, 2}, {TorusCylinderUnion, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, 9, 4}, {TorusCylinderCommon, [3]*big.Rat{r(1, 1), r(1, 1), r(0, 1)}, math.Pow(3-math.Sqrt(2), 2), 1}, {TorusCylinderCommon, [3]*big.Rat{r(6, 1), r(0, 1), r(0, 1)}, 0, 2}, {TorusCylinderUnion, [3]*big.Rat{r(7, 1), r(0, 1), r(0, 1)}, 0, 1}, {TorusCylinderUnion, [3]*big.Rat{r(0, 1), r(0, 1), r(5, 1)}, 1, 8}} {
		s, _ := NewTorusCylinderMaterial(5, 2, 6, -3, 4, tc.op)
		v, e := s.BoundBoundaryDistance(ctx, tc.p, TorusCylinderDistanceOptions{SquaredTolerance: r(1, 1000)})
		if e != nil {
			t.Fatal(tc, e)
		}
		lo, _ := v.Lower.Float64()
		hi, _ := v.Upper.Float64()
		if tc.truth < lo-1e-12 || tc.truth > hi+1e-12 || v.UniqueOwner != tc.owner {
			t.Fatal(tc, lo, hi, v.PossibleOwners)
		}
		for _, support := range v.Supports {
			if support.Owner == 0 {
				continue
			}
			w := support.Witness
			rho2, _ := w.RadialSquared.Float64()
			base, _ := w.HeightBase.Float64()
			rad, _ := w.HeightRadicand.Float64()
			z := base + float64(w.HeightSign)*math.Sqrt(rad)
			qx, _ := tc.p[0].Float64()
			qy, _ := tc.p[1].Float64()
			qz, _ := tc.p[2].Float64()
			d := math.Pow(math.Hypot(qx, qy)-math.Sqrt(rho2), 2) + math.Pow(qz-z, 2)
			up, _ := support.Upper.Float64()
			if d > up+1e-10 {
				t.Fatal("unattained upper", d, up)
			}
		}
		t.Log(tc.op, tc.owner, v.Cells, v.ExactOperations)
	}
	s, _ := NewTorusCylinderMaterial(5, 2, 5, -3, 4, TorusCylinderCommon)
	p := [3]*big.Rat{r(5, 1), r(0, 1), r(2, 1)}
	v, e := s.BoundBoundaryDistance(ctx, p, TorusCylinderDistanceOptions{SquaredTolerance: r(1, 1000)})
	if e != nil || v.PossibleOwners != 3 || v.UniqueOwner != 0 || v.Upper.Sign() != 0 {
		t.Fatal("rim not false unique", v, e)
	}
	for _, o := range []TorusCylinderDistanceOptions{{SquaredTolerance: r(1, 1000), MaxCells: 1}, {SquaredTolerance: r(1, 1000), Arithmetic: SurfacePatchOptions{MaxOperations: 1}}, {SquaredTolerance: r(0, 1)}} {
		g, e := s.BoundBoundaryDistance(ctx, [3]*big.Rat{r(1, 1), r(2, 1), r(3, 1)}, o)
		if e == nil || !reflect.DeepEqual(g, TorusCylinderDistance{}) {
			t.Fatal("atomic limit", g, e)
		}
	}
	q := [3]*big.Rat{r(4, 1), r(1, 1), r(3, 2)}
	coarse, e := s.BoundBoundaryDistance(ctx, q, TorusCylinderDistanceOptions{SquaredTolerance: r(1, 100)})
	if e != nil {
		t.Fatal(e)
	}
	fine, e := s.BoundBoundaryDistance(ctx, q, TorusCylinderDistanceOptions{SquaredTolerance: r(1, 10000)})
	if e != nil || fine.Lower.Cmp(coarse.Lower) < 0 || fine.Upper.Cmp(coarse.Upper) > 0 {
		t.Fatal("refinement", e)
	}
	fail, e := s.BoundBoundaryDistance(ctx, q, TorusCylinderDistanceOptions{SquaredTolerance: r(1, 10000), Arithmetic: SurfacePatchOptions{MaxOperations: fine.ExactOperations - 1}})
	if e == nil || !reflect.DeepEqual(fail, TorusCylinderDistance{}) {
		t.Fatal("cumulative budget", e)
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if v, e := s.BoundBoundaryDistance(c, p, TorusCylinderDistanceOptions{SquaredTolerance: r(1, 1000)}); e == nil || !reflect.DeepEqual(v, TorusCylinderDistance{}) {
		t.Fatal("cancel")
	}
}
