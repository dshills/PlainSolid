package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCertifiedQuadraticSeamDistinctCurvedCharts(t *testing.T) {
	slowtest.Skip(t)
	k := []float64{0, 0, 1, 1}
	sa, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, e := geom.NewSurfacePatch(sa, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Chart B has x=1-u and z=x*v+F(x,v), where F=x²-x+v/2+3v²/4.
	// The rational seam is exactly F=0, so these distinct curved supports
	// agree there while their UV coordinates and parameter ranges differ.
	var poles []geom.Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			u, v := float64(i)/2, float64(j)/2
			x := 1 - u
			z := x*v - u + .5*v
			if i == 2 {
				z++
			}
			if j == 2 {
				z += .75
			}
			poles = append(poles, geom.Vec3{X: x, Y: v, Z: z})
		}
	}
	knots := []float64{0, 0, 0, 1, 1, 1}
	sb, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := geom.NewSurfacePatch(sb, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	ca, e := geom.NewBSpline(2, []geom.Vec3{{}, {X: .5, Y: 1}, {X: 1}}, knots, []float64{1, .5, 1})
	if e != nil {
		t.Fatal(e)
	}
	cb, e := geom.NewBSpline(2, []geom.Vec3{{X: 1}, {X: .5, Y: 1}, {}}, []float64{2, 2, 2, 6, 6, 6}, []float64{1, .5, 1})
	if e != nil {
		t.Fatal(e)
	}
	ta, tb := SplineUV{Curve: ca, Range: UVInterval{Max: 1}}, SplineUV{Curve: cb, Range: UVInterval{Min: 2, Max: 6}}
	options := QuadraticTrimSeamOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .01}}
	result, e := CertifyQuadraticTrimSeam(context.Background(), a, b, ta, tb, options)
	if e != nil {
		t.Fatal(e)
	}
	if result.IdentityChecks != 17 || len(result.Points) < 3 || len(result.SquaredErrors[0]) != len(result.SquaredErrors[1]) {
		t.Fatal("missing paired proof")
	}
	for i, p := range result.ExactPoints {
		if p[2].Cmp(new(big.Rat).Mul(p[0], p[1])) != 0 {
			t.Fatal("independent exact model seam")
		}
		f := new(big.Rat).Add(new(big.Rat).Sub(new(big.Rat).Mul(p[0], p[0]), p[0]), new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(1, 2), p[1]), new(big.Rat).Mul(big.NewRat(3, 4), new(big.Rat).Mul(p[1], p[1]))))
		if f.Sign() != 0 {
			t.Fatal("independent exact conic seam")
		}
		if new(big.Rat).Add(result.ExactUV[0][i][0], result.ExactUV[1][i][0]).Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal("chart correspondence")
		}
		if result.SourceParameters[1][i] != 2+4*result.SourceParameters[0][i] {
			t.Fatal("source parameter ancestry")
		}
	}
	for chart := 0; chart < 2; chart++ {
		for _, bound := range result.SquaredErrors[chart] {
			if bound.Cmp(big.NewRat(1, 10000)) > 0 {
				t.Fatal("unproved chord")
			}
		}
	}
	reversed := tb
	reversed.Reversed = true
	bad, e := CertifyQuadraticTrimSeam(context.Background(), a, b, ta, reversed, options)
	if e == nil || len(bad.Points) != 0 {
		t.Fatal("opposite parameterization accepted")
	}
	// Endpoint equality alone is insufficient: perturb only an interior pole.
	altered := append([]geom.Vec3(nil), poles...)
	altered[4].Z += .125
	different, e := geom.NewBSplineSurface(2, 2, 3, 3, altered, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	wrong, e := geom.NewSurfacePatch(different, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bad, e = CertifyQuadraticTrimSeam(context.Background(), a, wrong, ta, tb, options)
	if e == nil || len(bad.Points) != 0 {
		t.Fatal("endpoint-only seam accepted")
	}
	for _, opt := range []QuadraticTrimSeamOptions{{Tessellation: options.Tessellation, MaxIdentityChecks: 16}, {Tessellation: QuadraticTrimTessellationOptions{Deflection: .01, MaxChecks: 1}}, {Tessellation: QuadraticTrimTessellationOptions{Deflection: .0001, MaxSegments: 1}}} {
		bad, e = CertifyQuadraticTrimSeam(context.Background(), a, b, ta, tb, opt)
		if e == nil || len(bad.Points) != 0 {
			t.Fatal("partial budget result")
		}
	}

	line, e := geom.NewBSpline(1, []geom.Vec3{{}, {X: 1}}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	straight := SplineUV{Curve: line, Range: UVInterval{Max: 1}}
	edgeA := ta
	edgeA.Reversed = true
	joined, e := StitchQuadraticTrimFacesCertified(context.Background(), QuadraticTrimFaceJoinInput{Patch: a, Outer: []SplineUV{straight, edgeA}, SharedTrim: 1}, QuadraticTrimFaceJoinInput{Patch: b, Outer: []SplineUV{straight, tb}, SharedTrim: 1}, QuadraticTrimFaceJoinOptions{Face: CurvedQuadraticTrimMeshOptions{Deflection: .08}, MaxSharedSegments: 256})
	if e != nil {
		t.Fatal("stitched two curved faces", e)
	}
	total, _ := joined.FillSquaredDistance.Float64()
	if total > .08*.08 {
		t.Fatal("uncertified stitched fill")
	}
	if len(joined.Mesh.Vertices) != len(joined.Faces[0].Mesh.Vertices)+len(joined.Faces[1].Mesh.Vertices)-len(joined.SharedVertices) {
		t.Fatal("incorrect seam identity welding")
	}
	edges := map[[2]int]int{}
	for _, triangle := range joined.Mesh.Triangles {
		for j, a := range triangle {
			b := triangle[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}]++
		}
	}
	if len(joined.Mesh.Vertices)-len(edges)+len(joined.Mesh.Triangles) != 1 {
		t.Fatal("two-face sheet topology")
	}
	for i, id := range joined.SharedVertices {
		if i > 0 && joined.SharedParameters[i-1].Cmp(joined.SharedParameters[i]) >= 0 {
			t.Fatal("seam parameter order")
		}
		p := joined.Mesh.Vertices[id]
		if p.Sub(geom.Vec3{X: p.X, Y: p.Y, Z: p.X * p.Y}).Length() > 1e-14 {
			t.Fatal("model seam")
		}
	}
	failed, e := StitchQuadraticTrimFacesCertified(context.Background(), QuadraticTrimFaceJoinInput{Patch: a, Outer: []SplineUV{straight, edgeA}, SharedTrim: 1}, QuadraticTrimFaceJoinInput{Patch: b, Outer: []SplineUV{straight, tb}, SharedTrim: 1}, QuadraticTrimFaceJoinOptions{Face: CurvedQuadraticTrimMeshOptions{Deflection: .08}, MaxSharedSegments: 1})
	if e == nil || len(failed.Mesh.Vertices) != 0 {
		t.Fatal("partial join on seam partition limit")
	}
	t.Logf("joined %d triangles and %d shared vertices, squared fill bound %g", len(joined.Mesh.Triangles), len(joined.SharedVertices), total)
	t.Logf("%d shared samples, %d paired chord checks", len(result.Points), result.Checks)
}
