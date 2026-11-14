package geom

import (
	"context"
	"math/big"
	"testing"
)

func projectedTwoParabolas(t *testing.T) (*SurfacePatch, *SurfacePatch) {
	t.Helper()
	var net [2][][4]*big.Rat
	for i := range 5 {
		for j := range 3 {
			u, v := big.NewRat(int64(i), 4), big.NewRat(int64(j), 2)
			u2 := big.NewRat(int64(i*(i-1)), 12)
			u4 := new(big.Rat)
			if i == 4 {
				u4.SetInt64(1)
			}
			vv := new(big.Rat)
			if j == 2 {
				vv.SetInt64(1)
			}
			f := new(big.Rat).Sub(vv, v)
			f.Sub(f, new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(1, 4), u2), v))
			f.Add(f, big.NewRat(3, 16))
			f.Add(f, new(big.Rat).Mul(big.NewRat(1, 8), u2))
			f.Add(f, new(big.Rat).Mul(big.NewRat(1, 64), u4))
			base := new(big.Rat).Mul(u, v)
			base.Quo(base, big.NewRat(64, 1))
			net[0] = append(net[0], [4]*big.Rat{u, v, base, big.NewRat(1, 1)})
			net[1] = append(net[1], [4]*big.Rat{u, v, new(big.Rat).Add(base, f), big.NewRat(1, 1)})
		}
	}
	a, e := NewExactHomogeneousBezierSurface(context.Background(), 4, 2, net[0], SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewExactHomogeneousBezierSurface(context.Background(), 4, 2, net[1], SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return a, b
}

func TestProjectedGraphUnionTwoCurvedBranches(t *testing.T) {
	ctx := context.Background()
	a, b := projectedTwoParabolas(t)
	candidate := SurfacePairParameterBox{ParameterRectangle{.125, .875, 0, 1}, ParameterRectangle{0, 1, 0, 1}}
	var targets []SurfacePairTubeRegion
	for _, base := range []float64{.25, .75} {
		for i := range 6 {
			lo := .125 + float64(i)/8
			hi := lo + .125
			v0 := base + (lo-.03125)*(lo-.03125)/8 - .015625
			v1 := base + (hi+.03125)*(hi+.03125)/8 + .015625
			targets = append(targets, SurfacePairTubeRegion{FreeAxis: 0, Box: SurfacePairParameterBox{ParameterRectangle{lo, hi, v0, v1}, ParameterRectangle{lo - .03125, hi + .03125, v0, v1}}})
		}
	}

	out, e := CertifyCommonXYGraphCellInTubes(ctx, a, b, candidate, targets, SurfaceTubeUnionCoverageOptions{MaxOperations: 16000000})
	if e != nil || !out.Covered {
		t.Fatal(out.ExactOperations, out.PartitionPieces, out.VisitedCells, e)
	}
	t.Log("projected", out.ExactOperations, "strata", out.PartitionPieces, "tested", out.VisitedCells)
	for _, partial := range [][]SurfacePairTubeRegion{targets[:6], targets[6:]} {
		missing, e := CertifyCommonXYGraphCellInTubes(ctx, a, b, candidate, partial, SurfaceTubeUnionCoverageOptions{MaxOperations: 16000000})
		if e == nil || missing.Covered || len(missing.Unresolved) == 0 {
			t.Fatal("missed parabola", e)
		}
	}

	if _, e = CertifyCommonXYGraphCellInTubes(ctx, a, b, candidate, targets, SurfaceTubeUnionCoverageOptions{MaxOperations: out.ExactOperations}); e != nil {
		t.Fatal("exact quota", e)
	}
	for _, limit := range []SurfaceTubeUnionCoverageOptions{{MaxOperations: out.ExactOperations - 1}, {MaxPieces: out.PartitionPieces - 1}, {MaxCells: 1}} {
		r, e := CertifyCommonXYGraphCellInTubes(ctx, a, b, candidate, targets, limit)
		if e == nil || r.Covered || len(r.Pending) == 0 {
			t.Fatal("atomic limit", limit, e)
		}
	}
	generic, e := CertifySurfacePairCellInTubesContext(ctx, a, b, candidate, targets, SurfaceTubeUnionCoverageOptions{MaxOperations: 16000000, MaxCells: 128, MaxDepth: 4})
	t.Log("generic", generic.Covered, generic.ExactOperations, generic.VisitedCells, e)
	if e != nil || !generic.Covered || out.ExactOperations >= generic.ExactOperations {
		t.Fatal("independent generic comparison", generic.ExactOperations, e)
	}
	limited, e := CertifySurfacePairCellInTubesContext(ctx, a, b, candidate, targets, SurfaceTubeUnionCoverageOptions{MaxOperations: out.ExactOperations, MaxCells: 128, MaxDepth: 4})
	if e == nil || limited.Covered {
		t.Fatal("generic lower-budget comparison", e)
	}
	forged := []SurfacePairTubeRegion{{Box: candidate, FreeAxis: 0}}
	if r, e := CertifyCommonXYGraphCellInTubes(ctx, a, b, candidate, forged, SurfaceTubeUnionCoverageOptions{}); e == nil || r.Covered {
		t.Fatal("unverified target", e)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	r, e := CertifyCommonXYGraphCellInTubes(cancelled, a, b, candidate, targets, SurfaceTubeUnionCoverageOptions{})
	if e != context.Canceled || r.Covered {
		t.Fatal(e)
	}
}

func TestProjectedStrataRetainBoundaryContacts(t *testing.T) {
	ctx := context.Background()
	b := &polynomialBudget{maxOperations: 1000000, maxBits: 4096}
	// f=v has zero on an edge: weak Bernstein signs prove only the open
	// rectangle. The separately evaluated edge must never be excluded.
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	c := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, v} })
	for _, tt := range []struct {
		r    projectedExactRectangle
		want bool
	}{{projectedExactRectangle{{big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(1, 1)}}, true}, {projectedExactRectangle{{big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(0, 1)}}, false}, {projectedExactRectangle{{big.NewRat(0, 1), big.NewRat(0, 1)}, {big.NewRat(0, 1), big.NewRat(0, 1)}}, false}} {
		got, e := commonGraphStratumNonzero(ctx, a, c, tt.r, b)
		if e != nil || got != tt.want {
			t.Fatal(got, e)
		}
	}
	r, e := CertifyCommonXYGraphCellInTubes(ctx, a, c, SurfacePairParameterBox{a.Domain(), c.Domain()}, nil, SurfaceTubeUnionCoverageOptions{})
	if e == nil || r.Covered {
		t.Fatal("lost boundary contact", e)
	}
}

func TestProjectedGraphRationalWeightsAndProjectionGate(t *testing.T) {
	ctx := context.Background()
	a, c := projectedTwoParabolas(t)
	multiply := func(p *SurfacePatch) *SurfacePatch {
		n := p.degreeU
		controls := make([][4]*big.Rat, (n+2)*p.countV)
		for i := 0; i < n+2; i++ {
			for j := 0; j < p.countV; j++ {
				for k := range 4 {
					q := new(big.Rat)
					if i <= n {
						q.Mul(big.NewRat(int64(n+1-i), int64(n+1)), p.net[i*p.countV+j][k])
					}
					if i > 0 {
						q.Add(q, new(big.Rat).Mul(big.NewRat(int64(2*i), int64(n+1)), p.net[(i-1)*p.countV+j][k]))
					}
					controls[i*p.countV+j][k] = q
				}
			}
		}
		out, e := NewExactHomogeneousBezierSurface(ctx, n+1, p.degreeV, controls, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	a, c = multiply(a), multiply(c)
	box := SurfacePairParameterBox{ParameterRectangle{0, 1, 0, .125}, ParameterRectangle{0, 1, 0, .125}}
	r, e := CertifyCommonXYGraphCellInTubes(ctx, a, c, box, nil, SurfaceTubeUnionCoverageOptions{})
	if e != nil || !r.Covered {
		t.Fatal("positive rational numerator", e)
	}
	controls := make([][4]*big.Rat, len(a.net))
	for i, h := range a.net {
		for k, q := range h {
			controls[i][k] = new(big.Rat).Set(q)
		}
		controls[i][0].Mul(controls[i][0], big.NewRat(2, 1))
	}
	warped, e := NewExactHomogeneousBezierSurface(ctx, a.degreeU, a.degreeV, controls, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r, e := CertifyCommonXYGraphCellInTubes(ctx, warped, c, box, nil, SurfaceTubeUnionCoverageOptions{}); e == nil || r.Covered {
		t.Fatal("unproved common projection", e)
	}
}
