package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func wrappedFaceChainFixture(t *testing.T, n int, flip, hook bool) []*WrappedAlgebraicOpenShell {
	t.Helper()
	ctx := context.Background()
	ab, bc := wrappedAlgebraicChainFixture(t, false, flip)
	pairs := []*WrappedAlgebraicOpenShell{ab, bc}
	if n == 2 {
		return pairs[:1]
	}
	previous := bc.sources[1]
	for fi := 3; fi < n; fi++ {
		lo := 2 + float64(fi-3)/8
		hi := lo + .125
		xy := [][2]float64{{-.375, lo}, {.375, lo}, {.375, hi}, {-.375, hi}}
		if hook && fi == 3 {
			xy = [][2]float64{{-.375, 2}, {.375, 2}, {.875, 2.5}, {-.875, 2.5}, {-.875, -1.5}, {-.125, -1.5}, {-.125, -.5}, {-.625, -.5}, {-.625, 2}}
		}
		var loop []ExactQuadraticUV
		for i, p := range xy {
			next := xy[(i+1)%len(xy)]
			q, e := quadraticLine([2]*big.Rat{new(big.Rat).SetFloat64(p[0] + 4*float64(fi)), new(big.Rat).SetFloat64(p[1])}, [2]*big.Rat{new(big.Rat).SetFloat64(next[0] + 4*float64(fi)), new(big.Rat).SetFloat64(next[1])})
			if e != nil {
				t.Fatal(e)
			}
			loop = append(loop, q)
		}
		w := ab.sources[0].NativeFace().Face.Surface.(*geom.WrappedSurfacePatch)
		w, _, e := w.InsertKnot(ctx, 0, .0625*float64(fi), 1, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		next, e := NewWrappedAlgebraicQuadraticFace(ctx, w, loop, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: flip})
		if e != nil {
			t.Fatal(e)
		}
		pair, e := NewWrappedAlgebraicOpenShell(ctx, previous, next, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		pairs = append(pairs, pair)
		previous = next
	}
	return pairs
}
func TestWrappedAlgebraicFaceChain(t *testing.T) {
	ctx := context.Background()
	opts := geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}
	for _, tc := range []struct {
		n    int
		flip bool
	}{{2, false}, {4, false}, {4, true}, {8, false}} {
		pairs := wrappedFaceChainFixture(t, tc.n, tc.flip, false)
		p, e := CertifyWrappedAlgebraicFaceChainEmbedding(ctx, pairs, opts)
		if e != nil {
			t.Fatal(tc, e)
		}
		if p.FaceCount() != tc.n {
			t.Fatal("face count")
		}
		s, e := NewWrappedAlgebraicFaceChainShell(ctx, p, edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		if len(s.Vertices()) != 2*tc.n+2 || len(s.Edges()) != 3*tc.n+1 || len(s.Faces()) != tc.n || len(s.Boundary()) != 2*tc.n+2 {
			t.Fatal("chain incidence counts")
		}
		native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
		r, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !r.Complete {
			t.Fatal("continuous exported edges", r, e)
		}
		for i := 0; i < tc.n; i++ {
			shift, e := p.PeriodShift(i)
			if e != nil || shift[0].Int64() != int64(i) || shift[1].Sign() != 0 {
				t.Fatal("exact propagated occurrence", i, e)
			}
		}
		shared := 0
		for i := range s.Edges() {
			owners, _ := s.EdgeSources(i)
			if len(owners) == 2 {
				shared++
				if owners[0].Face+1 != owners[1].Face || owners[0].Reversed || !owners[1].Reversed {
					t.Fatal("shared ownership")
				}
			}
		}
		if shared != tc.n-1 {
			t.Fatal("mate count")
		}
		b := s.Boundary()
		for i, u := range b {
			if u.End != b[(i+1)%len(b)].Start || u.ReversedTraversal != tc.flip {
				t.Fatal("boundary")
			}
		}
		original := pairs[0]
		pairs[0] = nil
		if got, _ := p.PairShell(0); got != original {
			t.Fatal("mutable caller pairs")
		}
		pairs[0] = original
		vertices := s.Vertices()
		vertices[0].X = 99
		faces := s.Faces()
		faces[0].Outer[0].Edge = 99
		if s.Vertices()[0].X == 99 || s.Faces()[0].Outer[0].Edge == 99 {
			t.Fatal("mutable native")
		}
		low := opts
		low.MaxOperations = p.ExactOperations() - 1
		if q, e := CertifyWrappedAlgebraicFaceChainEmbedding(ctx, pairs, low); q != nil || e == nil {
			t.Fatal("atomic embedding budget")
		}
		if q, e := NewWrappedAlgebraicFaceChainShell(ctx, p, edgecheck.Options{MaxWork: s.ExactOperations() - 1}); q != nil || e == nil {
			t.Fatal("atomic native budget")
		}
	}
}
func TestWrappedAlgebraicFaceChainRejects(t *testing.T) {
	ctx := context.Background()
	opts := geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}
	pairs := wrappedFaceChainFixture(t, 4, false, true)
	for _, pair := range pairs {
		if _, e := CertifyWrappedAlgebraicOpenShellEmbedding(ctx, pair, opts); e != nil {
			t.Fatal("all declared pair embeddings", e)
		}
	}
	if p, e := CertifyWrappedAlgebraicFaceChainEmbedding(ctx, pairs, opts); p != nil || e == nil {
		t.Fatal("fourth face overlaps first")
	}
	good := wrappedFaceChainFixture(t, 4, false, false)
	wrong := append([]*WrappedAlgebraicOpenShell(nil), good...)
	wrong[1], wrong[2] = wrong[2], wrong[1]
	if p, e := CertifyWrappedAlgebraicFaceChainEmbedding(ctx, wrong, opts); p != nil || e == nil {
		t.Fatal("unbound lift propagation")
	}
	tooMany := append(append([]*WrappedAlgebraicOpenShell(nil), good...), good...)
	tooMany = append(tooMany, good...)
	if p, e := CertifyWrappedAlgebraicFaceChainEmbedding(ctx, tooMany, opts); p != nil || e == nil {
		t.Fatal("hard face cap")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if p, e := CertifyWrappedAlgebraicFaceChainEmbedding(canceled, good, opts); p != nil || e == nil {
		t.Fatal("cancel")
	}
	p, e := CertifyWrappedAlgebraicFaceChainEmbedding(ctx, good, opts)
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewWrappedAlgebraicFaceChainShell(ctx, p, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	faces := s.Faces()
	faces[2].Outer[0].Reversed = false
	if _, e := validateWrappedFaceChainIncidence(ctx, s.vertices, s.edges, faces, s.vertexSources, s.edgeSources); e == nil {
		t.Fatal("broken fan orientation")
	}
	vs := append([][]WrappedOpenVertexSource(nil), s.vertexSources...)
	for i, v := range vs {
		if len(v) == 2 {
			vs[i] = []WrappedOpenVertexSource{v[0], {Face: 7, Vertex: v[1].Vertex}}
			break
		}
	}
	if _, e := validateWrappedFaceChainIncidence(ctx, s.vertices, s.edges, s.faces, vs, s.edgeSources); e == nil {
		t.Fatal("disconnected owner fan")
	}
}
