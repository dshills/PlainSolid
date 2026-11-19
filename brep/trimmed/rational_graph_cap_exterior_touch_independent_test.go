//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestAlgebraicGraphExteriorTouchIndependent(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	q := big.NewRat
	stock := makeWarpedSquareRationalGraphTube(t, 1./8)
	const z = .5 - 11./64
	planes := [2][3]geom.Vec3{{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}}, {{Z: z}, {X: 1, Z: z}, {Y: 1, Z: z + 1./8}}}
	section := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	pair, e := stock.ClipAlgebraicGraphCapPair(ctx, planes, section, RationalGraphShellRegionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	opts := geom.ExactCurvePlaneEventOptions{}
	model, e := pair.PrepareVerticalMaterial(ctx, section, opts)
	if e != nil {
		t.Fatal(e)
	}
	// The upper void boundary is y=2+x²/8. Its tangent at x=7/4
	// is y=207/128+7x/16; boundary minus tangent=(x-7/4)²/8.
	// Thus both open sides of this finite line lie strictly in the void.
	origin := [3]*big.Rat{q(0, 1), q(207, 128), q(9, 16)}
	direction := [3]*big.Rat{q(1, 1), q(7, 16), q(0, 1)}
	extent := [2]*big.Rat{q(13, 8), q(15, 8)}
	for _, reverse := range []bool{false, true} {
		d := direction
		bounds := extent
		want := q(7, 4)
		if reverse {
			d = [3]*big.Rat{q(-1, 1), q(-7, 16), q(0, 1)}
			bounds = [2]*big.Rat{q(-15, 8), q(-13, 8)}
			want.Neg(want)
		}
		got, e := model.IntersectRationalLineWithTouches(ctx, origin, d, bounds, opts)
		if e != nil || len(got.Material) != 0 || len(got.Contacts) != 1 {
			t.Fatalf("exterior tangent: %+v %v", got, e)
		}
		hit := got.Contacts[0]
		if !hit.Touch || hit.Entry || hit.Cap != -1 || hit.StockFace < 0 {
			t.Fatal("touch owner/state", hit)
		}
		sign, e := hit.Parameter.SignPolynomial(ctx, []*big.Rat{new(big.Rat).Neg(want), q(1, 1)}, opts)
		if e != nil || sign.Sign != 0 {
			t.Fatal("tangent parameter", sign, e)
		}
		old, e := model.IntersectRationalLine(ctx, origin, d, bounds, opts)
		if !errors.Is(e, geom.ErrConvergence) || len(old.Contacts)+len(old.Material) != 0 {
			t.Fatal("legacy rejection", old, e)
		}
	}
	capOrigin := origin
	capOrigin[2] = q(1, 2)
	capHit, e := pair.Caps[0].IntersectRationalLineWithTouches(ctx, capOrigin, direction, extent, opts)
	if e != nil || len(capHit.Material) != 0 || len(capHit.Contacts) != 1 || !capHit.Contacts[0].Touch || capHit.Contacts[0].CurveParameter == nil {
		t.Fatal("cap exterior tangent", capHit, e)
	}
	// Independent nearby fiber queries: the two rational probes are in the
	// void, while the central fiber is boundary-only on the original wall.
	for _, x := range []*big.Rat{q(13, 8), q(7, 4), q(15, 8)} {
		y := new(big.Rat).Add(q(207, 128), new(big.Rat).Mul(q(7, 16), x))
		fiber, e := model.VerticalFiber(ctx, [2]*big.Rat{x, y})
		if e != nil {
			t.Fatal(e)
		}
		if x.Cmp(q(7, 4)) == 0 {
			if !fiber.Found || !fiber.BoundaryOnly {
				t.Fatal("central boundary", fiber)
			}
		} else if fiber.Found {
			t.Fatal("exterior open sides", fiber)
		}
	}
	// Excluding the tangent from the finite window excludes every event.
	empty, e := model.IntersectRationalLineWithTouches(ctx, origin, direction, [2]*big.Rat{q(13, 8), q(27, 16)}, opts)
	if e != nil || len(empty.Contacts)+len(empty.Material) != 0 {
		t.Fatal("finite clipping", empty, e)
	}
	bad, e := model.IntersectRationalLineWithTouches(ctx, origin, direction, extent, geom.ExactCurvePlaneEventOptions{MaxOperations: 1})
	if !errors.Is(e, geom.ErrConvergence) || len(bad.Contacts)+len(bad.Material) != 0 {
		t.Fatal("atomic budget", bad, e)
	}
}
