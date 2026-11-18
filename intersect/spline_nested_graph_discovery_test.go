package intersect_test

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"

	. "plainsolid/intersect"
	"testing"
)

func TestNestedQuarticAutomaticRadialCover(t *testing.T) {
	slowtest.Skip(t)
	nestedRadialCover(t, [2]float64{.5, .5})
}
func TestShiftedQuarticAutomaticRadialCover(t *testing.T) {
	slowtest.Skip(t)
	nestedRadialCover(t, [2]float64{7.0 / 16, 9.0 / 16})
}
func TestAnisotropicQuarticAutomaticCover(t *testing.T) {
	slowtest.Skip(t)
	nestedRadialCover(t, [2]float64{.5, .5}, 4)
}
func nestedRadialCover(t *testing.T, center [2]float64, metricValues ...float64) {
	metricValue := 1.
	if len(metricValues) > 0 {
		metricValue = metricValues[0]
	}
	metric := new(big.Rat).SetFloat64(metricValue)
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	var factors [2]map[[2]int]*big.Rat
	for i, radius := range []float64{5.0 / 32, 5.0 / 16} {
		x, y, r := new(big.Rat).SetFloat64(center[0]), new(big.Rat).SetFloat64(center[1]), new(big.Rat).SetFloat64(radius)
		c0 := new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(metric, new(big.Rat).Mul(y, y))), new(big.Rat).Mul(r, r))
		factors[i] = map[[2]int]*big.Rat{{0, 0}: c0, {1, 0}: new(big.Rat).Mul(big.NewRat(-2, 1), x), {0, 1}: new(big.Rat).Mul(metric, new(big.Rat).Mul(big.NewRat(-2, 1), y)), {2, 0}: big.NewRat(1, 1), {0, 2}: metric}
	}
	power := map[[2]int]*big.Rat{}
	for ij, x := range factors[0] {
		for kl, y := range factors[1] {
			key := [2]int{ij[0] + kl[0], ij[1] + kl[1]}
			if power[key] == nil {
				power[key] = new(big.Rat)
			}
			power[key].Add(power[key], new(big.Rat).Mul(x, y))
		}
	}
	var poles []geom.Vec3
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			z := new(big.Rat)
			for ij, q := range power {
				if ij[0] > i || ij[1] > j {
					continue
				}
				fu := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(ij[0])), new(big.Int).Binomial(4, int64(ij[0])))
				fv := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(j), int64(ij[1])), new(big.Int).Binomial(4, int64(ij[1])))
				z.Add(z, new(big.Rat).Mul(q, new(big.Rat).Mul(fu, fv)))
			}
			z.Mul(z, big.NewRat(9, 4))
			value, e := geom.RepresentedExactScalar(z)
			if e != nil {
				t.Fatal(e)
			}
			poles = append(poles, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: value})
		}
	}
	knots := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	b, e := geom.NewBSplineSurface(4, 4, 5, 5, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}
	cuts := []float64{.25, .3125, .375, .40625, .5, .59375, .625, .6875, .75}
	o := SplinePairTraceOptions{}
	o.GraphFallback.Enabled = true
	o.GraphFallback.UCuts, o.GraphFallback.VCuts = cuts, cuts
	o.GraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.GraphFallback.MaxArcs = 512
	o.GraphFallback.MaxTubes = 128
	o.Branches.PruneSeparatedEndpoints = true
	o.Coverage.Isolation.ParameterTolerance = .125
	o.Coverage.Cell.MaxOperations = 16000000
	patch, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	for _, x := range cuts {
		patch, _ = patch.InsertUKnot(x, 1, geom.SurfacePatchOptions{})
		patch, _ = patch.InsertVKnot(x, 1, geom.SurfacePatchOptions{})
	}
	plane, _ := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	dbg, de := geom.TraceSurfacePlaneContext(ctx, patch, plane, o.GraphFallback.Plane)
	if metricValue == 1 && center == [2]float64{.5, .5} && (de != nil || !dbg.Complete || len(dbg.Contours) != 2 || len(dbg.Isolation.Unresolved) != 0) {
		t.Fatal("exact nested trace", de)
	}

	exactA, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	exactB, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	radial, e := geom.DiscoverSplineGraphConcentricEllipses(ctx, exactA, exactB, geom.GraphPointContactOptions{})
	if e != nil || !radial.Proven || radial.Radius[0].RatString() != "5/32" || radial.Radius[1].RatString() != "5/16" {
		t.Fatal("factor proof", radial, e)
	}
	if metricValue != 1 {
		if _, e := geom.DiscoverSplineGraphRadialPair(ctx, exactA, exactB, geom.GraphPointContactOptions{}); e == nil {
			t.Fatal("radial API accepted ellipse")
		}
		if radial.MetricY.Cmp(metric) != 0 || radial.RadiusY[0].RatString() != "5/64" || radial.RadiusY[1].RatString() != "5/32" {
			t.Fatal("anisotropic factor", radial)
		}
	}
	changed := b.Data()
	changed.Poles[0].Z += 1.0 / 1048576
	bad, _ := geom.NewBSplineSurface(changed.DegreeU, changed.DegreeV, changed.CountU, changed.CountV, changed.Poles, changed.KnotsU, changed.KnotsV, changed.Weights)
	badPatch, _ := geom.NewSurfacePatch(bad, geom.SurfacePatchOptions{})
	if _, e := geom.DiscoverSplineGraphConcentricEllipses(ctx, exactA, badPatch, geom.GraphPointContactOptions{}); e == nil {
		t.Fatal("accepted nonradial perturbation")
	}
	discover := DiscoverRadialGraphPairTubes
	if metricValue != 1 {
		discover = DiscoverConcentricEllipseGraphPairTubes
		o.GraphFallback.MaxTubes = 256
	}
	seeds, e := discover(ctx, pa, pb, geom.SurfacePatchOptions{}, o.GraphFallback)
	if e != nil {
		t.Fatal(e)
	}
	if metricValue == 1 && len(seeds.Attempts) >= 128 {
		t.Fatal("compact radial tube budget", len(seeds.Attempts))
	}
	t.Log("proved seed count", len(seeds.Attempts))
	o.Branches.MaxComparisons = 4096
	o.MaxTopologyPairs = 1000000
	o.Coverage.MaxCandidateTests = 4096
	o.Coverage.Isolation.ParameterTolerance = .125
	result, e := CertifySplinePairBranches(ctx, pa, pb, seeds, o)
	if e != nil || !result.Complete || len(result.Curves) != 2 {
		t.Fatal("nested", len(seeds.Attempts), result.Issues, len(result.BranchGraph.Branches), e)
	}
}
