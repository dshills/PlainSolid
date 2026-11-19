package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestNativeObliqueCavityNormalsGuards(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := big.NewRat
	frame := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	source, e := geom.NewObliqueCylinderTorusCavityMaterial(context.Background(), 3, 1, 8, -6, 6, frame, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	point := [3]*big.Rat{r(17, 5), r(0, 1), r(-16, 5)}
	if out, e := source.BoundaryUnitNormalsRational(ctx, point, geom.SurfacePatchOptions{}); !errors.Is(e, context.Canceled) || out.Owners != 0 {
		t.Fatal("cancel atomic", e)
	}
	first, e := source.BoundaryUnitNormalsRational(context.Background(), point, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	first.Units[0][0].SetInt64(999)
	second, e := source.BoundaryUnitNormalsRational(context.Background(), point, geom.SurfacePatchOptions{})
	if e != nil || second.Units[0][0].Cmp(r(-3, 5)) != 0 {
		t.Fatal("owned output", e)
	}
	point[0] = new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 1000))
	if out, e := source.BoundaryUnitNormalsRational(context.Background(), point, geom.SurfacePatchOptions{MaxCoefficientBits: 128}); e == nil || out.Owners != 0 {
		t.Fatal("coefficient atomic")
	}
}
