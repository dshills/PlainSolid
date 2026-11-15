package trimmed

import (
	"context"
	"math/big"
	"testing"
)

func refinedCoincidenceFixture(t *testing.T, swap bool) (Solid, Solid) {
	a, b := coincidenceFaceFixture(t, swap, false, false)
	for i, u := range b.faces[0].Outer {
		q, ok := u.Curve.(ExactBezierUV)
		if !ok {
			continue
		}
		left, right, e := q.Split(big.NewRat(1, 4))
		if e != nil {
			t.Fatal(e)
		}
		middle, last, e := right.Split(big.NewRat(2, 3))
		if e != nil {
			t.Fatal(e)
		}
		var pieces []Use
		for j, q := range []ExactBezierUV{left, middle, last} {
			start, e := q.EvalUV(0)
			if e != nil {
				t.Fatal(e)
			}
			end, e := q.EvalUV(1)
			if e != nil {
				t.Fatal(e)
			}
			pieces = append(pieces, Use{Edge: 200 + j, UVStart: start, UVEnd: end, Curve: q})
		}
		loop := append([]Use(nil), b.faces[0].Outer[:i]...)
		loop = append(loop, pieces...)
		loop = append(loop, b.faces[0].Outer[i+1:]...)
		b.faces[0].Outer = loop
		return a, b
	}
	t.Fatal("missing curved original use")
	return a, b
}
func TestSplineFaceCoincidenceRefinedCubic(t *testing.T) {
	for _, swap := range []bool{false, true} {
		a, b := refinedCoincidenceFixture(t, swap)
		r, e := a.CertifySplineFaceCoincidence(context.Background(), 0, b, 0, SplineFaceCoincidenceOptions{})
		if e != nil || !r.Complete || len(r.Uses) != 10 {
			t.Fatal(r, e)
		}
		var pieces []CoincidentSplineUse
		for _, p := range r.Uses {
			if p.EdgeA == 10 {
				pieces = append(pieces, p)
			}
		}
		if len(pieces) != 3 {
			t.Fatal(pieces)
		}
		for i, p := range pieces {
			if p.RangeA[0].Cmp([]*big.Rat{new(big.Rat), big.NewRat(1, 4), big.NewRat(3, 4)}[i]) != 0 || p.RangeA[1].Cmp([]*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4), big.NewRat(1, 1)}[i]) != 0 || p.EdgeB < 200 || p.EdgeB > 202 {
				t.Fatal("missing exact coverage/owners", pieces)
			}
			if p.RangeB[0].Cmp(coincidentTraversalRange(r.Map.Orientation())[0]) != 0 {
				t.Fatal("wrong orientation")
			}
		}
		// Both adjacent original pieces retain their separate IDs at a cut.
		if pieces[0].EdgeB == pieces[1].EdgeB || pieces[0].RangeA[1].Cmp(pieces[1].RangeA[0]) != 0 {
			t.Fatal("lost join")
		}
	}
}
func TestSplineFaceCoincidenceRefinementRejects(t *testing.T) {
	a, b := refinedCoincidenceFixture(t, false)
	for i, u := range b.faces[0].Outer {
		if u.Edge == 201 {
			q := u.Curve.(ExactBezierUV)
			h := q.HomogeneousControls()
			h[1][1].Add(h[1][1], big.NewRat(1, 128))
			b.faces[0].Outer[i].Curve, _ = NewExactBezierUV(h)
		}
	}
	r, e := a.CertifySplineFaceCoincidence(context.Background(), 0, b, 0, SplineFaceCoincidenceOptions{})
	if e == nil || r.Complete || len(r.Uses) > 0 {
		t.Fatal("false interior identity", r, e)
	}
	a, b = refinedCoincidenceFixture(t, false)
	for i, u := range b.faces[0].Outer {
		if u.Edge == 201 {
			b.faces[0].Outer = append(b.faces[0].Outer[:i], b.faces[0].Outer[i+1:]...)
			break
		}
	}
	if r, e = a.CertifySplineFaceCoincidence(context.Background(), 0, b, 0, SplineFaceCoincidenceOptions{}); e == nil || r.Complete {
		t.Fatal("gap", r, e)
	}
	a, b = refinedCoincidenceFixture(t, false)
	if r, e = b.CertifySplineFaceCoincidence(context.Background(), 0, a, 0, SplineFaceCoincidenceOptions{}); e == nil || r.Complete {
		t.Fatal("unsupported reverse refinement", r, e)
	}
}
