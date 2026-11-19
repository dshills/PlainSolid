//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/intersect"
	"testing"
)

func TestNativeNonunitRadialAnnulus(t *testing.T) {
	slowtest.Skip(t)
	if testing.Short() {
		t.Skip("complete non-unit two-circle native annulus")
	}
	ctx := context.Background()
	a, b, pa, pb := nonunitRadialShells(t)
	roof, e := geom.NewSurfacePatch(pb, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	jet, e := roof.EvaluateRational(ctx, big.NewRat(79, 128), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
	if e != nil || jet.Point[2].Cmp(big.NewRat(65625, 16777216)) != 0 {
		t.Fatal("independent physical annulus height", jet.Point, e)
	}
	for _, u := range []*big.Rat{big.NewRat(1, 2), big.NewRat(11, 16)} {
		jet, e := roof.EvaluateRational(ctx, u, big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
		if e != nil || jet.Point[2].Sign() >= 0 {
			t.Fatal("hole/exterior height", jet.Point, e)
		}
	}
	seeds := nonunitRadialTopologySeeds(t, pa, pb)
	t.Log("original ordered tubes", len(seeds.Attempts))
	o := AlgebraicShellSectionOptions{Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{0, 1}, Tubes: seeds}}}
	o.Trace.Coverage.RadialOverlap = &intersect.RadialOverlapCoverageOptions{AffineCharts: true, Center: [2]*big.Rat{big.NewRat(3, 1), big.NewRat(1, 2)}, Radii: []*big.Rat{big.NewRat(5, 32), big.NewRat(5, 16)}, Proof: geom.RadialTubeCoverageOptions{MaxOperations: 128000000, MaxTubes: 512}}
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	o.Trace.Branches.PruneSeparatedEndpoints = true
	o.Trace.Branches.MaxComparisons = 4096
	o.Trace.MaxTopologyPairs = 1000000
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil {
		t.Fatal("section", e)
	}
	result, e := section.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal("material", e)
	}
	if len(result.faces) != 2 || result.Topology.InnerWires != 2 || result.Topology.EulerCharacteristic != 0 {
		t.Fatal(result.Topology)
	}
	native, e := result.NativeTubeRegionComponents(ctx, 1e-7, edgecheck.Options{})
	if e != nil || len(native) != 1 {
		t.Fatal("native", len(native), e)
	}
	n := native[0]
	if n.Topology().EulerCharacteristic != 0 || n.Topology().InnerWires != 2 || len(n.Edges()) != len(seeds.Attempts) {
		t.Fatal(n.Topology(), len(n.Edges()))
	}
	for _, f := range n.Faces() {
		if f.Surface != a.faces[0].Surface && f.Surface != b.faces[1].Surface {
			t.Fatal("original support owner")
		}
	}
	counts := make([]int, len(n.Edges()))
	directions := make([]int, len(n.Edges()))
	for _, f := range n.Faces() {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				counts[u.Edge]++
				sign := 1
				if u.Reversed != f.Reversed {
					sign = -1
				}
				directions[u.Edge] += sign
			}
		}
	}
	for i := range counts {
		if counts[i] != 2 || directions[i] != 0 {
			t.Fatal("original wire incidence", i, counts[i], directions[i])
		}
	}
	bad := result
	bad.faces = append([]AlgebraicShellBooleanFace(nil), result.faces...)
	bad.faces[0].Holes = nil
	if got, e := bad.NativeTubeRegionComponents(ctx, 1e-7, edgecheck.Options{}); e == nil || got != nil {
		t.Fatal("missing original inner wire", len(got), e)
	}
	t.Log("native annulus: Euler", n.Topology().EulerCharacteristic, "inner wires", n.Topology().InnerWires, "original edges", len(n.Edges()))
}
