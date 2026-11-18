//go:build research

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestNativeObliqueTorusCavityArea(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	pose := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(1, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(1, 8)}}
	s, _, e := ObliqueTorusCavity(ctx, 3, 1, 8, -6, 6, pose, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := geom.TorusCylinderAreaOptions{AreaTolerance: r(1, 1000000000)}
	a, e := BoundObliqueTorusCavityArea(ctx, s, 3, 1, 8, -6, 6, pose, o)
	if e != nil {
		t.Fatal(e)
	}
	for i, want := range []float64{12 * math.Pi * math.Pi, 192 * math.Pi, 64 * math.Pi, 64 * math.Pi} {
		lo, _ := a.BySupport[i][0].Float64()
		hi, _ := a.BySupport[i][1].Float64()
		if want < lo-1e-12 || want > hi+1e-12 {
			t.Fatal(i, lo, hi, want)
		}
	}
	if len(a.SupportFaces[0]) != 16 || a.MaterialNormalSigns != [4]int{-1, 1, 1, 1} {
		t.Fatal("owners")
	}
	wrong := pose
	wrong[0][3] = r(1, 5)
	if _, e := BoundObliqueTorusCavityArea(ctx, s, 3, 1, 8, -6, 6, wrong, o); e == nil {
		t.Fatal("different exact source")
	}
	o.Arithmetic.MaxOperations = 1
	if result, e := BoundObliqueTorusCavityArea(ctx, s, 3, 1, 8, -6, 6, pose, o); e == nil || result.Lower != nil {
		t.Fatal("atomic budget")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := BoundObliqueTorusCavityArea(canceled, s, 3, 1, 8, -6, 6, pose, geom.TorusCylinderAreaOptions{AreaTolerance: r(1, 100)}); e == nil {
		t.Fatal("cancellation")
	}
}
