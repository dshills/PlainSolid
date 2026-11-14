package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func wrappedCycleArea(cycle []Use) *big.Rat {
	area := new(big.Rat)
	for _, u := range cycle {
		area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(ratInset(u.UVStart.X), ratInset(u.UVEnd.Y)), new(big.Rat).Mul(ratInset(u.UVStart.Y), ratInset(u.UVEnd.X))))
	}
	return area.Quo(area, big.NewRat(2, 1))
}

func TestWrappedFaceRegionDifferenceHoleAndLift(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	refined, _, e := w.InsertKnot(ctx, 0, .125, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	opts := WrappedRectangleFaceOverlapOptions{ProjectionAxes: [2]int{1, 2}}
	for _, reversed := range []bool{false, true} {
		a := wrappedOverlapFace(t, w, geom.ParameterRectangle{UMin: 4.125, UMax: 4.5, VMin: -1, VMax: 2})
		a.Face.Reversed = reversed
		b := wrappedOverlapFace(t, refined, geom.ParameterRectangle{UMin: .25, UMax: .375, VMin: 0, VMax: 1})
		b.Face.Reversed = !reversed
		model := b.Edges[1].Curve.(surfaceTrimCurve)
		model.a, model.b = model.b, model.a
		b.Edges[1].Curve = model
		b.Edges[1].Start, b.Edges[1].End = b.Edges[1].End, b.Edges[1].Start
		b.Face.Outer[1].Reversed = true
		r, e := NewWrappedFaceRegionTopology(ctx, a, b, WrappedFaceRegionDifference, opts)
		if e != nil || r.Empty() {
			t.Fatal(r, e)
		}
		cycles := r.BoundaryCycles()
		if len(cycles) != 2 || len(r.Faces()) != 8 || len(r.Vertices()) != 16 || len(r.Edges()) != 24 {
			t.Fatal("wrong annular graph")
		}
		outer, hole := wrappedCycleArea(cycles[0]), wrappedCycleArea(cycles[1])
		if reversed {
			outer.Neg(outer)
			hole.Neg(hole)
		}
		if outer.Cmp(big.NewRat(9, 8)) != 0 || hole.Cmp(big.NewRat(-1, 8)) != 0 {
			t.Fatal("wrong hole orientation", outer, hole)
		}
		if r.SourceOrientations() != [2]bool{reversed, !reversed} {
			t.Fatal("orientation ancestry")
		}
		for _, c := range r.CellAncestry() {
			if c.Owners != 1 {
				t.Fatal("removed B interior retained")
			}
		}
		ancestry := r.EdgeAncestry()
		for _, u := range cycles[1] {
			list := ancestry[u.Edge]
			if len(list) != 1 || list[0].Face != 1 {
				t.Fatal("lost subtracted B boundary", list)
			}
			p := list[0].Parameters
			if u.Reversed {
				p[0], p[1] = p[1], p[0]
			}
			start, end := big.NewRat(1, 1), big.NewRat(0, 1)
			if reversed {
				start, end = end, start
			}
			if b.Face.Outer[list[0].Use].Reversed {
				start, end = end, start
			}
			if p[0].Cmp(start) != 0 || p[1].Cmp(end) != 0 {
				t.Fatal("wrong removed-owner edge direction", p)
			}
		}
		cycles[1][0].Edge = 999
		if r.BoundaryCycles()[1][0].Edge == 999 {
			t.Fatal("mutable boundary storage")
		}
		bad := opts
		bad.MaxOperations = r.ExactOperations() - 1
		if out, e := NewWrappedFaceRegionTopology(ctx, a, b, WrappedFaceRegionDifference, bad); e == nil || out != nil {
			t.Fatal("partial difference", e)
		}
	}
	// Partial overlap between independently lifted occurrences. The complete
	// continuous exterior remains in A's [4.125,4.375] chart, with B mapped by +4.
	a := wrappedOverlapFace(t, w, geom.ParameterRectangle{UMin: 4.125, UMax: 4.375, VMin: -1, VMax: 1})
	b := wrappedOverlapFace(t, refined, geom.ParameterRectangle{UMin: .25, UMax: .5, VMin: 0, VMax: 2})
	r, e := NewWrappedFaceRegionTopology(ctx, a, b, WrappedFaceRegionDifference, opts)
	if e != nil {
		t.Fatal(e)
	}
	if r.Empty() || len(r.BoundaryCycles()) != 1 || len(r.Faces()) != 3 || wrappedCycleArea(r.Exterior()).Cmp(big.NewRat(3, 8)) != 0 {
		t.Fatal("partial lifted difference")
	}
	for i, u := range r.Exterior() {
		next := r.Exterior()[(i+1)%len(r.Exterior())]
		if u.UVEnd != next.UVStart || u.UVStart.X < 4 {
			t.Fatal("lift discontinuity")
		}
	}
	// Empty is a successful exact result, unlike unresolved disconnected output.
	empty, e := NewWrappedFaceRegionTopology(ctx, a, a, WrappedFaceRegionDifference, opts)
	if e != nil || !empty.Empty() || len(empty.Faces()) != 0 || len(empty.Edges()) != 0 || len(empty.BoundaryCycles()) != 0 {
		t.Fatal(empty, e)
	}
	containing := wrappedOverlapFace(t, refined, geom.ParameterRectangle{UMin: 0, UMax: .5, VMin: -2, VMax: 2})
	empty, e = NewWrappedFaceRegionTopology(ctx, a, containing, WrappedFaceRegionDifference, opts)
	if e != nil || !empty.Empty() {
		t.Fatal("strictly contained A not empty", e)
	}
	var nilGraph *WrappedFaceRegionTopology
	if nilGraph.Empty() {
		t.Fatal("nil treated as empty proof")
	}
	a = wrappedOverlapFace(t, w, geom.ParameterRectangle{UMin: .125, UMax: .5, VMin: 0, VMax: 1})
	b = wrappedOverlapFace(t, refined, geom.ParameterRectangle{UMin: .25, UMax: .375, VMin: -1, VMax: 2})
	if out, e := NewWrappedFaceRegionTopology(ctx, a, b, WrappedFaceRegionDifference, opts); e == nil || out != nil {
		t.Fatal("unsupported disconnected difference published", e)
	}
	// Truly straddling the fundamental seam remains unresolved by the unchanged
	// embedding guard; using different integer-period lifts above does not relax it.
	a = wrappedOverlapFace(t, w, geom.ParameterRectangle{UMin: -.125, UMax: .125, VMin: 0, VMax: 1})
	b = wrappedOverlapFace(t, refined, geom.ParameterRectangle{UMin: 0, UMax: .25, VMin: .5, VMax: 2})
	if out, e := NewWrappedFaceRegionTopology(ctx, a, b, WrappedFaceRegionDifference, opts); e == nil || out != nil {
		t.Fatal("unproved true seam accepted", e)
	}
}

func TestWrappedBoundaryCyclesRejectMalformedHole(t *testing.T) {
	rect := func(x0, x1, y0, y1 float64, reverse bool) []Use {
		p := []geom.Vec2{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
		if reverse {
			p[1], p[3] = p[3], p[1]
		}
		out := make([]Use, 4)
		for i := range out {
			out[i] = Use{UVStart: p[i], UVEnd: p[(i+1)%4]}
		}
		return out
	}
	outer := rect(0, 4, 0, 4, false)
	for name, hole := range map[string][]Use{"wrong winding": rect(1, 2, 1, 2, false), "outside": rect(5, 6, 1, 2, true), "touch": rect(0, 2, 1, 2, true), "cross": rect(3, 5, 1, 2, true)} {
		if _, e := validateWrappedBoundaryCycles([][]Use{outer, hole}, false); e == nil {
			t.Fatal(name)
		}
	}
	hole := rect(1, 2, 1, 2, true)
	hole[1].UVEnd.X += .5
	if _, e := validateWrappedBoundaryCycles([][]Use{outer, hole}, false); e == nil {
		t.Fatal("broken loop accepted")
	}
	if _, e := validateWrappedBoundaryCycles([][]Use{rect(1, 2, 1, 2, true), outer}, false); e != nil {
		t.Fatal("deterministic exterior ordering", e)
	}
}
