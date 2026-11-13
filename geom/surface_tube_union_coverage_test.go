package geom

import (
	"context"
	"errors"
	"testing"
)

func TestSurfaceTubeUnionCoverage(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .5, t - .5} })
	tubes := []SurfacePairTubeRegion{{Box: SurfacePairParameterBox{ParameterRectangle{.2, .55, .4, .6}, ParameterRectangle{.1, .7, .4, .6}}}, {Box: SurfacePairParameterBox{ParameterRectangle{.45, .8, .4, .6}, ParameterRectangle{.3, .9, .4, .6}}}}
	candidate := SurfacePairParameterBox{ParameterRectangle{.25, .75, 0, 1}, ParameterRectangle{0, 1, 0, 1}}
	r, e := CertifySurfacePairCellInTubes(a, b, candidate, tubes, SurfaceTubeUnionCoverageOptions{})
	if e != nil || !r.Covered || len(r.Tubes) != 2 || len(r.OutsideExcluded) == 0 {
		t.Fatal(r, e)
	}
	r, e = CertifySurfacePairCellInTubes(a, b, candidate, tubes, SurfaceTubeUnionCoverageOptions{MaxPieces: 1})
	if !errors.Is(e, ErrConvergence) || r.Covered || len(r.Pending) == 0 {
		t.Fatal(r, e)
	}
	// A real gap between tubes must remain unresolved.
	tubes[0].Box.A.UMax = .4
	tubes[1].Box.A.UMin = .6
	r, e = CertifySurfacePairCellInTubes(a, b, candidate, tubes, SurfaceTubeUnionCoverageOptions{MaxDepth: 3})
	if !errors.Is(e, ErrConvergence) || r.Covered || len(r.Unresolved) == 0 {
		t.Fatal(r, e)
	}
}

func TestSurfaceTubeUnionEmptyMeansExclusion(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, t, 1} })
	candidate := SurfacePairParameterBox{a.Domain(), b.Domain()}
	r, e := CertifySurfacePairCellInTubes(a, b, candidate, nil, SurfaceTubeUnionCoverageOptions{})
	if e != nil || !r.Covered || len(r.OutsideExcluded) != 1 {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = CertifySurfacePairCellInTubesContext(ctx, a, b, candidate, nil, SurfaceTubeUnionCoverageOptions{})
	if !errors.Is(e, context.Canceled) || r.Covered || len(r.Pending) != 1 || r.Pending[0] != candidate {
		t.Fatal(r, e)
	}
}
