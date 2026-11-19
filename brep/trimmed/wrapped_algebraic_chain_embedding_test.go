//go:build research

package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func wrappedAlgebraicChainFixture(t *testing.T, hook, flip bool) (*WrappedAlgebraicOpenShell, *WrappedAlgebraicOpenShell) {
	t.Helper()
	ctx := context.Background()
	a, b := wrappedAlgebraicMateFaces(t, false, false)
	points := [][2]float64{{-.375, 1}, {.375, 1}, {.375, 2}, {-.375, 2}}
	if hook {
		points = [][2]float64{{-.375, 1}, {.375, 1}, {.75, 2}, {-.75, 2}, {-.75, -1.5}, {-.125, -1.5}, {-.125, -.5}, {-.5, -.5}, {-.5, 1}}
	}
	var loop []ExactQuadraticUV
	for i, p := range points {
		n := points[(i+1)%len(points)]
		q, e := quadraticLine([2]*big.Rat{new(big.Rat).SetFloat64(p[0] + 8), new(big.Rat).SetFloat64(p[1])}, [2]*big.Rat{new(big.Rat).SetFloat64(n[0] + 8), new(big.Rat).SetFloat64(n[1])})
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, q)
	}
	w := a.NativeFace().Face.Surface.(*geom.WrappedSurfacePatch)
	w, _, e := w.InsertKnot(ctx, 0, .25, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	opts := WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: flip}
	c, e := NewWrappedAlgebraicQuadraticFace(ctx, w, loop, nil, opts)
	if e != nil {
		t.Fatal("third source", e)
	}
	if flip {
		for _, f := range []**WrappedAlgebraicQuadraticFace{&a, &b} {
			*f, e = NewWrappedAlgebraicQuadraticFace(ctx, (*f).NativeFace().Face.Surface.(*geom.WrappedSurfacePatch), (*f).OriginalLoops()[0], nil, opts)
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	ab, e := NewWrappedAlgebraicOpenShell(ctx, a, b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	bc, e := NewWrappedAlgebraicOpenShell(ctx, b, c, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	return ab, bc
}
func TestWrappedAlgebraicChainEmbedding(t *testing.T) {
	ctx := context.Background()
	opts := geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}
	for _, flip := range []bool{false, true} {
		ab, bc := wrappedAlgebraicChainFixture(t, false, flip)
		p, e := CertifyWrappedAlgebraicChainEmbedding(ctx, ab, bc, opts)
		if e != nil {
			t.Fatal(flip, e)
		}
		if len(p.Boundary()) != 8 || p.ProjectionAxes() != opts.ProjectionAxes || p.LiftedBounds().VMax != 2 {
			t.Fatal("global region")
		}
		for i := 0; i < 3; i++ {
			shift, e := p.PeriodShift(i)
			if e != nil || shift[0].Int64() != int64(i) || shift[1].Sign() != 0 {
				t.Fatal("lift", i, shift, e)
			}
			shift[0].SetInt64(99)
			again, _ := p.PeriodShift(i)
			if again[0].Int64() != int64(i) {
				t.Fatal("mutable shift")
			}
		}
		if q, _ := p.PairShell(1); q != bc {
			t.Fatal("source binding")
		}
		boundary := p.Boundary()
		boundary[0].Face = 99
		if p.Boundary()[0].Face == 99 {
			t.Fatal("mutable exterior")
		}
		// Consecutive native source curves retain their original (possibly +8)
		// lifted evaluators; aligned UV is independent certificate data only.
		for _, edge := range p.Boundary() {
			var source *WrappedAlgebraicQuadraticFace
			switch edge.Face {
			case 0:
				source = ab.sources[0]
			case 1:
				source = ab.sources[1]
			case 2:
				source = bc.sources[1]
			}
			uv, _, e := edge.UV.eval(.5)
			if e != nil {
				t.Fatal(e)
			}
			point, e := ab.faces[0].Surface.Eval(uv.X, uv.Y)
			if e != nil {
				t.Fatal(e)
			}
			native, e := source.NativeFace().Edges[edge.LocalEdge].Curve.Eval(.5)
			if e != nil {
				t.Fatal(e)
			}
			if point.Sub(native).Length() > 1e-12 {
				t.Fatal("original lifted evaluator", edge.Face, point, native)
			}
		}
		low := opts
		low.MaxOperations = p.ExactOperations() - 1
		if q, e := CertifyWrappedAlgebraicChainEmbedding(ctx, ab, bc, low); q != nil || e == nil {
			t.Fatal("atomic cumulative budget")
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if q, e := CertifyWrappedAlgebraicChainEmbedding(canceled, ab, bc, opts); q != nil || e == nil {
			t.Fatal("cancel")
		}
	}
}
func TestWrappedAlgebraicChainRejectsNonAdjacentOverlap(t *testing.T) {
	ctx := context.Background()
	ab, bc := wrappedAlgebraicChainFixture(t, true, false)
	opts := geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}
	// Both declared adjacent pairs independently have globally disjoint interiors.
	for _, s := range []*WrappedAlgebraicOpenShell{ab, bc} {
		if _, e := CertifyWrappedAlgebraicOpenShellEmbedding(ctx, s, opts); e != nil {
			t.Fatal("adjacent positive", e)
		}
	}
	// C's lower hook overlaps A, invisible to those two pairwise certificates.
	if p, e := CertifyWrappedAlgebraicChainEmbedding(ctx, ab, bc, opts); p != nil || e == nil {
		t.Fatal("nonadjacent overlap accepted")
	}
	other, _ := wrappedAlgebraicChainFixture(t, false, false)
	if p, e := CertifyWrappedAlgebraicChainEmbedding(ctx, other, bc, opts); p != nil || e == nil {
		t.Fatal("unbound middle source accepted")
	}
}

func TestWrappedAlgebraicChainRejectsReusedMate(t *testing.T) {
	ctx := context.Background()
	ab, _ := wrappedAlgebraicChainFixture(t, false, false)
	ba, e := NewWrappedAlgebraicOpenShell(ctx, ab.sources[1], ab.sources[0], [2]int{0, 0}, [2]int{0, 2}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	opts := geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}
	if _, e := CertifyWrappedAlgebraicOpenShellEmbedding(ctx, ba, opts); e != nil {
		t.Fatal("reversed pair", e)
	}
	if p, e := CertifyWrappedAlgebraicChainEmbedding(ctx, ab, ba, opts); p != nil || e == nil {
		t.Fatal("same mate reused as third face")
	}
}
