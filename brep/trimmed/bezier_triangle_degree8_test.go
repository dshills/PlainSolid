package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestExactOcticTriangleResidual(t *testing.T) {
	ctx := context.Background()
	for _, rational := range []bool{false, true} {
		var controls [][4]*big.Rat
		for i := 0; i <= 8; i++ {
			for j := 0; j <= 8; j++ {
				z, w := int64(0), int64(1)
				if i == 8 && j == 8 {
					z = 1
					if rational {
						w = 2
					}
				}
				controls = append(controls, [4]*big.Rat{big.NewRat(int64(i), 8), big.NewRat(int64(j), 8), big.NewRat(z, 1), big.NewRat(w, 1)})
			}
		}
		p, e := geom.NewExactHomogeneousBezierSurface(ctx, 8, 8, controls, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		uv := [3][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(0, 1)}, {big.NewRat(0, 1), big.NewRat(1, 1)}}
		model := [3][3]*big.Rat{{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(0, 1)}, {big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(0, 1)}}
		b, e := BoundRationalBezierTriangleDeviation(ctx, p, uv, model, BezierTriangleBoundOptions{MaxDegree: 8})
		if e != nil {
			t.Fatal(e)
		}
		// Pure u^8 v^8 has degree16 control 1/C(16,8). With W=1+u^8v^8,
		// Hxyz-W*T has degree17 and maximal controls (minusX,0,Z) or
		// (0,minusY,Z), each nonzero magnitude 1/C(17,8).
		want := big.NewRat(1, 12870*12870)
		if rational {
			want = big.NewRat(2, 24310*24310)
		}
		if !b.BothDirections || b.SquaredDistance.Cmp(want) != 0 {
			t.Fatal("independent degree16/17 coefficient", rational, b.SquaredDistance, want)
		}
		for _, o := range []BezierTriangleBoundOptions{{MaxDegree: 6}, {MaxDegree: 9}, {MaxDegree: 8, MaxOperations: b.Operations - 1}} {
			failed, e := BoundRationalBezierTriangleDeviation(ctx, p, uv, model, o)
			if e == nil || failed.BothDirections || failed.SquaredDistance != nil {
				t.Fatal("atomic degree/work gate", e)
			}
		}
		span, e := BoundMultiSpanSplineTriangleDeviation(ctx, p,
			[3]geom.Vec2{{}, {X: 1}, {Y: 1}}, [3]geom.Vec3{{}, {X: 1}, {Y: 1}},
			MultiSpanTriangleBoundOptions{MaxDegree: 8})
		if e != nil || !span.BothDirections || span.SquaredDistance.Cmp(want) != 0 {
			t.Fatal("original span degree8 dispatch", span, e)
		}
		work, e := newTubeLensSpanWork(MultiSpanTubeLensMeshOptions{Span: MultiSpanTriangleBoundOptions{MaxDegree: 8}, MaxResidualOperations: 1})
		if e != nil {
			t.Fatal(e)
		}
		if l2, _, e := work.lipschitz(ctx, p); e != nil || l2.Sign() <= 0 {
			t.Fatal("original Jacobian", e)
		}
		if _, e := work.bound(ctx, p, uv, [3]geom.Vec3{{}, {X: 1}, {Y: 1}}); e == nil {
			t.Fatal("cumulative residual quota")
		}
		t.Log("rational", rational, "operations", b.Operations)
	}
}
