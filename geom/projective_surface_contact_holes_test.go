package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestProjectiveSurfaceContactsClipHolesInBothOriginalCharts(t *testing.T) {
	ctx := context.Background()
	a, b := projectiveTinyLinearHeightCharts(t)
	axes := [2]int{0, 1}
	aHole := ExactSurfaceRectangle{ExactParameterInterval{big.NewRat(2, 5), big.NewRat(3, 5)}, ExactParameterInterval{big.NewRat(9, 16), big.NewRat(5, 8)}}
	bHole := ExactSurfaceRectangle{ExactParameterInterval{big.NewRat(-1, 1), big.NewRat(-1, 2)}, ExactParameterInterval{big.NewRat(25, 4), big.NewRat(13, 2)}}
	o := ProjectiveSurfaceContactGraphOptions{Cover: ProjectiveSurfaceContactCoverOptions{
		OriginalHoles: [2][]ExactSurfaceRectangle{{aHole}, {bHole}},
	}}
	g, err := TraceProjectiveSurfaceContacts(ctx, a, b, axes, o)
	if err != nil || !g.Complete || len(g.Contours) != 3 || len(g.Cover.CommonHoles) != 2 || len(g.Cover.ExcludedCells) == 0 {
		t.Fatalf("expected three paths and two excluded original holes: %v, complete %v, arcs %d, paths %d, incomplete %v",
			err, g.Complete, len(g.Arcs), len(g.Contours), g.Cover.IncompleteCells)
	}
	first, second := g.Cover.CommonHoles[0].V, g.Cover.CommonHoles[1].V
	if first.Lower.Cmp(big.NewRat(1, 8)) != 0 || first.Upper.Cmp(big.NewRat(1, 4)) != 0 ||
		second.Lower.Cmp(big.NewRat(1, 2)) != 0 || second.Upper.Cmp(big.NewRat(27, 38)) != 0 {
		t.Fatal("current proof lost a nonbinary hole cut or chart owner", first, second)
	}
	hits, outer := 0, 0
	for _, v := range g.Vertices {
		assertProjectiveBoundaryRoot(t, v.OriginalUV[0][0], big.NewRat(1, 2))
		assertProjectiveBoundaryRoot(t, v.OriginalUV[1][1], big.NewRat(32, 5))
		if v.OnTrimBoundary {
			hits++
		}
		if v.OnOverlapBoundary {
			outer++
		}
	}
	if hits != 4 || outer != 2 {
		t.Fatal("hole terminals must remain distinct from the two initial exterior terminals", hits, outer)
	}
	test := big.NewRat(1, 8)
	checked := false
	for i := range g.Arcs {
		// Returned public masks can be edited, while sealed replay must
		// retain the real source material and exact hole boundary.
		g.Cover.CommonHoles = nil
		q, _, err := g.EvaluateArcRational(ctx, i, test, SurfacePatchEvaluationOptions{})
		if errors.Is(err, ErrDomain) {
			continue
		}
		if err != nil || !q.OnTrimBoundary {
			t.Fatal("replayed exact terminal lost its original hole identity", q, err)
		}
		assertProjectiveBoundaryRoot(t, q.OriginalUV[0][1], big.NewRat(9, 16))
		checked = true
	}
	if !checked {
		t.Fatal("no arc reaches the verified hole boundary")
	}
	for _, bad := range []ExactSurfaceRectangle{
		{aHole.U, ExactParameterInterval{big.NewRat(3, 5), big.NewRat(2, 3)}},
		{aHole.U, ExactParameterInterval{big.NewRat(1, 4), big.NewRat(9, 16)}},
	} {
		opt := o
		opt.Cover.OriginalHoles[0] = []ExactSurfaceRectangle{aHole, bad}
		v, err := TraceProjectiveSurfaceContacts(ctx, a, b, axes, opt)
		if !errors.Is(err, ErrConvergence) || v.Complete || len(v.Arcs)+len(v.Vertices) != 0 {
			t.Fatal("overlapping walls or a partial overlap hole require further material proof", v, err)
		}
	}
}
