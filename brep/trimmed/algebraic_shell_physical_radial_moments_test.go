package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestNativePhysicalRadialAnnulusMoments(t *testing.T) {
	if testing.Short() {
		t.Skip("complete non-unit annular material construction")
	}
	ctx := context.Background()
	a, b, pa, pb := nonunitRadialShells(t)
	seeds := nonunitRadialTopologySeeds(t, pa, pb)
	center := [2]*big.Rat{big.NewRat(3, 1), big.NewRat(1, 2)}
	radii := []*big.Rat{big.NewRat(5, 32), big.NewRat(5, 16)}
	o := AlgebraicShellSectionOptions{Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{0, 1}, Tubes: seeds}}}
	o.Trace.Coverage.RadialOverlap = &intersect.RadialOverlapCoverageOptions{AffineCharts: true, Center: center, Radii: radii, Proof: geom.RadialTubeCoverageOptions{MaxOperations: 128000000, MaxTubes: 512}}
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	o.Trace.Branches.PruneSeparatedEndpoints = true
	o.Trace.Branches.MaxComparisons = 4096
	o.Trace.MaxTopologyPairs = 1000000
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil {
		t.Fatal(e)
	}
	s, e := section.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	t.Log("complete original annular material recovered")
	out, e := s.PhysicalRadialTubeRegionMassMoments(ctx, center, radii, geom.RadialGraphMomentOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(out.Components) != 1 || out.Components[0].Band != 1 || out.Components[0].InnerBranch < 0 || out.Components[0].OriginalFaces != [2]int{0, 1} {
		t.Fatal("native band ownership", out.Components)
	}
	eq := func(a, b *big.Rat) {
		t.Helper()
		if a == nil || a.Cmp(b) != 0 {
			t.Fatal(a, b)
		}
	}
	eq(out.Moments.Planar.Total.VolumePiCoefficient, big.NewRat(421875, 2147483648))
	eq(out.Moments.Planar.Total.ProjectedAreaPiCoefficient, big.NewRat(75, 1024))
	eq(out.Moments.Planar.Total.CentroidXY[0], center[0])
	eq(out.Moments.Planar.Total.CentroidXY[1], center[1])
	eq(out.Moments.Height.CentroidZ, big.NewRat(3375, 2097152))
	work := out.Moments.ExactOperations
	if _, e = s.PhysicalRadialTubeRegionMassMoments(ctx, center, radii, geom.RadialGraphMomentOptions{MaxOperations: work}); e != nil {
		t.Fatal("exact quota", e)
	}
	if bad, e := s.PhysicalRadialTubeRegionMassMoments(ctx, center, radii, geom.RadialGraphMomentOptions{MaxOperations: work - 1}); e == nil || len(bad.Components) != 0 || bad.Moments.Height.CentroidZ != nil {
		t.Fatal("atomic quota", e)
	}
	// Public diagnostics and returned band labels cannot change actual selection.
	s.Faces = nil
	s.Edges = nil
	s.Vertices = nil
	out.Components[0].Band = 0
	out.Moments.Height.CentroidZ.SetInt64(999)
	again, e := s.PhysicalRadialTubeRegionMassMoments(ctx, center, radii, geom.RadialGraphMomentOptions{})
	if e != nil || again.Components[0].Band != 1 {
		t.Fatal(e)
	}
	eq(again.Moments.Height.CentroidZ, big.NewRat(3375, 2097152))
	rejects := func(bad AlgebraicShellBoolean) {
		t.Helper()
		x, e := bad.PhysicalRadialTubeRegionMassMoments(ctx, center, radii, geom.RadialGraphMomentOptions{})
		if e == nil || len(x.Components) != 0 || x.Moments.Height.CentroidZ != nil {
			t.Fatal("forged native source", e)
		}
	}
	bad := s
	bad.faces = append([]AlgebraicShellBooleanFace(nil), s.faces...)
	bad.faces[0].Holes = nil
	rejects(bad)
	bad = s
	bad.faces = append([]AlgebraicShellBooleanFace(nil), s.faces...)
	bad.faces[0].Reversed = !bad.faces[0].Reversed
	rejects(bad)
	bad = s
	source := *s.source
	source.a.faces = append([]Face(nil), s.source.a.faces...)
	source.a.faces[0].Surface = s.source.b.faces[1].Surface
	bad.source = &source
	rejects(bad)
	// Equivalent decoded original supports must bind by stored source identity,
	// not pointer equality. Their original UV maps are unchanged.
	equivalent := s
	copySource := *s.source
	copySource.a.faces = append([]Face(nil), s.source.a.faces...)
	copySource.b.faces = append([]Face(nil), s.source.b.faces...)
	for _, f := range []*Face{&copySource.a.faces[0], &copySource.b.faces[1]} {
		raw, err := parametric.EncodeSurface(f.Surface)
		if err != nil {
			t.Fatal(err)
		}
		restored, err := parametric.DecodeSurface(raw, 4096)
		if err != nil {
			t.Fatal(err)
		}
		if restored == f.Surface {
			t.Fatal("not an independent support")
		}
		f.Surface = restored
	}
	equivalent.source = &copySource
	rebound, err := equivalent.PhysicalRadialTubeRegionMassMoments(ctx, center, radii, geom.RadialGraphMomentOptions{})
	if err != nil {
		t.Fatal("equivalent original supports", err)
	}
	eq(rebound.Moments.Planar.Total.VolumePiCoefficient, big.NewRat(421875, 2147483648))
	for _, rr := range [][]*big.Rat{{big.NewRat(5, 32)}, {big.NewRat(1, 8), big.NewRat(5, 16)}} {
		x, e := s.PhysicalRadialTubeRegionMassMoments(ctx, center, rr, geom.RadialGraphMomentOptions{})
		if e == nil || len(x.Components) != 0 {
			t.Fatal("wrong complete factors", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if x, e := s.PhysicalRadialTubeRegionMassMoments(cancelled, center, radii, geom.RadialGraphMomentOptions{}); e != context.Canceled || len(x.Components) != 0 {
		t.Fatal(e)
	}
	t.Log("native physical band", again.Components[0].Band, "charged operations", work)
}
