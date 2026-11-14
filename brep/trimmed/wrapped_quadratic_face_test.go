package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func wrappedQuadraticLoop(t *testing.T) []ExactQuadraticUV {
	t.Helper()
	pts := [][2]*big.Rat{{big.NewRat(-1, 4), big.NewRat(-1, 1)}, {big.NewRat(1, 4), big.NewRat(-1, 1)}, {big.NewRat(1, 4), new(big.Rat)}, {big.NewRat(-1, 4), new(big.Rat)}}
	out := make([]ExactQuadraticUV, 4)
	var e error
	for i := range out {
		out[i], e = quadraticLine(pts[i], pts[(i+1)%4])
		if e != nil {
			t.Fatal(e)
		}
	}
	out[2], e = NewExactQuadraticUV([3][3]*big.Rat{{big.NewRat(1, 1), new(big.Rat), big.NewRat(4, 1)}, {new(big.Rat), big.NewRat(2, 1), big.NewRat(2, 1)}, {big.NewRat(-1, 4), new(big.Rat), big.NewRat(1, 1)}})
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func TestWrappedQuadraticFace(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	loop := wrappedQuadraticLoop(t)
	o := WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}}
	r, e := NewWrappedQuadraticFace(ctx, w, loop, nil, o)
	if e != nil {
		t.Fatal(e)
	}
	f := r.NativeFace()
	if f.Face.Surface != w || len(f.Edges) != 4 || len(f.Face.Outer) != 4 {
		t.Fatal("authored edge split or support replaced")
	}
	p, e := r.Composition(0, 2)
	if e != nil || !p.Complete || len(p.Pieces) != 2 || p.Pieces[0].End.Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal("exact seam ancestry", e)
	}
	p.Pieces[0].End.SetInt64(99)
	again, _ := r.Composition(0, 2)
	if again.Pieces[0].End.Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal("mutable proof")
	}
	open := Solid{vertices: f.Vertices, edges: f.Edges, faces: []Face{f.Face}, tolerance: 1e-9}
	agreement, e := open.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !agreement.Complete {
		t.Fatal("native continuous model", e)
	}
	// Independent exact original UV and polynomial profile at rational samples.
	for _, s := range []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(2, 3), big.NewRat(1, 1)} {
		one := new(big.Rat).Sub(big.NewRat(1, 1), s)
		den := new(big.Rat).Mul(new(big.Rat).Sub(big.NewRat(2, 1), s), new(big.Rat).Sub(big.NewRat(2, 1), s))
		u := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(one, one), new(big.Rat).Mul(big.NewRat(1, 4), new(big.Rat).Mul(s, s))), den)
		v := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(s, one)), den)
		x := new(big.Rat).Sub(big.NewRat(2, 1), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(u, u)))
		y := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Abs(u)), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(u, u)))
		if u.Sign() < 0 {
			y.Neg(y)
		}
		pieces, _ := r.Composition(0, 2)
		for _, part := range pieces.Pieces {
			if s.Cmp(part.Start) < 0 || s.Cmp(part.End) > 0 {
				continue
			}
			local := new(big.Rat).Quo(new(big.Rat).Sub(s, part.Start), new(big.Rat).Sub(part.End, part.Start))
			net, _, e := part.Curve.BezierControlNet(ctx, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for level := len(net.Control) - 1; level > 0; level-- {
				for j := 0; j < level; j++ {
					for axis := 0; axis < 4; axis++ {
						net.Control[j][axis] = new(big.Rat).Add(new(big.Rat).Mul(new(big.Rat).Sub(big.NewRat(1, 1), local), net.Control[j][axis]), new(big.Rat).Mul(local, net.Control[j+1][axis]))
					}
				}
			}
			for axis, want := range [3]*big.Rat{x, y, v} {
				if new(big.Rat).Quo(net.Control[0][axis], net.Control[0][3]).Cmp(want) != 0 {
					t.Fatal("independent rational model", s, axis)
				}
			}
		}
	}
	refined, _, e := w.InsertKnot(ctx, 0, .125, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	same, _, e := compareWrappedQuadraticUse(ctx, f.Edges[2], refined, f.Face.Outer[2], edgecheck.Options{})
	if e != nil || !same {
		t.Fatal("independent knot basis", e)
	}
	reversedUse := f.Face.Outer[2]
	reversedUse.Curve = reversePlaneQuadratic(loop[2])
	reversedUse.UVStart, reversedUse.UVEnd = reversedUse.UVEnd, reversedUse.UVStart
	reversedUse.Reversed = true
	same, _, e = compareWrappedQuadraticUse(ctx, f.Edges[2], refined, reversedUse, edgecheck.Options{})
	if e != nil || !same {
		t.Fatal("reversed whole parameter proof", e)
	}
	changed := loop[2].HomogeneousControls()
	changed[1][1].Add(changed[1][1], big.NewRat(1, 16))
	bad, e := NewExactQuadraticUV(changed)
	if e != nil {
		t.Fatal(e)
	}
	use := f.Face.Outer[2]
	use.Curve = bad
	same, _, e = compareWrappedQuadraticUse(ctx, f.Edges[2], w, use, edgecheck.Options{})
	if e != nil || same {
		t.Fatal("matched endpoints hid different curve", e)
	}
	low := o
	low.MaxOperations = r.ExactOperations() - 1
	if q, e := NewWrappedQuadraticFace(ctx, w, loop, nil, low); e == nil || q != nil {
		t.Fatal("atomic quota")
	}
	low = o
	low.MaxPieces = 1
	if q, e := NewWrappedQuadraticFace(ctx, w, loop, nil, low); e == nil || q != nil {
		t.Fatal("atomic piece cap")
	}
	cancel, stop := context.WithCancel(ctx)
	stop()
	if q, e := NewWrappedQuadraticFace(cancel, w, loop, nil, o); e == nil || q != nil {
		t.Fatal("atomic cancel")
	}
}

func TestWrappedQuadraticFaceHoleAndIrrational(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	loop := wrappedQuadraticLoop(t)
	o := WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}}
	p := [][2]*big.Rat{{big.NewRat(-1, 16), big.NewRat(-3, 4)}, {big.NewRat(-1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-3, 4)}}
	var hole []ExactQuadraticUV
	for i, a := range p {
		q, e := quadraticLine(a, p[(i+1)%len(p)])
		if e != nil {
			t.Fatal(e)
		}
		hole = append(hole, q)
	}
	r, e := NewWrappedQuadraticFace(ctx, w, loop, [][]ExactQuadraticUV{hole}, o)
	if e != nil {
		t.Fatal(e)
	}
	f := r.NativeFace()
	open := Solid{vertices: f.Vertices, edges: f.Edges, faces: []Face{f.Face}, tolerance: 1e-9}
	proof, e := open.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal("curved loop plus seam hole", e)
	}
	controls := [3][3]*big.Rat{{big.NewRat(1, 4), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 8), big.NewRat(1, 1), big.NewRat(1, 1)}, {big.NewRat(-1, 4), new(big.Rat), big.NewRat(1, 1)}}
	loop[2], e = NewExactQuadraticUV(controls)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := NewWrappedQuadraticFace(ctx, w, loop, nil, o); e == nil || r != nil {
		t.Fatal("irrational seam native publication")
	}
}
