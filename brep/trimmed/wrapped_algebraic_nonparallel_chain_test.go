package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func wrappedBentSupport(t *testing.T, kind int) *geom.WrappedSurfacePatch {
	t.Helper()
	profile := []geom.Vec2{{X: 2}, {X: 2, Y: 2}, {Y: 2}, {X: -2, Y: 2}, {X: -2}, {X: -2, Y: -2}, {Y: -2}, {X: 2, Y: -2}, {X: 2}}
	lo, hi := -1., 1.
	if kind == 1 {
		hi = 2
	}
	if kind == 2 {
		lo, hi = -2, 2
	}
	var poles []geom.Vec3
	for _, p := range profile {
		for j, v := range []float64{lo, (lo + hi) / 2, hi} {
			x, z := v, v
			if kind == 0 {
				square := []float64{1, -1, 1}[j]
				x = p.X * square / 8
			} else if kind == 2 {
				x = 1 + v
				z = 1 + 2*v
			}
			poles = append(poles, geom.Vec3{X: x, Y: p.Y, Z: z})
		}
	}
	s, e := geom.NewBSplineSurface(2, 2, 9, 3, poles, []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}, []float64{lo, lo, lo, hi, hi, hi}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(context.Background(), p, geom.WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	return w
}
func wrappedNonparallelFixture(t *testing.T) (*WrappedAlgebraicOpenShell, *WrappedAlgebraicOpenShell, [2]WrappedCoordinateSeparator, WrappedCoordinateSeparator) {
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
		faces[i], e = NewWrappedAlgebraicQuadraticFace(ctx, wrappedBentSupport(t, i), loops[i], nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
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
	first := WrappedCoordinateSeparator{Axis: 2, Level: new(big.Rat), ParameterAxes: [2]int{1, 1}, Scale: [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}, Offset: [2]*big.Rat{new(big.Rat), new(big.Rat)}}
	second := WrappedCoordinateSeparator{Axis: 0, Level: big.NewRat(1, 1), ParameterAxes: [2]int{1, 1}, Scale: [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}, Offset: [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}}
	outer := WrappedCoordinateSeparator{Axis: 2, Level: big.NewRat(1, 2), ParameterAxes: [2]int{1, 1}, Scale: [2]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1)}, Offset: [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}}
	return ab, bc, [2]WrappedCoordinateSeparator{first, second}, outer
}
func TestWrappedAlgebraicNonparallelChain(t *testing.T) {
	ctx := context.Background()
	ab, bc, h, outer := wrappedNonparallelFixture(t)
	s, e := NewWrappedAlgebraicNonparallelChain(ctx, ab, bc, h, outer, geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Vertices()) != 8 || len(s.Edges()) != 10 || len(s.Faces()) != 3 || len(s.Boundary()) != 8 {
		t.Fatal("indexed native chain")
	}
	a, _ := s.Adjacent(0)
	b, _ := s.Adjacent(1)
	if a.Separator().Axis != 2 || b.Separator().Axis != 0 {
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
	if q, e := NewWrappedAlgebraicNonparallelChain(ctx, ab, bc, h, outer, geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: s.ExactOperations() - 1}}); q != nil || e == nil {
		t.Fatal("atomic cumulative budget")
	}
}
func TestWrappedAlgebraicNonparallelChainRejects(t *testing.T) {
	ctx := context.Background()
	ab, bc, h, outer := wrappedNonparallelFixture(t)
	for _, level := range []*big.Rat{big.NewRat(1, 1), big.NewRat(-2, 1)} {
		bad := outer
		bad.Level = level
		if p, e := NewWrappedAlgebraicNonparallelChain(ctx, ab, bc, h, bad, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
			t.Fatal("touching or same-side outer witness")
		}
	}
	bad := outer
	bad.Scale = [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}
	if p, e := NewWrappedAlgebraicNonparallelChain(ctx, ab, bc, h, bad, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("unproved outer affine identity")
	}
	other, _, _, _ := wrappedNonparallelFixture(t)
	if p, e := NewWrappedAlgebraicNonparallelChain(ctx, other, bc, h, outer, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("unbound original middle")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if p, e := NewWrappedAlgebraicNonparallelChain(canceled, ab, bc, h, outer, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("cancel")
	}
}

func TestWrappedAlgebraicNonparallelChainReversed(t *testing.T) {
	ctx := context.Background()
	ab, bc, h, outer := wrappedNonparallelFixture(t)
	reversePair := func(p *WrappedAlgebraicOpenShell) *WrappedAlgebraicOpenShell {
		a, _ := p.pair.Owner(1)
		b, _ := p.pair.Owner(0)
		q, e := NewWrappedAlgebraicOpenShell(ctx, p.sources[1], p.sources[0], [2]int{a.Loop, a.Edge}, [2]int{b.Loop, b.Edge}, p.pair.ReversedParameter(), edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		return q
	}
	swap := func(q WrappedCoordinateSeparator) WrappedCoordinateSeparator {
		q.ParameterAxes[0], q.ParameterAxes[1] = q.ParameterAxes[1], q.ParameterAxes[0]
		q.Scale[0], q.Scale[1] = q.Scale[1], q.Scale[0]
		q.Offset[0], q.Offset[1] = q.Offset[1], q.Offset[0]
		return q
	}
	s, e := NewWrappedAlgebraicNonparallelChain(ctx, reversePair(bc), reversePair(ab), [2]WrappedCoordinateSeparator{swap(h[1]), swap(h[0])}, swap(outer), geom.SurfaceAffineCoordinateOptions{})
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
