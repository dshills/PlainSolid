//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestAlgebraicShellImprintNestedLoops(t *testing.T) {
	slowtest.Skip(t)
	if testing.Short() {
		t.Skip("independent complete quartic shell section")
	}
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{-.125, -.125, 0}, [3]float64{1.125, 1.125, 1}, 0)
	b := nestedQuarticRoofSolid(t)
	x, e := algebraicShellTracePatch(a.faces[4], geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	y, e := algebraicShellTracePatch(b.faces[1], geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	h, e := intersect.DiscoverRadialGraphPairTubes(ctx, x, y, geom.SurfacePatchOptions{}, intersect.SplinePlanePairDiscoveryOptions{MaxTubes: 128})
	if e != nil {
		t.Fatal("nested loop proposals", e)
	}
	o := AlgebraicShellSectionOptions{Trace: nestedTubeSectionOptions().Trace, Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{4, 1}, Tubes: h}}}
	o.Trace.Coverage.OverlappingAffineXYFallback = true
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !section.Complete || len(section.Wires) != 2 {
		t.Fatal("complete nested original shell section", section.Complete, len(section.Wires), e)
	}
	measure := AlgebraicShellBooleanOptions{}
	measure.Imprint.Exact.MaxOperations = 512000000
	var common AlgebraicShellBoolean
	t.Run("imprint and native annulus", func(t *testing.T) {
		common = checkNestedShellOperations(t, ctx, section, measure)
	})
	t.Run("volume and native reload", func(t *testing.T) {
		if common.source == nil {
			var e error
			common, e = section.CombineAlgebraic(ctx, parametric.Intersection, measure)
			if e != nil {
				t.Fatal("nested common", e)
			}
		}
		dr := 25.0/256 - 25.0/1024
		want := math.Pi * (9.0 / 16) * dr * dr * dr / 6
		v, e := common.BoundVolume(ctx, AlgebraicShellVolumeOptions{MaxDepth: 7})
		if e != nil {
			t.Fatal("nested common volume", e)
		}
		lo, _ := v.Volume.Lower.Float64()
		hi, _ := v.Volume.Upper.Float64()
		if lo > want || hi < want || hi <= 0 {
			t.Fatal("common volume excludes analytic annulus", v.Volume, want)
		}
		var native bytes.Buffer
		if e := WriteAlgebraicShellBooleanJSON(&native, common); e != nil {
			t.Fatal(e)
		}
		back, e := ReadAlgebraicShellBooleanJSON(ctx, &native, JSONLimits{}, o, measure)
		if e != nil || back.Topology != common.Topology || len(back.faces) != 2 {
			t.Fatal("native reload did not reprove the nested common", back.Topology, e)
		}
	})
}

func checkNestedShellOperations(t *testing.T, ctx context.Context, section AlgebraicShellSectionResult,
	measure AlgebraicShellBooleanOptions) AlgebraicShellBoolean {
	t.Helper()
	results, e := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection, parametric.Difference}, measure)
	if e != nil {
		t.Fatal("nested arrangement and assembly", e)
	}
	common, cut := results[0], results[1]
	for i, s := range common.parts {
		if s.Topology.EulerCharacteristic != 2 || len(s.Faces) != 8 || s.Topology.InnerWires != 2 {
			t.Fatal("imprint must preserve original topological sphere", i, s.Topology)
		}
		count := [3]int{}
		for _, f := range s.Faces {
			if f.Original == [2]int{4, 1}[i] {
				count[f.Fragment] = len(f.Holes)
			}
		}
		if count[0] != 1 || count[1]+count[2] != 1 {
			t.Fatal("expected original exterior, an annulus and an inner disk", count)
		}
	}
	if len(common.faces) != 2 || common.Topology.InnerWires != 2 || common.Topology.EulerCharacteristic != 0 {
		t.Fatal("common retains one annular material region", common.Topology)
	}
	if cut.Topology.EulerCharacteristic != 2 || len(cut.faces) != 8 || cut.Topology.InnerWires != 2 {
		t.Fatal("stock difference follows both nested rims", cut.Topology)
	}
	if partial, e := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection, parametric.Intersection}, measure); !errors.Is(e, geom.ErrInvalid) || partial != nil {
		t.Fatal("duplicate operation returned partial results", e)
	}
	exported, e := common.NativeTubeAnnulus(ctx, 1e-7, edgecheck.Options{})
	if e != nil || len(exported.Faces()) != 2 || exported.Topology().EulerCharacteristic != 0 || exported.Topology().InnerWires != 2 {
		t.Fatal("native annular boundary", exported.Topology(), e)
	}
	for i, f := range exported.Faces() {
		source, e := common.OriginalFace(i)
		if e != nil || f.Surface != source.Surface || len(f.Holes) != 1 {
			t.Fatal("native output must retain original face charts", i, e)
		}
	}
	if bad, e := cut.NativeTubeAnnulus(ctx, 1e-7, edgecheck.Options{}); !errors.Is(e, geom.ErrConvergence) || len(bad.Faces()) != 0 {
		t.Fatal("rejected out-of-scope retained source edges", e)
	}
	return common
}
