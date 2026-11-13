package unify

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestUnifyExactHistory(t *testing.T) {
	s := cells(t, box(0, 0, 0, 1, 1, 1), box(1, 0, 0, 3, 1, 1), box(0, 1, 0, 3, 2, 1))
	r, h, e := UnifyWithHistory(context.Background(), s, HistoryOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if h.MaxSquaredDisplacement.Sign() != 0 || len(h.Edges) != len(s.Edges()) || len(h.Vertices) != len(s.Vertices()) {
		t.Fatal("report shape")
	}
	a, z := historyPoints(s.Vertices()), historyPoints(r.Solid.Vertices())
	removed, located, third, deleted := false, false, false, false
	for i, row := range h.Edges {
		old := s.Edges()[i]
		for _, span := range row.Spans {
			next := r.Solid.Edges()[span.Edge]
			for end := 0; end < 2; end++ {
				u, v := span.Source.Start, span.Target.Start
				if end == 1 {
					u, v = span.Source.End, span.Target.End
				}
				for k := range 3 {
					x := add(a[old.Start][k], mul(u, sub(a[old.End][k], a[old.Start][k])))
					y := add(z[next.Start][k], mul(v, sub(z[next.End][k], z[next.Start][k])))
					if x.Cmp(y) != 0 {
						t.Fatal("incorrect exact interval map", i, span.Edge)
					}
				}
				if v.Cmp(big.NewRat(1, 3)) == 0 || v.Cmp(big.NewRat(2, 3)) == 0 {
					third = true
				}
			}
		}
		if len(row.Deleted) > 0 {
			deleted = true
			if len(row.Spans) != 0 || len(row.Deleted) != 1 || row.Deleted[0].Start.Sign() != 0 || row.Deleted[0].End.Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal("fixture seam only partly deleted", i)
			}
		}
	}
	for i, row := range h.Vertices {
		if row.Vertex >= 0 {
			if s.Vertices()[i] != r.Solid.Vertices()[row.Vertex] || len(row.Edges) != 0 {
				t.Fatal("retained vertex mapping")
			}
			continue
		}
		removed = true
		for _, hit := range row.Edges {
			located = true
			edge := r.Solid.Edges()[hit.Edge]
			for k := range 3 {
				if add(z[edge.Start][k], mul(hit.Parameter, sub(z[edge.End][k], z[edge.Start][k]))).Cmp(a[i][k]) != 0 {
					t.Fatal("removed vertex location")
				}
			}
		}
	}
	if !removed || !located || !third || !deleted {
		t.Fatal("fixture missed history cases", removed, located, third, deleted)
	}
	_, again, e := UnifyWithHistory(context.Background(), s, HistoryOptions{})
	if e != nil || !reflect.DeepEqual(h, again) {
		t.Fatal("nondeterministic", e)
	}
	h.MaxSquaredDisplacement.SetInt64(99)
	h.Edges[0].Deleted = nil
	if again.MaxSquaredDisplacement.Sign() != 0 {
		t.Fatal("shared report data")
	}
}

func TestUnifyHistoryHolesAndLimits(t *testing.T) {
	s := cells(t, box(0, 0, 0, 6, 2, 2), box(0, 4, 0, 6, 6, 2), box(0, 2, 0, 2, 4, 2), box(4, 2, 0, 6, 4, 2))
	r, h, e := UnifyWithHistory(context.Background(), s, HistoryOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Solid.Faces()) != 10 || h.MaxSquaredDisplacement.Sign() != 0 {
		t.Fatal("hole history")
	}
	for _, o := range []HistoryOptions{{MaxWork: 1}, {MaxWork: -1}} {
		out, report, e := UnifyWithHistory(context.Background(), s, o)
		if e == nil || len(out.Solid.Vertices()) != 0 || len(report.Edges) != 0 {
			t.Fatal("partial budget result", e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e := UnifyWithHistory(ctx, s, HistoryOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestHistoryPartialCoverageIsExplicit(t *testing.T) {
	gaps := historyGaps([]ParameterInterval{{big.NewRat(1, 4), big.NewRat(1, 2)}, {big.NewRat(3, 4), big.NewRat(1, 1)}})
	want := []ParameterInterval{{big.NewRat(0, 1), big.NewRat(1, 4)}, {big.NewRat(1, 2), big.NewRat(3, 4)}}
	if !reflect.DeepEqual(gaps, want) {
		t.Fatal(gaps)
	}
	p := historyPoints([]geom.Vec3{{}, {X: 1}, {X: .5, Y: 1e-20}})
	if _, ok := historyParameter(p[0], p[1], p[2]); ok {
		t.Fatal("near-collinear point treated as exact")
	}
}
