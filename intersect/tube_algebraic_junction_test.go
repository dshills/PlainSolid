package intersect

import (
	"context"
	"plainsolid/geom"
	"testing"
)

func TestAutonomousAlgebraicMixedAxisJunction(t *testing.T) {
	ctx := context.Background()
	knots := []float64{0, 0, 0, 1, 1, 1}
	poles := make([]geom.Vec3, 9)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := 0.
			if i == 2 {
				z++
			}
			if j == 2 {
				z++
			}
			poles[3*i+j] = geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z}
		}
	}
	source, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	k := []float64{0, 0, 1, 1}
	plane, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}, {X: 1, Y: 1, Z: .5}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa, e := geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pb, e := geom.NewSurfacePatch(plane, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	domains := [][2]geom.ParameterRectangle{{{UMin: 7.0 / 32, UMax: .25, VMin: 5.0 / 8, VMax: 11.0 / 16}, {UMin: 3.0 / 16, UMax: 9.0 / 32, VMin: 19.0 / 32, VMax: 23.0 / 32}}, {{UMin: 7.0 / 32, UMax: 10.0 / 32, VMin: 19.0 / 32, VMax: 23.0 / 32}, {UMin: .25, UMax: 9.0 / 32, VMin: 5.0 / 8, VMax: 11.0 / 16}}}
	var proofs [2]geom.VerifiedSurfaceTube
	var seeds SplineSeedReport
	for i, d := range domains {
		a, e := pa.Trim(d[0])
		if e != nil {
			t.Fatal(e)
		}
		b, e := pb.Trim(d[1])
		if e != nil {
			t.Fatal(e)
		}
		options := geom.SurfacePairTubeOptions{FreeAxis: 2 * i}
		cert, e := geom.CertifySurfacePairTube(a, b, options)
		if e != nil || !cert.ProvenBranch {
			t.Fatal(cert, e)
		}
		proofs[i], e = geom.NewVerifiedSurfaceTube(a, b, options)
		if e != nil {
			t.Fatal(e)
		}
		seeds.Attempts = append(seeds.Attempts, SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: d[0], DomainB: d[1]}, Tube: &cert})
	}
	// Exercise the fallback directly: no exact or approximate rational hint is
	// supplied. Both independently discovered endpoints have v=sqrt(7/16).
	connected, ops, e := algebraicGraphTubeJunction(ctx, proofs[0], proofs[1], true, false, 8000000, 16384)
	if e != nil || !connected || ops == 0 {
		t.Fatal(connected, ops, e)
	}
	different, _, e := algebraicGraphTubeJunction(ctx, proofs[0], proofs[1], true, true, 8000000, 16384)
	if e != nil || different {
		t.Fatal("different endpoints joined", different, e)
	}

	patchA, patchB := Patch{Surface: source, Min: geom.Vec2{}, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: plane, Min: geom.Vec2{}, Max: geom.Vec2{X: 1, Y: 1}}
	joined, e := JoinSplineTubeBranches(ctx, patchA, patchB, seeds, TubeBranchOptions{})
	if e != nil || joined.Truncated || len(joined.Branches) != 1 || len(joined.Branches[0].Cells) != 2 || joined.Branches[0].StartUnresolved || joined.Branches[0].EndUnresolved {
		t.Fatal(joined, e)
	}
	curves, e := SplineBranchCurves(ctx, patchA, patchB, seeds, joined.Branches[0], TubeCurveOptions{})
	if e != nil || len(curves) != 2 {
		t.Fatal(curves, e)
	}
	if _, e = RebuildTubeCurves(ctx, curves[0].recipe.Data()); e != nil {
		t.Fatal(e)
	}
}
