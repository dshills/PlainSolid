package intersect_test

import (
	"context"
	"plainsolid/geom"
	"plainsolid/internal/testgraphfixture"
	"plainsolid/intersect"
	"testing"
)

func TestTripleRadialEndpointDegree(t *testing.T) {
	ctx := context.Background()
	a, b, e := testgraphfixture.TripleRadialGraphs()
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := intersect.Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, intersect.Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}
	// An original free interval on the small circle, independent from proposals.
	da := geom.ParameterRectangle{UMin: .497, UMax: .51471875, VMin: .419, VMax: .426}
	db := geom.ParameterRectangle{UMin: .5, UMax: .51171875, VMin: .419, VMax: .426}
	xa, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	xb, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	xa, e = xa.Trim(da)
	if e != nil {
		t.Fatal(e)
	}
	xb, e = xb.Trim(db)
	if e != nil {
		t.Fatal(e)
	}
	proof, e := geom.CertifySurfacePairTube(xa, xb, geom.SurfacePairTubeOptions{FreeAxis: 2, MaxOperations: 8000000})
	if e != nil || !proof.ProvenBranch {
		t.Fatal("tube", e)
	}
	seeds := intersect.SplineSeedReport{Attempts: []intersect.SplineSeedCell{{Cell: geom.SurfaceSurfaceCell{DomainA: da, DomainB: db}, Tube: &proof}}}
	curves, e := intersect.SplineBranchCurves(ctx, pa, pb, seeds, intersect.SplineTubeBranch{Cells: []intersect.OrientedTubeCell{{Cell: 0, FreeAxis: 2, Reversed: proof.FreeTangentSign < 0}}}, intersect.TubeCurveOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := curves[0].DiscoverAlgebraicGraphEndpoint(ctx, 1, geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !r.Proven {
		t.Fatal("algebraic original endpoint", e)
	}
}
