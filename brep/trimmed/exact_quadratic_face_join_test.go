//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func scaledPartitionGraph(t *testing.T, scale float64, more bool) *geom.SurfacePatch {
	t.Helper()
	cuts := []float64{0, .5, 1}
	if more {
		cuts = []float64{0, .25, .5, .75, 1}
	}
	knots := []float64{0, 0, 0}
	var poles []geom.Vec3
	for span := 0; span+1 < len(cuts); span++ {
		a, b := cuts[span], cuts[span+1]
		xs := [3]float64{a, (a + b) / 2, b}
		fs := [3]float64{a * a, a * b, b * b}
		if (a+b)/2 > .5 {
			fs = [3]float64{.25 + (a-.5)*(a-.5), .25 + (a-.5)*(b-.5), .25 + (b-.5)*(b-.5)}
		}
		for i := 0; i < 3; i++ {
			if span > 0 && i == 0 {
				continue
			}
			for j := 0; j < 2; j++ {
				y := scale * float64(j)
				poles = append(poles, geom.Vec3{X: xs[i], Y: y, Z: fs[i] + xs[i]*y})
			}
		}
		if span+2 < len(cuts) {
			knots = append(knots, cuts[span+1], cuts[span+1])
		}
	}
	knots = append(knots, 1, 1, 1)
	s, e := geom.NewBSplineSurface(2, 1, len(poles)/2, 2, poles, knots, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func exactNativeLine(t *testing.T, a, b geom.Vec3) ExactQuadraticUV {
	t.Helper()
	var h [3][3]*big.Rat
	for i, p := range []geom.Vec3{a, a.Add(b).Scale(.5), b} {
		h[i] = [3]*big.Rat{ratInset(p.X), ratInset(p.Y), big.NewRat(1, 1)}
	}
	q, e := NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	return q
}
func mixedNativeJoinFixture(t *testing.T) (ExactQuadraticFaceJoinInput, ExactQuadraticFaceJoinInput) {
	t.Helper()
	a := ExactQuadraticFaceJoinInput{Patch: scaledPartitionGraph(t, 3, false), NativeOuter: []ExactQuadraticUV{nativeNonbinaryTrim(t)}}
	points := []geom.Vec3{{X: 1, Y: .125}, {X: 1, Y: .5}, {Y: .5}, {Y: .125}}
	for i := 0; i+1 < len(points); i++ {
		a.NativeOuter = append(a.NativeOuter, exactNativeLine(t, points[i], points[i+1]))
	}
	b := ExactQuadraticFaceJoinInput{Patch: scaledPartitionGraph(t, 2, true)}
	c, e := geom.NewBSpline(2, []geom.Vec3{{Y: .1875}, {X: .5, Y: .5}, {X: 1, Y: .1875}}, []float64{10, 10, 10, 26, 26, 26}, []float64{1, .5, .25})
	if e != nil {
		t.Fatal(e)
	}
	b.SplineOuter = []SplineUV{{Curve: c, Range: UVInterval{Min: 10, Max: 26}, Reversed: true}}
	points = []geom.Vec3{{Y: .1875}, {}, {X: 1}, {X: 1, Y: .1875}}
	for i := 0; i+1 < len(points); i++ {
		c, e := geom.NewBSpline(1, []geom.Vec3{points[i], points[i+1]}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		b.SplineOuter = append(b.SplineOuter, SplineUV{Curve: c, Range: UVInterval{Max: 1}})
	}
	return a, b
}
func TestNativeExactQuadraticMixedFaceJoin(t *testing.T) {
	a, b := mixedNativeJoinFixture(t)
	o := ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .3}}}}
	got, e := StitchExactQuadraticFacesCertified(context.Background(), a, b, o)
	if e != nil {
		t.Fatal(e)
	}
	if !got.Agreement.Complete || !got.Agreement.Equal || len(got.Agreement.Intervals) < 4 {
		t.Fatal("missing whole symbolic identity")
	}
	if got.Trims[0][0].Source.HomogeneousControls()[1][1].Cmp(big.NewRat(1, 6)) != 0 {
		t.Fatal("native coefficient lost")
	}
	mapping := got.SourceParameterMaps[1][0]
	if mapping.Offset.Cmp(big.NewRat(26, 1)) != 0 || mapping.Scale.Cmp(big.NewRat(-16, 1)) != 0 {
		t.Fatal("original spline range mapping")
	}
	required := map[string]bool{"2/5": false, "2/3": false, "6/7": false}
	for i, q := range got.SharedParameters {
		if _, ok := required[q.RatString()]; ok {
			required[q.RatString()] = true
		}
		uv, _, e := a.NativeOuter[0].EvaluateRational(q)
		if e != nil {
			t.Fatal(e)
		}
		x, y := uv[0], new(big.Rat).Mul(big.NewRat(3, 1), uv[1])
		f := new(big.Rat).Mul(x, x)
		if x.Cmp(big.NewRat(1, 2)) > 0 {
			d := new(big.Rat).Sub(x, big.NewRat(1, 2))
			f.Add(big.NewRat(1, 4), new(big.Rat).Mul(d, d))
		}
		z := new(big.Rat).Add(f, new(big.Rat).Mul(x, y))
		p := geom.Vec3{}
		p.X, _ = x.Float64()
		p.Y, _ = y.Float64()
		p.Z, _ = z.Float64()
		if got.Mesh.Vertices[got.SharedVertices[i]] != p {
			t.Fatal("independent true seam witness")
		}
	}
	for q, found := range required {
		if !found {
			t.Fatal("omitted rational cut", q)
		}
	}
	area := 0.
	edges := map[[2]int]int{}
	balances := map[[2]int]int{}
	for _, tri := range got.Mesh.Triangles {
		a, b, c := got.Mesh.Vertices[tri[0]], got.Mesh.Vertices[tri[1]], got.Mesh.Vertices[tri[2]]
		v := ((b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)) / 2
		area += v
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
	// This is signed area, not a display-embedding assertion. Boundary sample
	// replacement can fold triangles while preserving image-distance bounds.
	exactArea := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(3, 1), got.Faces[0].Boundary.Region.PolygonUVArea), new(big.Rat).Mul(big.NewRat(2, 1), got.Faces[1].Boundary.Region.PolygonUVArea))
	if exactArea.Cmp(big.NewRat(3, 2)) != 0 {
		t.Fatal("independent exact UV fill coverage", exactArea)
	}
	if math.Abs(area-1.5) > 1e-12 {
		t.Fatal("independent planar projection coverage", area)
	}
	if len(got.Mesh.Vertices)-len(edges)+len(got.Mesh.Triangles) != 1 {
		t.Fatal("sheet not disk")
	}
	for i := 0; i+1 < len(got.SharedVertices); i++ {
		a, b := got.SharedVertices[i], got.SharedVertices[i+1]
		if a > b {
			a, b = b, a
		}
		key := [2]int{a, b}
		if edges[key] != 2 || balances[key] != 0 {
			t.Fatal("opposite seam incidence")
		}
	}
	if got.FillSquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.3), ratInset(.3))) > 0 {
		t.Fatal("distance")
	}
	t.Logf("triangles=%d seam=%d identity intervals=%d exact operations=%d", len(got.Mesh.Triangles), len(got.SharedVertices), len(got.Agreement.Intervals), got.Agreement.ExactOperations)
	limited := o
	limited.Agreement.MaxEvents = 2
	if r, e := StitchExactQuadraticFacesCertified(context.Background(), a, b, limited); !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("partial agreement budget", e)
	}
	h := a.NativeOuter[0].HomogeneousControls()
	h[1][1].Add(h[1][1], big.NewRat(1, 257))
	a.NativeOuter[0], e = NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := StitchExactQuadraticFacesCertified(context.Background(), a, b, o); e == nil || len(r.Mesh.Triangles) != 0 {
		t.Fatal("false whole-seam identity")
	}
}

func TestNativeExactQuadraticBothOwnersAndLimits(t *testing.T) {
	a, b := mixedNativeJoinFixture(t)
	for _, s := range b.SplineOuter {
		h, e := quadraticTrimSourceControls(s)
		if e != nil {
			t.Fatal(e)
		}
		q, e := NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
		b.NativeOuter = append(b.NativeOuter, q)
	}
	b.SplineOuter = nil
	o := ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .3}}}}
	result, e := StitchExactQuadraticFacesCertified(context.Background(), a, b, o)
	if e != nil || !result.Agreement.Equal {
		t.Fatal(e)
	}
	o.MaxSharedVertices = 3
	if r, e := StitchExactQuadraticFacesCertified(context.Background(), a, b, o); !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("partial seam budget", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := StitchExactQuadraticFacesCertified(ctx, a, b, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	b.SplineOuter = []SplineUV{{}}
	if _, e := StitchExactQuadraticFacesCertified(context.Background(), a, b, o); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("ambiguous representation", e)
	}
}
