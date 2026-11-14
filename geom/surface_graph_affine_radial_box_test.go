package geom_test

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestAffineRadialPhysicalBox(t *testing.T) {
	ctx := context.Background()
	base, _ := coverageCircleSources(t)
	proof, e := geom.CertifyAffineRadialGraphContours(ctx, base[0], base[1], [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, []*big.Rat{big.NewRat(1, 4)}, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	box := geom.ParameterRectangle{UMin: .25, UMax: .75, VMin: .125, VMax: .875}
	got, work, e := proof.PhysicalBox(ctx, 1, box, geom.SurfacePatchOptions{})
	if e != nil || got[0][0].Cmp(big.NewRat(1, 4)) != 0 || got[1][1].Cmp(big.NewRat(7, 8)) != 0 || proof.OriginalOrientation(1) != 1 {
		t.Fatal(got, work, e)
	}
	got[0][0].SetInt64(999)
	if _, _, e = proof.PhysicalBox(ctx, 1, box, geom.SurfacePatchOptions{MaxOperations: work}); e != nil {
		t.Fatal(e)
	}
	if out, _, e := proof.PhysicalBox(ctx, 1, box, geom.SurfacePatchOptions{MaxOperations: work - 1}); e == nil || out[0][0] != nil {
		t.Fatal("budget", e)
	}
	if out, _, e := proof.PhysicalBox(ctx, 1, geom.ParameterRectangle{UMin: -1, UMax: 1, VMin: 0, VMax: 1}, geom.SurfacePatchOptions{}); e == nil || out[0][0] != nil {
		t.Fatal("domain", e)
	}
	var controls [][4]*big.Rat
	for i := range 2 {
		for j := range 2 {
			controls = append(controls, [4]*big.Rat{big.NewRat(int64(1-i), 1), big.NewRat(int64(j), 1), new(big.Rat), big.NewRat(1, 1)})
		}
	}
	reflected, e := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, controls, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rp, e := geom.CertifyAffineRadialGraphContours(ctx, reflected, base[1], [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, []*big.Rat{big.NewRat(1, 4)}, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rb, _, e := rp.PhysicalBox(ctx, 0, geom.ParameterRectangle{UMin: 0, UMax: .25, VMin: 0, VMax: 1}, geom.SurfacePatchOptions{})
	if e != nil || rp.OriginalOrientation(0) != -1 || rb[0][0].Cmp(big.NewRat(3, 4)) != 0 || rb[0][1].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("signed original chart", rb, e)
	}

}
