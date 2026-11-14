package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestMixedProjectedDirectedRegionHoles(t *testing.T) {
	points := [][2]*big.Rat{}
	for _, p := range [][2]int64{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {2, 2}, {2, 4}, {4, 4}, {4, 2}, {6, 6}, {6, 8}, {8, 8}, {8, 6}} {
		points = append(points, [2]*big.Rat{big.NewRat(p[0], 1), big.NewRat(p[1], 1)})
	}
	loops := [][]int{{0, 1, 2, 3}, {4, 5, 6, 7}, {8, 9, 10, 11}}
	p := partitionFillProof{ctx: context.Background(), points: points, limit: 100000}
	tri, area, e := mixedTriangulateDirectedRegion(&p, loops, 100)
	if e != nil || area.Cmp(big.NewRat(92, 1)) != 0 || len(tri) != 14 {
		t.Fatal("two holes", len(tri), area, e)
	}
	for name, bad := range map[string][][]int{"outer reversal": {{3, 2, 1, 0}, {4, 5, 6, 7}}, "hole reversal": {{0, 1, 2, 3}, {7, 6, 5, 4}}, "duplicate": {{0, 1, 2, 3}, {4, 5, 6, 7}, {4, 5, 6, 7}}, "wrong nest": {{4, 7, 6, 5}, {3, 2, 1, 0}}, "missing edge": {{0, 1, 2}, {4, 5, 6, 7}}} {
		q := partitionFillProof{ctx: context.Background(), points: points, limit: 100000}
		if got, _, e := mixedTriangulateDirectedRegion(&q, bad, 100); got != nil || e == nil {
			t.Fatal(name, "accepted")
		}
	}
	nested := append(append([][2]*big.Rat(nil), points...), [2]*big.Rat{big.NewRat(5, 2), big.NewRat(5, 2)}, [2]*big.Rat{big.NewRat(5, 2), big.NewRat(7, 2)}, [2]*big.Rat{big.NewRat(7, 2), big.NewRat(7, 2)}, [2]*big.Rat{big.NewRat(7, 2), big.NewRat(5, 2)})
	q := partitionFillProof{ctx: context.Background(), points: nested, limit: 100000}
	if got, _, e := mixedTriangulateDirectedRegion(&q, [][]int{loops[0], loops[1], {12, 13, 14, 15}}, 100); got != nil || e == nil {
		t.Fatal("nested holes accepted")
	}
	q = partitionFillProof{ctx: context.Background(), points: points, limit: 1}
	if got, _, e := mixedTriangulateDirectedRegion(&q, loops, 100); got != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("budget", e)
	}
	edges := [][]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}}
	uses := [][]AlgebraicShellBooleanUse{{{Edge: 0}, {Edge: 1}, {Edge: 2}, {Edge: 3}}}
	if rings, e := mixedDirectedNetworkLoops(edges, uses, 10); e != nil || len(rings) != 1 {
		t.Fatal("native owner walk", e)
	}
	uses[0][1].Reversed = true
	if rings, e := mixedDirectedNetworkLoops(edges, uses, 10); rings != nil || e == nil {
		t.Fatal("unmatched original edge accepted")
	}
}

func TestMixedUnionProjectedHoleRegion(t *testing.T) {
	ctx := context.Background()
	a, b, p, o := mixedWallFixture(t)
	section, e := SectionSplineShellsMixedAlgebraic(ctx, a, b, p, o)
	if e != nil {
		t.Fatal(e)
	}
	bo := AlgebraicShellBooleanOptions{}
	bo.Imprint.Exact.MaxOperations = 128000000
	bodies, e := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Union}, bo)
	if e != nil {
		t.Fatal(e)
	}
	body := bodies[0]
	n, e := body.TessellateEmbeddedBoundaryCertified(ctx, AlgebraicShellBoundaryEmbeddingOptions{Boundary: AlgebraicShellBoundaryOptions{Deflection: .02, MaxOperations: 128000000, AutoSectionProjection: true, PlanarSectionFallback: true}})
	if e != nil {
		t.Fatal(e)
	}
	holes := 0
	for id, f := range body.Faces() {
		if len(f.Holes) == 0 {
			continue
		}
		holes++
		t.Log("original holed face", id, "operand", f.Operand, "source", f.Original)
		r, e := n.ProjectFaceRegionCertified(ctx, id, [2]int{0, 1}, MixedProjectedRegionOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(r.Loops()) != 2 || r.Area().Sign() <= 0 || len(r.OriginalUses()) != 2 {
			t.Fatal("lost holes")
		}
		if !reflect.DeepEqual(r.OriginalUses(), append([][]AlgebraicShellBooleanUse{f.Outer}, f.Holes...)) {
			t.Fatal("lost original directed owners")
		}
		mirrored, e := n.ProjectFaceRegionCertified(ctx, id, [2]int{1, 0}, MixedProjectedRegionOptions{})
		if e != nil {
			t.Fatal("reflected projection", e)
		}
		_, direction, _ := mirrored.Projection()
		if direction != -1 || mirrored.Area().Cmp(r.Area()) != 0 || !reflect.DeepEqual(mirrored.OriginalUses(), r.OriginalUses()) {
			t.Fatal("projection changed source ownership")
		}
		// Every triangle centroid must be outside the inner ring. This independent
		// rational ray test complements complete area and edge-incidence checks.
		points, loops := r.Coordinates(), r.Loops()
		for _, tri := range r.Triangles() {
			m := [2]*big.Rat{new(big.Rat), new(big.Rat)}
			for _, v := range tri {
				for k := 0; k < 2; k++ {
					m[k].Add(m[k], points[v][k])
				}
			}
			for k := 0; k < 2; k++ {
				m[k].Quo(m[k], big.NewRat(3, 1))
			}
			w := 0
			for j, x := range loops[1] {
				y := loops[1][(j+1)%len(loops[1])]
				a, b := points[x], points[y]
				cross := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(m[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(m[0], a[0])))
				if a[1].Cmp(m[1]) <= 0 && b[1].Cmp(m[1]) > 0 && cross.Sign() > 0 {
					w++
				}
				if a[1].Cmp(m[1]) > 0 && b[1].Cmp(m[1]) <= 0 && cross.Sign() < 0 {
					w--
				}
			}
			if w != 0 {
				t.Fatal("triangle fills original projected hole")
			}
		}
		points[0][0].SetInt64(999)
		loops[0][0] = -1
		if r.Coordinates()[0][0].Cmp(big.NewRat(999, 1)) == 0 || r.Loops()[0][0] < 0 {
			t.Fatal("mutable region")
		}
		for _, limit := range []MixedProjectedRegionOptions{{MaxVertices: 3}, {MaxTriangles: 1}, {MaxOperations: 1}, {MaxPredicates: 1}} {
			if got, e := n.ProjectFaceRegionCertified(ctx, id, [2]int{0, 1}, limit); got != nil || e == nil {
				t.Fatal("partial budget publication")
			}
		}
		c, cancel := context.WithCancel(ctx)
		cancel()
		if got, e := n.ProjectFaceRegionCertified(c, id, [2]int{0, 1}, MixedProjectedRegionOptions{}); got != nil || !errors.Is(e, context.Canceled) {
			t.Fatal("cancellation", e)
		}
		ops, pred := r.Work()
		t.Log("region", len(r.Triangles()), "triangles", len(r.Loops()[0]), len(r.Loops()[1]), "loop vertices", ops, pred, "work")
	}
	if holes != 1 {
		t.Fatal("expected original Union hole", holes)
	}
}
