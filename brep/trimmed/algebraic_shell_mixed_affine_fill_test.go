//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestMixedOriginalAffineFragmentFill(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b, p, o := mixedWallFixture(t)
	section, e := SectionSplineShellsMixedAlgebraic(ctx, a, b, p, o)
	if e != nil {
		t.Fatal(e)
	}
	bc := AlgebraicShellBooleanOptions{}
	bc.Imprint.Exact.MaxOperations = 128000000
	bodies, e := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection}, bc)
	if e != nil {
		t.Fatal(e)
	}
	body := bodies[0]
	n, e := body.TessellateEmbeddedBoundaryCertified(ctx, AlgebraicShellBoundaryEmbeddingOptions{Boundary: AlgebraicShellBoundaryOptions{Deflection: .02, MaxOperations: 128000000, AutoSectionProjection: true, PlanarSectionFallback: true}})
	if e != nil {
		t.Fatal(e)
	}
	var ids []int
	roof := -1
	for i, f := range body.Faces() {
		if f.Operand == 0 && f.Original == 4 || f.Operand == 1 && f.Original == 0 {
			ids = append(ids, i)
		}
		if f.Original == 1 {
			roof = i
		}
	}
	if len(ids) != 2 || roof < 0 {
		t.Fatal("fixture ownership", ids, roof)
	}
	r, e := n.FillAffineFragmentsCertified(ctx, ids, MixedAffineFragmentFillOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, fragments := r.Mesh(), r.Fragments()
	if len(fragments) != 2 || len(r.UnfilledFaces()) != len(body.Faces())-2 || r.Deflection() < .02 || r.Deflection() > .06 || !reflect.DeepEqual(m.Vertices, n.Vertices()) {
		t.Fatal("lost partial/network scope", len(fragments), len(r.UnfilledFaces()), len(body.Faces()), r.Deflection())
	}
	for _, f := range fragments {
		chain := map[[2]int]int{}
		sum := new(big.Rat)
		uv := map[int][2]*big.Rat{}
		maximum := new(big.Rat)
		for _, x := range f.PlaneResidual {
			a := new(big.Rat).Abs(x)
			if a.Cmp(maximum) > 0 {
				maximum.Set(a)
			}
		}
		want := new(big.Rat).Add(ratInset(.02), new(big.Rat).Mul(big.NewRat(2, 1), maximum))
		if f.ErrorUpper.Cmp(want) != 0 || ratInset(r.Deflection()).Cmp(want) < 0 {
			t.Fatal("unproved displacement bound")
		}
		source, e := body.OriginalFace(f.Face)
		if e != nil {
			t.Fatal(e)
		}
		patch, e := exactFaceSupportPatch(source, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		chart, e := geom.NewAffineSurfaceChart(ctx, patch, geom.AffineSurfaceChartOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for j, id := range f.BoundaryVertices {
			uv[id] = f.UV[j]
			point, e := chart.EvalExact(f.UV[j][0], f.UV[j][1])
			if e != nil {
				t.Fatal(e)
			}
			point[f.ResidualAxis].Add(point[f.ResidualAxis], f.PlaneResidual[j])
			v := m.Vertices[id]
			for k, x := range []float64{v.X, v.Y, v.Z} {
				if point[k].Cmp(ratInset(x)) != 0 {
					t.Fatal("moved original-chart point")
				}
			}
		}
		reversed := body.Faces()[f.Face].Reversed
		for _, tri := range f.Triangles {
			q := tri
			if reversed {
				q[1], q[2] = q[2], q[1]
			}
			a, b, c := uv[q[0]], uv[q[1]], uv[q[2]]
			area := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(c[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(c[0], a[0])))
			if area.Sign() <= 0 {
				t.Fatal("triangle orientation")
			}
			sum.Add(sum, new(big.Rat).Quo(area, big.NewRat(2, 1)))
			for k := 0; k < 3; k++ {
				partitionChainAdd(chain, q[k], q[(k+1)%3], 1)
			}
		}
		for j, id := range f.BoundaryVertices {
			partitionChainAdd(chain, id, f.BoundaryVertices[(j+1)%len(f.BoundaryVertices)], -1)
		}
		if len(chain) != 0 || sum.Cmp(f.AreaUV) != 0 {
			t.Fatal("incomplete original boundary fill")
		}
	}
	ops, pred, pairs := r.Work()
	segments, _ := n.Work()
	if pairs < len(m.Triangles)*segments || ops <= 0 || pred <= 0 {
		t.Fatal("incomplete contact checks", pairs)
	}
	t.Log("faces", ids, "triangles", len(m.Triangles), "operations", ops, "predicates", pred, "network/triangle pairs", pairs, "deflection", r.Deflection())
	fragments[0].UV[0][0].SetInt64(999)
	m.Vertices[0].X += 1
	if r.Fragments()[0].UV[0][0].Cmp(big.NewRat(999, 1)) == 0 || r.Mesh().Vertices[0] == m.Vertices[0] {
		t.Fatal("mutable certificate")
	}
	for _, limit := range []MixedAffineFragmentFillOptions{{MaxOperations: 1}, {MaxPredicates: 1}, {MaxPairs: 1}, {MaxTriangles: 1}, {MaxVertices: 3}} {
		if got, e := n.FillAffineFragmentsCertified(ctx, ids, limit); got != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("non-atomic budget", limit, e)
		}
	}
	if got, e := n.FillAffineFragmentsCertified(ctx, []int{roof}, MixedAffineFragmentFillOptions{}); got != nil || e == nil {
		t.Fatal("nonaffine roof accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := n.FillAffineFragmentsCertified(canceled, ids, MixedAffineFragmentFillOptions{}); got != nil || !errors.Is(e, context.Canceled) {
		t.Fatal("cancellation", e)
	}
}

func TestMixedAffineSegmentTriangleIncidence(t *testing.T) {
	// Triangle contains a coplanar chord, a piercing segment and a genuine
	// indexed boundary edge. Equal positions with different IDs are not mates.
	points := [][2]*big.Rat{}
	for _, xy := range [][2]int64{{0, 0}, {4, 0}, {0, 4}, {1, 1}, {2, 1}, {0, 0}, {1, 1}} {
		points = append(points, [2]*big.Rat{big.NewRat(xy[0], 1), big.NewRat(xy[1], 1)})
	}
	p := partitionFillProof{ctx: context.Background(), points: points, limit: 100}
	s := make([]*big.Rat, len(points))
	for i := range s {
		s[i] = new(big.Rat)
	}
	s[6].SetInt64(1)
	for _, pair := range [][2]int{{3, 4}, {5, 1}, {3, 6}} {
		if e := mixedAffineSegmentTriangle(&p, s, [3]int{0, 1, 2}, pair[0], pair[1]); e == nil {
			t.Fatal("unrelated contact accepted", pair)
		}
	}
	if e := mixedAffineSegmentTriangle(&p, s, [3]int{0, 1, 2}, 0, 1); e != nil {
		t.Fatal(e)
	}
	// A rounded nonplanar display triangle is a graph z=x over the chart.
	// Test against its actual plane, not the underlying source plane z=0.
	s[1].SetInt64(4)
	s[3].SetInt64(2)
	s[4].SetInt64(3)
	if e := mixedAffineSegmentTriangle(&p, s, [3]int{0, 1, 2}, 3, 4); e != nil {
		t.Fatal("disjoint elevated segment", e)
	}
	s[3].SetInt64(0)
	if e := mixedAffineSegmentTriangle(&p, s, [3]int{0, 1, 2}, 3, 4); e == nil {
		t.Fatal("missed tilted triangle crossing")
	}
}
