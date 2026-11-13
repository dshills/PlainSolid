package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestAffineQuadraticLoopAreaIndependentCircle(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 3, Z: 1}, {X: 2, Z: 1}, {X: 2, Y: 3, Z: 2}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	controls := [][3]geom.Vec3{{{X: .75, Y: .5}, {X: .75, Y: .75}, {X: .5, Y: .75}}, {{X: .5, Y: .75}, {X: .25, Y: .75}, {X: .25, Y: .5}}, {{X: .25, Y: .5}, {X: .25, Y: .25}, {X: .5, Y: .25}}, {{X: .5, Y: .25}, {X: .75, Y: .25}, {X: .75, Y: .5}}}
	var trims []SplineUV
	for _, p := range controls {
		c, e := geom.NewBSpline(2, p[:], []float64{0, 0, 0, 1, 1, 1}, []float64{.5, .5, 1})
		if e != nil {
			t.Fatal(e)
		}
		trims = append(trims, SplineUV{Curve: c, Range: UVInterval{Max: 1}})
	}
	var previous *big.Rat
	for _, deflection := range []float64{.04, .004} {
		result, e := BoundAffineQuadraticTrimLoopArea(context.Background(), patch, trims, QuadraticTrimLoopOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: deflection}})
		if e != nil {
			t.Fatal(e)
		}
		lo, _ := result.SourceUVArea.Lower.Float64()
		hi, _ := result.SourceUVArea.Upper.Float64()
		if !(lo < math.Pi/16 && hi > math.Pi/16) {
			t.Fatal("independent circle area", lo, hi)
		}
		polygon, _ := result.PolygonUVArea.Float64()
		if polygon >= math.Pi/16 {
			t.Fatal("inscribed polygon distinction", polygon)
		}
		for i, factor := range []float64{-3, -2, 6} {
			lower, _ := result.ModelAreaVector[i].Lower.Float64()
			upper, _ := result.ModelAreaVector[i].Upper.Float64()
			expected := factor * math.Pi / 16
			if expected < lower || expected > upper {
				t.Fatal("oriented model area vector", i, lower, upper)
			}
		}
		width := new(big.Rat).Sub(result.SourceUVArea.Upper, result.SourceUVArea.Lower)
		if previous != nil && width.Cmp(previous) >= 0 {
			t.Fatal("partition did not improve area enclosure")
		}
		previous = width
		t.Logf("deflection %.3g, %d chords, UV area [%g,%g]", deflection, len(result.Boundary.Segments), lo, hi)
	}
	// Straight sides give an exact integral, even on this oblique chart.
	corners := []geom.Vec3{{X: .25, Y: .25}, {X: .75, Y: .25}, {X: .75, Y: .75}, {X: .25, Y: .75}}
	trims = nil
	for i, p := range corners {
		c, e := geom.NewBSpline(1, []geom.Vec3{p, corners[(i+1)%4]}, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		trims = append(trims, SplineUV{Curve: c, Range: UVInterval{Max: 1}})
	}
	exact, e := BoundAffineQuadraticTrimLoopArea(context.Background(), patch, trims, QuadraticTrimLoopOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .01}})
	if e != nil {
		t.Fatal(e)
	}
	if exact.SourceUVArea.Lower.Cmp(big.NewRat(1, 4)) != 0 || exact.SourceUVArea.Upper.Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal("exact square integral")
	}
	for i, expected := range []*big.Rat{big.NewRat(-3, 4), big.NewRat(-1, 2), big.NewRat(3, 2)} {
		if exact.ModelAreaVector[i].Lower.Cmp(expected) != 0 || exact.ModelAreaVector[i].Upper.Cmp(expected) != 0 {
			t.Fatal("exact oblique area")
		}
	}
	curved, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	nonaffine, e := geom.NewSurfacePatch(curved, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bad, e := BoundAffineQuadraticTrimLoopArea(context.Background(), nonaffine, trims, QuadraticTrimLoopOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .01}})
	if e == nil || bad.SourceUVArea.Lower != nil {
		t.Fatal("curved support accepted as affine")
	}
}
