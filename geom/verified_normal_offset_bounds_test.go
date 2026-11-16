package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetBoundsEncloseWholeRestrictedPhysicalCell(t *testing.T) {
	ctx := context.Background()
	source := normalOffsetParabolicCylinder(t, false)
	xy := NormalOffsetProjection{Axes: [2]int{0, 1}}
	cell := ParameterRectangle{UMin: .625, UMax: .75, VMin: .25, VMax: .75}
	for _, distance := range []float64{.25, -.25} {
		surface, _, err := NewVerifiedNormalOffsetPatch(ctx, source, new(big.Rat).SetFloat64(distance), xy, NormalOffsetCoverageOptions{})
		if err != nil || surface.Projection() != xy {
			t.Fatal("unable to construct the verified analytic offset", err)
		}
		b, err := surface.BoundsRational(ctx, cell, SurfacePatchOptions{})
		if err != nil || b.Lower[1].Cmp(big.NewRat(1, 4)) != 0 || b.Upper[1].Cmp(big.NewRat(3, 4)) != 0 {
			t.Fatal("restriction lost the exact axial bounds", b, err)
		}
		// Compare the analytic algebraic coordinates with the rational
		// bounds without rounding square roots or subtracting floats.
		for i := range 17 {
			u := cell.UMin + (cell.UMax-cell.UMin)*float64(i)/16
			x := new(big.Rat).SetFloat64(2*u - 1)
			z := new(big.Rat).Mul(x, x)
			d := new(big.Rat).SetFloat64(distance)
			radical := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(big.NewRat(4, 1), z))
			xx := new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(-2, 1), x), d)
			for _, c := range []struct {
				base, numerator *big.Rat
				axis            int
			}{{x, xx, 0}, {z, d, 2}} {
				if compareExactOffsetCoordinate(c.base, c.numerator, radical, b.Lower[c.axis]) < 0 ||
					compareExactOffsetCoordinate(c.base, c.numerator, radical, b.Upper[c.axis]) > 0 {
					t.Fatal("directed cell lost an exact algebraic coordinate", u, c.axis, b)
				}
			}
			for j := range 17 {
				yy := cell.VMin + (cell.VMax-cell.VMin)*float64(j)/16
				if yy < b.Bounds.Min.Y || yy > b.Bounds.Max.Y {
					t.Fatal("directed cell lost an exact axial coordinate", u, yy, b)
				}
			}
		}
		short, err := surface.BoundsRational(ctx, cell, SurfacePatchOptions{MaxOperations: b.ExactOperations - 1})
		if !errors.Is(err, ErrConvergence) || short.Lower[0] != nil || short.Upper[0] != nil {
			t.Fatal("insufficient work cannot publish an offset hull", short, err)
		}
		outside := cell
		outside.VMin = -.25
		if _, err := surface.BoundsRational(ctx, outside, SurfacePatchOptions{}); !errors.Is(err, ErrDomain) {
			t.Fatal("offset bounds clipped an invalid original domain", err)
		}
	}
}

// Compare base + numerator/sqrt(radical) with the rational bound.
func compareExactOffsetCoordinate(base, numerator, radical, bound *big.Rat) int {
	d := new(big.Rat).Sub(bound, base)
	if d.Sign() == 0 || numerator.Sign() == 0 || d.Sign() != numerator.Sign() {
		return numerator.Cmp(d)
	}
	rl := new(big.Rat).Mul(d, d)
	rl.Mul(rl, radical)
	num := new(big.Rat).Mul(numerator, numerator)
	return numerator.Sign() * num.Cmp(rl)
}
