package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
	"time"
)

func TestNativeLensIndependentRadialCoverage(t *testing.T) {
	ctx := context.Background()
	a, b, hints := transverseSplineCapsDomain(t, true)
	options := AlgebraicShellSectionOptions{Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{0, 1}, Tubes: hints}}}
	options.Trace.Coverage.RadialOverlap = &intersect.RadialOverlapCoverageOptions{Center: [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, Radii: []*big.Rat{big.NewRat(5, 16)}}
	options.Trace.Coverage.Cell.MaxOperations = 16000000
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, options)
	if e != nil {
		t.Fatal(e)
	}
	result, e := section.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	native, e := result.NativeTubeRegionComponents(ctx, 1e-7, edgecheck.Options{})
	if e != nil || len(native) != 1 {
		t.Fatal(len(native), e)
	}
	if native[0].Topology().EulerCharacteristic != 2 || len(native[0].Faces()) != 2 {
		t.Fatal(native[0].Topology())
	}
	for _, f := range native[0].Faces() {
		if f.Surface != a.faces[0].Surface && f.Surface != b.faces[1].Surface {
			t.Fatal("original support lost")
		}
	}
	pa := intersect.Patch{Surface: a.faces[0].Surface, Max: geom.Vec2{X: 1, Y: 1}}
	pb := intersect.Patch{Surface: b.faces[1].Surface, Max: geom.Vec2{X: 1, Y: 1}}
	proposals, e := intersect.DiscoverOverlappingRadialGraphContourTubes(ctx, pa, pb, options.Trace.Coverage.RadialOverlap.Center, options.Trace.Coverage.RadialOverlap.Radii, geom.SurfacePatchOptions{}, geom.RadialTubeCoverageOptions{})
	if e != nil || len(proposals.Attempts) != 36 {
		t.Fatal("independent proposal lane", len(proposals.Attempts), e)
	}
	if partial, e := intersect.DiscoverOverlappingRadialGraphContourTubes(ctx, pa, pb, options.Trace.Coverage.RadialOverlap.Center, options.Trace.Coverage.RadialOverlap.Radii, geom.SurfacePatchOptions{}, geom.RadialTubeCoverageOptions{MaxTubes: 1}); e == nil || len(partial.Attempts) != 0 {
		t.Fatal("proposal atomic quota", e)
	}
	start := time.Now()
	report, e := intersect.CertifySplineTubeCoverage(ctx, pa, pb, hints, options.Trace.Coverage)
	if e != nil {
		t.Fatal(e)
	}
	t.Log("radial coverage", report.RadialOverlap.ExactOperations, "overlapping cells", report.RadialOverlap.Transferred, "elapsed", time.Since(start))
	generic := options.Trace.Coverage
	generic.RadialOverlap = nil
	generic.Isolation.ParameterTolerance = .25
	generic.Isolation.SeparatingProjections = true
	start = time.Now()
	fallback, e := intersect.CertifySplineTubeCoverage(ctx, pa, pb, hints, generic)
	if e != nil || !fallback.CoversAllContacts || fallback.RadialOverlap != nil {
		t.Fatal("generic unchanged", e)
	}
	t.Log("generic candidates", len(fallback.Isolation.Candidates), "elapsed", time.Since(start))
	for _, limit := range []int{1, report.RadialOverlap.ExactOperations - 1} {
		small := options.Trace.Coverage
		q := *small.RadialOverlap
		small.RadialOverlap = &q
		q.Proof.MaxOperations = limit
		got, e := intersect.CertifySplineTubeCoverage(ctx, pa, pb, hints, small)
		if e == nil || got.CoversAllContacts || got.RadialOverlap != nil {
			t.Fatal("atomic budget", limit, e)
		}
	}
	exact := options.Trace.Coverage
	q := *exact.RadialOverlap
	exact.RadialOverlap = &q
	q.Proof.MaxOperations = report.RadialOverlap.ExactOperations
	if _, e := intersect.CertifySplineTubeCoverage(ctx, pa, pb, hints, exact); e != nil {
		t.Fatal("exact operation budget", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := intersect.CertifySplineTubeCoverage(cancelled, pa, pb, hints, options.Trace.Coverage); !errors.Is(e, context.Canceled) || got.CoversAllContacts {
		t.Fatal(e)
	}
	missing := hints
	missing.Attempts = missing.Attempts[:len(missing.Attempts)/2]
	report, e = intersect.CertifySplineTubeCoverage(ctx, pa, pb, missing, options.Trace.Coverage)
	if e == nil || report.CoversAllContacts || report.RadialOverlap != nil {
		t.Fatal("omitted supplied branch accepted", e)
	}
}
