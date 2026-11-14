package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestObliqueCavityAreaLimits(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	pose := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	s, e := NewObliqueCylinderTorusCavityMaterial(ctx, 3, 1, 8, -6, 6, pose, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []TorusCylinderAreaOptions{{AreaTolerance: r(1, 100), Arithmetic: SurfacePatchOptions{MaxOperations: 1}}, {AreaTolerance: new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 512)), PrecisionBits: 16}} {
		a, e := s.BoundArea(ctx, o)
		if e == nil || a.Lower != nil {
			t.Fatal("atomic precision/budget")
		}
	}
	a, e := s.BoundArea(ctx, TorusCylinderAreaOptions{AreaTolerance: r(1, 1000000)})
	if e != nil {
		t.Fatal(e)
	}
	if a.ExactOperations == 0 || new(big.Rat).Sub(a.Upper, a.Lower).Cmp(r(1, 1000000)) > 0 {
		t.Fatal("bound")
	}
	a.BySupport[0][0].SetInt64(-1)
	b, e := s.BoundArea(ctx, TorusCylinderAreaOptions{AreaTolerance: r(1, 1000000)})
	if e != nil || b.BySupport[0][0].Sign() <= 0 {
		t.Fatal("ownership", e)
	}
}
