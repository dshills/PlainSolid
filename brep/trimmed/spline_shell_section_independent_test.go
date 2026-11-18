package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Partitioning is an imprint, not a Boolean: both complete original
// boundaries must survive, including all source and generated edge ancestry.
func checkIndependentSectionPartition(t *testing.T, parts [2]SplineShellPartition) {
	t.Helper()
	if len(parts[0].SectionEdges) == 0 || len(parts[0].SectionEdges) != len(parts[1].SectionEdges) {
		t.Fatal("unpaired section edges")
	}
	for _, p := range parts {
		faces := make([]bool, len(p.Solid.faces))
		edges := make([]bool, len(p.Solid.edges))
		for _, row := range p.Faces {
			if len(row) == 0 {
				t.Fatal("deleted original face")
			}
			for _, id := range row {
				if id < 0 || id >= len(faces) || faces[id] {
					t.Fatal("face ancestry", id)
				}
				faces[id] = true
			}
		}
		for _, row := range append(append([][]int{}, p.Edges...), p.SectionEdges) {
			for _, id := range row {
				if id < 0 || id >= len(edges) || edges[id] {
					t.Fatal("edge ancestry", id)
				}
				edges[id] = true
			}
		}
		for _, ok := range append(faces, edges...) {
			if !ok {
				t.Fatal("unaccounted topology")
			}
		}
		if p.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("changed shell Euler characteristic")
		}
	}
	for i, ai := range parts[0].SectionEdges {
		a, b := parts[0].Solid.edges[ai], parts[1].Solid.edges[parts[1].SectionEdges[i]]
		ca, oka := a.Curve.(surfaceTrimCurve)
		cb, okb := b.Curve.(surfaceTrimCurve)
		if !oka || !okb {
			t.Fatal("missing exact surface trims")
		}
		ta, ra, oka := trimTube(ca.curve)
		tb, rb, okb := trimTube(cb.curve)
		sa, oka2 := ca.surface.(*geom.BSplineSurface)
		sb, okb2 := cb.surface.(*geom.BSplineSurface)
		if !oka || !okb || !oka2 || !okb2 || !ta.CertifiesPairedTube(tb, sa, sb) {
			t.Fatal("unproved paired section", i)
		}
		av, bv := []geom.Vec3{parts[0].Solid.vertices[a.Start], parts[0].Solid.vertices[a.End]}, []geom.Vec3{parts[1].Solid.vertices[b.Start], parts[1].Solid.vertices[b.End]}
		if ra != rb {
			bv[0], bv[1] = bv[1], bv[0]
		}
		if av[0] != bv[0] || av[1] != bv[1] {
			t.Fatal("paired model vertices differ", i)
		}
	}
}

func TestIndependentSectionPartitionPreservesMass(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b, sections := multipleOpenSplineFixture(t)
	parts, e := SplitSectionedSplineShells(ctx, a, b, sections, SplineShellOpenOptions{})
	if e != nil {
		t.Fatal(e)
	}
	checkIndependentSectionPartition(t, parts)
	if len(parts[0].SectionEdges) != 24 {
		t.Fatal("section segment count")
	}
	for i, s := range []Solid{a, b} {
		before, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		after, e := parts[i].Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(before.Volume-after.Volume) > 1e-11 || math.Abs(before.Area-after.Area) > 1e-11 || before.Centroid.Sub(after.Centroid).Length() > 1e-11 {
			t.Fatal("imprint changed mass", before, after)
		}
	}
	bad, e := SplitSectionedSplineShells(ctx, a, b, sections[:len(sections)-1], SplineShellOpenOptions{})
	if !errors.Is(e, geom.ErrConvergence) || len(bad[0].Solid.faces) != 0 || len(bad[1].Solid.faces) != 0 {
		t.Fatal("partial result", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e = SplitSectionedSplineShells(canceled, a, b, sections, SplineShellOpenOptions{})
	if !errors.Is(e, context.Canceled) || len(bad[0].Solid.faces) != 0 || len(bad[1].Solid.faces) != 0 {
		t.Fatal("cancellation", e)
	}
}

func TestIndependentSectionPartitionMixed(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{23.0 / 32, 1, 1}, 0)
	b, _ := twoPeakSplineRoof(t)
	o := SplineShellOpenOptions{}
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	sections, e := mixedSplineSections(t, a, b, o.Trace)
	if e != nil {
		t.Fatal(e)
	}
	report, e := SectionSplineShells(ctx, a, b, SplineShellCombineOptions{Sections: sections, Open: o})
	if e != nil || !report.Complete || len(report.Wires) != 2 {
		t.Fatal("mixed section graph", e)
	}
	parts, e := SplitSectionedSplineShells(ctx, a, b, sections, o)
	if e != nil {
		t.Fatal(e)
	}
	checkIndependentSectionPartition(t, parts)
	n := 0
	for _, w := range report.Wires {
		n += len(w.Pieces)
	}
	if n != len(parts[0].SectionEdges) {
		t.Fatal("wire/partition segment disagreement")
	}
	if testing.Short() {
		return
	}
	// This bounded mesh comparison independently checks material preservation;
	// it is not a certified quadrature or exact-volume claim for the curved roof.
	const r = 5.0 / 32
	wants := []float64{23.0 / 32, (7.0 / 8) * (1.0 / 2) * (1 + r*r - math.Pow(7.0/32, 2)/3 - 1.0/48)}
	for i, p := range parts {
		m, e := p.Solid.refine(ctx, 2, 0, 0)
		if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-wants[i]) > .004 {
			t.Fatal("mixed imprint volume", i, m.SignedVolume(), wants[i], e)
		}
	}
}
