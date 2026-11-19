//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func wrappedThirdBandSupport(t *testing.T, negative bool) *geom.WrappedSurfacePatch {
	t.Helper()
	profile := []geom.Vec2{{X: 2}, {X: 2, Y: 2}, {Y: 2}, {X: -2, Y: 2}, {X: -2}, {X: -2, Y: -2}, {Y: -2}, {X: 2, Y: -2}, {X: 2}}
	var poles []geom.Vec3
	lo, hi := 1., 3.
	scale := .5
	if negative {
		lo, hi = -3, -1
		scale = -.5
	}
	for _, p := range profile {
		for j, v := range []float64{lo, (lo + hi) / 2, hi} {
			x := 3. / 16
			if j == 1 {
				x = -1. / 16
			}
			poles = append(poles, geom.Vec3{X: p.X + x, Y: p.Y, Z: scale * v})
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
func wrappedSeparatedChainFixture(t *testing.T, negative bool) (*WrappedAlgebraicOpenShell, *WrappedAlgebraicOpenShell, [2]WrappedCoordinateSeparator) {
	t.Helper()
	ctx := context.Background()
	ab, h0 := wrappedSeparatedAlgebraicFixture(t)
	lo, hi := 2., 3.
	edge := 0
	rev := true
	scale := big.NewRat(1, 2)
	if negative {
		lo, hi = -3, -2
		edge = 2
		rev = false
		scale = big.NewRat(-1, 2)
	}
	c, e := NewWrappedAlgebraicQuadraticFace(ctx, wrappedThirdBandSupport(t, negative), wrappedBandLoop(t, 7.75, 8.25, lo, hi), nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: negative})
	if e != nil {
		t.Fatal(e)
	}
	bc, e := NewWrappedAlgebraicOpenShell(ctx, ab.sources[1], c, [2]int{0, 2}, [2]int{0, edge}, rev, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	h1 := WrappedCoordinateSeparator{Axis: 2, Level: big.NewRat(1, 1), ParameterAxes: [2]int{1, 1}, Scale: [2]*big.Rat{big.NewRat(2, 1), scale}, Offset: [2]*big.Rat{new(big.Rat), new(big.Rat)}}
	return ab, bc, [2]WrappedCoordinateSeparator{h0, h1}
}
func TestWrappedAlgebraicSeparatedChain(t *testing.T) {
	ctx := context.Background()
	for _, negative := range []bool{false, true} {
		ab, bc, h := wrappedSeparatedChainFixture(t, negative)
		s, e := NewWrappedAlgebraicSeparatedChain(ctx, ab, bc, h, geom.SurfaceAffineCoordinateOptions{})
		if e != nil {
			t.Fatal(negative, e)
		}
		if len(s.Vertices()) != 8 || len(s.Edges()) != 10 || len(s.Faces()) != 3 || len(s.Boundary()) != 8 || s.SeparationGap().Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal("separated indexed disk")
		}
		native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
		r, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !r.Complete {
			t.Fatal("every original use", r, e)
		}
		source := []*WrappedAlgebraicQuadraticFace{ab.sources[0], ab.sources[1], bc.sources[1]}
		for i, f := range s.Faces() {
			if f.Surface != source[i].NativeFace().Face.Surface {
				t.Fatal("source chart replaced")
			}
		}
		for i, u := range s.Boundary() {
			if u.End != s.Boundary()[(i+1)%8].Start {
				t.Fatal("oriented exterior")
			}
		}
		p, _ := s.Adjacent(0)
		composition, _ := p.Shell().Pair().Composition(0)
		piece, _ := composition.Piece(0)
		root, _ := piece.Model.EndpointRoot(1)
		kind, e := root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil || kind.Kind != geom.ExactRootKindIrrational {
			t.Fatal("original irrational seam", e)
		}
		shared := 0
		for i := range s.Edges() {
			owners, _ := s.EdgeSources(i)
			if len(owners) == 2 {
				shared++
				if owners[0].Face == 1 && owners[1].Reversed == negative {
					t.Fatal("signed original mate parameter")
				}
			}
		}
		if shared != 2 {
			t.Fatal("mate ancestry")
		}
		// Reverse the complete original owner order; the strict plane gap reverses
		// sign but remains the same exact physical separation.
		cb0, _ := bc.pair.Owner(1)
		cb1, _ := bc.pair.Owner(0)
		cb, e := NewWrappedAlgebraicOpenShell(ctx, bc.sources[1], bc.sources[0], [2]int{cb0.Loop, cb0.Edge}, [2]int{cb1.Loop, cb1.Edge}, bc.pair.ReversedParameter(), edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		ba0, _ := ab.pair.Owner(1)
		ba1, _ := ab.pair.Owner(0)
		ba, e := NewWrappedAlgebraicOpenShell(ctx, ab.sources[1], ab.sources[0], [2]int{ba0.Loop, ba0.Edge}, [2]int{ba1.Loop, ba1.Edge}, ab.pair.ReversedParameter(), edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		reverseHint := func(q WrappedCoordinateSeparator) WrappedCoordinateSeparator {
			q.ParameterAxes[0], q.ParameterAxes[1] = q.ParameterAxes[1], q.ParameterAxes[0]
			q.Scale[0], q.Scale[1] = q.Scale[1], q.Scale[0]
			q.Offset[0], q.Offset[1] = q.Offset[1], q.Offset[0]
			return q
		}
		backward, e := NewWrappedAlgebraicSeparatedChain(ctx, cb, ba, [2]WrappedCoordinateSeparator{reverseHint(h[1]), reverseHint(h[0])}, geom.SurfaceAffineCoordinateOptions{})
		if e != nil || backward.SeparationGap().Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal("reversed source chain", e)
		}
		gap := s.SeparationGap()
		gap.SetInt64(99)
		if s.SeparationGap().Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal("mutable gap")
		}
		if p, e := NewWrappedAlgebraicSeparatedChain(ctx, ab, bc, h, geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: s.ExactOperations() - 1}}); p != nil || e == nil {
			t.Fatal("atomic cumulative work")
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if p, e := NewWrappedAlgebraicSeparatedChain(canceled, ab, bc, h, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
			t.Fatal("cancel")
		}
	}
}
func TestWrappedAlgebraicSeparatedChainRejectsNonadjacentContact(t *testing.T) {
	ctx := context.Background()
	ab, h0 := wrappedSeparatedAlgebraicFixture(t)
	// C repeats A's physical face under a different original lift. Both declared
	// adjacent pairs separately certify, but A and C coincide over positive area.
	loop := ab.sources[0].OriginalLoops()[0]
	for i, q := range loop {
		h := q.HomogeneousControls()
		for j := range h {
			h[j][0].Add(h[j][0], new(big.Rat).Mul(big.NewRat(8, 1), h[j][2]))
		}
		var e error
		loop[i], e = NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
	}
	c, e := NewWrappedAlgebraicQuadraticFace(ctx, ab.sources[0].NativeFace().Face.Surface.(*geom.WrappedSurfacePatch), loop, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	bc, e := NewWrappedAlgebraicOpenShell(ctx, ab.sources[1], c, [2]int{0, 0}, [2]int{0, 2}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	h1 := WrappedCoordinateSeparator{Axis: 2, Level: new(big.Rat), ParameterAxes: [2]int{1, 1}, Scale: [2]*big.Rat{big.NewRat(2, 1), big.NewRat(1, 1)}, Offset: [2]*big.Rat{new(big.Rat), new(big.Rat)}}
	for i, pair := range []*WrappedAlgebraicOpenShell{ab, bc} {
		h := h0
		if i == 1 {
			h = h1
		}
		if _, e := CertifyWrappedAlgebraicSeparatedPair(ctx, pair, h, geom.SurfaceAffineCoordinateOptions{}); e != nil {
			t.Fatal("adjacent exact separation", e)
		}
	}
	if p, e := NewWrappedAlgebraicSeparatedChain(ctx, ab, bc, [2]WrappedCoordinateSeparator{h0, h1}, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("nonadjacent coincident faces accepted")
	}
	a, b, h := wrappedSeparatedChainFixture(t, false)
	other, _, _ := wrappedSeparatedChainFixture(t, false)
	if p, e := NewWrappedAlgebraicSeparatedChain(ctx, other, b, h, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("inconsistent original middle mate")
	}
	h[1].Level.SetFrac64(1, 2)
	if p, e := NewWrappedAlgebraicSeparatedChain(ctx, a, b, h, geom.SurfaceAffineCoordinateOptions{}); p != nil || e == nil {
		t.Fatal("unproved second separator")
	}
}
