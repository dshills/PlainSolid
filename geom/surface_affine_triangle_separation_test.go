package geom

import (
	"context"
	"math/big"
	"testing"
)

func affineTriangleSeparationFixture(t *testing.T) (*SurfacePatch, *SurfacePatch, [2][3][2]*big.Rat, SurfaceAffineTriangleSeparator) {
	a, b := affineTriangleSupport(t, false, 0), affineTriangleSupport(t, true, 0)
	pt := func(u, v int64) [2]*big.Rat { return [2]*big.Rat{big.NewRat(u, 2), big.NewRat(v, 2)} }
	triangles := [2][3][2]*big.Rat{{pt(0, 0), pt(1, 0), pt(0, 1)}, {pt(2, 2), pt(1, 2), pt(2, 1)}}
	q := SurfaceAffineTriangleSeparator{Normal: [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1)}, UV: [2][2]*big.Rat{{big.NewRat(2, 1), big.NewRat(2, 1)}, {big.NewRat(2, 1), big.NewRat(2, 1)}}, Offset: [2]*big.Rat{new(big.Rat), new(big.Rat)}, Level: big.NewRat(2, 1)}
	return a, b, triangles, q
}
func TestSurfaceAffineTriangleSeparation(t *testing.T) {
	ctx := context.Background()
	a, b, tri, q := affineTriangleSeparationFixture(t)
	p, e := CertifySurfaceAffineTriangleSeparation(ctx, a, b, tri, q, SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if p.Sides() != ([2]int{-1, 1}) || p.Margins()[0].Cmp(big.NewRat(1, 1)) != 0 || p.Margins()[1].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("strict original scalar margins")
	}
	q.Level.SetInt64(99)
	if p.Separator().Level.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("aliased input")
	}
	copy := p.Margins()
	copy[0].SetInt64(5)
	if p.Margins()[0].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("aliased output")
	}
	for _, o := range []SurfaceAffineCoordinateOptions{{MaxCells: 1}, {SurfacePatchOptions: SurfacePatchOptions{MaxOperations: p.ExactOperations() - 1}}} {
		if got, e := CertifySurfaceAffineTriangleSeparation(ctx, a, b, tri, p.Separator(), o); e == nil || got != nil {
			t.Fatal("atomic bounds")
		}
	}
	q = p.Separator()
	q.Level.SetInt64(1)
	if got, e := CertifySurfaceAffineTriangleSeparation(ctx, a, b, tri, q, SurfaceAffineCoordinateOptions{}); e == nil || got != nil {
		t.Fatal("touching triangle")
	}
	q = p.Separator()
	q.Normal[0].SetInt64(2)
	if got, e := CertifySurfaceAffineTriangleSeparation(ctx, a, b, tri, q, SurfaceAffineCoordinateOptions{}); e == nil || got != nil {
		t.Fatal("wrong identity")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := CertifySurfaceAffineTriangleSeparation(c, a, b, tri, p.Separator(), SurfaceAffineCoordinateOptions{}); e == nil || got != nil {
		t.Fatal("cancel")
	}
}
