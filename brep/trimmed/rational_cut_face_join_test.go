//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func rationalCutJoinFixture(t *testing.T) (QuadraticTrimFaceJoinInput, QuadraticTrimFaceJoinInput) {
	t.Helper()
	p, outer := rationalCutFaceFixture(t)
	a := QuadraticTrimFaceJoinInput{Patch: p, Outer: outer}
	c, e := geom.NewBSpline(2, []geom.Vec3{{Y: .125}, {X: .5, Y: .5}, {X: 1, Y: .125}}, []float64{10, 10, 10, 26, 26, 26}, []float64{1, .5, .25})
	if e != nil {
		t.Fatal(e)
	}
	b := QuadraticTrimFaceJoinInput{Patch: p, Outer: []SplineUV{{Curve: c, Range: UVInterval{Min: 10, Max: 26}, Reversed: true}}}
	corners := []geom.Vec3{{Y: .125}, {}, {X: 1}, {X: 1, Y: .125}}
	for j := 0; j+1 < len(corners); j++ {
		c, e := geom.NewBSpline(1, []geom.Vec3{corners[j], corners[j+1]}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		b.Outer = append(b.Outer, SplineUV{Curve: c, Range: UVInterval{Max: 1}})
	}
	return a, b
}
func TestRationalCutQuadraticFaceJoin(t *testing.T) {
	slowtest.Skip(t)
	a, b := rationalCutJoinFixture(t)
	o := RationalCutFaceJoinOptions{Face: RationalCutFaceMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .2}}}}
	got, e := StitchRationalCutFacesCertified(context.Background(), a, b, o)
	if e != nil {
		t.Fatal(e)
	}
	present := map[string]bool{}
	for i, q := range got.SharedParameters {
		present[q.RatString()] = true
		// Independent original UV rational Bézier evaluator.
		m := new(big.Rat).Sub(big.NewRat(1, 1), q)
		b0 := new(big.Rat).Mul(m, m)
		b1 := new(big.Rat).Mul(m, q)
		b2 := new(big.Rat).Quo(new(big.Rat).Mul(q, q), big.NewRat(4, 1))
		w := new(big.Rat).Add(new(big.Rat).Add(b0, b1), b2)
		u := new(big.Rat).Quo(q, new(big.Rat).Sub(big.NewRat(2, 1), q))
		v := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Quo(new(big.Rat).Add(b0, b2), big.NewRat(8, 1)), new(big.Rat).Quo(b1, big.NewRat(2, 1))), w)
		jet, e := a.Patch.EvaluateRational(context.Background(), u, v, geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		point := geom.Vec3{}
		point.X, _ = jet.Point[0].Float64()
		point.Y, _ = jet.Point[1].Float64()
		point.Z, _ = jet.Point[2].Float64()
		if got.Mesh.Vertices[got.SharedVertices[i]] != point {
			t.Fatal("true shared curve not used", q)
		}
	}
	for _, q := range []string{"0", "2/5", "2/3", "6/7", "1"} {
		if !present[q] {
			t.Fatal("missing knot cut", q)
		}
	}
	edges := map[[2]int]int{}
	balances := map[[2]int]int{}
	for _, tri := range got.Mesh.Triangles {
		for j, a := range tri {
			b := tri[(j+1)%3]
			sign := 1
			if a > b {
				a, b = b, a
				sign = -1
			}
			key := [2]int{a, b}
			edges[key]++
			balances[key] += sign
		}
	}
	if len(got.Mesh.Vertices)-len(edges)+len(got.Mesh.Triangles) != 1 {
		t.Fatal("not disk")
	}
	for i := 0; i+1 < len(got.SharedVertices); i++ {
		a, b := got.SharedVertices[i], got.SharedVertices[i+1]
		if a > b {
			a, b = b, a
		}
		key := [2]int{a, b}
		if edges[key] != 2 || balances[key] != 0 {
			t.Fatal("opposite incidence")
		}
	}
	// The two polygon fills partition the independently known UV rectangle
	// [0,1]x[0,3/4], despite the genuine curved shared source boundary.
	totalUV := new(big.Rat)
	for _, face := range got.Faces {
		totalUV.Add(totalUV, face.Boundary.Region.PolygonUVArea)
	}
	if totalUV.Cmp(big.NewRat(3, 4)) != 0 {
		t.Fatal("independent total UV area", totalUV)
	}
	projected := 0.
	for _, tri := range got.Mesh.Triangles {
		a, b, c := got.Mesh.Vertices[tri[0]], got.Mesh.Vertices[tri[1]], got.Mesh.Vertices[tri[2]]
		area := ((b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)) / 2
		if area >= 0 {
			t.Fatal("display fold")
		}
		projected += area
	}
	if math.Abs(projected+.75) > 1e-12 {
		t.Fatal("independent display coverage", projected)
	}
	target := new(big.Rat).Mul(ratInset(.2), ratInset(.2))
	if got.FillSquaredDistance.Cmp(target) > 0 {
		t.Fatal("distance")
	}
	t.Logf("triangles=%d seam=%d cells=%d/%d checks=%d/%d", len(got.Mesh.Triangles), len(got.SharedVertices), got.Faces[0].SpanCellChecks, got.Faces[1].SpanCellChecks, got.Faces[0].TriangleChecks, got.Faces[1].TriangleChecks)
	o.MaxSharedVertices = 3
	if r, e := StitchRationalCutFacesCertified(context.Background(), a, b, o); !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("partial vertex budget", e)
	}
	o.MaxSharedVertices = 0
	o.Face.Face.MaxSpanCells = got.Faces[0].SpanCellChecks - 1
	if r, e := StitchRationalCutFacesCertified(context.Background(), a, b, o); !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("partial cumulative budget", e)
	}
}

func TestRationalCutFaceJoinInvalidInputs(t *testing.T) {
	a, b := rationalCutJoinFixture(t)
	for _, d := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, e := StitchRationalCutFacesCertified(context.Background(), a, b, RationalCutFaceJoinOptions{Face: RationalCutFaceMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: d}}}}); !errors.Is(e, geom.ErrInvalid) {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := StitchRationalCutFacesCertified(ctx, a, b, RationalCutFaceJoinOptions{Face: RationalCutFaceMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .2}}}}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
