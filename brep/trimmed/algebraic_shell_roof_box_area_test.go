package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func checkWeightedRoofStrictSurfaceArea(t *testing.T, ctx context.Context, cut, common *MixedRoofMaterialQuery) {
	t.Helper()
	opt := MixedRoofAreaOptions{}
	for i, q := range []*MixedRoofMaterialQuery{common, cut} {
		r, err := q.BoundStrictSurfaceArea(ctx, opt)
		if err != nil || len(r.Faces) != 12 {
			t.Fatal("strict original retained areas", i, r, err)
		}
		occt := new(big.Rat).SetFloat64([2]float64{3.3923117286024835, 12.284286477470193}[i])
		lo, _ := r.Area.Lower.Float64()
		hi, _ := r.Area.Upper.Float64()
		value, _ := occt.Float64()
		if math.Max(lo-value, value-hi) > 1e-11 {
			t.Fatal("independent OCCT area lies outside the exact hull", r.Area, occt)
		}
		for j, owner := range r.Faces {
			if owner.Owner != (MixedRoofFaceOwner{j / 6, j % 6}) || owner.Retained != (i == 1 || j >= 6) || owner.Reversed != (i == 1 && j >= 6) ||
				(owner.Area.Upper.Sign() > 0) != owner.Retained {
				t.Fatal("wrong retained face, direction or area", i, owner)
			}
		}
		if i == 0 {
			pose, err := placement.NewRationalRigid([3][4]*big.Rat{
				{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(-1, 1), big.NewRat(2, 1)},
				{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(0, 1)},
				{big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(0, 1)},
			}, 256)
			if err != nil {
				t.Fatal(err)
			}
			placed, err := q.Place(pose)
			if err != nil {
				t.Fatal(err)
			}
			world, err := placed.BoundStrictSurfaceArea(ctx, opt)
			if err != nil || world.Area.Lower.Cmp(r.Area.Lower) != 0 || world.Area.Upper.Cmp(r.Area.Upper) != 0 {
				t.Fatal("proper rigid area must be invariant", world, err)
			}
		}
	}
	if got, err := cut.BoundStrictSurfaceArea(ctx, MixedRoofAreaOptions{MaxCells: 1}); !errors.Is(err, geom.ErrConvergence) || got.Area.Lower != nil {
		t.Fatal("area work limit exposed a partial graph", got, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := cut.BoundStrictSurfaceArea(cancelled, opt); !errors.Is(err, context.Canceled) || got.Area.Lower != nil {
		t.Fatal("area cancellation exposed a partial graph", got, err)
	}
}

func TestStrictRoofAreaRejectsCrossing(t *testing.T) {
	ctx := context.Background()
	a, _ := crossingFootprintRoofShell(t, false)
	c, _ := crossingFootprintRoofShell(t, true)
	boxes := [2]*mixedRoofBox{}
	for i, body := range []Solid{a, c} {
		p, err := proveMixedRoofBoxMode(ctx, body, 1, 2000000, true)
		if err != nil {
			t.Fatal(err)
		}
		boxes[i] = p
	}
	q := &MixedRoofMaterialQuery{op: parametric.Intersection, box: boxes}
	got, err := q.BoundStrictSurfaceArea(ctx, MixedRoofAreaOptions{})
	if !errors.Is(err, geom.ErrUnrepresentable) || got.Area.Lower != nil || len(got.Faces) != 0 {
		t.Fatal("crossing footprints cannot retain whole faces", got, err)
	}
}
