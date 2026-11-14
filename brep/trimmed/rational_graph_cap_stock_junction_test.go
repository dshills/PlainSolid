package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestAlgebraicStockLineJunctions(t *testing.T) {
	ctx := context.Background()
	stock := makeSquareRationalGraphTube(t)
	section := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	regions := RationalGraphShellRegionOptions{}
	opts := geom.ExactCurvePlaneEventOptions{MaxOperations: 12000000}
	planes := [2][3]geom.Vec3{
		{{Z: .25}, {Y: 1, Z: .25}, {X: 1, Z: .25}},
		{{Z: .75}, {X: 1, Z: .75}, {Y: 1, Z: .75}},
	}
	pair, err := stock.ClipAlgebraicGraphBand(ctx, planes, section, regions)
	if err != nil {
		t.Fatal(err)
	}
	m, err := pair.PrepareVerticalMaterial(ctx, section, opts)
	if err != nil {
		t.Fatal(err)
	}

	q := big.NewRat
	vec := func(x, y, z *big.Rat) [3]*big.Rat { return [3]*big.Rat{x, y, z} }
	extent := [2]*big.Rat{q(-1, 1), q(5, 1)}
	checkRoot := func(t *testing.T, root *geom.ExactAlgebraicRoot, want *big.Rat) {
		t.Helper()
		a, err := root.Classify(ctx, opts)
		if err != nil || a.Kind != geom.ExactRootKindRational || a.Rational.Cmp(want) != 0 {
			t.Fatalf("root = %v, want %s (%v)", a, want, err)
		}
	}

	t.Run("two original walls", func(t *testing.T) {
		a, d := vec(q(-1, 1), q(-1, 1), q(1, 2)), vec(q(1, 1), q(1, 1), q(0, 1))
		got, err := m.IntersectRationalLineWithJunctions(ctx, a, d, extent, opts)
		if err != nil || len(got.Contacts) != 4 || len(got.Material) != 2 {
			t.Fatalf("sleeve diagonal: %v, %v", got, err)
		}
		owners := map[int]bool{}
		for i, hit := range got.Contacts {
			checkRoot(t, hit.Parameter, q(int64(i+1), 1))
			if hit.Entry != (i%2 == 0) || hit.Touch || !hit.Junction || hit.StockFace < 0 ||
				hit.OtherStockFace < 0 || hit.StockFace == hit.OtherStockFace || hit.Cap != -1 || hit.OtherCap != -1 {
				t.Fatal("missing distinct original wall owners", i, hit)
			}
			owners[hit.StockFace], owners[hit.OtherStockFace] = true, true
		}
		if len(owners) != 8 {
			t.Fatal("all eight wall owners must survive", owners)
		}
		pose, err := placement.NewRationalRigid([3][4]*big.Rat{
			{q(3, 5), q(0, 1), q(4, 5), q(1, 7)},
			{q(0, 1), q(1, 1), q(0, 1), q(-1, 3)},
			{q(-4, 5), q(0, 1), q(3, 5), q(1, 11)},
		}, 4096)
		if err != nil {
			t.Fatal(err)
		}
		w, err := pair.PlaceVerticalMaterialRigid(ctx, pose, section, opts)
		if err != nil {
			t.Fatal(err)
		}
		worldA, err := pose.TransformPoint(ctx, a, false, 4096)
		if err != nil {
			t.Fatal(err)
		}
		worldD, err := pose.TransformVector(ctx, d, false, 4096)
		if err != nil {
			t.Fatal(err)
		}
		world, err := w.IntersectLineRationalWithJunctions(ctx, worldA, worldD, extent, opts)
		if err != nil || len(world.Contacts) != len(got.Contacts) || len(world.Material) != len(got.Material) {
			t.Fatal("world placement changed junctions", world, err)
		}
		for i, hit := range world.Contacts {
			checkRoot(t, hit.Parameter, q(int64(i+1), 1))
			if hit.StockFace != got.Contacts[i].StockFace || hit.OtherStockFace != got.Contacts[i].OtherStockFace || hit.Entry != got.Contacts[i].Entry {
				t.Fatal("world placement changed original owners", i, hit)
			}
		}
		if failed, err := m.IntersectRationalLineWithTouches(ctx, a, d, extent, opts); !errors.Is(err, geom.ErrConvergence) || len(failed.Contacts) != 0 {
			t.Fatal("legacy touches must still reject corners", failed, err)
		}
	})

	t.Run("cap meets wall", func(t *testing.T) {
		out := vec(q(-1, 1), q(1, 2), q(-3, 4))
		d := vec(q(1, 1), q(0, 1), q(1, 1))
		got, err := m.IntersectRationalLineWithJunctions(ctx, out, d, extent, opts)
		if err != nil || len(got.Contacts) != 2 || len(got.Material) != 1 {
			t.Fatal("joint entry then cap exit", got, err)
		}
		h := got.Contacts[0]
		if !h.Junction || !h.Entry || h.Touch || h.Cap != 0 || h.OtherCap != -1 ||
			h.StockFace < 0 || h.OtherStockFace != -1 || got.Contacts[1].Cap != 1 || got.Contacts[1].Entry {
			t.Fatal("expected cap and wall owners on one event", got)
		}
		checkRoot(t, h.Parameter, q(1, 1))
		checkRoot(t, got.Contacts[1].Parameter, q(3, 2))
		out[2], d[2] = q(5, 4), q(-1, 1)
		got, err = m.IntersectRationalLineWithJunctions(ctx, out, d, extent, opts)
		if err != nil || len(got.Contacts) != 1 || len(got.Material) != 0 ||
			!got.Contacts[0].Touch || got.Contacts[0].Entry || got.Contacts[0].StockFace != h.StockFace || got.Contacts[0].Cap != 0 {
			t.Fatal("a wall entry and cap exit at the same point cancel to a touch", got, err)
		}
		checkRoot(t, got.Contacts[0].Parameter, q(1, 1))
	})

	t.Run("generated common seam", func(t *testing.T) {
		planes[0] = [3]geom.Vec3{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}}
		planes[1] = [3]geom.Vec3{{Z: 5. / 16}, {X: 1, Z: 7. / 16}, {Y: 1, Z: 5. / 16}}
		r, err := stock.ClipAlgebraicGraphCapPair(ctx, planes, section, regions)
		if err != nil {
			t.Fatal(err)
		}
		v, err := r.PrepareVerticalMaterial(ctx, section, opts)
		if err != nil {
			t.Fatal(err)
		}
		a, d := vec(q(3, 2), q(1, 2), q(1, 2)), vec(q(1, 1), q(0, 1), q(1, 16))
		got, err := v.IntersectRationalLineWithJunctions(ctx, a, d, extent, opts)
		if err != nil || len(got.Contacts) != 2 || len(got.Material) != 1 {
			t.Fatal("seam entry then exit on stock wall", got, err)
		}
		h := got.Contacts[0]
		if !h.Junction || !h.Entry || h.Touch || h.Cap != 0 || h.OtherCap != 1 || h.StockFace != -1 || h.OtherStockFace != -1 {
			t.Fatal("common seam owns both caps and no manufactured wall", h)
		}
		checkRoot(t, h.Parameter, q(0, 1))
	})
}
