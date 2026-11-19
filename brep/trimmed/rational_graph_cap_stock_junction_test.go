//go:build research

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
		worldA, err = pose.TransformPoint(ctx, vec(q(0, 1), q(0, 1), q(0, 1)), false, 4096)
		if err != nil {
			t.Fatal(err)
		}
		worldD, err = pose.TransformVector(ctx, vec(q(0, 1), q(0, 1), q(2, 1)), false, 4096)
		if err != nil {
			t.Fatal(err)
		}
		run, err := w.IntersectVerticalBoundaryRational(ctx, worldA, worldD, extent, opts)
		if err != nil || !run.Found || run.Extent[0].Cmp(q(1, 8)) != 0 || run.Extent[1].Cmp(q(3, 8)) != 0 ||
			run.Caps != [2]int{0, 1} || run.StockFaces[0] < 0 || run.StockFaces[1] < 0 {
			t.Fatal("placed stock axis must preserve the run and all endpoint owners", run, err)
		}
		// Reuse the diagonal in the lower cap under the same tilted pose.
		worldA, err = pose.TransformPoint(ctx, vec(q(-1, 1), q(-1, 1), q(1, 4)), false, 4096)
		if err != nil {
			t.Fatal(err)
		}
		worldD, err = pose.TransformVector(ctx, d, false, 4096)
		if err != nil {
			t.Fatal(err)
		}
		capLine, err := w.IntersectCapLineRationalWithJunctions(ctx, 0, worldA, worldD, extent, opts)
		if err != nil || len(capLine.Contacts) != 4 || len(capLine.Material) != 2 {
			t.Fatal("placed cap must retain its shared trim corners", capLine, err)
		}
		for i, hit := range capLine.Contacts {
			checkRoot(t, hit.Parameter, q(int64(i+1), 1))
			cap := pair.caps[0]
			if !hit.Junction || cap.wires[hit.Wire][hit.Use].Source.Face != world.Contacts[i].StockFace ||
				cap.wires[hit.Wire][hit.OtherUse].Source.Face != world.Contacts[i].OtherStockFace {
				t.Fatal("placed cap must retain both original directed uses", i, hit)
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
		along := vec(q(0, 1), q(1, 1), q(0, 1))
		for _, tc := range []struct {
			window [2]*big.Rat
			want   [2]*big.Rat
			count  int
		}{
			{[2]*big.Rat{q(-1, 1), q(4, 1)}, [2]*big.Rat{q(0, 1), q(1, 1)}, 2},
			{[2]*big.Rat{q(1, 2), q(3, 4)}, [2]*big.Rat{q(1, 2), q(3, 4)}, 1},
			{[2]*big.Rat{q(1, 1), q(1, 1)}, [2]*big.Rat{q(1, 1), q(1, 1)}, 1},
			{[2]*big.Rat{q(5, 4), q(7, 4)}, [2]*big.Rat{}, 0},
		} {
			runs, err := r.caps[0].IntersectRationalBoundaryRuns(ctx, vec(q(3, 2), q(0, 1), q(1, 2)), along, tc.window, opts)
			if err != nil || len(runs.Runs) != tc.count {
				t.Fatal("generated seam portions", tc, runs, err)
			}
			seen := 0
			for _, run := range runs.Runs {
				use := r.caps[0].wires[run.Wire][run.Use]
				if use.Seam < 0 || run.CurveParameters[0] == nil || run.CurveParameters[1] == nil {
					t.Fatal("must retain exact generated use ancestry", run)
				}
				r1 := tc.want
				s, err := run.Extent[0].Classify(ctx, opts)
				if err != nil || s.Kind != geom.ExactRootKindRational {
					t.Fatal("seam interval", s, err)
				}
				if tc.count == 2 && s.Rational.Cmp(q(2, 1)) == 0 {
					r1 = [2]*big.Rat{q(2, 1), q(3, 1)}
					seen |= 2
				} else {
					seen |= 1
				}
				checkRoot(t, run.Extent[0], r1[0])
				checkRoot(t, run.Extent[1], r1[1])
			}
			if tc.count == 2 && seen != 3 {
				t.Fatal("both sides of the through void must remain separate", runs)
			}
		}

		vertical := vec(q(0, 1), q(0, 1), q(1, 1))
		for _, tc := range []struct {
			y     *big.Rat
			walls int
		}{{q(1, 2), 0}, {q(0, 1), 1}} {
			pinch, err := v.IntersectRationalLineWithJunctions(ctx, vec(q(3, 2), tc.y, q(0, 1)), vertical, extent, opts)
			if err != nil || len(pinch.Material) != 0 || len(pinch.Contacts) != 1 {
				t.Fatal("collapsed vertical fiber is one isolated contact", tc, pinch, err)
			}
			h := pinch.Contacts[0]
			checkRoot(t, h.Parameter, q(1, 2))
			if !h.Touch || !h.Junction || h.Entry || h.Cap != 0 || h.OtherCap != 1 ||
				(h.StockFace >= 0) != (tc.walls == 1) || h.OtherStockFace != -1 {
				t.Fatal("collapsed fiber lost an original face owner", tc, h)
			}
		}
		wall := vec(q(5, 2), q(0, 1), q(0, 1))
		if got, err := v.IntersectRationalLineWithJunctions(ctx, wall, vertical, extent, opts); !errors.Is(err, geom.ErrConvergence) || len(got.Contacts) != 0 {
			t.Fatal("isolated-event API must reject nonzero wall runs", got, err)
		}
		for _, tc := range []struct {
			a, d    [3]*big.Rat
			window  [2]*big.Rat
			rangeAt [2]*big.Rat
			caps    [2]int
			walls   int
		}{
			{wall, vertical, extent, [2]*big.Rat{q(1, 2), q(5, 8)}, [2]int{0, 1}, 1},
			{wall, vertical, [2]*big.Rat{q(9, 16), q(1, 1)}, [2]*big.Rat{q(9, 16), q(5, 8)}, [2]int{-1, 1}, 1},
			{vec(q(3, 1), q(0, 1), q(1, 1)), vec(q(0, 1), q(0, 1), q(-2, 1)), extent,
				[2]*big.Rat{q(5, 32), q(1, 4)}, [2]int{1, 0}, 2},
			{vec(q(3, 2), q(1, 2), q(0, 1)), vertical, extent,
				[2]*big.Rat{q(1, 2), q(1, 2)}, [2]int{0, 1}, 0},
		} {
			got, err := v.IntersectRationalVerticalBoundary(ctx, tc.a, tc.d, tc.window, opts)
			if err != nil || !got.Found || got.Caps != tc.caps || got.Extent[0].Cmp(tc.rangeAt[0]) != 0 || got.Extent[1].Cmp(tc.rangeAt[1]) != 0 ||
				(got.StockFaces[0] >= 0) != (tc.walls >= 1) || (got.StockFaces[1] >= 0) != (tc.walls >= 2) {
				t.Fatal("coincident boundary interval and original face owners", tc, got, err)
			}
		}
		if got, err := v.IntersectRationalVerticalBoundary(ctx, vec(q(5, 2), q(1, 2), q(0, 1)), vertical, extent, opts); err != nil || got.Found {
			t.Fatal("interior station must not produce a coincident boundary run", got, err)
		}
	})
}
