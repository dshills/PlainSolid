//go:build research

package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func wrappedFanFixture(t *testing.T, n int, flip bool) []*WrappedAlgebraicOpenShell {
	t.Helper()
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	faces := make([]*WrappedAlgebraicQuadraticFace, n)
	rays := [][2]float64{{.5, 0}, {0, .5}, {-.5, 0}, {0, -.5}}
	if n == 3 {
		rays = [][2]float64{{.5, 0}, {-.5, .5}, {-.5, -.5}}
	}
	for fi := range faces {
		next := (fi + 1) % n
		points := [][2]float64{{0, 0}, rays[fi]}
		if n == 4 {
			points = append(points, [2]float64{rays[fi][0] + rays[next][0], rays[fi][1] + rays[next][1]})
		}
		points = append(points, rays[next])
		var loop []ExactQuadraticUV
		for i, p := range points {
			q := points[(i+1)%len(points)]
			edge, e := quadraticLine([2]*big.Rat{new(big.Rat).SetFloat64(p[0] + 4*float64(fi)), new(big.Rat).SetFloat64(p[1])}, [2]*big.Rat{new(big.Rat).SetFloat64(q[0] + 4*float64(fi)), new(big.Rat).SetFloat64(q[1])})
			if e != nil {
				t.Fatal(e)
			}
			loop = append(loop, edge)
		}
		refined, _, e := w.InsertKnot(ctx, 0, float64(fi+1)/16, 1, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		faces[fi], e = NewWrappedAlgebraicQuadraticFace(ctx, refined, loop, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: flip})
		if e != nil {
			t.Fatal("source face", fi, e)
		}
	}
	pairs := make([]*WrappedAlgebraicOpenShell, n)
	last := n - 1
	if n == 3 {
		last = 2
	}
	for i := range pairs {
		var e error
		pairs[i], e = NewWrappedAlgebraicOpenShell(ctx, faces[i], faces[(i+1)%n], [2]int{0, last}, [2]int{0, 0}, true, edgecheck.Options{})
		if e != nil {
			t.Fatal("declared cyclic mate", i, e)
		}
	}
	return pairs
}
func TestWrappedAlgebraicFan(t *testing.T) {
	ctx := context.Background()
	opts := geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}
	for _, tc := range []struct {
		n    int
		flip bool
	}{{3, false}, {4, false}, {4, true}} {
		pairs := wrappedFanFixture(t, tc.n, tc.flip)
		p, e := CertifyWrappedAlgebraicFanEmbedding(ctx, pairs, opts)
		if e != nil {
			t.Fatal("embedding", tc, e)
		}
		s, e := NewWrappedAlgebraicFanShell(ctx, p, edgecheck.Options{})
		if e != nil {
			t.Fatal("native", tc, e)
		}
		boundary := tc.n
		if tc.n == 4 {
			boundary = 8
		}
		if len(s.Vertices()) != boundary+1 || len(s.Edges()) != boundary+tc.n || len(s.Faces()) != tc.n || len(s.Boundary()) != boundary {
			t.Fatal("fan counts")
		}
		center := s.CenterVertex()
		if s.Vertices()[center] != (geom.Vec3{X: 2}) {
			t.Fatal("exact center")
		}
		owners, e := s.VertexSources(center)
		if e != nil || len(owners) != tc.n {
			t.Fatal("circular owner fan")
		}
		for i := 0; i < tc.n; i++ {
			cv, e := p.CenterVertex(i)
			if e != nil || cv != 0 {
				t.Fatal("original center", i, e)
			}
			shift, _ := p.PeriodShift(i)
			if shift[0].Int64() != int64(i) || shift[1].Sign() != 0 {
				t.Fatal("original period lift")
			}
		}
		native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
		r, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !r.Complete {
			t.Fatal("all native associations", r, e)
		}
		for i, u := range s.Boundary() {
			if u.Start == center || u.End == center || u.End != s.Boundary()[(i+1)%boundary].Start || u.ReversedTraversal != tc.flip {
				t.Fatal("exterior")
			}
		}
		low := opts
		low.MaxOperations = p.ExactOperations() - 1
		if q, e := CertifyWrappedAlgebraicFanEmbedding(ctx, pairs, low); q != nil || e == nil {
			t.Fatal("atomic global work")
		}
		if q, e := NewWrappedAlgebraicFanShell(ctx, p, edgecheck.Options{MaxWork: s.ExactOperations() - 1}); q != nil || e == nil {
			t.Fatal("atomic native work")
		}
		vertices := s.Vertices()
		vertices[center].X = 99
		owners[0].Face = 99
		if s.Vertices()[center].X == 99 {
			t.Fatal("mutable center")
		}
		again, _ := s.VertexSources(center)
		if again[0].Face == 99 {
			t.Fatal("mutable owner")
		}
	}
}
func TestWrappedAlgebraicFanRejects(t *testing.T) {
	ctx := context.Background()
	opts := geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}
	pairs := wrappedFanFixture(t, 4, false)
	for _, bad := range [][]*WrappedAlgebraicOpenShell{pairs[:3], {pairs[0], pairs[1], pairs[2], pairs[0]}, {pairs[0], pairs[2], pairs[1], pairs[3]}} {
		if p, e := CertifyWrappedAlgebraicFanEmbedding(ctx, bad, opts); p != nil || e == nil {
			t.Fatal("open/duplicate/misordered fan")
		}
	}
	// A changed closing occurrence must not be accepted as consistent holonomy.
	altered := append([]*WrappedAlgebraicOpenShell(nil), pairs...)
	closing := *pairs[3]
	mate := *closing.pair
	owner, _ := mate.Owner(1)
	for side := 0; side < 2; side++ {
		owner.UV[side][0].Add(owner.UV[side][0], big.NewRat(4, 1))
	}
	mate.owners[1] = owner
	closing.pair = &mate
	altered[3] = &closing
	if q, e := CertifyWrappedAlgebraicFanEmbedding(ctx, altered, opts); q != nil || e == nil {
		t.Fatal("altered cyclic lift")
	}
	p, e := CertifyWrappedAlgebraicFanEmbedding(ctx, pairs, opts)
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewWrappedAlgebraicFanShell(ctx, p, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	broken := *s.graph
	broken.faces = s.Faces()
	broken.faces[3].Outer[3].Reversed = !broken.faces[3].Outer[3].Reversed
	if _, e := validateWrappedFanIncidence(ctx, &broken, s.center); e == nil {
		t.Fatal("broken closing radial orientation")
	}
	broken = *s.graph
	broken.vertexSources = append([][]WrappedOpenVertexSource(nil), s.graph.vertexSources...)
	broken.vertexSources[s.center] = broken.vertexSources[s.center][:3]
	if _, e := validateWrappedFanIncidence(ctx, &broken, s.center); e == nil {
		t.Fatal("missing circular sector")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := CertifyWrappedAlgebraicFanEmbedding(canceled, pairs, opts); q != nil || e == nil {
		t.Fatal("cancel")
	}
}

func TestWrappedAlgebraicFanRejectsNonadjacentOverlap(t *testing.T) {
	ctx := context.Background()
	pairs := wrappedFanFixture(t, 4, false)
	xy := [][2]float64{{0, 0}, {-.5, 0}, {-.875, 0}, {-.875, -.875}, {.875, -.875}, {.875, .875}, {.25, .875}, {.25, .25}, {.75, .25}, {.75, -.75}, {0, -.75}, {0, -.5}}
	var loop []ExactQuadraticUV
	for i, p := range xy {
		q := xy[(i+1)%len(xy)]
		edge, e := quadraticLine([2]*big.Rat{new(big.Rat).SetFloat64(p[0] + 8), new(big.Rat).SetFloat64(p[1])}, [2]*big.Rat{new(big.Rat).SetFloat64(q[0] + 8), new(big.Rat).SetFloat64(q[1])})
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, edge)
	}
	hooked, e := NewWrappedAlgebraicQuadraticFace(ctx, pairs[2].sources[0].NativeFace().Face.Surface.(*geom.WrappedSurfacePatch), loop, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal("simple hooked disk", e)
	}
	pairs[1], e = NewWrappedAlgebraicOpenShell(ctx, pairs[1].sources[0], hooked, [2]int{0, 3}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	pairs[2], e = NewWrappedAlgebraicOpenShell(ctx, hooked, pairs[2].sources[1], [2]int{0, len(loop) - 1}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	opts := geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}
	for i, pair := range pairs {
		if _, e := CertifyWrappedAlgebraicOpenShellEmbedding(ctx, pair, opts); e != nil {
			t.Fatal("each adjacent pair is embedded", i, e)
		}
	}
	if p, e := CertifyWrappedAlgebraicFanEmbedding(ctx, pairs, opts); p != nil || e == nil {
		t.Fatal("opposite face overlap accepted")
	}
}
