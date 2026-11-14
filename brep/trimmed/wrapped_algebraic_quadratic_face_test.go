package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func wrappedAlgebraicLoop(t *testing.T) []ExactQuadraticUV {
	t.Helper()
	loop := wrappedQuadraticLoop(t)
	q, e := NewExactQuadraticUV([3][3]*big.Rat{{big.NewRat(1, 4), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 4), big.NewRat(1, 1), big.NewRat(1, 1)}, {big.NewRat(-1, 4), new(big.Rat), big.NewRat(1, 1)}})
	if e != nil {
		t.Fatal(e)
	}
	loop[2] = q
	return loop
}
func TestWrappedAlgebraicQuadraticFace(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	loop := wrappedAlgebraicLoop(t)
	opts := WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}}
	if old, e := NewWrappedQuadraticFace(ctx, w, loop, nil, opts); e == nil || old != nil {
		t.Fatal("strict rational constructor widened")
	}
	for _, reverse := range []bool{false, true} {
		opts.Reversed = reverse
		r, e := NewWrappedAlgebraicQuadraticFace(ctx, w, loop, nil, opts)
		if e != nil {
			t.Fatal(e)
		}
		f := r.NativeFace()
		if f.Face.Surface != w || f.Face.Reversed != reverse || len(f.Vertices) != 4 || len(f.Edges) != 4 || len(f.Face.Outer) != 4 {
			t.Fatal("authored topology changed")
		}
		proof, e := r.Composition(0, 2)
		if e != nil || proof.Len() != 2 {
			t.Fatal(proof, e)
		}
		piece, _ := proof.Piece(0)
		root, _ := piece.Model.EndpointRoot(1)
		kind, e := root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil || kind.Kind != geom.ExactRootKindIrrational {
			t.Fatal(kind, e)
		}
		open := Solid{vertices: f.Vertices, edges: f.Edges, faces: []Face{f.Face}, tolerance: 1e-9}
		agreement, e := open.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !agreement.Complete {
			t.Fatal("native continuous agreement", agreement, e)
		}
		for _, s := range []float64{0, .25, .5, .75, 1} {
			u, v := .25-.5*s*s, 2*s*(1-s)
			want := geom.Vec3{X: 2 - 2*u*u, Y: math.Copysign(4*math.Abs(u)-2*u*u, u), Z: v}
			got, e := f.Edges[2].Curve.Eval(s)
			if e != nil || got.Sub(want).Length() > 1e-13 {
				t.Fatal("original evaluator", got, want, e)
			}
		}
		refined, _, e := w.InsertKnot(ctx, 0, .125, 1, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		same, _, e := compareWrappedQuadraticUse(ctx, f.Edges[2], refined, f.Face.Outer[2], edgecheck.Options{})
		if e != nil || !same {
			t.Fatal("independent source refinement", e)
		}
		reversed := f.Face.Outer[2]
		reversed.Curve = reversePlaneQuadratic(loop[2])
		reversed.UVStart, reversed.UVEnd = reversed.UVEnd, reversed.UVStart
		reversed.Reversed = true
		same, _, e = compareWrappedQuadraticUse(ctx, f.Edges[2], refined, reversed, edgecheck.Options{})
		if e != nil || !same {
			t.Fatal("reversed exact agreement", e)
		}
		h := loop[2].HomogeneousControls()
		h[1][1].Add(h[1][1], big.NewRat(1, 16))
		bad, e := NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
		use := f.Face.Outer[2]
		use.Curve = bad
		same, _, e = compareWrappedQuadraticUse(ctx, f.Edges[2], w, use, edgecheck.Options{})
		if e != nil || same {
			t.Fatal("endpoint-only equality", e)
		}
		low := opts
		low.MaxOperations = r.ExactOperations() - 1
		if q, e := NewWrappedAlgebraicQuadraticFace(ctx, w, loop, nil, low); e == nil || q != nil {
			t.Fatal("atomic work")
		}
	}
	points := [][2]*big.Rat{{big.NewRat(-1, 16), big.NewRat(-3, 4)}, {big.NewRat(-1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-3, 4)}}
	var hole []ExactQuadraticUV
	for i, p := range points {
		q, e := quadraticLine(p, points[(i+1)%4])
		if e != nil {
			t.Fatal(e)
		}
		hole = append(hole, q)
	}
	withHole, e := NewWrappedAlgebraicQuadraticFace(ctx, w, loop, [][]ExactQuadraticUV{hole}, opts)
	if e != nil {
		t.Fatal(e)
	}
	native := withHole.NativeFace()
	if len(native.Face.Holes) != 1 || len(native.Edges) != 8 || len(native.Vertices) != 8 {
		t.Fatal("hole topology")
	}
	open := Solid{vertices: native.Vertices, edges: native.Edges, faces: []Face{native.Face}, tolerance: 1e-9}
	agreement, e := open.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !agreement.Complete {
		t.Fatal("hole agreement", e)
	}
	low := opts
	low.MaxPieces = 1
	if q, e := NewWrappedAlgebraicQuadraticFace(ctx, w, loop, nil, low); e == nil || q != nil {
		t.Fatal("atomic pieces")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := NewWrappedAlgebraicQuadraticFace(canceled, w, loop, nil, opts); e == nil || q != nil {
		t.Fatal("cancel")
	}
}

func TestWrappedAlgebraicQuadraticFaceUnrepresentableEndpoints(t *testing.T) {
	ctx := context.Background()
	points := [][2]*big.Rat{{big.NewRat(-1, 4), big.NewRat(-1, 1)}, {big.NewRat(1, 3), big.NewRat(-1, 1)}, {big.NewRat(1, 3), new(big.Rat)}, {big.NewRat(-1, 4), new(big.Rat)}}
	var loop []ExactQuadraticUV
	for i, a := range points {
		b := points[(i+1)%len(points)]
		h := [3][3]*big.Rat{{a[0], a[1], big.NewRat(1, 1)}, {new(big.Rat).Quo(new(big.Rat).Add(a[0], b[0]), big.NewRat(2, 1)), new(big.Rat).Quo(new(big.Rat).Add(a[1], b[1]), big.NewRat(2, 1)), big.NewRat(1, 1)}, {b[0], b[1], big.NewRat(1, 1)}}
		q, e := NewRationalEndpointQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, q)
	}
	r, e := NewWrappedAlgebraicQuadraticFace(ctx, wrappedOverlapSupport(t), loop, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if !errors.Is(e, geom.ErrUnrepresentable) || r != nil {
		t.Fatal("rounded true endpoint", r, e)
	}
}
