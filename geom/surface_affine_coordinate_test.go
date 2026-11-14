package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSurfaceAffineCoordinateRational(t *testing.T) {
	ctx := context.Background()
	var controls [][4]*big.Rat
	for i := 0; i < 2; i++ {
		for j := 0; j < 3; j++ {
			w := []*big.Rat{big.NewRat(1, 1), big.NewRat(3, 2), big.NewRat(2, 1)}[j]
			z := []*big.Rat{big.NewRat(1, 3), big.NewRat(3, 2), big.NewRat(14, 3)}[j]
			y := []*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(2, 1)}[j]
			controls = append(controls, [4]*big.Rat{new(big.Rat).Mul(big.NewRat(int64(i), 1), w), y, z, w})
		}
	}
	p, e := NewExactHomogeneousBezierSurface(ctx, 1, 2, controls, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertVKnot(.25, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot(.5, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := CertifySurfaceAffineCoordinate(ctx, p, 2, 1, big.NewRat(2, 1), big.NewRat(1, 3), SurfaceAffineCoordinateOptions{})
	if e != nil || !r.Proven || r.Cells != 4 {
		t.Fatal(r, e)
	}
	if r, e := CertifySurfaceAffineCoordinate(ctx, p, 0, 0, big.NewRat(1, 1), new(big.Rat), SurfaceAffineCoordinateOptions{}); e != nil || !r.Proven {
		t.Fatal("other parameter axis", r, e)
	}
	for _, o := range []SurfaceAffineCoordinateOptions{{MaxCells: 3}, {SurfacePatchOptions: SurfacePatchOptions{MaxOperations: r.ExactOperations - 1}}} {
		if r, e := CertifySurfaceAffineCoordinate(ctx, p, 2, 1, big.NewRat(2, 1), big.NewRat(1, 3), o); e == nil || r.Proven {
			t.Fatal("atomic proof bounds")
		}
	}
	controls[1][2] = new(big.Rat).Add(controls[1][2], big.NewRat(1, 4096))
	bad, e := NewExactHomogeneousBezierSurface(ctx, 1, 2, controls, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r, e := CertifySurfaceAffineCoordinate(ctx, bad, 2, 1, big.NewRat(2, 1), big.NewRat(1, 3), SurfaceAffineCoordinateOptions{}); e != nil || r.Proven {
		t.Fatal("same endpoints, altered interior", r, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := CertifySurfaceAffineCoordinate(canceled, p, 2, 1, big.NewRat(2, 1), big.NewRat(1, 3), SurfaceAffineCoordinateOptions{}); e == nil || r.Proven {
		t.Fatal("cancel")
	}
}
