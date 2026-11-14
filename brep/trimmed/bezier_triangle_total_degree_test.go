package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestExactSexticTriangleTotalDegree(t *testing.T) {
	ctx := context.Background()
	for _, mixed := range []bool{false, true} {
		var h [][4]*big.Rat
		for i := 0; i <= 6; i++ {
			for j := 0; j <= 6; j++ {
				z := int64(0)
				if mixed {
					if i == 6 && j == 6 {
						z = 1
					}
				} else {
					if i == 6 {
						z++
					}
					if j == 6 {
						z++
					}
				}
				h = append(h, [4]*big.Rat{big.NewRat(int64(i), 6), big.NewRat(int64(j), 6), big.NewRat(z, 1), big.NewRat(1, 1)})
			}
		}
		p, e := geom.NewExactHomogeneousBezierSurface(ctx, 6, 6, h, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		uv := [3][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(0, 1)}, {big.NewRat(0, 1), big.NewRat(1, 1)}}
		var model [3][3]*big.Rat
		for i, q := range uv {
			jet, e := p.EvaluateRational(ctx, q[0], q[1], geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			model[i] = jet.Point
		}
		b, e := BoundRationalBezierTriangleDeviation(ctx, p, uv, model, BezierTriangleBoundOptions{MaxDegree: 6})
		if e != nil {
			t.Fatal(e)
		}
		// For u^6+v^6 minus its linear interpolant, degree-six Bernstein
		// coefficients lie in [-1,0] and include -1. For u^6*v^6 the sole
		// nonzero degree-twelve coefficient is (6!6!)/12! = 1/924.
		want := big.NewRat(1, 1)
		if mixed {
			want = big.NewRat(1, 924*924)
		}
		if !b.BothDirections || b.SquaredDistance.Cmp(want) != 0 {
			t.Fatal("independent triangular coefficients", mixed, b.SquaredDistance, want)
		}
		if failed, e := BoundRationalBezierTriangleDeviation(ctx, p, uv, model, BezierTriangleBoundOptions{MaxDegree: 6, MaxOperations: b.Operations - 1}); e == nil || failed.BothDirections || failed.SquaredDistance != nil {
			t.Fatal("atomic exact work deficit", e)
		}
		if _, e := BoundRationalBezierTriangleDeviation(ctx, p, uv, model, BezierTriangleBoundOptions{MaxDegree: 6, MaxOperations: b.Operations}); e != nil {
			t.Fatal("exact work", e)
		}
		t.Log("mixed total-degree12", mixed, "operations", b.Operations)
	}
}
