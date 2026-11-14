package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func wrappedCovectorChainFixture(t *testing.T) (*WrappedAlgebraicOpenShell, *WrappedAlgebraicOpenShell, [2]WrappedCovectorSeparator, WrappedCovectorSeparator) {
	t.Helper()
	ctx := context.Background()
	loops := [][]ExactQuadraticUV{wrappedBandLoop(t, -.25, .25, -1, 0), wrappedBandLoop(t, 3.75, 4.25, 0, 1), wrappedBandLoop(t, 7.75, 8.25, 0, 1)}
	q, e := NewExactQuadraticUV([3][3]*big.Rat{{big.NewRat(1, 4), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 4), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(-1, 4), new(big.Rat), big.NewRat(1, 1)}})
	if e != nil {
		t.Fatal(e)
	}
	loops[0][2] = q
	h := reversePlaneQuadratic(q).HomogeneousControls()
	for i := range h {
		h[i][0].Add(h[i][0], new(big.Rat).Mul(big.NewRat(4, 1), h[i][2]))
	}
	loops[1][0], e = NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	var faces [3]*WrappedAlgebraicQuadraticFace
	for i := range faces {
		original := wrappedBentSupport(t, i)
		moved, _, err := original.Transformed(ctx, geom.Affine{{1, .125, 0, 0}, {0, 1, 0, 0}, {0, .125, 1, 0}}, geom.WrappedSurfaceOptions{})
		if err != nil {
			t.Fatal(err)
		}
		faces[i], e = NewWrappedAlgebraicQuadraticFace(ctx, moved, loops[i], nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
		if e != nil {
			t.Fatal(e)
		}
	}
	ab, e := NewWrappedAlgebraicOpenShell(ctx, faces[0], faces[1], [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	bc, e := NewWrappedAlgebraicOpenShell(ctx, faces[1], faces[2], [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	first := WrappedCovectorSeparator{Normal: [3]*big.Rat{new(big.Rat), big.NewRat(-1, 8), big.NewRat(1, 1)}, Level: new(big.Rat), ParameterAxes: [2]int{1, 1}, Scale: [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}, Offset: [2]*big.Rat{new(big.Rat), new(big.Rat)}}
	second := WrappedCovectorSeparator{Normal: [3]*big.Rat{big.NewRat(1, 1), big.NewRat(-1, 8), new(big.Rat)}, Level: big.NewRat(1, 1), ParameterAxes: [2]int{1, 1}, Scale: [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}, Offset: [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}}
	outer := WrappedCovectorSeparator{Normal: [3]*big.Rat{new(big.Rat), big.NewRat(-1, 8), big.NewRat(1, 1)}, Level: big.NewRat(1, 2), ParameterAxes: [2]int{1, 1}, Scale: [2]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1)}, Offset: [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}}
	return ab, bc, [2]WrappedCovectorSeparator{first, second}, outer
}
func TestWrappedAlgebraicCovectorChain(t *testing.T) {
	ctx := context.Background()
	ab, bc, h, outer := wrappedCovectorChainFixture(t)
	s, e := NewWrappedAlgebraicCovectorChain(ctx, ab, bc, h, outer, geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Vertices()) != 8 || len(s.Edges()) != 10 || len(s.Faces()) != 3 || len(s.Boundary()) != 8 {
		t.Fatal("indexed native chain")
	}
	a, _ := s.Adjacent(0)
	b, _ := s.Adjacent(1)
	if a.Separator().Normal[1].Cmp(big.NewRat(-1, 8)) != 0 || b.Separator().Normal[1].Cmp(big.NewRat(-1, 8)) != 0 {
		t.Fatal("nonparallel original planes")
	}
	margins := s.OuterSeparation().Margins()
	if s.OuterSeparation().Sides() != ([2]int{-1, 1}) || margins[0].Cmp(big.NewRat(1, 2)) != 0 || margins[1].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("strict entire retained-face margins")
	}
	margins[0].SetInt64(99)
	if s.OuterSeparation().Margins()[0].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("mutable margin")
	}
	native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
	r, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	composition, _ := ab.Pair().Composition(0)
	piece, _ := composition.Piece(0)
	root, _ := piece.Model.EndpointRoot(1)
	kind, e := root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
	if e != nil || kind.Kind != geom.ExactRootKindIrrational {
		t.Fatal("irrational original seam", e)
	}
	// A is genuinely curved away from its planar shared edge: X=V²/4 at U=0.
	w := ab.sources[0].NativeFace().Face.Surface.(*geom.WrappedSurfacePatch)
	point, e := w.EvaluateRational(ctx, new(big.Rat), big.NewRat(-1, 2), geom.SurfacePatchEvaluationOptions{})
	if e != nil || point.Point[0].Cmp(big.NewRat(1, 16)) != 0 {
		t.Fatal("independent curved support", e)
	}
	if q, e := NewWrappedAlgebraicCovectorChain(ctx, ab, bc, h, outer, geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: s.ExactOperations() - 1}}); q != nil || e == nil {
		t.Fatal("atomic cumulative budget")
	}
}
func TestWrappedAlgebraicCovectorChainRejects(t *testing.T) {
	ctx := context.Background()
	ab, bc, h, outer := wrappedCovectorChainFixture(t)
	for _, level := range []*big.Rat{big.NewRat(1, 1), big.NewRat(-2, 1)} {
		bad := outer
		bad.Level = level
		if p, e := NewWrappedAlgebraicCovectorChain(ctx, ab, bc, h, bad, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
			t.Fatal("touching or same-side outer witness")
		}
	}
	bad := outer
	bad.Scale = [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}
	if p, e := NewWrappedAlgebraicCovectorChain(ctx, ab, bc, h, bad, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("unproved outer affine identity")
	}
	other, _, _, _ := wrappedCovectorChainFixture(t)
	if p, e := NewWrappedAlgebraicCovectorChain(ctx, other, bc, h, outer, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("unbound original middle")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if p, e := NewWrappedAlgebraicCovectorChain(canceled, ab, bc, h, outer, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("cancel")
	}
}

func TestWrappedAlgebraicCovectorChainMateAndUnits(t *testing.T) {
	ctx := context.Background()
	ab, bc, h, outer := wrappedCovectorChainFixture(t)
	bad := h
	bad[1].Level = new(big.Rat).Add(h[1].Level, big.NewRat(1, 1024))
	if p, e := NewWrappedAlgebraicCovectorChain(ctx, ab, bc, bad, outer, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("mate off proposed plane")
	}
	// A negative rational rescaling changes scalar sides and margins, not the plane.
	factor := big.NewRat(-3, 7)
	for i := range outer.Normal {
		outer.Normal[i] = new(big.Rat).Mul(outer.Normal[i], factor)
	}
	outer.Level = new(big.Rat).Mul(outer.Level, factor)
	for i := range outer.Scale {
		outer.Scale[i] = new(big.Rat).Mul(outer.Scale[i], factor)
		outer.Offset[i] = new(big.Rat).Mul(outer.Offset[i], factor)
	}
	p, e := NewWrappedAlgebraicCovectorChain(ctx, ab, bc, h, outer, geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if p.OuterSeparation().Sides() != ([2]int{1, -1}) || p.OuterSeparation().Margins()[0].Cmp(big.NewRat(3, 14)) != 0 {
		t.Fatal("raw covector units")
	}
	outer.Normal[0].SetInt64(99)
	if p.OuterSeparation().Separator().Normal[0].Sign() != 0 {
		t.Fatal("aliased input covector")
	}
}

func TestWrappedAlgebraicCovectorChainReversed(t *testing.T) {
	ctx := context.Background()
	ab, bc, h, outer := wrappedCovectorChainFixture(t)
	reversePair := func(p *WrappedAlgebraicOpenShell) *WrappedAlgebraicOpenShell {
		a, _ := p.pair.Owner(1)
		b, _ := p.pair.Owner(0)
		q, e := NewWrappedAlgebraicOpenShell(ctx, p.sources[1], p.sources[0], [2]int{a.Loop, a.Edge}, [2]int{b.Loop, b.Edge}, p.pair.ReversedParameter(), edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		return q
	}
	swap := func(q WrappedCovectorSeparator) WrappedCovectorSeparator {
		q.ParameterAxes[0], q.ParameterAxes[1] = q.ParameterAxes[1], q.ParameterAxes[0]
		q.Scale[0], q.Scale[1] = q.Scale[1], q.Scale[0]
		q.Offset[0], q.Offset[1] = q.Offset[1], q.Offset[0]
		return q
	}
	s, e := NewWrappedAlgebraicCovectorChain(ctx, reversePair(bc), reversePair(ab), [2]WrappedCovectorSeparator{swap(h[1]), swap(h[0])}, swap(outer), geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if s.OuterSeparation().Sides() != ([2]int{1, -1}) {
		t.Fatal("original world halfspaces")
	}
	native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
	r, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
}
