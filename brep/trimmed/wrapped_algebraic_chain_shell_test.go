//go:build research

package trimmed

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func TestWrappedAlgebraicChainShell(t *testing.T) {
	ctx := context.Background()
	for _, flip := range []bool{false, true} {
		ab, bc := wrappedAlgebraicChainFixture(t, false, flip)
		embedding, e := CertifyWrappedAlgebraicChainEmbedding(ctx, ab, bc, geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}})
		if e != nil {
			t.Fatal(e)
		}
		s, e := NewWrappedAlgebraicChainShell(ctx, embedding, edgecheck.Options{})
		if e != nil {
			t.Fatal(flip, e)
		}
		if s.Embedding() != embedding || len(s.Vertices()) != 8 || len(s.Edges()) != 10 || len(s.Faces()) != 3 || len(s.Boundary()) != 8 {
			t.Fatal("indexed topology")
		}
		native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
		report, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !report.Complete {
			t.Fatal("all exported uses", report, e)
		}
		shared := 0
		for i := range s.Edges() {
			owners, e := s.EdgeSources(i)
			if e != nil {
				t.Fatal(e)
			}
			if len(owners) == 2 {
				shared++
				if owners[0].Face+1 != owners[1].Face || owners[0].Reversed || !owners[1].Reversed {
					t.Fatal("mate ancestry", owners)
				}
			}
		}
		if shared != 2 {
			t.Fatal("mate count")
		}
		boundary := s.Boundary()
		for i, u := range boundary {
			if u.End != boundary[(i+1)%len(boundary)].Start || u.ReversedTraversal != flip {
				t.Fatal("oriented exterior")
			}
		}
		sources := []*WrappedAlgebraicQuadraticFace{ab.sources[0], ab.sources[1], bc.sources[1]}
		for i, f := range s.Faces() {
			original := sources[i].NativeFace().Face
			if f.Surface != original.Surface || f.Reversed != original.Reversed {
				t.Fatal("original support")
			}
			for j, u := range f.Outer {
				if u.UVStart != original.Outer[j].UVStart || u.UVEnd != original.Outer[j].UVEnd || u.Curve != original.Outer[j].Curve {
					t.Fatal("original lifted pcurve")
				}
			}
		}
		vertices := s.Vertices()
		vertices[0].X = 99
		faces := s.Faces()
		faces[0].Outer[0].Edge = 99
		boundary[0].Face = 99
		owners, _ := s.EdgeSources(0)
		owners[0].Face = 99
		if s.Vertices()[0].X == 99 || s.Faces()[0].Outer[0].Edge == 99 || s.Boundary()[0].Face == 99 {
			t.Fatal("mutable exported topology")
		}
		if q, e := NewWrappedAlgebraicChainShell(ctx, embedding, edgecheck.Options{MaxWork: s.ExactOperations() - 1}); q != nil || e == nil {
			t.Fatal("atomic work")
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if q, e := NewWrappedAlgebraicChainShell(canceled, embedding, edgecheck.Options{}); q != nil || e == nil {
			t.Fatal("cancel")
		}
	}
}
func TestWrappedAlgebraicChainShellRejectsIncidence(t *testing.T) {
	ctx := context.Background()
	ab, bc := wrappedAlgebraicChainFixture(t, false, false)
	p, e := CertifyWrappedAlgebraicChainEmbedding(ctx, ab, bc, geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}})
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewWrappedAlgebraicChainShell(ctx, p, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	faces := s.Faces()
	faces[1].Outer[0].Reversed = false
	if _, e := validateWrappedChainIncidence(ctx, s.vertices, s.edges, faces, s.vertexSources, s.edgeSources); e == nil {
		t.Fatal("wrong mate direction")
	}
	if q, e := NewWrappedAlgebraicChainShell(ctx, &WrappedAlgebraicChainEmbedding{}, edgecheck.Options{}); q != nil || e == nil {
		t.Fatal("empty certificate")
	}
}
