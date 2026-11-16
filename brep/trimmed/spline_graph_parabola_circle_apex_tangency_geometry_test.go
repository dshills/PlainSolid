package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func parabolaCircleApexTangencyStock(t *testing.T, axis, diskRegion int) [3]*geom.BSplineSurface {
	t.Helper()
	cy := big.NewRat(int64(1+diskRegion), 4)
	return parabolaCircleCrossingsTestStock(t, axis, cy, big.NewRat(3, 8), big.NewRat(1, 64), false)
}

func TestParabolaCircleApexTangencyGeometry(t *testing.T) {
	ctx := context.Background()
	for axis := range 2 {
		for side := range 2 {
			stock := parabolaCircleApexTangencyStock(t, axis, side)
			for order := range 2 {
				t.Run(fmt.Sprintf("%d/%d/%d", axis, side, order), func(t *testing.T) {
					patches := parabolaTwoCirclesPatches(t, stock[1+order], stock[2-order])
					got, err := geom.DiscoverSplineGraphParabolaCircleApexTangency(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
					if err != nil {
						t.Fatal(err)
					}
					signs := [3]int{1 - 2*order, 2*order - 1}
					signs[2] = -signs[side]
					if got.FreeAxis != axis || got.DiskRegion != side || got.RegionSides != signs ||
						got.Parameters[axis].Cmp(big.NewRat(1, 2)) != 0 || got.Parameters[1-axis].Cmp(big.NewRat(3, 8)) != 0 ||
						got.Point[axis].Cmp(got.Parameters[axis]) != 0 || got.Point[1-axis].Cmp(got.Parameters[1-axis]) != 0 ||
						got.Point[2].Cmp(big.NewRat(2, 1)) != 0 || got.ExactOperations == 0 {
						t.Fatal("changed original apex or signed regions", got)
					}
					if got.CircleCenter[axis].Cmp(big.NewRat(1, 2)) != 0 || got.CircleCenter[1-axis].Cmp(big.NewRat(int64(1+side), 4)) != 0 ||
						got.RadiusSquared.Cmp(big.NewRat(1, 64)) != 0 || got.SubstitutedSecondDerivative.Cmp(big.NewRat(int64(33-2*side), 16)) != 0 ||
						got.ParabolaSecondDerivative.Cmp(big.NewRat(1, 4)) != 0 || got.CircleSecondDerivative.Cmp(big.NewRat(int64(16*side-8), 1)) != 0 {
						t.Fatal("changed source circle or exact ordinary tangency jets", got)
					}
					for j, owner := range got.Owners {
						jet, err := patches[j].EvaluateRational(ctx, owner.Parameters[0], owner.Parameters[1], geom.SurfacePatchEvaluationOptions{})
						if err != nil {
							t.Fatal(err)
						}
						first, second := jet.Du, jet.Dv
						if axis == 1 {
							first, second = jet.Dv, jet.Du
						}
						for k := range jet.Point {
							if jet.Point[k].Cmp(got.Point[k]) != 0 || first[k].Cmp(owner.FreeTangent[k]) != 0 ||
								second[k].Cmp(owner.DependentPartial[k]) != 0 {
								t.Fatal("original roof lost its exact tangent and curvature lift", j, k)
							}
						}
					}
					for i, exit := range got.ParabolaExits {
						if exit[axis].Cmp(big.NewRat(int64(i), 1)) != 0 || exit[1-axis].Cmp(big.NewRat(13, 32)) != 0 ||
							exit[2].Cmp(big.NewRat(2, 1)) != 0 {
							t.Fatal("original stock exit changed", i, exit)
						}
					}
				})
			}
		}
	}
}

func TestParabolaCircleApexTangencyRefusals(t *testing.T) {
	ctx := context.Background()
	baseY, radius := big.NewRat(3, 8), big.NewRat(1, 64)
	for _, c := range []struct {
		name         string
		cy, apex, r2 *big.Rat
		changed      bool
		axis, work   int
	}{
		{"separated", big.NewRat(3, 4), baseY, radius, false, 0, 0},
		{"two transverse nodes", big.NewRat(49, 128), baseY, big.NewRat(1, 16), false, 0, 0},
		{"inside stock but intersects", big.NewRat(1, 2), baseY, big.NewRat(1, 32), false, 0, 0},
		{"stock tangent", big.NewRat(1, 8), big.NewRat(1, 4), radius, false, 0, 0},
		{"changed coefficients", big.NewRat(1, 2), baseY, radius, true, 0, 0},
		{"wrong chart axis", big.NewRat(1, 2), baseY, radius, false, 1, 0},
		{"short proof", big.NewRat(1, 2), baseY, radius, false, 0, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := parabolaCircleCrossingsTestStock(t, 0, c.cy, c.apex, c.r2, c.changed)
			p := parabolaTwoCirclesPatches(t, src[1], src[2])
			got, err := geom.DiscoverSplineGraphParabolaCircleApexTangency(ctx, p[0], p[1], c.axis, geom.GraphPointContactOptions{MaxOperations: c.work})
			if got != nil || !errors.Is(err, geom.ErrConvergence) {
				t.Fatal("wrong original geometry passed exact tangency", got, err)
			}
		})
	}
	p := parabolaCircleCrossingPatches(t, parabolaCircleApexTangencyStock(t, 0, 1))
	for _, outside := range []func() error{
		func() error {
			_, e := geom.DiscoverSplineGraphParabolaCircle(ctx, p[0], p[1], 0, geom.GraphPointContactOptions{})
			return e
		},
		func() error {
			_, e := geom.DiscoverSplineGraphParabolaCircleCrossings(ctx, p[0], p[1], 0, geom.GraphPointContactOptions{})
			return e
		},
	} {
		if err := outside(); !errors.Is(err, geom.ErrConvergence) {
			t.Fatal("legacy separate or transverse discovery admitted the apex tangency", err)
		}
	}
}
