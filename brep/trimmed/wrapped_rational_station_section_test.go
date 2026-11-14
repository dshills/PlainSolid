package trimmed

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"

	"plainsolid/geom"
)

func TestRationalStationExactHorizontalSections(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	bands := []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 1, 3, 1, .75), rationalStationBand(t, 1, 3, 1, 2, .75)}
	body, h, err := NewWrappedRationalStationLoft(ctx, bands, 1e-8, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		height, scale *big.Rat
		cap           int
		owners        []int
	}{
		{r(0, 1), r(3, 1), h.CapFaces[0], []int{h.WallFaces[0]}},
		{r(1, 3), r(7, 3), -1, []int{h.WallFaces[0]}},
		{r(1, 1), r(1, 1), -1, []int{h.WallFaces[0], h.WallFaces[1]}},
		{r(7, 3), r(5, 3), -1, []int{h.WallFaces[1]}},
		{r(3, 1), r(2, 1), h.CapFaces[1], []int{h.WallFaces[1]}},
	} {
		result, err := body.SectionWrappedRationalStationLoftRational(ctx, tc.height, geom.SurfacePatchOptions{})
		slices.Sort(tc.owners)
		if err != nil || result.Cap != tc.cap || result.Inner != nil || !slices.Equal(result.Outer.Faces, tc.owners) ||
			result.Outer.Section.Height.Cmp(tc.height) != 0 || result.Outer.Section.Radius.Cmp(tc.scale) != 0 {
			t.Fatal(tc.height, "wrong original section ownership or scale", result, err)
		}
		// At the first quadrant's midpoint the weighted quadratic has
		// equal normalized offsets (1/4+3/8)/(1/2+3/8) = 5/7.
		center := [2]*big.Rat{r(1, 4), r(-1, 2)}
		offset := new(big.Rat).Mul(r(5, 7), tc.scale)
		point, err := result.Outer.Section.Curve.Eval(.5)
		x, _ := new(big.Rat).Add(center[0], offset).Float64()
		y, _ := new(big.Rat).Add(center[1], offset).Float64()
		z, _ := tc.height.Float64()
		if err != nil || point.Sub(geom.Vec3{X: x, Y: y, Z: z}).Length() > 1e-14 {
			t.Fatal(tc.height, "section did not preserve the original weighted curve", point, err)
		}
	}
	if got, err := body.SectionWrappedRationalStationLoftRational(ctx, r(10, 3), geom.SurfacePatchOptions{}); !errors.Is(err, geom.ErrDomain) || got.Outer.Section.Curve != nil {
		t.Fatal("out-of-domain section leaked output", got, err)
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	if got, err := body.SectionWrappedRationalStationLoftRational(ctx, tiny, geom.SurfacePatchOptions{MaxCoefficientBits: 64}); !errors.Is(err, geom.ErrConvergence) || got.Outer.Section.Curve != nil {
		t.Fatal("section ignored its coefficient bound", got, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := body.SectionWrappedRationalStationLoftRational(cancelled, r(1, 3), geom.SurfacePatchOptions{}); !errors.Is(err, context.Canceled) || got.Outer.Section.Curve != nil {
		t.Fatal("cancellation must not return a partial section", got, err)
	}
}
