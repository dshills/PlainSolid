package geom

import (
	"context"
	"math/big"
	"testing"
)

func affineTriangleFanSupport(t *testing.T, owner int, shift int64) *SurfacePatch {
	t.Helper()
	var controls [][4]*big.Rat
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			u, v := big.NewRat(int64(i), 2), big.NewRat(int64(j), 2)
			z := big.NewRat(shift, 64)
			if owner == 0 {
				if i == 2 {
					z.Add(z, big.NewRat(1, 1))
				}
				if j == 2 {
					z.Add(z, big.NewRat(1, 1))
				}
				z.Sub(z, new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(u, v)))
			}
			if owner == 2 {
				if i == 2 {
					z.Add(z, big.NewRat(1, 1))
				}
				z.Sub(z, u)
				z.Add(z, big.NewRat(1, 4))
			}
			controls = append(controls, [4]*big.Rat{u, v, z, big.NewRat(1, 1)})
		}
	}
	p, e := NewExactHomogeneousBezierSurface(context.Background(), 2, 2, controls, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot([]float64{.25, .5, .75}[owner], 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestSurfaceAffineTriangleVertexChain(t *testing.T) {
	ctx := context.Background()
	a, b, c := affineTriangleFanSupport(t, 0, 0), affineTriangleFanSupport(t, 1, 0), affineTriangleFanSupport(t, 2, 0)
	pt := func(u, v int64) [2]*big.Rat { return [2]*big.Rat{big.NewRat(u, 2), big.NewRat(v, 2)} }
	ta := [3][2]*big.Rat{pt(1, 1), pt(0, 0), pt(0, 1)}
	tb := [3][2]*big.Rat{pt(1, 1), pt(0, 0), pt(1, 0)}
	tc := [3][2]*big.Rat{pt(1, 1), pt(1, 0), pt(2, 0)}
	q := SurfaceAffineTriangleSeparator{Normal: [3]*big.Rat{big.NewRat(1, 1), big.NewRat(-1, 2), new(big.Rat)}, UV: [2][2]*big.Rat{{big.NewRat(1, 1), big.NewRat(-1, 2)}, {big.NewRat(1, 1), big.NewRat(-1, 2)}}, Offset: [2]*big.Rat{new(big.Rat), new(big.Rat)}, Level: big.NewRat(1, 4)}
	ab, e := CertifySurfaceAffineTriangleMate(ctx, a, b, [2][3][2]*big.Rat{ta, tb}, [3]*big.Rat{big.NewRat(1, 1), big.NewRat(-1, 1), new(big.Rat)}, [2][2]*big.Rat{{big.NewRat(1, 1), big.NewRat(-1, 1)}, {big.NewRat(1, 1), big.NewRat(-1, 1)}}, q.Offset, new(big.Rat), SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal("AB continuous mate", e)
	}
	tb2 := [3][2]*big.Rat{tb[0], tb[2], tb[1]}
	bc, e := CertifySurfaceAffineTriangleMate(ctx, b, c, [2][3][2]*big.Rat{tb2, tc}, [3]*big.Rat{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}, [2][2]*big.Rat{{big.NewRat(1, 1), new(big.Rat)}, {big.NewRat(1, 1), new(big.Rat)}}, q.Offset, big.NewRat(1, 2), SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal("BC continuous mate", e)
	}
	originalB, _, _ := ab.Source(1)
	otherB, _, _ := bc.Source(0)
	if originalB != otherB {
		t.Fatal("original middle binding")
	}
	pair := [2][3][2]*big.Rat{ta, tc}
	contact, e := CertifySurfaceAffineTriangleVertexContact(ctx, a, c, pair, [2]int{0, 0}, q, SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if contact.Sides() != ([2]int{-1, 1}) || contact.Point()[0].Cmp(big.NewRat(1, 2)) != 0 || contact.Point()[1].Cmp(big.NewRat(1, 2)) != 0 || contact.Point()[2].Sign() != 0 {
		t.Fatal("exact common original point")
	}
	if p, e := CertifySurfaceAffineTriangleSeparation(ctx, a, c, pair, q, SurfaceAffineCoordinateOptions{}); e == nil || p != nil {
		t.Fatal("strict separation must reject genuine shared vertex")
	}
	badC := affineTriangleFanSupport(t, 2, 1)
	if p, e := CertifySurfaceAffineTriangleVertexContact(ctx, a, badC, pair, [2]int{0, 0}, q, SurfaceAffineCoordinateOptions{}); e == nil || p != nil {
		t.Fatal("same plane but different model point")
	}
	extra := pair
	extra[0][1] = [2]*big.Rat{big.NewRat(1, 4), new(big.Rat)}
	if p, e := CertifySurfaceAffineTriangleVertexContact(ctx, a, c, extra, [2]int{0, 0}, q, SurfaceAffineCoordinateOptions{}); e == nil || p != nil {
		t.Fatal("extra plane contact")
	}
	if p, e := CertifySurfaceAffineTriangleVertexContact(ctx, a, c, pair, [2]int{1, 0}, q, SurfaceAffineCoordinateOptions{}); e == nil || p != nil {
		t.Fatal("wrong original vertex")
	}
	if p, e := CertifySurfaceAffineTriangleVertexContact(ctx, a, c, pair, [2]int{0, 0}, q, SurfaceAffineCoordinateOptions{SurfacePatchOptions: SurfacePatchOptions{MaxOperations: contact.ExactOperations() - 1}}); e == nil || p != nil {
		t.Fatal("atomic endpoint budget")
	}
	copy := contact.Point()
	copy[0].SetInt64(99)
	if contact.Point()[0].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("mutable point")
	}
}
