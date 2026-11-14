package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestAlgebraicStockExactCommonLine(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	s := makeSquareRationalGraphTube(t)
	section := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	pair, err := s.ClipAlgebraicGraphBand(ctx, [2][3]geom.Vec3{
		{{Z: .25}, {Y: 1, Z: .25}, {X: 1, Z: .25}},
		{{Z: .75}, {X: 1, Z: .75}, {Y: 1, Z: .75}},
	}, section, RationalGraphShellRegionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	opts := geom.ExactCurvePlaneEventOptions{MaxOperations: 12000000}
	m, err := pair.PrepareVerticalMaterial(ctx, section, opts)
	if err != nil {
		t.Fatal(err)
	}
	at := func(x, y, z *big.Rat) AlgebraicGraphStockOccurrence {
		p, err := placement.NewRationalRigid([3][4]*big.Rat{
			{q(1, 1), q(0, 1), q(0, 1), x}, {q(0, 1), q(1, 1), q(0, 1), y}, {q(0, 1), q(0, 1), q(1, 1), z},
		}, 4096)
		if err != nil {
			t.Fatal(err)
		}
		return AlgebraicGraphStockOccurrence{pair, m, p}
	}
	first := at(q(0, 1), q(0, 1), q(0, 1))
	a, d := [3]*big.Rat{q(0, 1), q(1, 2), q(1, 2)}, [3]*big.Rat{q(1, 1), q(0, 1), q(0, 1)}
	extent := [2]*big.Rat{q(-1, 1), q(7, 1)}
	assertRoot := func(root *geom.ExactAlgebraicRoot, want *big.Rat) {
		t.Helper()
		c, err := root.Classify(ctx, opts)
		if err != nil || c.Kind != geom.ExactRootKindRational || c.Rational.Cmp(want) != 0 {
			t.Fatal("incorrect common endpoint", c, want, err)
		}
	}
	got, err := first.CoverCommonLineRational(ctx, at(q(1, 2), q(0, 1), q(0, 1)), a, d, extent, opts)
	if err != nil || len(got.Spans) != 1 || len(got.Points) != 2 {
		t.Fatal("common interior has both independent wall endpoints", got, err)
	}
	p := got.Spans[0]
	assertRoot(p.Extent[0], q(1, 2))
	assertRoot(p.Extent[1], q(3, 1))
	if !p.InteriorOfBoth {
		t.Fatal("this line crosses the interior of both stocks", p)
	}
	assertRoot(got.Points[0].Extent[0], q(1, 2))
	assertRoot(got.Points[1].Extent[0], q(3, 1))
	if len(got.Points[0].First.StockFaces) != 0 || len(got.Points[0].Second.StockFaces) != 1 ||
		len(got.Points[1].First.StockFaces) != 1 || len(got.Points[1].Second.StockFaces) != 0 {
		t.Fatal("common endpoints must retain independently attained wall owners", got.Points)
	}
	for _, side := range []int64{-1, 0, 1} {
		// The right wall of the first stock meets the left wall of
		// the second, or separates by a sub-binary64 displacement.
		eps := new(big.Rat).SetFrac(big.NewInt(side), new(big.Int).Lsh(big.NewInt(1), 100))
		x := new(big.Rat).Add(q(3, 1), eps)
		out, err := first.CoverCommonLineRational(ctx, at(x, q(0, 1), q(0, 1)), a, d, extent, opts)
		if err != nil {
			t.Fatal(side, err)
		}
		switch side {
		case -1:
			if len(out.Spans) != 1 || len(out.Points) != 2 || !out.Spans[0].InteriorOfBoth {
				t.Fatal("positive-width overlap must not round to a point", out)
			}
			assertRoot(out.Spans[0].Extent[0], x)
			assertRoot(out.Spans[0].Extent[1], q(3, 1))
		case 0:
			if len(out.Spans) != 0 || len(out.Points) != 1 || len(out.Points[0].First.StockFaces) != 1 || len(out.Points[0].Second.StockFaces) != 1 {
				t.Fatal("zero-dimensional common needs both independent walls", out)
			}
			assertRoot(out.Points[0].Extent[0], q(3, 1))
		case 1:
			if len(out.Spans)+len(out.Points) != 0 {
				t.Fatal("strict gap must not round to contact", out)
			}
		}
	}
	a = [3]*big.Rat{q(1, 2), q(0, 1), q(0, 1)}
	d = [3]*big.Rat{q(0, 1), q(0, 1), q(1, 1)}
	out, err := first.CoverCommonLineRational(ctx, at(q(0, 1), q(0, 1), q(1, 4)), a, d, extent, opts)
	if err != nil || len(out.Spans) != 1 || len(out.Points) != 2 || out.Spans[0].InteriorOfBoth ||
		len(out.Spans[0].First.StockFaces) != 1 || len(out.Spans[0].Second.StockFaces) != 1 {
		t.Fatal("common coincident walls retain independent ownership", out, err)
	}
	assertRoot(out.Spans[0].Extent[0], q(1, 2))
	assertRoot(out.Spans[0].Extent[1], q(3, 4))
	// Read the original wall partitions as well as the total interval.
	a = [3]*big.Rat{q(0, 1), q(0, 1), q(1, 2)}
	d = [3]*big.Rat{q(1, 1), q(0, 1), q(0, 1)}
	side, err := first.CoverCommonLineRational(ctx, at(q(1, 2), q(0, 1), q(0, 1)), a, d, extent, opts)
	if err != nil || len(side.Spans) != 5 || len(side.Points) != 6 {
		t.Fatal("preserve each distinct wall chart transition across the shared run", side, err)
	}
	ends := []*big.Rat{q(1, 2), q(1, 1), q(3, 2), q(2, 1), q(5, 2), q(3, 1)}
	for i, p := range side.Spans {
		assertRoot(p.Extent[0], ends[i])
		assertRoot(p.Extent[1], ends[i+1])
		if p.InteriorOfBoth || len(p.First.StockFaces) != 1 || len(p.Second.StockFaces) != 1 {
			t.Fatal("relative interior of each shared run has one original wall per stock", p)
		}
	}
	bad, err := first.CoverCommonLineRational(ctx, first, a, d, extent, geom.ExactCurvePlaneEventOptions{MaxOperations: 1})
	if !errors.Is(err, geom.ErrConvergence) || len(bad.Spans)+len(bad.Points) != 0 {
		t.Fatal("two-body line proof must fail atomically", bad, err)
	}
}
