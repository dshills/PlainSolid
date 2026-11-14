package intersect

import (
	"context"
	"plainsolid/geom"
	"testing"
)

func boundarySiteCurvedPair(t *testing.T, bent bool) (Patch, Patch) {
	t.Helper()
	return boundarySiteCurvedPairWithTilt(t, bent, false)
}
func boundarySiteCurvedPairWithTilt(t *testing.T, bent, tilted bool) (Patch, Patch) {
	t.Helper()
	var pair [2]Patch
	for owner := 0; owner < 2; owner++ {
		lo, hi := 0., 1.
		if owner == 1 {
			lo, hi = -.5, 1.5
		}
		poles := []geom.Vec3{}
		for i := 0; i < 3; i++ {
			for j := 0; j < 2; j++ {
				x := lo + (hi-lo)*float64(i)/2
				y := float64(j)
				z := []float64{lo * lo, lo * hi, hi * hi}[i] / 8
				if owner == 1 {
					z += y - 13./64
					y = 19. / 64
					if bent && i == 1 {
						y += 1. / 64
					}
				}
				if tilted {
					y += x/2 + z/4
				}
				poles = append(poles, geom.Vec3{X: x, Y: y, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 1, 3, 2, poles, []float64{lo, lo, lo, hi, hi, hi}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		pair[owner] = Patch{Surface: s, Min: geom.Vec2{X: lo}, Max: geom.Vec2{X: hi, Y: 1}}
	}
	return pair[0], pair[1]
}

func TestAutomaticAffineChartBoundarySites(t *testing.T) {
	ctx := context.Background()
	a, b := boundarySiteCurvedPair(t, false)
	if r, e := DiscoverAffineChartPairTubes(ctx, a, b, geom.SurfacePatchOptions{}, AffineChartDiscoveryOptions{LatticeDivisions: 8}); e == nil || len(r.Attempts) != 0 {
		t.Fatal("finite lattice found nonlattice branch")
	}
	for _, reverse := range []bool{false, true} {
		x, y := a, b
		if reverse {
			x, y = y, x
		}
		seeds, e := DiscoverAffineChartPairTubes(ctx, x, y, geom.SurfacePatchOptions{}, AffineChartDiscoveryOptions{BoundaryPlaneSites: true})
		if e != nil {
			t.Fatal(reverse, e)
		}
		o := SplinePairTraceOptions{}
		o.Coverage.Isolation.ParameterTolerance = .25
		o.Coverage.Isolation.SeparatingProjections = true
		o.Coverage.Cell.MaxOperations = 16000000
		result, e := CertifySplinePairBranches(ctx, x, y, seeds, o)
		if e != nil || !result.Complete || len(result.Curves) != 1 {
			t.Fatal(reverse, result.Issues, e)
		}
	}
	outside := b
	outside.Min.Y = .25 // Exact inverse is outside the retained original target chart.
	if r, e := DiscoverAffineChartPairTubes(ctx, a, outside, geom.SurfacePatchOptions{}, AffineChartDiscoveryOptions{BoundaryPlaneSites: true}); e == nil || len(r.Attempts) != 0 {
		t.Fatal("outside target chart accepted")
	}
	a, b = boundarySiteCurvedPair(t, true)
	if r, e := DiscoverAffineChartPairTubes(ctx, a, b, geom.SurfacePatchOptions{}, AffineChartDiscoveryOptions{BoundaryPlaneSites: true}); e == nil || len(r.Attempts) != 0 {
		t.Fatal("corner plane accepted bent support")
	}
	a, b = boundarySiteCurvedPair(t, false)
	if r, e := DiscoverAffineChartPairTubes(ctx, a, b, geom.SurfacePatchOptions{}, AffineChartDiscoveryOptions{BoundaryPlaneSites: true, MaxOperations: 100}); e == nil || len(r.Attempts) != 0 {
		t.Fatal("partial budget result")
	}
}

func TestAutomaticAffineChartObliqueBoundarySites(t *testing.T) {
	ctx := context.Background()
	a, b := boundarySiteCurvedPairWithTilt(t, false, true)
	if r, e := DiscoverAffineChartPairTubes(ctx, a, b, geom.SurfacePatchOptions{}, AffineChartDiscoveryOptions{LatticeDivisions: 8}); e == nil || len(r.Attempts) != 0 {
		t.Fatal("lattice accepted off-grid roots")
	}
	for _, reverse := range []bool{false, true} {
		x, y := a, b
		if reverse {
			x, y = y, x
		}
		seeds, e := DiscoverAffineChartPairTubes(ctx, x, y, geom.SurfacePatchOptions{}, AffineChartDiscoveryOptions{BoundaryPlaneSites: true})
		if e != nil {
			t.Fatal(reverse, e)
		}
		o := SplinePairTraceOptions{}
		o.Coverage.Isolation.ParameterTolerance = .25
		o.Coverage.Isolation.SeparatingProjections = true
		o.Coverage.Cell.MaxOperations = 16000000
		result, e := CertifySplinePairBranches(ctx, x, y, seeds, o)
		if e != nil || !result.Complete || len(result.Curves) != 1 {
			t.Fatal(reverse, result.Issues, e)
		}
	}
	a, b = boundarySiteCurvedPairWithTilt(t, true, true)
	if r, e := DiscoverAffineChartPairTubes(ctx, a, b, geom.SurfacePatchOptions{}, AffineChartDiscoveryOptions{BoundaryPlaneSites: true}); e == nil || len(r.Attempts) != 0 {
		t.Fatal("three corners proved bent oblique support")
	}
}
