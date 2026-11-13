package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestRationalGraphSplineShellAffineExactTerminals(t *testing.T) {
	ctx := context.Background()
	s, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	// z=(1+x)/3 crosses four stock edges at points that cannot be
	// represented in binary64. A section does not need to alter the solid.
	q := [3]geom.Vec3{{X: -1}, {X: 2, Z: 1}, {X: -1, Y: 1}}
	r, e := s.SectionRationalGraphSplineShell(ctx, q, RationalGraphShellSectionOptions{VerifyModelEdges: true})
	if e != nil || !r.Complete || len(r.Wires) != 1 || len(r.Wires[0]) != 4 {
		t.Fatal("exact rational box section", r, e)
	}
	seen := map[[2]int64]bool{}
	for _, b := range r.Branches {
		p, ok := b.Pieces[0].(ExactQuadraticUV)
		if !ok {
			t.Fatal("missing exact chart line", b)
		}
		exact, display := p.Endpoints()
		for i, end := range b.Ends {
			x, y, z := end.Point[0], end.Point[1], end.Point[2]
			if new(big.Rat).Mul(z, big.NewRat(3, 1)).Cmp(new(big.Rat).Add(x, big.NewRat(1, 1))) != 0 {
				t.Fatal("rounded exact endpoint", end)
			}
			seen[[2]int64{x.Num().Int64(), y.Num().Int64()}] = true
			u, v := ratInset(display[i].X), ratInset(display[i].Y)
			if exact[i][0].Cmp(u) == 0 && exact[i][1].Cmp(v) == 0 {
				t.Fatal("expected one chart coordinate to retain nonbinary data", exact[i], display[i])
			}
		}
	}
	if len(seen) != 4 {
		t.Fatal("missing corner columns", seen)
	}
	limited := RationalGraphShellSectionOptions{VerifyModelEdges: true, EdgeContacts: geom.ComposedPlaneContactsOptions{EventOptions: geom.ExactCurvePlaneEventOptions{MaxOperations: 1}}}
	if bad, e := s.SectionRationalGraphSplineShell(ctx, q, limited); e == nil || bad.Complete || len(bad.Branches) != 0 {
		t.Fatal("exhausted independent model-edge verification returned a wire", bad, e)
	}
	for _, c := range []struct {
		h    float64
		pass bool
	}{{2, true}, {1, false}} {
		p := [3]geom.Vec3{{Z: c.h}, {X: 1, Z: c.h}, {Y: 1, Z: c.h}}
		r, e := s.SectionRationalGraphSplineShell(ctx, p, RationalGraphShellSectionOptions{})
		if len(r.Branches) != 0 || len(r.Wires) != 0 || r.Complete != c.pass || (e == nil) != c.pass {
			t.Fatal("disjoint or unsupported coincident plane", c, r, e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := s.SectionRationalGraphSplineShell(canceled, q, RationalGraphShellSectionOptions{}); !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal("canceled section published a result", r, e)
	}
}
