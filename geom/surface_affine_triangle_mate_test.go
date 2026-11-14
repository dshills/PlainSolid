package geom

import (
	"context"
	"math/big"
	"testing"
)

func affineTriangleSupport(t *testing.T, curved bool, shift int64) *SurfacePatch {
	t.Helper()
	var h [][4]*big.Rat
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			u, v := big.NewRat(int64(i), 2), big.NewRat(int64(j), 2)
			q := new(big.Rat)
			if curved {
				q.SetInt64(1)
				if i == 2 {
					q.Add(q, big.NewRat(1, 1))
				}
				if j == 2 {
					q.Add(q, big.NewRat(1, 1))
				}
				q.Add(q, new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(u, v)))
				q.Sub(q, new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(u, v)))
				q.Quo(q, big.NewRat(4, 1))
			}
			q.Add(q, big.NewRat(shift, 64))
			h = append(h, [4]*big.Rat{new(big.Rat).Add(u, q), new(big.Rat).Sub(v, q), new(big.Rat).Add(u, v), big.NewRat(1, 1)})
		}
	}
	p, e := NewExactHomogeneousBezierSurface(context.Background(), 2, 2, h, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if curved {
		p, e = p.InsertUKnot(.25, 1, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = p.InsertVKnot(.5, 1, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	return p
}
func TestSurfaceAffineTriangleMate(t *testing.T) {
	ctx := context.Background()
	a, b := affineTriangleSupport(t, false, 0), affineTriangleSupport(t, true, 0)
	pt := func(u, v int64) [2]*big.Rat { return [2]*big.Rat{big.NewRat(u, 1), big.NewRat(v, 1)} }
	triangles := [2][3][2]*big.Rat{{pt(0, 1), pt(1, 0), pt(0, 0)}, {pt(0, 1), pt(1, 0), pt(1, 1)}}
	normal := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1)}
	uv := [2][2]*big.Rat{{big.NewRat(2, 1), big.NewRat(2, 1)}, {big.NewRat(2, 1), big.NewRat(2, 1)}}
	offsets := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	p, e := CertifySurfaceAffineTriangleMate(ctx, a, b, triangles, normal, uv, offsets, big.NewRat(2, 1), SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if p.Sides() != ([2]int{-1, 1}) {
		t.Fatal("sides")
	}
	source, copy, e := p.Source(1)
	if e != nil || source != b {
		t.Fatal("source")
	}
	copy[0][0].SetInt64(9)
	_, again, _ := p.Source(1)
	if again[0][0].Sign() != 0 {
		t.Fatal("aliased UV")
	}
	// Each identity remains exact after a tangent translation, but the continuous mate does not.
	bad := affineTriangleSupport(t, true, 1)
	if q, e := CertifySurfaceAffineTriangleMate(ctx, a, bad, triangles, normal, uv, offsets, big.NewRat(2, 1), SurfaceAffineCoordinateOptions{}); q != nil || e == nil {
		t.Fatal("unmatched full model edge")
	}
	for _, opts := range []SurfaceAffineCoordinateOptions{{MaxCells: 1}, {SurfacePatchOptions: SurfacePatchOptions{MaxOperations: p.ExactOperations() - 1}}} {
		if q, e := CertifySurfaceAffineTriangleMate(ctx, a, b, triangles, normal, uv, offsets, big.NewRat(2, 1), opts); q != nil || e == nil {
			t.Fatal("atomic bounds")
		}
	}
	triangles[1][2] = pt(0, 0)
	if q, e := CertifySurfaceAffineTriangleMate(ctx, a, b, triangles, normal, uv, offsets, big.NewRat(2, 1), SurfaceAffineCoordinateOptions{}); q != nil || e == nil {
		t.Fatal("same side")
	}
}
