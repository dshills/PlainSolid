package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSurfaceNormalOffsetCoverageParabolicCylinder(t *testing.T) {
	ctx := context.Background()
	var h [][4]*big.Rat
	for i, x := range []int64{-1, 0, 1} {
		for _, y := range []int64{0, 1} {
			h = append(h, [4]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1), big.NewRat([]int64{1, -1, 1}[i], 1), big.NewRat(1, 1)})
		}
	}
	p, err := NewExactHomogeneousBezierSurface(ctx, 2, 1, h, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// S(u,v)=(2u-1,v,(2u-1)^2). The minimum positive focal
	// distance is 1/2 at u=1/2, for every v.
	inside, err := p.CertifyNormalOffsetCoverage(ctx, big.NewRat(49, 100), NormalOffsetCoverageOptions{})
	if err != nil || !inside.Complete || len(inside.Proven) < 2 || len(inside.Unresolved) != 0 {
		t.Fatal("failed to partition and certify the subcritical offset", len(inside.Proven), len(inside.Unresolved), err)
	}
	checkNormalOffsetCover(t, p.Domain(), inside)
	for _, d := range []*big.Rat{big.NewRat(1, 2), big.NewRat(3, 5)} {
		touch, err := p.CertifyNormalOffsetCoverage(ctx, d, NormalOffsetCoverageOptions{MaxDepth: 8})
		if !errors.Is(err, ErrConvergence) || touch.Complete || len(touch.Proven) == 0 || len(touch.Unresolved) == 0 {
			t.Fatal("focal displacement was not retained as unresolved", d, len(touch.Proven), len(touch.Unresolved), err)
		}
		checkNormalOffsetCover(t, p.Domain(), touch)
		for _, r := range touch.Proven {
			if r.UMin <= .5 && r.UMax >= .5 {
				t.Fatal("claimed coverage includes the known focal line", r, d)
			}
		}
	}
	for _, limit := range []NormalOffsetCoverageOptions{
		{MaxCells: 1}, {Patch: SurfacePatchOptions{MaxOperations: 26}}, {Patch: SurfacePatchOptions{MaxPoles: 6}},
	} {
		bounded, err := p.CertifyNormalOffsetCoverage(ctx, big.NewRat(49, 100), limit)
		if !errors.Is(err, ErrConvergence) || bounded.Complete || len(bounded.Unresolved) == 0 {
			t.Fatal("bounded proof did not retain unresolved work", bounded, err)
		}
		checkNormalOffsetCover(t, p.Domain(), bounded)
	}
}

func TestSurfaceNormalOffsetCoverageRetainsOriginalKnotPieces(t *testing.T) {
	ctx := context.Background()
	s, err := NewBSplineSurface(1, 1, 3, 2,
		[]Vec3{{-3, 0, 0}, {-3, 1, 0}, {0, 0, 0}, {0, 1, 0}, {2, 0, 2}, {2, 1, 2}},
		[]float64{-2, -2, 1, 5, 5}, []float64{0, 0, 2, 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewSurfacePatch(s, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Two planar spans meet at a crease. Local offset regularity on both
	// is independently provable even though their normals differ.
	got, err := p.CertifyNormalOffsetCoverage(ctx, big.NewRat(-100, 1), NormalOffsetCoverageOptions{})
	want := []ParameterRectangle{{-2, 1, 0, 2}, {1, 5, 0, 2}}
	if err != nil || !got.Complete || len(got.Proven) != 2 || got.Proven[0] != want[0] || got.Proven[1] != want[1] {
		t.Fatal("original knot ownership was lost", got, err)
	}
	checkNormalOffsetCover(t, p.Domain(), got)
}

func TestSurfaceNormalOffsetCoverageWithNonconstantWeights(t *testing.T) {
	ctx := context.Background()
	var h [][4]*big.Rat
	for _, u := range []int64{0, 1} {
		for _, v := range []int64{0, 1} {
			h = append(h, [4]*big.Rat{big.NewRat(2*u, 1), big.NewRat(v*(1+u), 1), new(big.Rat), big.NewRat(1+u, 1)})
		}
	}
	p, err := NewExactHomogeneousBezierSurface(ctx, 1, 1, h, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// S=(2u/(1+u),v,0) is regular, and its signed curvature is zero.
	got, err := p.CertifyNormalOffsetCoverage(ctx, big.NewRat(10000, 1), NormalOffsetCoverageOptions{})
	if err != nil || !got.Complete {
		t.Fatal("nonconstant weights obscured a regular plane", len(got.Proven), len(got.Unresolved), err)
	}
	checkNormalOffsetCover(t, p.Domain(), got)
}

func checkNormalOffsetCover(t *testing.T, domain ParameterRectangle, got NormalOffsetCoverage) {
	t.Helper()
	area := func(r ParameterRectangle) *big.Rat {
		u := new(big.Rat).Sub(new(big.Rat).SetFloat64(r.UMax), new(big.Rat).SetFloat64(r.UMin))
		v := new(big.Rat).Sub(new(big.Rat).SetFloat64(r.VMax), new(big.Rat).SetFloat64(r.VMin))
		return u.Mul(u, v)
	}
	all := append(append([]ParameterRectangle(nil), got.Proven...), got.Unresolved...)
	sum := new(big.Rat)
	for i, r := range all {
		if !r.Valid() || r.UMin < domain.UMin || r.UMax > domain.UMax || r.VMin < domain.VMin || r.VMax > domain.VMax {
			t.Fatal("cell lies outside the original parameter cover", r)
		}
		for _, s := range all[:i] {
			if max(r.UMin, s.UMin) < min(r.UMax, s.UMax) && max(r.VMin, s.VMin) < min(r.VMax, s.VMax) {
				t.Fatal("cells overlap in their interiors", r, s)
			}
		}
		sum.Add(sum, area(r))
	}
	if sum.Cmp(area(domain)) != 0 {
		t.Fatal("offset proof lost original parameter area", sum, area(domain))
	}
}
