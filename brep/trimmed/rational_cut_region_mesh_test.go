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

func exactRationalCutCircle(t *testing.T, r float64, reverse bool) []SplineUV {
	t.Helper()
	x, y := .5, .5
	poles := [][3]geom.Vec3{{{X: x + r, Y: y}, {X: x + r, Y: y + r}, {X: x, Y: y + r}}, {{X: x, Y: y + r}, {X: x - r, Y: y + r}, {X: x - r, Y: y}}, {{X: x - r, Y: y}, {X: x - r, Y: y - r}, {X: x, Y: y - r}}, {{X: x, Y: y - r}, {X: x + r, Y: y - r}, {X: x + r, Y: y}}}
	var loop []SplineUV
	for _, p := range poles {
		c, e := geom.NewBSpline(2, p[:], []float64{-2, -2, -2, 6, 6, 6}, []float64{.5, .5, 1})
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, SplineUV{Curve: c, Range: UVInterval{Min: -2, Max: 6}})
	}
	if reverse {
		for i, j := 0, len(loop)-1; i < j; i, j = i+1, j-1 {
			loop[i], loop[j] = loop[j], loop[i]
		}
		for i := range loop {
			loop[i].Reversed = true
		}
	}
	return loop
}
func TestRationalCutCurvedAnnularFill(t *testing.T) {
	p := partitionedSeamPatch(t, false)
	outer, hole := exactRationalCutCircle(t, .375, false), exactRationalCutCircle(t, .125, true)
	cuts := make([][][]*big.Rat, 2)
	cuts[0] = make([][]*big.Rat, 4)
	cuts[1] = make([][]*big.Rat, 4)
	cuts[0][1] = []*big.Rat{big.NewRat(1, 3)}
	cuts[1][0] = []*big.Rat{big.NewRat(1, 3)}
	o := RationalCutFaceMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .1}}}
	got, e := TessellateRationalCutRegionCertified(context.Background(), p, outer, [][]SplineUV{hole}, cuts, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.BoundaryLoops) != 2 || len(got.LoopTrims) != 2 {
		t.Fatal("missing hole")
	}
	for li, loop := range got.Boundary.Region.Loops {
		radius := big.NewRat(3, 8)
		if li == 1 {
			radius = big.NewRat(1, 8)
		}
		for _, uv := range loop.Boundary.ExactUV {
			x, y := new(big.Rat).Sub(uv[0], big.NewRat(1, 2)), new(big.Rat).Sub(uv[1], big.NewRat(1, 2))
			norm := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
			if norm.Cmp(new(big.Rat).Mul(radius, radius)) != 0 {
				t.Fatal("not exact circle")
			}
		}
	}
	// Input hole trim0 is normalized at reversed index3 and normalized q=2/3.
	found := false
	for j, original := range got.Boundary.Region.OriginalSource[1] {
		if original != 0 {
			continue
		}
		for _, q := range got.LoopTrims[1][j].NormalizedParameters {
			if q.Cmp(big.NewRat(2, 3)) == 0 {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("lost original hole traversal cut")
	}
	holeUV := got.Boundary.Region.Loops[1].Boundary.ExactUV
	minRadius := big.NewRat(1, 64)
	for i, a := range holeUV {
		b := holeUV[(i+1)%len(holeUV)]
		x := new(big.Rat).Sub(new(big.Rat).Quo(new(big.Rat).Add(a[0], b[0]), big.NewRat(2, 1)), big.NewRat(1, 2))
		y := new(big.Rat).Sub(new(big.Rat).Quo(new(big.Rat).Add(a[1], b[1]), big.NewRat(2, 1)), big.NewRat(1, 2))
		norm := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
		if norm.Cmp(minRadius) < 0 {
			minRadius = norm
		}
	}
	if minRadius.Cmp(big.NewRat(1, 64)) >= 0 {
		t.Fatal("fixture must have polygon hole strictly smaller than circle")
	}
	area := new(big.Rat)
	edges := map[[2]int]int{}
	for _, tri := range got.Mesh.Triangles {
		a, b, c := got.ExactUV[tri[0]], got.ExactUV[tri[1]], got.ExactUV[tri[2]]
		det := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(c[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(c[0], a[0])))
		if det.Sign() <= 0 {
			t.Fatal("orientation")
		}
		area.Add(area, new(big.Rat).Quo(det, big.NewRat(2, 1)))
		norm := new(big.Rat)
		for k := 0; k < 2; k++ {
			center := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Add(a[k], b[k]), c[k]), big.NewRat(3, 1))
			center.Sub(center, big.NewRat(1, 2))
			norm.Add(norm, new(big.Rat).Mul(center, center))
		}
		if norm.Cmp(minRadius) < 0 {
			t.Fatal("hole filled")
		}
		for j, a := range tri {
			b := tri[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}]++
		}
	}
	if area.Cmp(got.Boundary.Region.PolygonUVArea) != 0 {
		t.Fatal("exact signed area invariant")
	}
	if len(got.Mesh.Vertices)-len(edges)+len(got.Mesh.Triangles) != 0 {
		t.Fatal("annulus Euler characteristic")
	}
	approximate, _ := area.Float64()
	if math.Abs(approximate-math.Pi/8) > .01 {
		t.Fatal("independent circular annulus area", approximate)
	}
	if got.FillSquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.1), ratInset(.1))) > 0 {
		t.Fatal("deflection")
	}
	segments, checks := 0, 0
	for _, loop := range got.LoopTrims {
		for _, trim := range loop {
			segments += len(trim.SquaredErrors)
			checks += trim.Checks
		}
	}
	t.Logf("triangles=%d boundary=%d checks=%d area=%g analytic=%g cells=%d", len(got.Mesh.Triangles), segments, checks, approximate, math.Pi/8, got.SpanCellChecks)
	for _, which := range []int{0, 1, 2} {
		limited := o
		if which == 0 {
			limited.Face.Mesh.Boundary.Tessellation.MaxSegments = segments - 1
		} else if which == 1 {
			limited.Face.Mesh.Boundary.Tessellation.MaxChecks = checks - 1
		} else {
			limited.Face.MaxSpanCells = got.SpanCellChecks - 1
		}
		r, e := TessellateRationalCutRegionCertified(context.Background(), p, outer, [][]SplineUV{hole}, cuts, limited)
		if !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
			t.Fatal("non-atomic cumulative budget", which, e)
		}
	}
}
func TestRationalCutRegionRejectsInvalidHoles(t *testing.T) {
	p := partitionedSeamPatch(t, false)
	outer := exactRationalCutCircle(t, .375, false)
	o := RationalCutFaceMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .1}}}
	for _, hole := range [][]SplineUV{exactRationalCutCircle(t, .125, false), exactRationalCutCircle(t, .375, true), exactRationalCutCircle(t, .5, true)} {
		if r, e := TessellateRationalCutRegionCertified(context.Background(), p, outer, [][]SplineUV{hole}, nil, o); e == nil || len(r.Mesh.Triangles) != 0 {
			t.Fatal("invalid hole accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := TessellateRationalCutRegionCertified(ctx, p, outer, nil, nil, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestRationalCutAnnulusIndependentObliqueArea(t *testing.T) {
	// Exact affine chart S=(2u,3v,u+v), Jacobian cross=(-3,-2,6)
	// has norm7. Author it as two spline spans independently.
	var poles []geom.Vec3
	for _, u := range []float64{0, .5, 1} {
		for _, v := range []float64{0, 1} {
			poles = append(poles, geom.Vec3{X: 2 * u, Y: 3 * v, Z: u + v})
		}
	}
	surface, e := geom.NewBSplineSurface(1, 1, 3, 2, poles, []float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := RationalCutFaceMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .05}}}
	got, e := TessellateRationalCutRegionCertified(context.Background(), p, exactRationalCutCircle(t, .375, false), [][]SplineUV{exactRationalCutCircle(t, .125, true)}, nil, o)
	if e != nil {
		t.Fatal(e)
	}
	polygonArea, _ := got.Boundary.Region.PolygonUVArea.Float64()
	if math.Abs(got.Mesh.Area()-7*polygonArea) > 1e-12 {
		t.Fatal("independent affine Jacobian area")
	}
	if math.Abs(got.Mesh.Area()-7*math.Pi/8) > .015 {
		t.Fatal("independent analytic annulus area", got.Mesh.Area(), 7*math.Pi/8)
	}
	t.Logf("model polygon area=%g true area=%g triangles=%d", got.Mesh.Area(), 7*math.Pi/8, len(got.Mesh.Triangles))
}
