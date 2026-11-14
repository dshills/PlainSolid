package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestBubbleRoofAlgebraicMaterial(t *testing.T) {
	ctx := context.Background()
	a, _ := crossingFootprintRoofShellShape(t, false, true)
	b, _ := crossingFootprintRoofShellShape(t, true, true)
	o := AlgebraicShellSectionOptions{}
	o.Trace.UniformOverlapGraphFallback = intersect.UniformGraphDiscoveryOptions{GridDivisions: 16, AxisDivisions: [2]int{8, 16}}
	o.Trace.UniformOverlapGraphFallback.Enabled = true
	o.Trace.UniformOverlapGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.UniformOverlapGraphFallback.MaxArcs = 1024
	o.Trace.UniformOverlapGraphFallback.MaxTubes = 1024
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	o.Trace.Branches.PruneSeparatedEndpoints = true
	o.Trace.Branches.MaxComparisons = 4096
	o.Trace.MaxTopologyPairs = 1000000
	o.Trace.Coverage.MaxCandidateTests = 4096
	o.Trace.Coverage.IsoPlaneFallback = true
	o.Trace.PlaneFallback.Enabled = true
	o.Trace.PlaneFallback.Plane.Arcs.ResolveSubdivisionCorners = true

	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !section.Complete {
		t.Fatal(e)
	}
	bc := AlgebraicShellBooleanOptions{}
	bc.Imprint.Exact.MaxOperations = 128000000
	results, e := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}, bc)
	if e != nil || len(results) != 3 {
		t.Fatal("combine", e)
	}
	// Integrate f=X(1-X)Y(1-Y)/64 over the unit overlap, then integrate
	// the negative part of (X-1/2)^2-[1/16-Y(1-Y)/64]. The latter is
	// -(4/3) integral_0^1 [1/16-Y(1-Y)/64]^(3/2) dY.
	common := .5 + 1.0/2304 - 77.0/6144 - (225.0/8192)*math.Asinh(1/math.Sqrt(15))
	av := 1.25 + 115.0/294912
	bv := .625 + 5.0/192 + 115.0/294912 + 115.0/49152
	for i, want := range []float64{common, av + bv - common, av - common} {
		result := results[i]
		if result.Topology.EulerCharacteristic != []int{2, 2, 4}[i] {
			t.Fatal("material topology", i, result.Topology)
		}
		roof, e := result.PrepareRoofMaterialQuery(ctx, MixedRoofQueryOptions{})
		if e != nil {
			t.Fatal("prepare original quartic roof line", i, e)
		}
		// The original stock Z on X=Y=t is t²(1-t)²/64. The
		// intersection hides it where the tool's roof is lower.
		r := big.NewRat
		quartic := []*big.Rat{r(-1, 4096), r(0, 1), r(1, 64), r(-1, 32), r(1, 64)}
		tool := []*big.Rat{r(767, 4096), r(-63, 64), r(1, 1), r(-1, 32), r(1, 64)}
		line, e := roof.IntersectLineRational(ctx,
			[3]*big.Rat{r(0, 1), r(0, 1), r(1, 4096)}, [3]*big.Rat{r(1, 1), r(1, 1), r(0, 1)},
			r(1, 16), r(15, 16), MixedRoofLineOptions{})
		if e != nil || len(line.Events) != [3]int{4, 0, 2}[i] || len(line.Spans) != [3]int{2, 1, 1}[i] {
			t.Fatal("quartic material section", i, line, e)
		}
		for j, event := range line.Events {
			// The inner roots of common and difference come from the tool;
			// the two outer roots of common come from the stock.
			p, owner := tool, 1
			if i == 0 && (j == 0 || j == len(line.Events)-1) {
				p, owner = quartic, 0
			}
			sign, e := event.Parameter.SignPolynomial(ctx, p, geom.ExactCurvePlaneEventOptions{})
			if e != nil || sign.Sign != 0 || len(event.Owners) != 1 || event.Owners[0] != (MixedRoofFaceOwner{owner, 1}) {
				t.Fatal("quartic witness lost original polynomial identity", i, j, sign, event, e)
			}
		}
		volume, e := result.BoundVolume(ctx, AlgebraicShellVolumeOptions{MaxDepth: 6, TubeSubdivisions: 8})
		if e != nil {
			t.Fatal("volume", i, e)
		}
		lo, _ := volume.Volume.Lower.Float64()
		hi, _ := volume.Volume.Upper.Float64()
		t.Log("volume", i, lo, hi, want)
		if lo > want || hi < want || lo <= 0 || hi-lo > 1.1 {
			t.Fatal("analytic volume outside exact bounds", i, lo, hi, want)
		}
		for face := range result.Faces {
			original, e := result.OriginalFace(face)
			owner := result.Faces[face]
			if e != nil || original.Surface != [2]Solid{a, b}[owner.Operand].faces[owner.Original].Surface {
				t.Fatal("lost original support", face, e)
			}
		}
	}
	for _, row := range []struct {
		p      geom.Vec3
		inside [3]bool
	}{
		{geom.Vec3{X: .5, Y: .5, Z: -.25}, [3]bool{true, true, false}},
		{geom.Vec3{X: .5, Y: .5, Z: -.75}, [3]bool{false, true, true}},
		{geom.Vec3{X: .125, Y: .5, Z: .05}, [3]bool{false, true, false}},
		{geom.Vec3{X: .5, Y: .5, Z: -.025}, [3]bool{false, true, true}},
	} {
		for i, result := range results {
			location, e := result.LocateMaterial(ctx, row.p, SplineShellBooleanOptions{})
			want := parametric.Outside
			if row.inside[i] {
				want = parametric.Inside
			}
			if e != nil || location != want {
				t.Fatal("material witness", i, row.p, location, e)
			}
		}
	}
	var data bytes.Buffer
	if e := WriteAlgebraicShellBooleanJSON(&data, results[0]); e != nil {
		t.Fatal("write", e)
	}
	reload, e := ReadAlgebraicShellBooleanJSON(ctx, bytes.NewReader(data.Bytes()), JSONLimits{}, o, bc)
	if e != nil || reload.Topology != results[0].Topology {
		t.Fatal("reload", e)
	}
	native, e := results[0].NativeRepresentable(ctx, 1e-8, NativeAlgebraicBooleanOptions{})
	if !errors.Is(e, geom.ErrUnrepresentable) || len(native.faces) != 0 {
		t.Fatal("rounded native endpoints", e)
	}
	limited, e := ReadAlgebraicShellBooleanJSON(ctx, bytes.NewReader(data.Bytes()), JSONLimits{MaxBytes: 1}, o, bc)
	if !errors.Is(e, geom.ErrConvergence) || len(limited.Faces) != 0 {
		t.Fatal("atomic persistence limit", e)
	}
	small := bc
	small.Imprint.Seam.Composition.MaxOperations = 1
	partial, e := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection, parametric.Union}, small)
	if e == nil || partial != nil {
		t.Fatal("atomic construction budget", e)
	}
}
