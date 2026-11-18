//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCoincidentRoofBoxVerticalPartition(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{2, 2, 2}, 0)
	b, _ := makeOpenSplineBox(t, [3]float64{2, 1, 1}, [3]float64{4, 3, 3}, 0)
	r := big.NewRat
	xy := [2]*big.Rat{r(2, 1), r(3, 2)}
	const O, B, I = parametric.Outside, parametric.Boundary, parametric.Inside
	for _, tc := range []struct {
		op            parametric.BooleanOp
		points, spans []parametric.Location
	}{{parametric.Union, []parametric.Location{O, B, B, B, B, O}, []parametric.Location{O, B, I, B, O}},
		{parametric.Intersection, []parametric.Location{O, O, O, O, O, O}, []parametric.Location{O, O, O, O, O}},
		{parametric.Difference, []parametric.Location{O, O, B, B, B, O}, []parametric.Location{O, O, B, B, O}}} {
		q, err := PrepareRoofBoxPointQuery(ctx, a, b, tc.op, MixedRoofQueryOptions{})
		if err != nil {
			t.Fatal(err)
		}
		// Nonunit, reversed parameterization: z=3−2t. Stations t=0,1/2,1,3/2
		// have one original owner each. The outer endpoints are clip-only.
		out, err := q.PartitionVerticalRational(ctx, xy, r(3, 1), r(-2, 1), r(-1, 3), r(7, 4), geom.SurfacePatchEvaluationOptions{})
		if err != nil || len(out.Stations) != 6 || len(out.Spans) != 5 {
			t.Fatal("exact covered vertical partition", tc.op, out, err)
		}
		cuts := []*big.Rat{r(-1, 3), r(0, 1), r(1, 2), r(1, 1), r(3, 2), r(7, 4)}
		for i, s := range out.Stations {
			if s.Parameter.Cmp(cuts[i]) != 0 || s.Point.Location != tc.points[i] || s.ClipEndpoint != (i == 0 || i == 5) {
				t.Fatal("ordered station ownership and state", tc.op, i, s)
			}
			if s.ClipEndpoint && len(s.SourceHeights) != 0 || !s.ClipEndpoint && len(s.SourceHeights) != 1 {
				t.Fatal("actual versus artificial height stations", s)
			}
		}
		for i, s := range out.Spans {
			if s.Start != i || s.End != i+1 || s.Location != tc.spans[i] {
				t.Fatal("open spans at shared side contact", tc.op, i, s)
			}
		}
		if limited, err := q.PartitionVerticalRational(ctx, xy, r(3, 1), r(-2, 1), r(-1, 3), r(7, 4), geom.SurfacePatchEvaluationOptions{MaxOperations: out.ExactOperations - 1}); !errors.Is(err, geom.ErrConvergence) || len(limited.Stations) != 0 {
			t.Fatal("partition quota must fail atomically", limited, err)
		}
	}
	q, err := PrepareRoofBoxPointQuery(ctx, a, a, parametric.Difference, MixedRoofQueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out, err := q.PartitionVerticalRational(ctx, [2]*big.Rat{r(1, 1), r(1, 1)}, r(0, 1), r(5, 1), r(2, 5), r(2, 5), geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(out.Stations) != 1 || len(out.Spans) != 0 || out.Stations[0].Point.Location != O || len(out.Stations[0].SourceHeights) != 2 || !out.Stations[0].ClipEndpoint {
		t.Fatal("singleton at coincident cancelled roofs", out, err)
	}
}
