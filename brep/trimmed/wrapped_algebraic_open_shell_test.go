package trimmed

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func TestWrappedAlgebraicOpenShell(t *testing.T) {
	ctx := context.Background()
	a, b := wrappedAlgebraicMateFaces(t, false, false)
	for _, flip := range []bool{false, true} {
		if flip {
			opts := WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: true}
			var e error
			a, e = NewWrappedAlgebraicQuadraticFace(ctx, a.NativeFace().Face.Surface.(*geom.WrappedSurfacePatch), a.OriginalLoops()[0], nil, opts)
			if e != nil {
				t.Fatal(e)
			}
			b, e = NewWrappedAlgebraicQuadraticFace(ctx, b.NativeFace().Face.Surface.(*geom.WrappedSurfacePatch), b.OriginalLoops()[0], nil, opts)
			if e != nil {
				t.Fatal(e)
			}
		}
		shell, e := NewWrappedAlgebraicOpenShell(ctx, a, b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		vertices, edges, faces, boundary := shell.Vertices(), shell.Edges(), shell.Faces(), shell.Boundary()
		if len(vertices) != 6 || len(edges) != 7 || len(faces) != 2 || len(boundary) != 6 || !shell.Pair().Proven() {
			t.Fatal("incidence counts")
		}
		open := Solid{vertices: vertices, edges: edges, faces: faces, tolerance: 1e-9}
		agreement, e := open.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !agreement.Complete {
			t.Fatal("exported native agreement", agreement, e)
		}
		if faces[0].Surface != a.NativeFace().Face.Surface || faces[1].Surface != b.NativeFace().Face.Surface || faces[1].Outer[0].UVStart != b.NativeFace().Face.Outer[0].UVStart || !faces[1].Outer[0].Reversed {
			t.Fatal("owner chart changed")
		}
		for i, u := range boundary {
			if u.End != boundary[(i+1)%len(boundary)].Start || u.ReversedTraversal != flip || u.Use.Edge == 2 {
				t.Fatal("exterior", boundary)
			}
		}
		sources, e := shell.EdgeSources(2)
		if e != nil || len(sources) != 2 || sources[0].Face != 0 || sources[0].Edge != 2 || sources[1].Face != 1 || sources[1].Edge != 0 || !sources[1].Reversed {
			t.Fatal(sources, e)
		}
		for _, v := range []int{2, 3} {
			sources, e := shell.VertexSources(v)
			if e != nil || len(sources) != 2 {
				t.Fatal("shared vertex ancestry", sources, e)
			}
		}
		vertices[0].X = 99
		faces[1].Outer[0].Edge = 99
		sources[0].Edge = 99
		boundary[0].Start = 99
		if shell.Vertices()[0].X == 99 || shell.Faces()[1].Outer[0].Edge == 99 || shell.Boundary()[0].Start == 99 {
			t.Fatal("mutable graph")
		}
		low := edgecheck.Options{MaxWork: shell.ExactOperations() - 1}
		if q, e := NewWrappedAlgebraicOpenShell(ctx, a, b, [2]int{0, 2}, [2]int{0, 0}, true, low); e == nil || q != nil {
			t.Fatal("atomic budget")
		}
	}
}
func TestWrappedAlgebraicOpenShellRejects(t *testing.T) {
	ctx := context.Background()
	a, b := wrappedAlgebraicMateFaces(t, false, false)
	// Oppositely oriented duplicate whole faces have a valid selected edge mate,
	// but all their other exact vertices/edges are undeclared coincidences.
	duplicate, e := NewWrappedAlgebraicQuadraticFace(ctx, a.NativeFace().Face.Surface.(*geom.WrappedSurfacePatch), a.OriginalLoops()[0], nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: true})
	if e != nil {
		t.Fatal(e)
	}
	if p, e := CertifyWrappedAlgebraicEdgePair(ctx, a, duplicate, [2]int{0, 2}, [2]int{0, 2}, false, edgecheck.Options{}); e != nil || !p.Proven() {
		t.Fatal("fixture pair", e)
	}
	if q, e := NewWrappedAlgebraicOpenShell(ctx, a, duplicate, [2]int{0, 2}, [2]int{0, 2}, false, edgecheck.Options{}); e == nil || q != nil {
		t.Fatal("undeclared duplicate accepted")
	}
	if q, e := NewWrappedAlgebraicOpenShell(ctx, a, b, [2]int{0, 2}, [2]int{0, 0}, false, edgecheck.Options{}); e == nil || q != nil {
		t.Fatal("wrong orientation")
	}
	good, e := NewWrappedAlgebraicOpenShell(ctx, a, b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	faces := good.Faces()
	faces[1].Outer[0].Reversed = false
	if _, e := validateWrappedAlgebraicPairIncidence(ctx, len(good.Vertices()), good.Edges(), faces, 2); e == nil {
		t.Fatal("invalid oriented link")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := NewWrappedAlgebraicOpenShell(canceled, a, b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{}); e == nil || q != nil {
		t.Fatal("cancel")
	}
}
