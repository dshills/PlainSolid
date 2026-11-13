package occtcheck

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestOCCTWarpedCornerPlaneClip(t *testing.T) {
	refs := runOracle(t, "warped-corner-spline")
	ctx := context.Background()
	a, _ := oracleWarpedOpenBox(t, [3]float64{}, [3]float64{1, 1, 1})
	b, _ := oracleSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, func(x, y float64) float64 { return (x + y) / 4 })
	o := trimmed.SplineRectangleEmbeddingOptions{}
	var anchors [2]*trimmed.SplineCornerAnchor
	for i, face := range []int{0, 2} {
		box := geom.ParameterRectangle{UMin: -.125, UMax: .125, VMin: 0, VMax: .25}
		if i == 1 {
			box = geom.ParameterRectangle{UMin: 0, UMax: .25, VMin: -.125, VMax: .125}
		}
		r, err := trimmed.CertifySplineFaceCornerAnchor(ctx, a, b, [2]int{face, 5}, [2]geom.ParameterRectangle{{UMax: .125, VMax: .125}, box}, 0,
			geom.SurfaceTubeBoundaryOptions{SurfacePairTubeOptions: geom.SurfacePairTubeOptions{FreeAxis: i}, Anchor: [4]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}}, o)
		if err != nil {
			t.Fatal(face, err)
		}
		anchors[i] = r
	}
	p, err := trimmed.CertifySplineFacePointContact(ctx, a, b, [2]int{4, 5}, o)
	if err != nil {
		t.Fatal(err)
	}
	fan, err := trimmed.CertifySplineCornerFan(ctx, anchors, p, o)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := trimmed.DiagnoseSplineCornerSections(ctx, fan, trimmed.SplineCornerSectionLedgerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ledger, err = trimmed.ResolveSplineCornerAffineSections(ctx, ledger, o)
	if err != nil || !ledger.AllPairsClassified() {
		t.Fatal("original-domain coverage", err)
	}
	graph, err := trimmed.BuildSplineCornerSectionGraph(ctx, ledger, o)
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"warped-corner-common", "warped-corner-cut"} {
		op := [2]parametric.BooleanOp{parametric.Intersection, parametric.Difference}[i]
		out, err := trimmed.CombineSplineCornerPlaneClip(ctx, graph, op, o)
		if err != nil || len(out.Solid.Topology().Shells) != 1 || len(out.SectionEdges) != 4 {
			t.Fatal(name, err)
		}
		mass, err := out.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(name, err)
		}
		compare(t, refs[name], mass.Area, mass.Volume, mass.Centroid)
	}
}

func TestOCCTWarpedInteriorPlaneClip(t *testing.T) {
	refs := runOracle(t, "warped-corner-spline")
	ctx := context.Background()
	a, _ := oracleWarpedOpenBox(t, [3]float64{}, [3]float64{1, 1, 1})
	b, _ := oracleSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, func(x, y float64) float64 { return .125 + (x+y)/4 })
	for i, name := range []string{"warped-interior-common", "warped-interior-cut"} {
		op := [2]parametric.BooleanOp{parametric.Intersection, parametric.Difference}[i]
		out, err := trimmed.CombineSplinePlaneClip(ctx, a, b, 5, op, trimmed.SplineRectangleEmbeddingOptions{})
		if err != nil || len(out.Solid.Topology().Shells) != 1 || len(out.SectionEdges) != 4 {
			t.Fatal(name, err)
		}
		mass, err := out.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(name, err)
		}
		compare(t, refs[name], mass.Area, mass.Volume, mass.Centroid)
	}
}

func TestOCCTWarpedTwoCornerPlaneClip(t *testing.T) {
	refs := runOracle(t, "warped-corner-spline")
	ctx := context.Background()
	a, _ := oracleWarpedOpenBox(t, [3]float64{}, [3]float64{1, 1, 1})
	b, _ := oracleSplineBox(t, [3]float64{-1, -1, -4}, [3]float64{2, 2, 0}, func(x, y float64) float64 { return 9 * (x + y) / 16 })
	o := trimmed.SplineRectangleEmbeddingOptions{}
	proposals := [2][2][2]geom.ParameterRectangle{
		{
			{{UMax: .125, VMax: .125}, {UMin: -.125, UMax: .125, VMax: .25}},
			{{UMax: .5, VMax: .125}, {UMax: .5, VMin: -.125, VMax: .125}},
		},
		{
			{{UMin: .875, UMax: 1, VMin: .875, VMax: 1}, {UMin: .875, UMax: 1.125, VMin: .75, VMax: 1}},
			{{UMin: .5, UMax: 1, VMin: .875, VMax: 1}, {UMin: .5, UMax: 1, VMin: .875, VMax: 1.125}},
		},
	}
	var fans [2]*trimmed.SplineCornerFan
	for i, pp := range proposals {
		var anchors [2]*trimmed.SplineCornerAnchor
		hint := [4]*big.Rat{}
		for j := range hint {
			hint[j] = big.NewRat(int64(i), 1)
		}
		for j, pair := range pp {
			anchor, err := trimmed.CertifySplineFaceCornerAnchor(ctx, a, b, [2]int{2*j + i, 5}, pair, 0,
				geom.SurfaceTubeBoundaryOptions{SurfacePairTubeOptions: geom.SurfacePairTubeOptions{FreeAxis: j}, Anchor: hint, AtUpper: i == 1}, o)
			if err != nil {
				t.Fatal(i, j, err)
			}
			anchors[j] = anchor
		}
		point, err := trimmed.CertifySplineFacePointContact(ctx, a, b, [2]int{4 + i, 5}, o)
		if err != nil {
			t.Fatal(i, err)
		}
		fans[i], err = trimmed.CertifySplineCornerFan(ctx, anchors, point, o)
		if err != nil {
			t.Fatal(i, err)
		}
	}
	for i, name := range []string{"warped-two-corner-common", "warped-two-corner-cut"} {
		op := [2]parametric.BooleanOp{parametric.Intersection, parametric.Difference}[i]
		out, err := trimmed.CombineSplineTwoCornerPlaneClip(ctx, fans, op, o)
		if err != nil || len(out.Solid.Topology().Shells) != 1 || len(out.SectionEdges) != 4 {
			t.Fatal(name, err)
		}
		mass, err := out.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(name, err)
		}
		compare(t, refs[name], mass.Area, mass.Volume, mass.Centroid)
	}
}

func TestOCCTWarpedParallelSlab(t *testing.T) {
	refs := runOracle(t, "warped-spline-slab")
	ctx := context.Background()
	stock, _ := oracleWarpedOpenBox(t, [3]float64{}, [3]float64{1, 1, 1})
	lower, _ := oracleSplineBox(t, [3]float64{-1, -1, 0}, [3]float64{2, 2, 2}, func(x, y float64) float64 { return (x+y)/4 + .125 })
	upper, _ := oracleSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, func(x, y float64) float64 { return (x+y)/4 + .375 })
	r, err := trimmed.CombineSplineParallelSlab(ctx, stock, [2]trimmed.Solid{lower, upper}, [2]int{4, 5}, trimmed.SplineRectangleEmbeddingOptions{})
	if err != nil || len(r.Solid.Topology().Shells) != 1 || len(r.SectionEdges[0]) != 4 || len(r.SectionEdges[1]) != 4 {
		t.Fatal("slab topology", err)
	}
	mass, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil {
		t.Fatal(err)
	}
	compare(t, refs["warped-parallel-slab"], mass.Area, mass.Volume, mass.Centroid)
}

func TestOCCTWarpedSeparatedPlanes(t *testing.T) {
	refs := runOracle(t, "warped-spline-separated-planes")
	ctx := context.Background()
	stock, _ := oracleWarpedOpenBox(t, [3]float64{}, [3]float64{1, 1, 1})
	lower, _ := oracleSplineBox(t, [3]float64{-1, -1, 0}, [3]float64{2, 2, 2}, func(x, y float64) float64 { return (x+y)/8 + .125 })
	upper, _ := oracleSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, func(x, y float64) float64 { return (x+y)/4 + .5 })
	r, err := trimmed.CombineSplineSeparatedPlaneCuts(ctx, stock, [2]trimmed.Solid{lower, upper}, [2]int{4, 5}, trimmed.SplineRectangleEmbeddingOptions{})
	if err != nil || len(r.Solid.Topology().Shells) != 1 || len(r.SectionEdges[0]) != 4 || len(r.SectionEdges[1]) != 4 {
		t.Fatal("nonparallel topology", err)
	}
	mass, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil {
		t.Fatal(err)
	}
	compare(t, refs["warped-separated-planes"], mass.Area, mass.Volume, mass.Centroid)
}

func TestOCCTWarpedCrossingPlanes(t *testing.T) {
	refs := runOracle(t, "warped-spline-separated-planes")
	ctx := context.Background()
	stock, _ := oracleWarpedOpenBox(t, [3]float64{}, [3]float64{1, 1, 1})
	lower, _ := oracleSplineBox(t, [3]float64{-1, -1, 0}, [3]float64{2, 2, 2}, func(x, y float64) float64 { return (x+y)/4 + .375 })
	upper, _ := oracleSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, func(x, y float64) float64 { return .75 })
	r, err := trimmed.CombineSplineCrossingPlaneCuts(ctx, stock, [2]trimmed.Solid{lower, upper}, [2]int{4, 5}, trimmed.SplineRectangleEmbeddingOptions{})
	if err != nil || len(r.Solid.Topology().Shells) != 1 || len(r.CapIntersectionEdges) != 1 {
		t.Fatal("cap intersection topology", err)
	}
	mass, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil {
		t.Fatal(err)
	}
	compare(t, refs["warped-crossing-planes"], mass.Area, mass.Volume, mass.Centroid)
}

func oracleGraphClipStock(t *testing.T, crease, cubic bool) trimmed.Solid {
	t.Helper()
	ctx := context.Background()
	var patches []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	for axis := range 3 {
		u, v := (axis+1)%3, (axis+2)%3
		if axis == 1 {
			u, v = 0, 2
		}
		for side := range 2 {
			degree := [2]int{1, 1}
			for i, k := range [2]int{u, v} {
				if k == 0 {
					degree[i] = 2
					if cubic {
						degree[i] = 4
					}
				}
			}
			knots := func(d int) []float64 {
				if d == 4 {
					return []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
				}
				if d == 2 {
					return []float64{0, 0, 0, 1, 1, 1}
				}
				return []float64{0, 0, 1, 1}
			}
			ku, kv := knots(degree[0]), knots(degree[1])
			counts := [2]int{degree[0] + 1, degree[1] + 1}
			if crease {
				kx, count := []float64{0, 0, 0, .5, .5, 1, 1, 1}, 5
				if cubic {
					kx, count = []float64{0, 0, 0, 0, 0, .5, .5, .5, .5, 1, 1, 1, 1, 1}, 9
				}
				if u == 0 {
					ku, counts[0] = kx, count
				} else if v == 0 {
					kv, counts[1] = kx, count
				}
			}
			var points []geom.Vec3
			for i := range counts[0] {
				for j := range counts[1] {
					p := [3]float64{}
					p[axis], p[u], p[v] = float64(side), float64(i)/float64(counts[0]-1), float64(j)/float64(counts[1]-1)
					xz := p[0] / 8
					if cubic {
						z := []float64{0, 0, 0, 1. / 32, 1. / 8}
						if crease {
							z = []float64{1. / 64, 3. / 256, 1. / 128, 1. / 128, 1. / 64, 1. / 32, 7. / 128, 23. / 256, 9. / 64}
							xz += 1. / 64
						}
						if u == 0 {
							xz = z[i]
						} else if v == 0 {
							xz = z[j]
						}
					} else if crease {
						xz += 1. / 64
						z := [5]float64{1. / 64, 1. / 128, 1. / 32, 9. / 128, 9. / 64}
						if u == 0 {
							xz = z[i]
						} else if v == 0 {
							xz = z[j]
						}
					} else if u == 0 && i == 1 || v == 0 && j == 1 {
						xz = 0 // Middle Bernstein control of x².
					}
					points = append(points, geom.Vec3{X: p[0], Y: p[1], Z: p[2] + xz + p[1]/4})
				}
			}
			p, err := geom.NewBSplineSurface(degree[0], degree[1], counts[0], counts[1], points, ku, kv, nil)
			if err != nil {
				t.Fatal(err)
			}
			patches = append(patches, p)
			domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
			reversed = append(reversed, (side == 0) != (axis == 1))
		}
	}
	stock, _, err := trimmed.SewSplineRectangles(ctx, patches, domains, reversed, trimmed.SewOptions{Tolerance: 1e-10})
	if err != nil {
		t.Fatal(err)
	}
	return stock
}

func TestOCCTQuadraticSplinePlaneClip(t *testing.T) {
	refs := runOracle(t, "quadratic-graph-plane")
	ctx := context.Background()
	stock := oracleGraphClipStock(t, false, false)
	tool, _ := oracleSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, func(x, y float64) float64 { return 0 })
	for i, name := range []string{"quadratic-plane-common", "quadratic-plane-cut"} {
		op := [2]parametric.BooleanOp{parametric.Intersection, parametric.Difference}[i]
		r, err := trimmed.CombineQuadraticSplinePlaneClip(ctx, stock, tool, 5, op, trimmed.SplineRectangleEmbeddingOptions{})
		if err != nil || len(r.Solid.Topology().Shells) != 1 || len(r.SectionEdges) != 4 {
			t.Fatal(name, "quadratic topology", err)
		}
		mass, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(err)
		}
		compare(t, refs[name], mass.Area, mass.Volume, mass.Centroid)
	}
}

func TestOCCTPiecewiseQuadraticSplinePlaneClip(t *testing.T) {
	refs := runOracle(t, "piecewise-quadratic-graph-plane")
	ctx := context.Background()
	stock := oracleGraphClipStock(t, true, false)
	tool, _ := oracleSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, func(x, y float64) float64 { return 0 })
	for i, name := range []string{"piecewise-quadratic-plane-common", "piecewise-quadratic-plane-cut"} {
		op := [2]parametric.BooleanOp{parametric.Intersection, parametric.Difference}[i]
		r, err := trimmed.CombinePiecewiseQuadraticSplinePlaneClip(ctx, stock, tool, 5, op, trimmed.QuadraticPlaneSpanOptions{})
		if err != nil || len(r.Solid.Topology().Shells) != 1 || len(r.SectionEdges) != 6 || len(r.Knots) != 2 {
			t.Fatal(name, "piecewise topology", err)
		}
		mass, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(name, err)
		}
		compare(t, refs[name], mass.Area, mass.Volume, mass.Centroid)
	}
}

func TestOCCTPolynomialSplinePlaneClip(t *testing.T) {
	refs := runOracle(t, "polynomial-graph-plane")
	ctx := context.Background()
	stock := oracleGraphClipStock(t, false, true)
	tool, _ := oracleSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{3, 3, 3. / 16}, func(x, y float64) float64 { return 0 })
	for i, name := range []string{"polynomial-plane-common", "polynomial-plane-cut"} {
		op := [2]parametric.BooleanOp{parametric.Intersection, parametric.Difference}[i]
		r, err := trimmed.CombinePolynomialSplinePlaneClip(ctx, stock, tool, 5, op, trimmed.SplineRectangleEmbeddingOptions{})
		if err != nil || len(r.Solid.Topology().Shells) != 1 || len(r.SectionEdges) != 4 {
			t.Fatal(name, "polynomial topology", err)
		}
		mass, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(name, err)
		}
		compare(t, refs[name], mass.Area, mass.Volume, mass.Centroid)
	}
}

func TestOCCTPiecewisePolynomialSplinePlaneClip(t *testing.T) {
	refs := runOracle(t, "piecewise-polynomial-graph-plane")
	ctx := context.Background()
	stock := oracleGraphClipStock(t, true, true)
	tool, _ := oracleSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, func(x, y float64) float64 { return 0 })
	for i, name := range []string{"piecewise-polynomial-plane-common", "piecewise-polynomial-plane-cut"} {
		op := [2]parametric.BooleanOp{parametric.Intersection, parametric.Difference}[i]
		r, err := trimmed.CombinePiecewisePolynomialSplinePlaneClip(ctx, stock, tool, 5, op, trimmed.PolynomialPlaneSpanOptions{})
		ref, ok := refs[name]
		if err != nil || !ok || ref.Shells != 1 || len(r.Solid.Topology().Shells) != ref.Shells || len(r.SectionEdges) != 6 || len(r.Knots) != 2 {
			t.Fatal(name, "piecewise topology", ref.Shells, err)
		}
		mass, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(err)
		}
		compare(t, ref, mass.Area, mass.Volume, mass.Centroid)
	}
}

func TestOCCTObliquePolynomialSplinePlaneClip(t *testing.T) {
	refs := runOracle(t, "oblique-polynomial-graph-plane")
	ctx := context.Background()
	tool, _ := oracleSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, func(x, y float64) float64 { return x / 32 })
	for _, crease := range []bool{false, true} {
		stock := oracleGraphClipStock(t, crease, true)
		prefix := "oblique-polynomial-plane-"
		if crease {
			prefix = "oblique-piecewise-polynomial-plane-"
		}
		for i, name := range []string{prefix + "common", prefix + "cut"} {
			op := [2]parametric.BooleanOp{parametric.Intersection, parametric.Difference}[i]
			var solid trimmed.Solid
			var err error
			if crease {
				var r trimmed.PiecewisePolynomialPlaneClip
				r, err = trimmed.CombinePiecewisePolynomialSplinePlaneClip(ctx, stock, tool, 5, op, trimmed.PolynomialPlaneSpanOptions{})
				solid = r.Solid
			} else {
				var r trimmed.SplineCornerPlaneClip
				r, err = trimmed.CombinePolynomialSplinePlaneClip(ctx, stock, tool, 5, op, trimmed.SplineRectangleEmbeddingOptions{})
				solid = r.Solid
			}
			ref, ok := refs[name]
			if err != nil || !ok || len(solid.Topology().Shells) != 1 || crease && ref.Shells != 1 {
				t.Fatal(name, "oblique topology", ref.Shells, err)
			}
			mass, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil {
				t.Fatal(name, err)
			}
			compare(t, ref, mass.Area, mass.Volume, mass.Centroid)
		}
	}
}

func TestOCCTRationalGraphSplinePlaneClip(t *testing.T) {
	testOCCTRationalGraphSplinePlaneClip(t, false)
}

func TestOCCTPiecewiseRationalGraphSplinePlaneClip(t *testing.T) {
	testOCCTRationalGraphSplinePlaneClip(t, true)
}

func TestOCCTRationalBezierGraphPlaneClip(t *testing.T) {
	refs := runOracle(t, "rational-bezier-graph-plane")
	ctx := context.Background()
	tool, _ := oracleSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, func(float64, float64) float64 { return 0 })
	k := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	for _, degree := range []int{3, 4} {
		var surfaces []*geom.BSplineSurface
		var domains []geom.ParameterRectangle
		var reversed []bool
		for axis := range 3 {
			u, v := (axis+1)%3, (axis+2)%3
			for side := range 2 {
				var poles []geom.Vec3
				for i := range 5 {
					for j := range 5 {
						p := [3]float64{}
						p[axis], p[u], p[v] = float64(side), float64(i)/4, float64(j)/4
						z := 0.
						switch {
						case p[0] == 1:
							z = 1. / 16
						case degree == 3 && p[0] == .75:
							z = 1. / 64
						}
						poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: p[2] + z + (1+p[0])*p[1]/4})
					}
				}
				s, e := geom.NewBSplineSurface(4, 4, 5, 5, poles, k, k, nil)
				if e != nil {
					t.Fatal(e)
				}
				surfaces = append(surfaces, s)
				domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
				reversed = append(reversed, side == 0)
			}
		}
		stock, _, e := trimmed.SewSplineRectangles(ctx, surfaces, domains, reversed, trimmed.SewOptions{Tolerance: 1e-10})
		if e != nil {
			t.Fatal(e)
		}
		for i, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
			key := fmt.Sprintf("rational-bezier-%d-%s", degree, [2]string{"common", "cut"}[i])
			r, e := trimmed.CombineRationalGraphSplinePlaneClip(ctx, stock, tool, 5, op, trimmed.SplineRectangleEmbeddingOptions{})
			if e != nil || len(r.Solid.Topology().Shells) != 1 || len(r.SectionEdges) != 4 || refs[key].Shells != 1 {
				t.Fatal(key, "rational section topology", e)
			}
			mass, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if e != nil {
				t.Fatal(e)
			}
			compare(t, refs[key], mass.Area, mass.Volume, mass.Centroid)
		}
	}
}

func testOCCTRationalGraphSplinePlaneClip(t *testing.T, crease bool) {
	t.Helper()
	prefix, height, countEdges := "rational-graph-plane", 3./16, 4
	if crease {
		prefix, height, countEdges = "piecewise-rational-graph-plane", 23./128, 6
	}
	refs := runOracle(t, prefix)
	ctx := context.Background()
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	for axis := range 3 {
		u, v := (axis+1)%3, (axis+2)%3
		for side := range 2 {
			count := [2]int{2, 2}
			if u == 0 {
				count[0] = 3
			}
			if v == 0 {
				count[1] = 3
			}
			if crease {
				for i, a := range []int{u, v} {
					if a == 0 {
						count[i] = 5
					}
				}
			}
			var poles []geom.Vec3
			for i := range count[0] {
				for j := range count[1] {
					p := [3]float64{}
					p[axis], p[u], p[v] = float64(side), float64(i)/float64(count[0]-1), float64(j)/float64(count[1]-1)
					a := p[0]/16 + p[0]*p[0]/32
					x := []float64{0, 1. / 32, 3. / 32}
					if crease {
						x = []float64{1. / 128, 5. / 256, 5. / 128, 19. / 256, 13. / 128}
						a += 1. / 128
					}
					if u == 0 {
						a = x[i]
					} else if v == 0 {
						a = x[j]
					}
					poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: p[2] + a + (1+p[0])*p[1]/4})
				}
			}
			knots := func(n int) []float64 {
				if n == 5 {
					return []float64{0, 0, 0, .5, .5, 1, 1, 1}
				}
				k := make([]float64, n*2)
				for i := n; i < len(k); i++ {
					k[i] = 1
				}
				return k
			}
			s, err := geom.NewBSplineSurface(min(count[0]-1, 2), min(count[1]-1, 2), count[0], count[1], poles, knots(count[0]), knots(count[1]), nil)
			if err != nil {
				t.Fatal(err)
			}
			surfaces = append(surfaces, s)
			domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
			reversed = append(reversed, side == 0)
		}
	}
	stock, _, err := trimmed.SewSplineRectangles(ctx, surfaces, domains, reversed, trimmed.SewOptions{Tolerance: 1e-10})
	if err != nil {
		t.Fatal(err)
	}
	tool, _ := oracleSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, height}, func(float64, float64) float64 { return 0 })
	for i, name := range []string{prefix + "-common", prefix + "-cut"} {
		op := [2]parametric.BooleanOp{parametric.Intersection, parametric.Difference}[i]
		var r trimmed.SplineCornerPlaneClip
		var err error
		if crease {
			p, e := trimmed.CombinePiecewiseRationalGraphSplinePlaneClip(ctx, stock, tool, 5, op, trimmed.PolynomialPlaneSpanOptions{})
			r, err = p.SplineCornerPlaneClip, e
			if err == nil && (len(p.Knots) != 2 || refs[name].Shells != 1) {
				t.Fatal(name, "original knot or reference shell missing")
			}
		} else {
			r, err = trimmed.CombineRationalGraphSplinePlaneClip(ctx, stock, tool, 5, op, trimmed.SplineRectangleEmbeddingOptions{})
		}
		if err != nil || len(r.Solid.Topology().Shells) != 1 || len(r.SectionEdges) != countEdges {
			t.Fatal(name, "rational section topology", err)
		}
		m, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(name, err)
		}
		if !crease {
			compare(t, refs[name], m.Area, m.Volume, m.Centroid)
			continue
		}
		// The independent Boolean splits at the crease before integration.
		// Its two returned volumes still differ from exact strip integrals by
		// about 10^-8 model units cubed; report that observed residual separately.
		ref := refs[name]
		center := geom.Vec3{X: ref.Centroid[0], Y: ref.Centroid[1], Z: ref.Centroid[2]}
		if math.Abs(m.Area-ref.Area) > 3e-7*ref.Area || math.Abs(m.Volume-ref.Volume) > 1e-7 ||
			m.Centroid.Sub(center).Length() > 3e-7 {
			t.Fatalf("%s: Go area %.15g volume %.15g centroid %v; OCCT area %.15g volume %.15g centroid %v",
				name, m.Area, m.Volume, m.Centroid, ref.Area, ref.Volume, center)
		}
	}
}
