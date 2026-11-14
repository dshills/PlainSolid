package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/testgraphfixture"
	"plainsolid/intersect"
	"testing"
)

func tripleRadialRoofSolid(t *testing.T) Solid {
	t.Helper()
	height, e := testgraphfixture.TripleRadialHeightControls()
	if e != nil {
		t.Fatal(e)
	}
	knots := func(n int) []float64 {
		k := make([]float64, 2*n)
		for i := n; i < len(k); i++ {
			k[i] = 1
		}
		return k
	}
	var supports []*geom.BSplineSurface
	var reversed []bool
	add := func(nu, nv int, rev bool, at func(int, int) geom.Vec3) {
		var poles []geom.Vec3
		for i := range nu {
			for j := range nv {
				poles = append(poles, at(i, j))
			}
		}
		s, e := geom.NewBSplineSurface(nu-1, nv-1, nu, nv, poles, knots(nu), knots(nv), nil)
		if e != nil {
			t.Fatal(e)
		}
		supports = append(supports, s)
		reversed = append(reversed, rev)
	}
	add(2, 2, true, func(i, j int) geom.Vec3 { return geom.Vec3{X: float64(i), Y: float64(j), Z: -4} })
	add(9, 9, false, func(i, j int) geom.Vec3 { return geom.Vec3{X: float64(i) / 8, Y: float64(j) / 8, Z: height[i][j]} })
	for _, edge := range []int{0, 8} {
		add(9, 2, edge == 0, func(i, j int) geom.Vec3 {
			return geom.Vec3{X: float64(edge) / 8, Y: float64(i) / 8, Z: [2]float64{-4, height[edge][i]}[j]}
		})
	}
	for _, edge := range []int{0, 8} {
		add(9, 2, edge != 0, func(i, j int) geom.Vec3 {
			return geom.Vec3{X: float64(i) / 8, Y: float64(edge) / 8, Z: [2]float64{-4, height[i][edge]}[j]}
		})
	}
	s, _, e := SewSplineRectangles(context.Background(), supports, make([]geom.ParameterRectangle, len(supports)), reversed, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func TestAlgebraicShellTripleRadialMaterialComponents(t *testing.T) {
	if testing.Short() {
		t.Skip("complete original sextic shell section and material")
	}
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{-.125, -.125, 0}, [3]float64{1.125, 1.125, 1}, 0)
	b := tripleRadialRoofSolid(t)
	x, e := algebraicShellTracePatch(a.faces[4], geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	y, e := algebraicShellTracePatch(b.faces[1], geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	seeds, e := intersect.DiscoverRadialGraphContourTubes(ctx, x, y, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, testgraphfixture.TripleRadialRadii(), geom.SurfacePatchOptions{}, intersect.SplinePlanePairDiscoveryOptions{})
	if e != nil {
		t.Fatal(e)
	}
	t.Log("original proposal tubes", len(seeds.Attempts))
	o := AlgebraicShellSectionOptions{Trace: nestedTubeSectionOptions().Trace, Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{4, 1}, Tubes: seeds}}}
	o.Trace.Coverage.OverlappingAffineXYFallback = true
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !section.Complete || len(section.Wires) != 3 {
		t.Fatal("complete original three-loop section", len(section.Wires), e)
	}
	t.Log("complete original section")
	options := AlgebraicShellBooleanOptions{}
	options.Imprint.Exact.MaxOperations = 512000000
	common, e := section.CombineAlgebraic(ctx, parametric.Intersection, options)
	if e != nil {
		t.Fatal("disk and annulus material", e)
	}
	if len(common.faces) != 4 || common.Topology.InnerWires != 2 || common.Topology.EulerCharacteristic != 2 {
		t.Fatal("global mixed region topology", common.Topology)
	}
	t.Log("complete original material", common.Topology)
	for branch, b := range common.source.branches {
		t.Log("original branch pieces", branch, len(b.Segments))
		if len(b.Segments) <= 128 {
			continue
		}
		var uses [2][]AlgebraicShellBooleanUse
		for _, face := range common.faces {
			for _, loop := range append([][]AlgebraicShellBooleanUse{face.Outer}, face.Holes...) {
				edge := common.index[loop[0].Edge]
				if common.parts[0].Section[edge.SectionPiece].Branch == branch {
					uses[face.Operand] = loop
				}
			}
		}
		if _, e := common.nativeWholeTubeRing(ctx, branch, uses); e == nil {
			t.Fatal("legacy 128-piece gate changed")
		}
	}
	native, e := common.NativeTubeRegionComponents(ctx, 1e-7, edgecheck.Options{})
	if e != nil || len(native) != 2 {
		t.Fatal("native disconnected disk and annulus", len(native), e)
	}
	t.Log("native component assembly complete")
	euler, holes, edges := 0, 0, 0
	for _, solid := range native {
		edges += len(solid.Edges())
		euler += solid.Topology().EulerCharacteristic
		holes += solid.Topology().InnerWires
		if len(solid.Faces()) != 2 {
			t.Fatal("original two-support component")
		}
		for _, f := range solid.Faces() {
			if f.Surface != a.faces[4].Surface && f.Surface != b.faces[1].Surface {
				t.Fatal("lost original support")
			}

		}
	}
	if euler != 2 || holes != 2 || edges != len(seeds.Attempts) {
		t.Fatal("native component topology", euler, holes)
	}
	// The component adapter must not publish the valid disk before rejecting
	// a later unmatched annular wire.
	bad := common
	bad.faces = append([]AlgebraicShellBooleanFace(nil), common.faces...)
	removed := false
	for i := range bad.faces {
		if len(bad.faces[i].Holes) > 0 {
			bad.faces[i].Holes = nil
			removed = true
			break
		}
	}
	if !removed {
		t.Fatal("missing annulus fixture")
	}
	if partial, e := bad.NativeTubeRegionComponents(ctx, 1e-7, edgecheck.Options{}); e == nil || partial != nil {
		t.Fatal("unmatched hole published components", len(partial), e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if partial, e := common.NativeTubeRegionComponents(cancelled, 1e-7, edgecheck.Options{}); e != context.Canceled || partial != nil {
		t.Fatal("cancelled native publication", e)
	}
	volume, e := common.BoundRadialTubeRegionVolume(ctx, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, testgraphfixture.TripleRadialRadii(), geom.RadialGraphVolumeOptions{PrecisionBits: 80})
	if e != nil {
		t.Fatal("original radial material bound", e)
	}
	wantCoefficient := big.NewRat(63333935546875, 864691128455135232)
	if volume.Volume.PiCoefficient.Cmp(wantCoefficient) != 0 || len(volume.Components) != 2 {
		t.Fatal("original radial coefficient/components", volume)
	}
	if len(volume.Volume.Bands) != 2 || volume.Volume.Bands[0].Band != 0 || volume.Volume.Bands[1].Band != 2 || volume.Volume.Bands[0].PiCoefficient.Cmp(big.NewRat(613935546875, 864691128455135232)) != 0 || volume.Volume.Bands[1].PiCoefficient.Cmp(big.NewRat(478515625, 6597069766656)) != 0 {
		t.Fatal("disk/annulus source coefficients", volume.Volume.Bands)
	}
	width := new(big.Rat).Sub(volume.Volume.Volume.Upper, volume.Volume.Volume.Lower)
	if volume.Volume.Volume.Lower.Sign() <= 0 || width.Cmp(new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))) > 0 {
		t.Fatal("nonpositive or loose enclosure", width)
	}
	lo, _ := volume.Volume.Volume.Lower.Float64()
	hi, _ := volume.Volume.Volume.Upper.Float64()
	t.Log("two native components, tight original-source volume enclosure", lo, hi, "exact coefficient", wantCoefficient, "width", width, "work", volume.Volume.ExactOperations)

}

// Integrate the independent radial polynomial with dA=pi*dq. The material
// consists of q in [0,s1] and [s2,s3], not the union of three disk interiors.
func tripleRadialAnalyticVolume() float64 {
	r := testgraphfixture.TripleRadialRadii()
	var q [3]float64
	for i, x := range r {
		v, _ := x.Float64()
		q[i] = v * v
	}
	sum := q[0] + q[1] + q[2]
	pairs := q[0]*q[1] + q[0]*q[2] + q[1]*q[2]
	product := q[0] * q[1] * q[2]
	f := func(x float64) float64 { return -(1225.0 / 256) * (x*x*x*x/4 - sum*x*x*x/3 + pairs*x*x/2 - product*x) }
	return math.Pi * (f(q[0]) - f(0) + f(q[2]) - f(q[1]))
}
