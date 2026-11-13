package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestAffineQuadraticRegionIndependentAnnulus(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 3, Z: 1}, {X: 2, Z: 1}, {X: 2, Y: 3, Z: 2}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	circle := func(cx, cy, r float64, clockwise bool) []SplineUV {
		controls := [][3]geom.Vec3{{{X: cx + r, Y: cy}, {X: cx + r, Y: cy + r}, {X: cx, Y: cy + r}}, {{X: cx, Y: cy + r}, {X: cx - r, Y: cy + r}, {X: cx - r, Y: cy}}, {{X: cx - r, Y: cy}, {X: cx - r, Y: cy - r}, {X: cx, Y: cy - r}}, {{X: cx, Y: cy - r}, {X: cx + r, Y: cy - r}, {X: cx + r, Y: cy}}}
		var loop []SplineUV
		for _, p := range controls {
			c, e := geom.NewBSpline(2, p[:], []float64{0, 0, 0, 1, 1, 1}, []float64{.5, .5, 1})
			if e != nil {
				t.Fatal(e)
			}
			loop = append(loop, SplineUV{Curve: c, Range: UVInterval{Max: 1}})
		}
		if clockwise {
			for i, j := 0, len(loop)-1; i < j; i, j = i+1, j-1 {
				loop[i], loop[j] = loop[j], loop[i]
			}
			for i := range loop {
				loop[i].Reversed = true
			}
		}
		return loop
	}
	outer, hole := circle(.5, .5, .375, false), circle(.5, .5, .125, true)
	options := QuadraticTrimLoopOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .01}}
	result, e := BoundAffineQuadraticTrimRegionArea(context.Background(), patch, outer, [][]SplineUV{hole}, options)
	if e != nil {
		t.Fatal(e)
	}
	if result.SourceProof.Status != trimvalidate.Certified || result.PolygonProof.Status != trimvalidate.Certified || len(result.Loops) != 2 {
		t.Fatal("missing region proof")
	}
	lo, _ := result.SourceUVArea.Lower.Float64()
	hi, _ := result.SourceUVArea.Upper.Float64()
	if !(lo < math.Pi/8 && hi > math.Pi/8) {
		t.Fatal("analytic annulus area", lo, hi)
	}
	for i, n := range []float64{-3, -2, 6} {
		a, _ := result.ModelAreaVector[i].Lower.Float64()
		b, _ := result.ModelAreaVector[i].Upper.Float64()
		expected := n * math.Pi / 8
		if expected < a || expected > b {
			t.Fatal("model annulus area", i, a, b)
		}
	}
	for i, mapping := range result.OriginalSource[1] {
		if mapping != 3-i {
			t.Fatal("hole source ancestry")
		}
	}

	// The chord polygon hole is strictly smaller than the circle hole.
	// Construct an exact point between one chord midpoint and its circular
	// source midpoint: it is polygon material but lies inside the source hole.
	boundary := result.Loops[1].Boundary
	segment := boundary.Segments[0]
	original := hole[result.OriginalSource[1][segment.Source]]
	original.Range = UVInterval{Min: segment.Start, Max: segment.End}
	original.Reversed = false
	if original.Range.Min > original.Range.Max {
		original.Range.Min, original.Range.Max = original.Range.Max, original.Range.Min
	}
	h, e := quadraticTrimSourceControls(original)
	if e != nil {
		t.Fatal(e)
	}
	var circular, mid, between [2]*big.Rat
	denominator := new(big.Rat).Add(new(big.Rat).Add(h[0][2], new(big.Rat).Mul(big.NewRat(2, 1), h[1][2])), h[2][2])
	for k := 0; k < 2; k++ {
		numerator := new(big.Rat).Add(new(big.Rat).Add(h[0][k], new(big.Rat).Mul(big.NewRat(2, 1), h[1][k])), h[2][k])
		circular[k] = new(big.Rat).Quo(numerator, denominator)
		mid[k] = new(big.Rat).Quo(new(big.Rat).Add(boundary.ExactUV[0][k], boundary.ExactUV[1][k]), big.NewRat(2, 1))
		between[k] = new(big.Rat).Quo(new(big.Rat).Add(circular[k], mid[k]), big.NewRat(2, 1))
	}
	radius := new(big.Rat)
	for _, x := range between {
		d := new(big.Rat).Sub(x, big.NewRat(1, 2))
		radius.Add(radius, new(big.Rat).Mul(d, d))
	}
	if radius.Cmp(big.NewRat(1, 64)) >= 0 {
		t.Fatal("witness not inside circular hole")
	}
	a, b := boundary.ExactUV[0], boundary.ExactUV[1]
	cross := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(between[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(between[0], a[0])))
	if cross.Sign() >= 0 {
		t.Fatal("witness not outside convex chord hole")
	}
	modelDistance := func(p, q [2]*big.Rat) *big.Rat {
		u := new(big.Rat).Sub(p[0], q[0])
		v := new(big.Rat).Sub(p[1], q[1])
		z := new(big.Rat).Add(u, v)
		return new(big.Rat).Add(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(u, u)), new(big.Rat).Mul(big.NewRat(9, 1), new(big.Rat).Mul(v, v))), new(big.Rat).Mul(z, z))
	}
	if result.FillSquaredDistance == nil || modelDistance(between, circular).Cmp(result.FillSquaredDistance) > 0 || modelDistance(between, mid).Cmp(result.FillSquaredDistance) > 0 || modelDistance(circular, mid).Cmp(result.FillSquaredDistance) > 0 {
		t.Fatal("filled region or source boundary distance witness exceeds certificate")
	}

	display, e := TessellateAffineQuadraticTrimRegionCertified(context.Background(), patch, outer, [][]SplineUV{hole}, AffineQuadraticTrimMeshOptions{Region: options})
	if e != nil {
		t.Fatal("certified annulus face mesh", e)
	}
	polygonArea, _ := display.Region.PolygonUVArea.Float64()
	if math.Abs(display.Mesh.Area()-7*polygonArea) > 1e-12 {
		t.Fatal("independent oblique polygon mesh area", display.Mesh.Area(), polygonArea)
	}
	if display.DisplaySquaredDistance.Sign() <= 0 || display.FillSquaredDistance.Cmp(display.Region.FillSquaredDistance) < 0 {
		t.Fatal("actual model rounding omitted")
	}
	edges := map[[2]int]int{}
	for i, triangle := range display.Mesh.Triangles {
		if display.OriginalFace[i] != -1 {
			t.Fatal("standalone face ancestry")
		}
		x, y := 0., 0.
		for j, a := range triangle {
			u, _ := display.ExactUV[a][0].Float64()
			v, _ := display.ExactUV[a][1].Float64()
			x += u / 3
			y += v / 3
			b := triangle[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}]++
		}
		if (x-.5)*(x-.5)+(y-.5)*(y-.5) >= .375*.375 {
			t.Fatal("triangle outside outer circular fill")
		}
		insideHole := true
		for j, a := range display.Region.Loops[1].Boundary.ExactUV {
			b := display.Region.Loops[1].Boundary.ExactUV[(j+1)%len(display.Region.Loops[1].Boundary.ExactUV)]
			ax, _ := a[0].Float64()
			ay, _ := a[1].Float64()
			bx, _ := b[0].Float64()
			by, _ := b[1].Float64()
			if (bx-ax)*(y-ay)-(by-ay)*(x-ax) < 0 {
				insideHole = false
				break
			}
		}
		if insideHole {
			t.Fatal("triangle bridges polygon hole")
		}
	}
	if len(display.Mesh.Vertices)-len(edges)+len(display.Mesh.Triangles) != 0 {
		t.Fatal("annulus mesh Euler characteristic")
	}
	for _, n := range edges {
		if n != 1 && n != 2 {
			t.Fatal("nonmanifold sheet")
		}
	}
	failed, e := TessellateAffineQuadraticTrimRegionCertified(context.Background(), patch, outer, [][]SplineUV{hole}, AffineQuadraticTrimMeshOptions{Region: options, MaxTriangles: 1})
	if e == nil || len(failed.Mesh.Vertices) != 0 {
		t.Fatal("partial mesh on triangle budget failure")
	}
	for _, holes := range [][][]SplineUV{{circle(.5, .5, .125, false)}, {hole, hole}, {circle(.75, .5, .125, true)}, {circle(.875, .5, .125, true)}} {
		bad, e := BoundAffineQuadraticTrimRegionArea(context.Background(), patch, outer, holes, options)
		if e == nil || len(bad.Loops) != 0 {
			t.Fatal("invalid hole configuration accepted")
		}
	}
	options.Tessellation.MaxSegments = 8
	bad, e := BoundAffineQuadraticTrimRegionArea(context.Background(), patch, outer, [][]SplineUV{hole}, options)
	if e == nil || len(bad.Loops) != 0 {
		t.Fatal("partial global budget")
	}
	t.Logf("annulus UV area [%g,%g] contains pi/8", lo, hi)
}
