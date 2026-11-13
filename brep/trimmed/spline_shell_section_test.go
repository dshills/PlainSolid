package trimmed

import (
	"context"
	"errors"
	"testing"

	"plainsolid/geom"
)

func TestSectionSplineShellsWireGraph(t *testing.T) {
	ctx := context.Background()
	a, b, sections := multipleOpenSplineFixture(t)
	for _, o := range []SplineShellCombineOptions{{}, {Sections: sections}} {
		r, e := SectionSplineShells(ctx, a, b, o)
		if e != nil || !r.Complete || len(r.Wires) != 2 || len(r.Pairs) != 12 || len(r.FacePairs) != 12 {
			t.Fatal(len(r.Wires), len(r.Pairs), r.Complete, e)
		}
		for _, wire := range r.Wires {
			if len(wire.Pieces) != 12 {
				t.Fatal("unexpected wire piece count", len(wire.Pieces))
			}
			for i, x := range wire.Pieces {
				y := wire.Pieces[(i+1)%len(wire.Pieces)]
				a, b := 1., 0.
				if x.Reversed {
					a = 0
				}
				if y.Reversed {
					b = 1
				}
				p, e := x.Segment.Evaluate(a)
				q, f := y.Segment.Evaluate(b)
				if _, ok := p.Bounds.Intersect(q.Bounds); e != nil || f != nil || !ok {
					t.Fatal("disconnected wire", i, p.Bounds, q.Bounds, e, f)
				}
			}
		}
	}
	r, e := SectionSplineShells(ctx, a, b, SplineShellCombineOptions{Sections: sections[:len(sections)-1]})
	if !errors.Is(e, geom.ErrConvergence) || r.Complete || len(r.Wires) != 0 {
		t.Fatal("accepted incomplete contact graph", r.Complete, len(r.Wires), e)
	}
	far, _ := makeOpenSplineBox(t, [3]float64{3, 3, 3}, [3]float64{4, 4, 4}, 0)
	r, e = SectionSplineShells(ctx, a, far, SplineShellCombineOptions{})
	if e != nil || !r.Complete || len(r.Wires) != 0 {
		t.Fatal("disjoint section set", r.Complete, len(r.Wires), e)
	}
}
