package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetNormalBoundsPreserveContinuousOrientation(t *testing.T) {
	ctx := context.Background()
	p := normalOffsetParabolicCylinder(t, false)
	rect := ParameterRectangle{UMin: .625, UMax: .75, VMin: .25, VMax: .75}
	check := func(d *big.Rat) RationalSurfaceOffsetPoint {
		t.Helper()
		o, _, err := NewVerifiedNormalOffsetPatch(ctx, p, d, NormalOffsetProjection{Axes: [2]int{0, 1}}, NormalOffsetCoverageOptions{})
		if err != nil {
			t.Fatal(err)
		}
		box, err := o.NormalBoundsRational(ctx, rect, SurfacePatchOptions{})
		if err != nil || box.Upper[0].Sign() >= 0 || box.Lower[2].Sign() <= 0 ||
			box.Lower[1].Sign() != 0 || box.Upper[1].Sign() != 0 {
			t.Fatal("normal enclosure lost its original orientation", box, err)
		}
		for i := range 17 {
			parameter := rect.UMin + (rect.UMax-rect.UMin)*float64(i)/16
			x := new(big.Rat).SetFloat64(2*parameter - 1)
			n := new(big.Rat).Mul(big.NewRat(-2, 1), x)
			radical := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(n, n))
			for _, v := range []struct {
				axis int
				num  *big.Rat
			}{{0, n}, {2, big.NewRat(1, 1)}} {
				base := new(big.Rat)
				if compareExactOffsetCoordinate(base, v.num, radical, box.Lower[v.axis]) < 0 ||
					compareExactOffsetCoordinate(base, v.num, radical, box.Upper[v.axis]) > 0 {
					t.Fatal("continuous source normal escaped directed rational interval", parameter, v.axis, box)
				}
			}
		}
		short, err := o.NormalBoundsRational(ctx, rect, SurfacePatchOptions{MaxOperations: box.ExactOperations - 1})
		if !errors.Is(err, ErrConvergence) || short.Lower[0] != nil || short.Upper[0] != nil {
			t.Fatal("short work must not publish a partial normal hull", short, err)
		}
		return box
	}
	positive, negative := check(big.NewRat(1, 4)), check(big.NewRat(-1, 4))
	for axis := range 3 {
		if positive.Lower[axis].Cmp(negative.Lower[axis]) != 0 || positive.Upper[axis].Cmp(negative.Upper[axis]) != 0 {
			t.Fatal("focal-free signed displacement changed the original normal direction", axis)
		}
	}
}
