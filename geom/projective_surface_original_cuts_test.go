package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestProjectiveSurfaceOriginalCutsUseBothRawClocks(t *testing.T) {
	ctx := context.Background()
	a, b := projectiveTinyLinearHeightCharts(t)
	axes := [2]int{0, 1}
	cuts := ProjectiveSurfaceOriginalCuts{
		{{big.NewRat(3, 8)}, {big.NewRat(5, 6), big.NewRat(1, 4)}},
		{{big.NewRat(-1, 1), big.NewRat(-2, 1)}, {big.NewRat(6, 1), big.NewRat(8, 1)}},
	}
	want := [2][]*big.Rat{
		{new(big.Rat), big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(13, 20), big.NewRat(1, 1)},
		{new(big.Rat), big.NewRat(1, 2), big.NewRat(2, 3), big.NewRat(9, 10), big.NewRat(1, 1)},
	}
	r, err := BuildProjectiveSurfaceHeightDifferenceWithOriginalCuts(ctx, a, b, axes, cuts, GeneralSurfaceHeightOptions{})
	if err != nil || !r.Complete || len(r.Cells) != 16 {
		t.Fatal("exact cut from each original chart must extend the atlas", r, err)
	}
	for i, cell := range r.Cells {
		x, y := i/4, i%4
		for axis, d := range [2]ExactParameterInterval{cell.Common.U, cell.Common.V} {
			index := [2]int{x, y}[axis]
			if d.Lower.Cmp(want[axis][index]) != 0 || d.Upper.Cmp(want[axis][index+1]) != 0 {
				t.Fatal("source coordinate cut was rounded or put on another axis", x, y, axis, d)
			}
		}
		// Inserted cuts must retain original supports rather than
		// substituting their narrower common proof rectangles as owners.
		if cell.OriginalCells[0] != (ParameterRectangle{0, 1, 0, 1}) {
			t.Fatal("replaced original A knot owner", cell.OriginalCells[0])
		}
		v, err := cell.Difference.EvaluateRational(ctx, big.NewRat(1, 2), big.NewRat(1, 2), SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		u, w := new(big.Rat).Add(want[0][x], want[0][x+1]), new(big.Rat).Add(want[1][y], want[1][y+1])
		u.Quo(u, big.NewRat(2, 1))
		w.Add(w, big.NewRat(2, 1)).Quo(w, big.NewRat(4, 1))
		if v.Point[0].Cmp(u) != 0 || v.Point[1].Cmp(w) != 0 {
			t.Fatal("selected exact midpoint moved on the original projection", u, w, v.Point)
		}
	}
	g, err := TraceProjectiveSurfaceContacts(ctx, a, b, axes, ProjectiveSurfaceContactGraphOptions{
		Cover: ProjectiveSurfaceContactCoverOptions{OriginalCuts: cuts},
	})
	if err != nil || !g.Complete || len(g.Contours) != 1 || len(g.Arcs) != 4 || len(g.Vertices) != 5 {
		t.Fatal("extra cut stations split proof arcs and preserve the original curve", err, g)
	}
	assertProjectiveContactChain(t, g, g.Contours[0])
	for _, v := range g.Vertices {
		assertProjectiveBoundaryRoot(t, v.OriginalUV[0][0], big.NewRat(1, 2))
	}
	for _, trial := range []struct {
		cuts ProjectiveSurfaceOriginalCuts
		opts GeneralSurfaceHeightOptions
		want error
	}{
		{cuts, GeneralSurfaceHeightOptions{MaxCells: 15}, ErrConvergence},
		{ProjectiveSurfaceOriginalCuts{{{nil}}}, GeneralSurfaceHeightOptions{}, ErrInvalid},
		{ProjectiveSurfaceOriginalCuts{{{big.NewRat(2, 1)}}}, GeneralSurfaceHeightOptions{}, ErrDomain},
	} {
		r, err := BuildProjectiveSurfaceHeightDifferenceWithOriginalCuts(ctx, a, b, axes, trial.cuts, trial.opts)
		if !errors.Is(err, trial.want) || r.Complete || len(r.Cells) != 0 {
			t.Fatal("unproved cuts cannot publish an incomplete contact grid", r, err)
		}
	}
}
