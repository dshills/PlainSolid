package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func wrappedSeriesBandSupport(t *testing.T, shift, bend, level float64) *geom.WrappedSurfacePatch {
	t.Helper()
	profile := []geom.Vec2{{X: 2}, {X: 2, Y: 2}, {Y: 2}, {X: -2, Y: 2}, {X: -2}, {X: -2, Y: -2}, {Y: -2}, {X: 2, Y: -2}, {X: 2}}
	var poles []geom.Vec3
	for _, p := range profile {
		for j, v := range []float64{-1, .5, 2} {
			square := []float64{1, -2, 4}[j]
			poles = append(poles, geom.Vec3{X: p.X + shift + bend*square, Y: p.Y, Z: level + v})
		}
	}
	s, e := geom.NewBSplineSurface(2, 2, 9, 3, poles, []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}, []float64{-1, -1, -1, 2, 2, 2}, nil)
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
func wrappedSeparatedSeriesFixture(t *testing.T, n int) ([]*WrappedAlgebraicOpenShell, []WrappedCoordinateSeparator) {
	t.Helper()
	ctx := context.Background()
	ab, h := wrappedSeparatedAlgebraicFixture(t)
	pairs := []*WrappedAlgebraicOpenShell{ab}
	hints := []WrappedCoordinateSeparator{h}
	previous := ab.sources[1]
	shift := 1. / 16
	for fi := 2; fi < n; fi++ {
		level := float64(fi - 1)
		bend := math.Ldexp(1, -fi-1)
		w := wrappedSeriesBandSupport(t, shift, bend, level)
		c, e := NewWrappedAlgebraicQuadraticFace(ctx, w, wrappedBandLoop(t, 4*float64(fi)-.25, 4*float64(fi)+.25, 0, 1), nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
		if e != nil {
			t.Fatal(e)
		}
		pair, e := NewWrappedAlgebraicOpenShell(ctx, previous, c, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		leftScale, leftOffset := big.NewRat(1, 1), big.NewRat(int64(fi-2), 1)
		if fi == 2 {
			leftScale = big.NewRat(2, 1)
			leftOffset = new(big.Rat)
		}
		q := WrappedCoordinateSeparator{Axis: 2, Level: big.NewRat(int64(fi-1), 1), ParameterAxes: [2]int{1, 1}, Scale: [2]*big.Rat{leftScale, big.NewRat(1, 1)}, Offset: [2]*big.Rat{leftOffset, big.NewRat(int64(fi-1), 1)}}
		pairs = append(pairs, pair)
		hints = append(hints, q)
		previous = c
		shift += bend
	}
	return pairs, hints
}
func TestWrappedAlgebraicSeparatedSeries(t *testing.T) {
	ctx := context.Background()
	for _, n := range []int{2, 5, 8} {
		pairs, h := wrappedSeparatedSeriesFixture(t, n)
		s, e := NewWrappedAlgebraicSeparatedSeries(ctx, pairs, h, geom.SurfaceAffineCoordinateOptions{})
		if e != nil {
			t.Fatal(n, e)
		}
		if s.FaceCount() != n || len(s.Vertices()) != 2*n+2 || len(s.Edges()) != 3*n+1 || len(s.Faces()) != n || len(s.Boundary()) != 2*n+2 {
			t.Fatal("indexed series")
		}
		native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
		r, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !r.Complete {
			t.Fatal(r, e)
		}
		for i := 0; i < n; i++ {
			for j := i + 2; j < n; j++ {
				gap, e := s.NonadjacentGap(i, j)
				if e != nil || gap.Cmp(big.NewRat(int64(j-i-1), 1)) != 0 {
					t.Fatal("all nonadjacent separation", i, j, gap, e)
				}
				opposite, e := s.NonadjacentGap(j, i)
				if e != nil || opposite.Cmp(gap) != 0 {
					t.Fatal("symmetric separation")
				}
				gap.SetInt64(99)
			}
		}
		if _, e := s.NonadjacentGap(0, 1); e == nil {
			t.Fatal("adjacent gap fabricated")
		}
		p, _ := s.Adjacent(0)
		composition, _ := p.Shell().Pair().Composition(0)
		piece, _ := composition.Piece(0)
		root, _ := piece.Model.EndpointRoot(1)
		kind, e := root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil || kind.Kind != geom.ExactRootKindIrrational {
			t.Fatal("original algebraic seam", e)
		}
		vertices := s.Vertices()
		vertices[0].X = 99
		if s.Vertices()[0].X == 99 {
			t.Fatal("mutable native")
		}
		if q, e := NewWrappedAlgebraicSeparatedSeries(ctx, pairs, h, geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: s.ExactOperations() - 1}}); q != nil || e == nil {
			t.Fatal("atomic total bound")
		}
	}
}
func TestWrappedAlgebraicSeparatedSeriesRejects(t *testing.T) {
	ctx := context.Background()
	pairs, h := wrappedSeparatedSeriesFixture(t, 5)
	// Return from the final original support to the previous one. The new pair
	// uses a freshly certified source object, but reverses ordering and repeats
	// nonadjacent material. It must never become a globally embedded series.
	last := pairs[len(pairs)-1]
	a, _ := last.pair.Owner(1)
	b, _ := last.pair.Owner(0)
	returned, e := NewWrappedAlgebraicQuadraticFace(ctx, last.sources[0].NativeFace().Face.Surface.(*geom.WrappedSurfacePatch), last.sources[0].OriginalLoops()[0], nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	back, e := NewWrappedAlgebraicOpenShell(ctx, last.sources[1], returned, [2]int{a.Loop, a.Edge}, [2]int{b.Loop, b.Edge}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	q := h[len(h)-1]
	q.Scale[0], q.Scale[1] = q.Scale[1], q.Scale[0]
	q.Offset[0], q.Offset[1] = q.Offset[1], q.Offset[0]
	if _, e := CertifyWrappedAlgebraicSeparatedPair(ctx, back, q, geom.SurfaceAffineCoordinateOptions{}); e != nil {
		t.Fatal("valid return adjacency", e)
	}
	if s, e := NewWrappedAlgebraicSeparatedSeries(ctx, append(pairs, back), append(h, q), geom.SurfaceAffineCoordinateOptions{}); s != nil || e == nil {
		t.Fatal("returning series accepted")
	}
	inconsistent := append([]WrappedCoordinateSeparator(nil), h...)
	inconsistent[2].Axis = 0
	if s, e := NewWrappedAlgebraicSeparatedSeries(ctx, pairs, inconsistent, geom.SurfaceAffineCoordinateOptions{}); s != nil || e == nil {
		t.Fatal("nonparallel order")
	}
	tooMany := append(append([]*WrappedAlgebraicOpenShell(nil), pairs...), pairs...)
	tooHints := append(append([]WrappedCoordinateSeparator(nil), h...), h...)
	if s, e := NewWrappedAlgebraicSeparatedSeries(ctx, tooMany, tooHints, geom.SurfaceAffineCoordinateOptions{}); s != nil || e == nil {
		t.Fatal("hard face cap")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if s, e := NewWrappedAlgebraicSeparatedSeries(canceled, pairs, h, geom.SurfaceAffineCoordinateOptions{}); s != nil || e == nil {
		t.Fatal("cancel")
	}
}

func TestWrappedAlgebraicSeparatedSeriesDescending(t *testing.T) {
	ctx := context.Background()
	pairs, h := wrappedSeparatedSeriesFixture(t, 5)
	var reversed []*WrappedAlgebraicOpenShell
	var hints []WrappedCoordinateSeparator
	for i := len(pairs) - 1; i >= 0; i-- {
		pair := pairs[i]
		a, _ := pair.pair.Owner(1)
		b, _ := pair.pair.Owner(0)
		q, e := NewWrappedAlgebraicOpenShell(ctx, pair.sources[1], pair.sources[0], [2]int{a.Loop, a.Edge}, [2]int{b.Loop, b.Edge}, pair.pair.ReversedParameter(), edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		reversed = append(reversed, q)
		hint := h[i]
		hint.Scale[0], hint.Scale[1] = hint.Scale[1], hint.Scale[0]
		hint.Offset[0], hint.Offset[1] = hint.Offset[1], hint.Offset[0]
		hint.ParameterAxes[0], hint.ParameterAxes[1] = hint.ParameterAxes[1], hint.ParameterAxes[0]
		hints = append(hints, hint)
	}
	s, e := NewWrappedAlgebraicSeparatedSeries(ctx, reversed, hints, geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	gap, e := s.NonadjacentGap(0, 4)
	if e != nil || gap.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("descending global separation", gap, e)
	}
	native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
	r, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
}
