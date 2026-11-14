package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func TestWrappedAlgebraicShellEmbedding(t *testing.T) {
	ctx := context.Background()
	for _, flip := range []bool{false, true} {
		a, b := wrappedAlgebraicMateFaces(t, false, false)
		if flip {
			for _, p := range []**WrappedAlgebraicQuadraticFace{&a, &b} {
				var e error
				*p, e = NewWrappedAlgebraicQuadraticFace(ctx, (*p).NativeFace().Face.Surface.(*geom.WrappedSurfacePatch), (*p).OriginalLoops()[0], nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: true})
				if e != nil {
					t.Fatal(e)
				}
			}
		}
		s, e := NewWrappedAlgebraicOpenShell(ctx, a, b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		o := geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}
		p, e := CertifyWrappedAlgebraicOpenShellEmbedding(ctx, s, o)
		if e != nil {
			t.Fatal(flip, e)
		}
		if p.Shell() != s || len(p.BoundaryUV()) != 6 || p.BPeriodShift()[0].Cmp(big.NewInt(1)) != 0 || p.BPeriodShift()[1].Sign() != 0 || p.ProjectionAxes() != o.ProjectionAxes {
			t.Fatal("binding")
		}
		shift := p.BPeriodShift()
		shift[0].SetInt64(99)
		loop := p.BoundaryUV()
		loop[0] = ExactQuadraticUV{}
		if p.BPeriodShift()[0].Int64() != 1 || p.BoundaryUV()[0].control[0][0] == nil {
			t.Fatal("mutable evidence")
		}
		o.MaxOperations = p.ExactOperations() - 1
		if q, e := CertifyWrappedAlgebraicOpenShellEmbedding(ctx, s, o); e == nil || q != nil {
			t.Fatal("atomic budget", e)
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if q, e := CertifyWrappedAlgebraicOpenShellEmbedding(canceled, s, geom.SurfaceInjectivityOptions{}); e == nil || q != nil {
			t.Fatal("cancel")
		}
	}
}

func TestWrappedAlgebraicShellEmbeddingRejectsOtherCrossings(t *testing.T) {
	ctx := context.Background()
	a, standard := wrappedAlgebraicMateFaces(t, false, false)
	xy := [][2]float64{{-.25, 0}, {.25, 0}, {.5, 1}, {-.5, 1}, {-.5, -1.25}, {-.125, -1.25}, {-.125, -.5}, {-.375, -.5}, {-.375, 0}}
	var loop []ExactQuadraticUV
	for i, p := range xy {
		n := xy[(i+1)%len(xy)]
		q, e := quadraticLine([2]*big.Rat{new(big.Rat).SetFloat64(p[0]), new(big.Rat).SetFloat64(p[1])}, [2]*big.Rat{new(big.Rat).SetFloat64(n[0]), new(big.Rat).SetFloat64(n[1])})
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, q)
	}
	loop[0] = reversePlaneQuadratic(a.OriginalLoops()[0][2])
	for i, q := range loop {
		h := q.HomogeneousControls()
		for j := range h {
			h[j][0].Add(h[j][0], new(big.Rat).Mul(big.NewRat(4, 1), h[j][2]))
		}
		var e error
		loop[i], e = NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
	}
	b, e := NewWrappedAlgebraicQuadraticFace(ctx, standard.NativeFace().Face.Surface.(*geom.WrappedSurfacePatch), loop, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal("simple hooked source", e)
	}
	s, e := NewWrappedAlgebraicOpenShell(ctx, a, b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal("valid incidence only", e)
	}
	if p, e := CertifyWrappedAlgebraicOpenShellEmbedding(ctx, s, geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}); e == nil || p != nil {
		t.Fatal("overlapping interiors accepted")
	}
}
