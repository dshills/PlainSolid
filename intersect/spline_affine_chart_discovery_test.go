package intersect

import (
	"context"
	"plainsolid/geom"
	"testing"
)

func affineChartCurvedPair(t *testing.T) (Patch, Patch) {
	t.Helper()
	return affineChartCurvedPairAt(t, .5, .25)
}
func affineChartCurvedPairAt(t *testing.T, fixedY, offsetZ float64) (Patch, Patch) {
	t.Helper()
	var out [2]Patch
	for owner := 0; owner < 2; owner++ {
		lo, hi := 0.0, 1.0
		vhi := 1.0
		if owner == 1 {
			lo, hi = .5, 1.5
			vhi = .5
		}
		var poles []geom.Vec3
		for i := 0; i < 3; i++ {
			for j := 0; j < 2; j++ {
				x := lo + (hi-lo)*float64(i)/2
				y := float64(j) * vhi
				z := []float64{lo * lo, lo * hi, hi * hi}[i]
				if owner == 1 {
					z += y - offsetZ
					y = fixedY
				}
				poles = append(poles, geom.Vec3{X: x, Y: y, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 1, 3, 2, poles, []float64{lo, lo, lo, hi, hi, hi}, []float64{0, 0, vhi, vhi}, nil)
		if e != nil {
			t.Fatal(e)
		}
		out[owner] = Patch{Surface: s, Min: geom.Vec2{X: lo}, Max: geom.Vec2{X: hi, Y: vhi}}
	}
	return out[0], out[1]
}
func TestAutomaticAffineChartCurvedSection(t *testing.T) {
	ctx := context.Background()
	a, b := affineChartCurvedPair(t)
	seeds, e := DiscoverAffineChartPairTubes(ctx, a, b, geom.SurfacePatchOptions{}, AffineChartDiscoveryOptions{})
	if e != nil || len(seeds.Attempts) < 4 {
		t.Fatal(len(seeds.Attempts), e)
	}
	o := SplinePairTraceOptions{}
	o.Coverage.Isolation.ParameterTolerance = .25
	o.Coverage.Isolation.SeparatingProjections = true
	o.Coverage.Cell.MaxOperations = 16000000
	r, e := CertifySplinePairBranches(ctx, a, b, seeds, o)
	if e != nil || !r.Complete || len(r.Curves) != 1 {
		t.Fatal("complete", r.Issues, e)
	}
	omitted := seeds
	omitted.Attempts = append([]SplineSeedCell(nil), seeds.Attempts[1:]...)
	bad, e := CertifySplinePairBranches(ctx, a, b, omitted, o)
	if e == nil || bad.Complete {
		t.Fatal("omitted interval accepted")
	}
	if got, e := DiscoverAffineChartPairTubes(ctx, a, b, geom.SurfacePatchOptions{}, AffineChartDiscoveryOptions{MaxOperations: 1}); e == nil || len(got.Attempts) != 0 {
		t.Fatal("atomic work cap")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := DiscoverAffineChartPairTubes(cancelled, a, b, geom.SurfacePatchOptions{}, AffineChartDiscoveryOptions{}); e != context.Canceled || len(got.Attempts) != 0 {
		t.Fatal(e)
	}
}

func TestAffineChartLatticeCannotHideOffLatticeBranch(t *testing.T) {
	ctx := context.Background()
	var owners [2]Patch
	for owner := 0; owner < 2; owner++ {
		lo, hi := 0.0, 1.0
		if owner == 1 {
			lo, hi = .5, 1.5
		}
		var poles []geom.Vec3
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				z := 0.0
				if owner == 1 {
					z += []float64{.0625, -.25, .4375}[j]
				}
				poles = append(poles, geom.Vec3{X: lo + (hi-lo)*float64(i)/2, Y: float64(j) / 2, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, []float64{lo, lo, lo, hi, hi, hi}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		owners[owner] = Patch{Surface: s, Min: geom.Vec2{X: lo}, Max: geom.Vec2{X: hi, Y: 1}}
	}
	// Lattice detects v=1/2, but cannot see the second exact branch v=1/8.
	seeds, e := DiscoverAffineChartPairTubes(ctx, owners[0], owners[1], geom.SurfacePatchOptions{}, AffineChartDiscoveryOptions{ConstantPaddingDivisor: 32})
	if e != nil {
		t.Fatal(e)
	}
	o := SplinePairTraceOptions{}
	o.Coverage.Isolation.ParameterTolerance = .25
	o.Coverage.Isolation.SeparatingProjections = true
	o.Coverage.Cell.MaxOperations = 16000000
	r, e := CertifySplinePairBranches(ctx, owners[0], owners[1], seeds, o)
	if e == nil || r.Complete {
		t.Fatal("off-lattice branch omitted")
	}
}

func TestAutomaticAffineChartQuarterLattice(t *testing.T) {
	ctx := context.Background()
	a, b := affineChartCurvedPairAt(t, .25, .125)
	if r, e := DiscoverAffineChartPairTubes(ctx, a, b, geom.SurfacePatchOptions{}, AffineChartDiscoveryOptions{}); e == nil || len(r.Attempts) != 0 {
		t.Fatal("default lattice unexpectedly discovers off-lattice path")
	}
	options := AffineChartDiscoveryOptions{LatticeDivisions: 4}
	seeds, e := DiscoverAffineChartPairTubes(ctx, a, b, geom.SurfacePatchOptions{}, options)
	if e != nil {
		t.Fatal(e)
	}
	o := SplinePairTraceOptions{}
	o.Coverage.Isolation.ParameterTolerance = .25
	o.Coverage.Isolation.SeparatingProjections = true
	o.Coverage.Cell.MaxOperations = 16000000
	result, e := CertifySplinePairBranches(ctx, a, b, seeds, o)
	if e != nil || !result.Complete || len(result.Curves) != 1 {
		t.Fatal(result.Issues, e)
	}
	for _, bad := range []AffineChartDiscoveryOptions{{LatticeDivisions: 1}, {LatticeDivisions: 9}, {LatticeDivisions: 4, MaxOperations: 100}} {
		r, e := DiscoverAffineChartPairTubes(ctx, a, b, geom.SurfacePatchOptions{}, bad)
		if e == nil || len(r.Attempts) != 0 {
			t.Fatal("non-atomic invalid lattice/budget", bad)
		}
	}
}
