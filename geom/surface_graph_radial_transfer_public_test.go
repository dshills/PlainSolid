package geom_test

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestRadialTransferOriginalCertificate(t *testing.T) {
	ctx := context.Background()
	p, _ := coverageCircleSources(t)
	proof, e := geom.CertifyAffineRadialGraphContours(ctx, p[0], p[1], [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, []*big.Rat{big.NewRat(1, 4)}, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	tiny := math.Ldexp(1, -40)
	for _, d := range []geom.ParameterRectangle{{UMin: .75, UMax: .875, VMin: .375, VMax: .625}, {UMin: .75, UMax: .875, VMin: .5, VMax: .625}, {UMin: .5 - tiny, UMax: .5 + tiny, VMin: .75 - tiny, VMax: .75 + tiny}} {
		got, e := proof.CertifyCellInTubes(ctx, geom.SurfacePairParameterBox{A: d, B: d}, nil, geom.SurfaceTubeUnionCoverageOptions{})
		if !errors.Is(e, geom.ErrConvergence) || got.Covered || len(got.Unresolved) != 1 {
			t.Fatal("uncovered exact contact", d, e, got)
		}
	}
	a := geom.ParameterRectangle{UMin: .4375, UMax: .5625, VMin: .6875, VMax: .8125}
	b := geom.ParameterRectangle{UMin: .46875, UMax: .53125, VMin: .6875, VMax: .8125}
	candidate := geom.SurfacePairParameterBox{A: a, B: b}
	targets := []geom.SurfacePairTubeRegion{{Box: candidate, FreeAxis: 2}}
	got, e := proof.CertifyCellInTubes(ctx, candidate, targets, geom.SurfaceTubeUnionCoverageOptions{})
	if e != nil || !got.Covered || len(got.Tubes) != 1 || !got.Tubes[0].ProvenBranch {
		t.Fatal("fresh original uniform target", got, e)
	}
	if _, e := proof.CertifyCellInTubes(ctx, candidate, targets, geom.SurfaceTubeUnionCoverageOptions{MaxOperations: got.ExactOperations}); e != nil {
		t.Fatal("exact work", e)
	}
	if bad, e := proof.CertifyCellInTubes(ctx, candidate, targets, geom.SurfaceTubeUnionCoverageOptions{MaxOperations: got.ExactOperations - 1}); e == nil || bad.Covered {
		t.Fatal("atomic work", bad, e)
	}
	full := geom.ParameterRectangle{UMax: 1, VMax: 1}
	targets[0].Box = geom.SurfacePairParameterBox{A: full, B: full}
	if bad, e := proof.CertifyCellInTubes(ctx, candidate, targets, geom.SurfaceTubeUnionCoverageOptions{}); e == nil || bad.Covered {
		t.Fatal("unproved broad target", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if bad, e := proof.CertifyCellInTubes(cancelled, candidate, nil, geom.SurfaceTubeUnionCoverageOptions{}); !errors.Is(e, context.Canceled) || bad.Covered {
		t.Fatal("cancel", e)
	}
}
