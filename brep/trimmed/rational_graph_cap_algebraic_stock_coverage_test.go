package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestAlgebraicStockCompleteLineCoverage(t *testing.T) {
	ctx := context.Background()
	stock := makeSquareRationalGraphTube(t)
	planes := [2][3]geom.Vec3{
		{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}},
		{{Z: 5. / 16}, {X: 1, Z: 7. / 16}, {Y: 1, Z: 5. / 16}},
	}
	section := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	pair, err := stock.ClipAlgebraicGraphCapPair(ctx, planes, section, RationalGraphShellRegionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	opts := geom.ExactCurvePlaneEventOptions{MaxOperations: 12000000}
	m, err := pair.PrepareVerticalMaterial(ctx, section, opts)
	if err != nil {
		t.Fatal(err)
	}
	q := big.NewRat
	d := [3]*big.Rat{q(0, 1), q(1, 1), q(0, 1)}
	extent := [2]*big.Rat{q(-1, 1), q(4, 1)}
	origin := [3]*big.Rat{q(3, 2), q(0, 1), q(1, 2)}
	assertRoot := func(r *geom.ExactAlgebraicRoot, want *big.Rat) {
		t.Helper()
		v, err := r.Classify(ctx, opts)
		if err != nil || v.Kind != geom.ExactRootKindRational || v.Rational.Cmp(want) != 0 {
			t.Fatal("incorrect exact stock line parameter", v, want, err)
		}
	}
	assertPiece := func(p AlgebraicGraphStockLinePiece, a, z *big.Rat, caps, walls int) {
		t.Helper()
		assertRoot(p.Extent[0], a)
		assertRoot(p.Extent[1], z)
		if len(p.Caps) != caps || len(p.StockFaces) != walls || p.BoundaryOnly != (caps+walls > 0) {
			t.Fatal("wrong relative interior ownership", p, caps, walls)
		}
	}
	got, err := m.CoverRationalLine(ctx, origin, d, extent, opts)
	if err != nil || len(got.Spans) != 2 || len(got.Points) != 4 {
		t.Fatal("closed material on shared cap seams", got, err)
	}
	assertPiece(got.Spans[0], q(0, 1), q(1, 1), 2, 0)
	assertPiece(got.Spans[1], q(2, 1), q(3, 1), 2, 0)
	for _, p := range got.Points {
		if len(p.Caps) != 2 || len(p.StockFaces) != 1 {
			t.Fatal("seam endpoint must report both caps and original wall", p)
		}
	}
	origin[0] = q(5, 2)
	got, err = m.CoverRationalLine(ctx, origin, d, extent, opts)
	if err != nil || len(got.Spans) != 1 || len(got.Points) != 2 {
		t.Fatal("cap-plane line through the full width", got, err)
	}
	assertPiece(got.Spans[0], q(0, 1), q(3, 1), 1, 0)
	for _, p := range got.Points {
		if len(p.Caps) != 1 || len(p.StockFaces) != 1 {
			t.Fatal("cap line wall edges", p)
		}
	}

	// The original wall run starts at the shared cap seam and ends
	// where two original vertical walls meet.
	origin = [3]*big.Rat{q(0, 1), q(0, 1), q(13, 32)}
	d = [3]*big.Rat{q(1, 1), q(0, 1), q(1, 16)}
	got, err = m.CoverRationalLine(ctx, origin, d, extent, opts)
	if err != nil || len(got.Spans) != 2 || len(got.Points) != 3 {
		t.Fatal("wall spans must keep both original chart partitions", got, err)
	}
	assertPiece(got.Spans[0], q(3, 2), q(2, 1), 0, 1)
	assertPiece(got.Spans[1], q(2, 1), q(3, 1), 0, 1)
	assertPiece(got.Points[0], q(3, 2), q(3, 2), 2, 1)
	assertPiece(got.Points[1], q(2, 1), q(2, 1), 0, 2)
	assertPiece(got.Points[2], q(3, 1), q(3, 1), 0, 2)
	// Z=X/4 enters through the horizontal cap at X=2 and
	// exits through the tilted cap at X=5/2, staying on Y=0.
	origin[2], d[2] = q(0, 1), q(1, 4)
	got, err = m.CoverRationalLine(ctx, origin, d, extent, opts)
	if err != nil || len(got.Spans) != 1 || len(got.Points) != 2 {
		t.Fatal("both cap halfspaces must clip a continuous wall run", got, err)
	}
	assertPiece(got.Spans[0], q(2, 1), q(5, 2), 0, 1)
	assertPiece(got.Points[0], q(2, 1), q(2, 1), 1, 2)
	assertPiece(got.Points[1], q(5, 2), q(5, 2), 1, 1)
	if got.Points[0].Caps[0] != 0 || got.Points[1].Caps[0] != 1 {
		t.Fatal("wrong cap ownership", got.Points)
	}
	pose, err := placement.NewRationalRigid([3][4]*big.Rat{
		{q(3, 5), q(0, 1), q(4, 5), q(1, 7)},
		{q(0, 1), q(1, 1), q(0, 1), q(-1, 3)},
		{q(-4, 5), q(0, 1), q(3, 5), q(1, 11)},
	}, 4096)
	if err != nil {
		t.Fatal(err)
	}
	placed, err := pair.PlaceVerticalMaterialRigid(ctx, pose, section, opts)
	if err != nil {
		t.Fatal(err)
	}
	a, err := pose.TransformPoint(ctx, origin, false, 4096)
	if err != nil {
		t.Fatal(err)
	}
	worldD, err := pose.TransformVector(ctx, d, false, 4096)
	if err != nil {
		t.Fatal(err)
	}
	world, err := placed.CoverLineRational(ctx, a, worldD, extent, opts)
	if err != nil || len(world.Spans) != 1 || len(world.Points) != 2 ||
		world.Spans[0].StockFaces[0] != got.Spans[0].StockFaces[0] {
		t.Fatal("proper placement must retain the exact wall and cap owners", world, err)
	}
	assertPiece(world.Spans[0], q(2, 1), q(5, 2), 0, 1)
	down := [3]*big.Rat{q(0, 1), q(0, 1), q(-2, 1)}
	for _, tc := range []struct {
		x, y           *big.Rat
		spans, points  int
		walls, ownerHi int
	}{
		{q(5, 2), q(1, 2), 1, 2, 0, 1},
		{q(5, 2), q(0, 1), 1, 2, 1, 1},
		{q(3, 2), q(1, 2), 0, 1, 0, 0},
		{q(0, 1), q(0, 1), 0, 0, 0, 0},
	} {
		v, err := m.CoverRationalLine(ctx, [3]*big.Rat{tc.x, tc.y, q(1, 1)}, down, extent, opts)
		if err != nil || len(v.Spans) != tc.spans || len(v.Points) != tc.points {
			t.Fatal("vertical material along the exact stock axis", tc, v, err)
		}
		if tc.spans == 1 {
			assertPiece(v.Spans[0], q(3, 16), q(1, 4), 0, tc.walls)
			assertPiece(v.Points[0], q(3, 16), q(3, 16), 1, tc.walls)
			assertPiece(v.Points[1], q(1, 4), q(1, 4), 1, tc.walls)
			if v.Points[0].Caps[0] != tc.ownerHi {
				t.Fatal("negative direction reversed the cap order", v)
			}
		} else if tc.points == 1 {
			assertPiece(v.Points[0], q(1, 4), q(1, 4), 2, 0)
		}
	}

	bad, err := m.CoverRationalLine(ctx, origin, d, extent, geom.ExactCurvePlaneEventOptions{MaxOperations: 1})
	if !errors.Is(err, geom.ErrConvergence) || len(bad.Spans)+len(bad.Points) != 0 {
		t.Fatal("complete spatial coverage is atomic", bad, err)
	}
}
