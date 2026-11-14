package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func tensorTriangleCyclicFixture(t *testing.T, n int) ([]*geom.SurfaceAffineTriangleMate, []AffineTriangleVertexPair) {
	t.Helper()
	ctx := context.Background()
	rat := func(x int64) *big.Rat { return big.NewRat(x, 8) }
	all := [8][2]*big.Rat{{rat(8), rat(4)}, {rat(7), rat(7)}, {rat(4), rat(8)}, {rat(1), rat(7)}, {rat(0), rat(4)}, {rat(1), rat(1)}, {rat(4), rat(0)}, {rat(7), rat(1)}}
	selection := map[int][]int{5: {0, 1, 3, 5, 6}, 6: {0, 1, 3, 4, 5, 7}, 7: {0, 1, 2, 3, 4, 5, 6}, 8: {0, 1, 2, 3, 4, 5, 6, 7}}[n]
	rim := make([][2]*big.Rat, n)
	for i, j := range selection {
		rim[i] = all[j]
	}
	center := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	lines := make([][3]*big.Rat, n)
	triangles := make([][3][2]*big.Rat, n)
	for i, v := range rim {
		a := new(big.Rat).Sub(v[1], center[1])
		b := new(big.Rat).Sub(center[0], v[0])
		c := new(big.Rat).Neg(new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1)))
		lines[i] = [3]*big.Rat{a, b, c}
		triangles[i] = [3][2]*big.Rat{center, rim[(i+1)%n], rim[i]}
	}
	normal := func(a, b *big.Rat) [3]*big.Rat {
		return [3]*big.Rat{a, b, new(big.Rat).Neg(new(big.Rat).Add(a, new(big.Rat).Quo(b, big.NewRat(8, 1))))}
	}
	patches := make([]*geom.SurfacePatch, n)
	for owner := range patches {
		l, m := lines[owner], lines[(owner+1)%n]
		coeff := map[[2]int]*big.Rat{}
		powers := [][2]int{{1, 0}, {0, 1}, {0, 0}}
		for i, p := range powers {
			for j, q := range powers {
				k := [2]int{p[0] + q[0], p[1] + q[1]}
				if coeff[k] == nil {
					coeff[k] = new(big.Rat)
				}
				coeff[k].Add(coeff[k], new(big.Rat).Mul(l[i], m[j]))
			}
		}
		var h [][4]*big.Rat
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				u, v := big.NewRat(int64(i), 2), big.NewRat(int64(j), 2)
				basis := func(k, d int) *big.Rat {
					if d == 0 {
						return big.NewRat(1, 1)
					}
					if d == 1 {
						return big.NewRat(int64(k), 2)
					}
					if k == 2 {
						return big.NewRat(1, 1)
					}
					return new(big.Rat)
				}
				z := new(big.Rat)
				for power, c := range coeff {
					term := new(big.Rat).Mul(c, basis(i, power[0]))
					term.Mul(term, basis(j, power[1]))
					z.Add(z, term)
				}
				z.Quo(z, big.NewRat(int64(64*(owner+1)), 1))
				h = append(h, [4]*big.Rat{new(big.Rat).Add(u, z), new(big.Rat).Add(v, new(big.Rat).Quo(z, big.NewRat(8, 1))), z, big.NewRat(1, 1)})
			}
		}
		p, e := geom.NewExactHomogeneousBezierSurface(ctx, 2, 2, h, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = p.InsertUKnot(float64(owner+1)/16, 1, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		patches[owner] = p
	}
	mates := make([]*geom.SurfaceAffineTriangleMate, n)
	for i := range mates {
		next := (i + 1) % n
		l := lines[next]
		q := [2]*big.Rat{l[0], l[1]}
		other := [3][2]*big.Rat{center, rim[next], rim[(i+2)%n]}
		p, e := geom.CertifySurfaceAffineTriangleMate(ctx, patches[i], patches[next], [2][3][2]*big.Rat{triangles[i], other}, normal(q[0], q[1]), [2][2]*big.Rat{q, q}, [2]*big.Rat{new(big.Rat), new(big.Rat)}, new(big.Rat).Neg(l[2]), geom.SurfaceAffineCoordinateOptions{})
		if e != nil {
			t.Fatal(e)
		}
		mates[i] = p
	}
	var pairs []AffineTriangleVertexPair
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if j == i+1 || i == 0 && j == n-1 {
				continue
			}
			var q [2]*big.Rat
			found := false
			for a := -8; a <= 8 && !found; a++ {
				for b := -8; b <= 8 && !found; b++ {
					if a == 0 && b == 0 {
						continue
					}
					signs := [2]int{}
					valid := true
					for owner, fi := range []int{i, j} {
						for k := 1; k < 3; k++ {
							v := triangles[fi][k]
							s := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(int64(a), 1), new(big.Rat).Sub(v[0], center[0])), new(big.Rat).Mul(big.NewRat(int64(b), 1), new(big.Rat).Sub(v[1], center[1]))).Sign()
							if s == 0 || signs[owner] != 0 && signs[owner] != s {
								valid = false
							}
							signs[owner] = s
						}
					}
					if valid && signs[0] == -signs[1] {
						q = [2]*big.Rat{big.NewRat(int64(a), 1), big.NewRat(int64(b), 1)}
						found = true
					}
				}
			}
			if !found {
				t.Fatal("no bounded exact proposal", i, j)
			}
			separator := geom.SurfaceAffineTriangleSeparator{Normal: normal(q[0], q[1]), UV: [2][2]*big.Rat{q, q}, Offset: [2]*big.Rat{new(big.Rat), new(big.Rat)}, Level: new(big.Rat).Quo(new(big.Rat).Add(q[0], q[1]), big.NewRat(2, 1))}
			proof, e := geom.CertifySurfaceAffineTriangleVertexContact(ctx, patches[i], patches[j], [2][3][2]*big.Rat{triangles[i], triangles[j]}, [2]int{0, 0}, separator, geom.SurfaceAffineCoordinateOptions{})
			if e != nil {
				t.Fatal(e)
			}
			pairs = append(pairs, AffineTriangleVertexPair{Faces: [2]int{i, j}, Proof: proof})
		}
	}
	return mates, pairs
}
func TestAffineTriangleCyclicFanShell(t *testing.T) {
	ctx := context.Background()
	for n := 5; n <= 8; n++ {
		mates, pairs := tensorTriangleCyclicFixture(t, n)
		s, e := NewAffineTriangleCyclicFanShell(ctx, mates, pairs, AffineTriangleShellOptions{})
		if e != nil {
			t.Fatal(n, e)
		}
		if len(s.Vertices()) != n+1 || len(s.Edges()) != 2*n || len(s.Boundary()) != n || len(s.Nonadjacent()) != n*(n-3)/2 {
			t.Fatal("complete global fan", n)
		}
		owners, e := s.VertexSources(s.CenterVertex())
		if e != nil || len(owners) != n {
			t.Fatal("all original center occurrences")
		}
		solid := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
		r, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !r.Complete {
			t.Fatal(n, r, e)
		}
		if n != 5 {
			continue
		}
		reversed := append([]AffineTriangleVertexPair(nil), pairs...)
		for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
			reversed[i], reversed[j] = reversed[j], reversed[i]
		}
		again, e := NewAffineTriangleCyclicFanShell(ctx, mates, reversed, AffineTriangleShellOptions{})
		if e != nil {
			t.Fatal(e)
		}
		a, _ := wrappedIndexedShellWitness(s.fan.graph.vertices, s.fan.graph.edges, s.fan.graph.faces, s.fan.graph.vertexSources, s.fan.graph.edgeSources, s.fan.graph.boundary)
		b, _ := wrappedIndexedShellWitness(again.fan.graph.vertices, again.fan.graph.edges, again.fan.graph.faces, again.fan.graph.vertexSources, again.fan.graph.edgeSources, again.fan.graph.boundary)
		if !reflect.DeepEqual(a, b) {
			t.Fatal("nondeterministic ancestry")
		}
		for _, bad := range [][]AffineTriangleVertexPair{pairs[:len(pairs)-1], append(append([]AffineTriangleVertexPair(nil), pairs[:len(pairs)-1]...), pairs[0])} {
			if p, e := NewAffineTriangleCyclicFanShell(ctx, mates, bad, AffineTriangleShellOptions{}); e == nil || p != nil {
				t.Fatal("missing or duplicate proof")
			}
		}
		stale := append([]AffineTriangleVertexPair(nil), pairs...)
		stale[0].Faces = [2]int{0, 1}
		if p, e := NewAffineTriangleCyclicFanShell(ctx, mates, stale, AffineTriangleShellOptions{}); e == nil || p != nil {
			t.Fatal("adjacent mislabel")
		}
		stale = append([]AffineTriangleVertexPair(nil), pairs...)
		stale[0].Proof = pairs[1].Proof
		if p, e := NewAffineTriangleCyclicFanShell(ctx, mates, stale, AffineTriangleShellOptions{}); e == nil || p != nil {
			t.Fatal("stale valid source proof")
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if p, e := NewAffineTriangleCyclicFanShell(canceled, mates, pairs, AffineTriangleShellOptions{}); e == nil || p != nil {
			t.Fatal("cancel")
		}
		for _, o := range []AffineTriangleShellOptions{{MaxWork: s.ExactOperations() - 1}, {Proof: edgecheck.Options{MaxIntervals: 1}}} {
			if p, e := NewAffineTriangleCyclicFanShell(ctx, mates, pairs, o); e == nil || p != nil {
				t.Fatal("atomic work bound")
			}
		}
		copy := s.Nonadjacent()
		copy[0].Faces = [2]int{0, 1}
		if s.Nonadjacent()[0].Faces == ([2]int{0, 1}) {
			t.Fatal("mutable proof table")
		}
	}
}
