package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestAlgebraicShellImprintOpenCutAndClosedLoopOnSameFace(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	// The stock's left wall bisects one dome and is far from the other.
	// Its lower plane receives an open semicircle and an independent
	// closed circle. The two-lobe roof is one original spline face.
	a, _ := makeOpenSplineBox(t, [3]float64{5.0 / 16, 0, 0}, [3]float64{1, 1, 1}, 0)
	b := disjointLoopRoof(t)
	d, e := faceUVBounds(b.faces[1])
	if e != nil {
		t.Fatal(e)
	}
	o := AlgebraicShellSectionOptions{}
	o.Trace.PlaneFallback.Enabled = true
	o.Trace.Coverage.OverlappingAffineXYFallback = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	var hints intersect.SplineSeedReport
	for i, center := range []float64{5.0 / 16, 11.0 / 16} {
		span := d
		if i == 0 {
			span.UMin, span.UMax = center, .5
		} else {
			span.UMin = .5
		}
		p, e := intersectSplineFace(a.faces[4], span)
		if e != nil {
			t.Fatal(e)
		}
		q, e := intersectSplineFace(b.faces[1], span)
		if e != nil {
			t.Fatal(e)
		}
		opts := o.Trace.GraphFallback
		opts.UCuts = []float64{center - 1.0/16, center - 3.0/64, center + 3.0/64, center + 1.0/16}
		if i == 0 {
			opts.UCuts = opts.UCuts[2:]
		}
		opts.VCuts = []float64{.5 - 1.0/16, .5 - 3.0/64, .5 + 3.0/64, .5 + 1.0/16}
		opts.Plane.Arcs.ResolveSubdivisionCorners = true
		r, e := intersect.DiscoverCommonXYGraphPairTubes(ctx, p, q, geom.SurfacePatchOptions{}, opts)
		if e != nil || len(r.Attempts) == 0 {
			t.Fatal("dome branch proposals", i, len(r.Attempts), e)
		}
		hints.Attempts = append(hints.Attempts, r.Attempts...)
	}
	o.Seeds = []AlgebraicShellSectionSeed{{Faces: [2]int{4, 1}, Tubes: hints}}
	// The vertical plane pairs have a different affine projection. Build
	// local proposals in their original UV coordinates; the full section
	// call below must still re-prove every supplied tube and pair coverage.
	x, e := intersectSplineFace(a.faces[0], shellSectionRect(.25, .75, 0, .125))
	if e != nil {
		t.Fatal(e)
	}
	y, e := intersectSplineFace(b.faces[1], shellSectionRect(9.0/32, 11.0/32, .25, .75))
	if e != nil {
		t.Fatal(e)
	}
	wall, e := intersect.DiscoverSplinePlanePairSections(ctx, x, y, geom.SurfacePatchOptions{}, o.Trace.PlaneFallback)
	if e != nil {
		t.Fatal("stock wall proposals", e)
	}
	o.Seeds = append(o.Seeds, AlgebraicShellSectionSeed{Faces: [2]int{0, 1}, Tubes: wall.Seeds})
	r, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !r.Complete || len(r.Wires) != 2 {
		for i, s := range r.Sections {
			t.Log(r.Pairs[i], s.Trace.Issues, len(s.Trace.Curves), len(s.Open), len(s.Trace.Coverage.Unresolved))
		}
		t.Fatal("mixed source contact is incomplete", r.Pairs, len(r.Wires), e)
	}
	bc := AlgebraicShellBooleanOptions{}
	bc.Imprint.Exact.MaxOperations = 128000000
	bodies, e := r.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection, parametric.Difference, parametric.Union}, bc)
	if e != nil {
		t.Fatal("arrange and classify both original shells", e)
	}
	common, cut, union := bodies[0], bodies[1], bodies[2]
	if common.Topology.EulerCharacteristic != 4 || cut.Topology.EulerCharacteristic != 2 || union.Topology.EulerCharacteristic != 0 {
		t.Fatal("expected two material regions, one cut stock and a union with one tunnel", common.Topology, cut.Topology, union.Topology)
	}
	var count, holes int
	for _, f := range common.parts[0].Faces {
		if f.Original == 4 {
			count++
			holes += len(f.Holes)
		}
	}
	if count != 3 || holes != 1 {
		t.Fatal("one original face must yield one hole and three fragments", count, holes)
	}
	for _, q := range []struct {
		p    geom.Vec3
		want parametric.Location
	}{
		{geom.Vec3{X: 5.0/16 + 1.0/32, Y: .5, Z: 1.0 / 1024}, parametric.Inside},
		{geom.Vec3{X: 5.0/16 - 1.0/32, Y: .5, Z: 1.0 / 1024}, parametric.Outside},
		{geom.Vec3{X: 11.0 / 16, Y: .5, Z: 1.0 / 1024}, parametric.Inside},
		{geom.Vec3{X: .5, Y: .5, Z: 1.0 / 1024}, parametric.Outside},
		{geom.Vec3{X: 11.0 / 16, Y: .5}, parametric.Boundary},
	} {
		loc, e := common.LocateMaterial(ctx, q.p, bc.Shells)
		if e != nil || loc != q.want {
			t.Fatal("original material membership", q.p, loc, q.want, e)
		}
	}
	v, e := common.BoundVolume(ctx, AlgebraicShellVolumeOptions{MaxDepth: 6, TubeSubdivisions: 4})
	if e != nil {
		t.Fatal("mixed contact volume enclosure", e)
	}
	// The two identical domes have height r²−x²−y² on their disks;
	// the stock retains half of one dome and all of the other.
	want := 3.0 / 4 * math.Pi * math.Pow(5.0/64, 4)
	lo, _ := v.Volume.Lower.Float64()
	hi, _ := v.Volume.Upper.Float64()
	if lo > want || hi < want || hi <= 0 {
		t.Fatal("volume enclosure missed the independent integral", lo, hi, want)
	}
	var recipe bytes.Buffer
	if e = WriteAlgebraicShellBooleanJSON(&recipe, common); e != nil {
		t.Fatal(e)
	}
	back, e := ReadAlgebraicShellBooleanJSON(ctx, &recipe, JSONLimits{}, o, bc)
	if e != nil || back.Topology != common.Topology {
		t.Fatal("native reload did not reprove mixed incidence", back.Topology, e)
	}
	for _, boundary := range []AlgebraicShellBoolean{common, cut, union} {
		native, e := boundary.NativeRepresentable(ctx, 1e-7, NativeAlgebraicBooleanOptions{})
		if e == nil {
			t.Fatal("circle segment uses nonrepresentable intermediate junctions; no promotion allowed", native.Topology())
		}
		if !errors.Is(e, geom.ErrUnrepresentable) || len(native.Faces()) != 0 {
			t.Fatal("expected exact representability guard", e)
		}
	}
	limited := bc.Imprint
	limited.Exact.MaxOperations = 1
	if bad, e := r.ImprintOriginalShells(ctx, limited); !errors.Is(e, geom.ErrConvergence) || len(bad[0].Faces)+len(bad[1].Faces) != 0 {
		t.Fatal("mixed imprint published a partial result at its exact-work limit", e)
	}
}

func TestAlgebraicShellImprintMatchesIndependentMixedFixture(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{23.0 / 32, 1, 1}, 0)
	b, _ := twoPeakSplineRoof(t)
	o := AlgebraicShellSectionOptions{}
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	seeds, e := mixedSplineSections(t, a, b, o.Trace)
	if e != nil {
		t.Fatal(e)
	}
	for _, q := range seeds {
		if len(q.TubeCells.Attempts) == 0 {
			continue
		}
		o.Seeds = append(o.Seeds, AlgebraicShellSectionSeed{Faces: [2]int{q.FaceA, q.FaceB}, Tubes: q.TubeCells})
	}
	r, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !r.Complete || len(r.Wires) != 2 {
		t.Fatal("mixed right-wall fixture", r.Pairs, len(r.Wires), e)
	}
	bc := AlgebraicShellBooleanOptions{}
	bc.Imprint.Exact.MaxOperations = 128000000
	results, e := r.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}, bc)
	if e != nil {
		t.Fatal(e)
	}
	const radius = 5.0 / 32
	common := .75 * math.Pi * math.Pow(radius, 4)
	roof := (7.0 / 8) * .5 * (1 + radius*radius - math.Pow(7.0/32, 2)/3 - 1.0/48)
	for i, want := range [3]float64{common, 23.0/32 + roof - common, 23.0/32 - common} {
		if results[i].Topology.EulerCharacteristic != [3]int{4, 0, 2}[i] {
			t.Fatal("independent fixture Euler characteristic", i, results[i].Topology)
		}
		v, e := results[i].BoundVolume(ctx, AlgebraicShellVolumeOptions{MaxDepth: 6, TubeSubdivisions: 4})
		if e != nil {
			t.Fatal(e)
		}
		lo, _ := v.Volume.Lower.Float64()
		hi, _ := v.Volume.Upper.Float64()
		if lo > want || hi < want {
			t.Fatal("independent fixture volume enclosure", i, lo, hi, want)
		}
	}
}
