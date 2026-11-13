package occtcheck

import (
	"context"
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
