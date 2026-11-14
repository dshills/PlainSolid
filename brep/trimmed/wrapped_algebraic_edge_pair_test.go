package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func wrappedAlgebraicMateFaces(t *testing.T, wrongCurve, reverseFaceB bool) (*WrappedAlgebraicQuadraticFace, *WrappedAlgebraicQuadraticFace) {
	t.Helper()
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	opts := WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}}
	a, e := NewWrappedAlgebraicQuadraticFace(ctx, w, wrappedAlgebraicLoop(t), nil, opts)
	if e != nil {
		t.Fatal(e)
	}
	refined, _, e := w.InsertKnot(ctx, 0, .125, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	refined, _, e = refined.InsertKnot(ctx, 1, .375, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	points := [][2]*big.Rat{{big.NewRat(-1, 4), new(big.Rat)}, {big.NewRat(1, 4), new(big.Rat)}, {big.NewRat(3, 8), big.NewRat(1, 1)}, {big.NewRat(-3, 8), big.NewRat(1, 1)}}
	var loop []ExactQuadraticUV
	for i, p := range points {
		q, e := quadraticLine(p, points[(i+1)%4])
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, q)
	}
	loop[0] = reversePlaneQuadratic(wrappedAlgebraicLoop(t)[2])
	for i, q := range loop {
		h := q.HomogeneousControls()
		for j := range h {
			h[j][0].Add(h[j][0], new(big.Rat).Mul(big.NewRat(4, 1), h[j][2]))
		}
		if i == 0 && wrongCurve {
			h[1][1].Add(h[1][1], big.NewRat(1, 16))
		}
		loop[i], e = NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
	}
	opts.Reversed = reverseFaceB
	b, e := NewWrappedAlgebraicQuadraticFace(ctx, refined, loop, nil, opts)
	if e != nil {
		t.Fatal(e)
	}
	return a, b
}
func TestWrappedAlgebraicEdgePair(t *testing.T) {
	ctx := context.Background()
	a, b := wrappedAlgebraicMateFaces(t, false, false)
	pair, e := CertifyWrappedAlgebraicEdgePair(ctx, a, b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil || !pair.Proven() || !pair.ReversedParameter() {
		t.Fatal(pair, e)
	}
	first, _ := pair.Owner(0)
	second, _ := pair.Owner(1)
	if first.NativeEdge != 2 || second.NativeEdge != 0 || first.UV[0][0].Cmp(big.NewRat(1, 4)) != 0 || second.UV[0][0].Cmp(big.NewRat(15, 4)) != 0 {
		t.Fatal("source occurrence", first, second)
	}
	x, _ := pair.Composition(0)
	y, _ := pair.Composition(1)
	if x.Len() == y.Len() {
		t.Fatal("fixture partitions not independent")
	}
	piece, _ := x.Piece(0)
	root, _ := piece.Model.EndpointRoot(1)
	kind, e := root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
	if e != nil || kind.Kind != geom.ExactRootKindIrrational {
		t.Fatal("missing irrational seam", kind, e)
	}
	for side := 0; side < 2; side++ {
		point, vertices, e := pair.Endpoint(side)
		if e != nil {
			t.Fatal(e)
		}
		wantY := big.NewRat(7, 8)
		if side == 1 {
			wantY.Neg(wantY)
		}
		if point[0].Cmp(big.NewRat(15, 8)) != 0 || point[1].Cmp(wantY) != 0 || point[2].Sign() != 0 || vertices != ([2]int{2 + side, 1 - side}) {
			t.Fatal("native incidence", point, vertices)
		}
		point[0].SetInt64(99)
	}
	first.UV[0][0].SetInt64(99)
	again, _ := pair.Owner(0)
	point, _, _ := pair.Endpoint(0)
	if again.UV[0][0].Cmp(big.NewRat(1, 4)) != 0 || point[0].Cmp(big.NewRat(15, 8)) != 0 {
		t.Fatal("mutable certificate")
	}
	for _, options := range []edgecheck.Options{{MaxWork: pair.ExactOperations() - 1}, {MaxIntervals: 1}} {
		if p, e := CertifyWrappedAlgebraicEdgePair(ctx, a, b, [2]int{0, 2}, [2]int{0, 0}, true, options); e == nil || p != nil {
			t.Fatal("atomic limit")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if p, e := CertifyWrappedAlgebraicEdgePair(canceled, a, b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{}); e == nil || p != nil {
		t.Fatal("cancellation")
	}
	// Reversing both faces retains opposite incidence on the same shared edge.
	ar, br := wrappedAlgebraicMateFaces(t, false, true)
	ar, e = NewWrappedAlgebraicQuadraticFace(ctx, ar.NativeFace().Face.Surface.(*geom.WrappedSurfacePatch), ar.OriginalLoops()[0], nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: true})
	if e != nil {
		t.Fatal(e)
	}
	flipped, e := CertifyWrappedAlgebraicEdgePair(ctx, ar, br, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil || !flipped.Proven() {
		t.Fatal("reversed face pair", e)
	}
	if (&WrappedAlgebraicEdgePair{}).Proven() {
		t.Fatal("empty certificate")
	}
}
func TestWrappedAlgebraicEdgePairRejects(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct{ wrongCurve, reverseFaceB, reverseParameter bool }{{true, false, true}, {false, true, true}, {false, false, false}} {
		a, b := wrappedAlgebraicMateFaces(t, tc.wrongCurve, tc.reverseFaceB)
		if pair, e := CertifyWrappedAlgebraicEdgePair(ctx, a, b, [2]int{0, 2}, [2]int{0, 0}, tc.reverseParameter, edgecheck.Options{}); e == nil || pair != nil {
			t.Fatal("invalid mate", tc, pair, e)
		}
	}
	a, b := wrappedAlgebraicMateFaces(t, false, false)
	if pair, e := CertifyWrappedAlgebraicEdgePair(ctx, a, b, [2]int{1, 0}, [2]int{0, 0}, true, edgecheck.Options{}); e == nil || pair != nil {
		t.Fatal("missing owner")
	}
	// Native endpoints must independently agree with their retained proof cells.
	corrupt := *a.face
	corrupt.native = a.NativeFace()
	corrupt.native.Vertices[2].X += .125
	if pair, e := CertifyWrappedAlgebraicEdgePair(ctx, &WrappedAlgebraicQuadraticFace{face: &corrupt}, b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{}); e == nil || pair != nil {
		t.Fatal("unbound vertex")
	}
}

func TestWrappedAlgebraicEdgePairModelBinding(t *testing.T) {
	ctx := context.Background()
	a, b := wrappedAlgebraicMateFaces(t, false, false)
	corrupt := *a.face
	corrupt.native = a.NativeFace()
	marker := corrupt.native.Edges[2].Curve.(wrappedAlgebraicTrimCurve)
	q := marker.curve.(ExactQuadraticUV)
	h := q.HomogeneousControls()
	h[1][1].Add(h[1][1], big.NewRat(1, 16))
	changed, e := NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	marker.curve = changed
	corrupt.native.Edges[2].Curve = marker
	if pair, e := CertifyWrappedAlgebraicEdgePair(ctx, &WrappedAlgebraicQuadraticFace{face: &corrupt}, b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{}); e == nil || pair != nil {
		t.Fatal("cached proof did not bind actual curve")
	}
}
