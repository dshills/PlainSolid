package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/intersect"
	"testing"
)

func TestNativeLensAffineRadialCoverage(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	_, b, hints := transverseSplineCapsDomain(t, true)
	var supports []*geom.BSplineSurface
	var reversed []bool
	lo, hi := [3]float64{-.125, -.125, 0}, [3]float64{1.125, 1.125, 1}
	for axis := 0; axis < 3; axis++ {
		u, v := (axis+1)%3, (axis+2)%3
		for side := 0; side < 2; side++ {
			var poles []geom.Vec3
			for i := 0; i < 2; i++ {
				for j := 0; j < 2; j++ {
					var xyz [3]float64
					xyz[axis] = []float64{lo[axis], hi[axis]}[side]
					xyz[u] = []float64{lo[u], hi[u]}[i]
					xyz[v] = []float64{lo[v], hi[v]}[j]
					poles = append(poles, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
				}
			}
			s, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
			if e != nil {
				t.Fatal(e)
			}
			supports = append(supports, s)
			reversed = append(reversed, side == 0)
		}
	}
	a, _, e := SewSplineRectangles(ctx, supports, make([]geom.ParameterRectangle, len(supports)), reversed, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	pa, e := algebraicShellTracePatch(a.faces[4], geom.ParameterRectangle{UMax: 1, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	pb, e := algebraicShellTracePatch(b.faces[1], geom.ParameterRectangle{UMax: 1, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	// The original topology lane is independently generated in the same physical
	// square. Re-map only its A proposal boxes; later original uniform proofs are
	// mandatory. The exact source stock map is X=5U/4-1/8,Y=5V/4-1/8.
	exactA, e := geom.NewSurfacePatch(supports[4], geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	exactB, e := geom.NewSurfacePatch(b.faces[1].Surface.(*geom.BSplineSurface), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := range hints.Attempts {
		originalFree := hints.Attempts[i].Cell.DomainA
		d := hints.Attempts[i].Cell.DomainB
		axis := hints.Attempts[i].Tube.FreeAxis + 2
		mapBound := func(v float64, lower bool) float64 {
			q := new(big.Rat).Mul(new(big.Rat).Add(new(big.Rat).SetFloat64(v), big.NewRat(1, 8)), big.NewRat(4, 5))
			f, _ := q.Float64()
			cmp := new(big.Rat).SetFloat64(f).Cmp(q)
			if lower && cmp > 0 {
				f = math.Nextafter(f, math.Inf(-1))
			}
			if !lower && cmp < 0 {
				f = math.Nextafter(f, math.Inf(1))
			}
			return f
		}
		hints.Attempts[i].Cell.DomainA = geom.ParameterRectangle{UMin: mapBound(d.UMin, true), UMax: mapBound(d.UMax, false), VMin: mapBound(d.VMin, true), VMax: mapBound(d.VMax, false)}
		hints.Attempts[i].Cell.DomainB = originalFree
		xa, e := exactA.Trim(hints.Attempts[i].Cell.DomainA)
		if e != nil {
			t.Fatal(e)
		}
		xb, e := exactB.Trim(originalFree)
		if e != nil {
			t.Fatal(e)
		}
		cert, e := geom.CertifySurfacePairTube(xa, xb, geom.SurfacePairTubeOptions{FreeAxis: axis})
		if e != nil || !cert.ProvenBranch {
			t.Fatal("original topology proposal", i, e)
		}
		hints.Attempts[i].Tube = &cert
	}
	options := AlgebraicShellSectionOptions{Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{4, 1}, Tubes: hints}}}
	options.Trace.Coverage.RadialOverlap = &intersect.RadialOverlapCoverageOptions{AffineCharts: true, Center: [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, Radii: []*big.Rat{big.NewRat(5, 16)}}
	options.Trace.Coverage.Cell.MaxOperations = 16000000
	report, e := intersect.CertifySplineTubeCoverage(ctx, pa, pb, hints, options.Trace.Coverage)
	if e != nil {
		t.Fatal("original affine cover", e)
	}
	t.Log("affine cover operations", report.RadialOverlap.ExactOperations)
	for _, limit := range []int{report.RadialOverlap.ExactOperations, report.RadialOverlap.ExactOperations - 1} {
		o := options.Trace.Coverage
		q := *o.RadialOverlap
		o.RadialOverlap = &q
		q.Proof.MaxOperations = limit
		got, e := intersect.CertifySplineTubeCoverage(ctx, pa, pb, hints, o)
		if limit == report.RadialOverlap.ExactOperations {
			if e != nil || !got.CoversAllContacts {
				t.Fatal("exact budget", e)
			}
		} else if e == nil || got.CoversAllContacts || got.RadialOverlap != nil {
			t.Fatal("atomic affine quota", e)
		}
	}
	missing := hints
	missing.Attempts = missing.Attempts[:len(missing.Attempts)/2]
	if got, e := intersect.CertifySplineTubeCoverage(ctx, pa, pb, missing, options.Trace.Coverage); e == nil || got.CoversAllContacts || got.RadialOverlap != nil {
		t.Fatal("omitted original affine branch", e)
	}
	strict := options.Trace.Coverage
	q := *strict.RadialOverlap
	strict.RadialOverlap = &q
	q.AffineCharts = false
	if got, e := intersect.CertifySplineTubeCoverage(ctx, pa, pb, hints, strict); e == nil || got.CoversAllContacts {
		t.Fatal("identity-only path silently widened", e)
	}
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, options)
	if e != nil {
		t.Fatal("section", e)
	}
	result, e := section.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal("material", e)
	}
	native, e := result.NativeTubeRegionComponents(ctx, 1e-7, edgecheck.Options{})
	if e != nil || len(native) != 1 {
		t.Fatal("native", len(native), e)
	}
	if native[0].Topology().EulerCharacteristic != 2 || len(native[0].Faces()) != 2 {
		t.Fatal(native[0].Topology())
	}
	for _, f := range native[0].Faces() {
		if f.Surface != a.faces[4].Surface && f.Surface != b.faces[1].Surface {
			t.Fatal("original support lost")
		}
	}
}
