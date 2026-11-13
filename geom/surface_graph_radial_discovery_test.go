package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactRadialPairFactorProof(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	x2 := []float64{3, 0, -1, 0, 3}
	x4 := []float64{1, -1, 1, -1, 1}
	var poles []Vec3
	ri, ro := 25.0/1024, 25.0/256
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			poles = append(poles, Vec3{float64(i) / 4, float64(j) / 4, (9*(x4[i]+x4[j]) + 2*x2[i]*x2[j] - 12*(ri+ro)*(x2[i]+x2[j]) + 144*ri*ro) / 64})
		}
	}
	k := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	s, e := NewBSplineSurface(4, 4, 5, 5, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := DiscoverSplineGraphRadialPair(ctx, a, b, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Radius[0].RatString() != "5/32" || r.Radius[1].RatString() != "5/16" || r.Side != -1 {
		t.Fatal(r, e)
	}
	if _, e := DiscoverSplineGraphRadialPair(ctx, a, b, GraphPointContactOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := DiscoverSplineGraphRadialPair(cancelled, a, b, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	poles[0].Z += 1.0 / 1048576
	s, _ = NewBSplineSurface(4, 4, 5, 5, poles, k, k, nil)
	bad, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	if _, e := DiscoverSplineGraphRadialPair(ctx, a, bad, GraphPointContactOptions{}); e == nil {
		t.Fatal("nonradial coefficient accepted")
	}
}

func TestExactRadialPairRationalCenter(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	for _, center := range [][2]*big.Rat{{big.NewRat(1, 3), big.NewRat(2, 5)}, {big.NewRat(7, 16), big.NewRat(9, 16)}} {
		radius := []*big.Rat{big.NewRat(1, 16), big.NewRat(1, 8)}
		factors := [2]map[[2]int]*big.Rat{}
		for i, r := range radius {
			c0 := new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(center[0], center[0]), new(big.Rat).Mul(center[1], center[1])), new(big.Rat).Mul(r, r))
			factors[i] = map[[2]int]*big.Rat{{0, 0}: c0, {1, 0}: new(big.Rat).Mul(big.NewRat(-2, 1), center[0]), {2, 0}: big.NewRat(1, 1), {0, 1}: new(big.Rat).Mul(big.NewRat(-2, 1), center[1]), {0, 2}: big.NewRat(1, 1)}
		}
		power := map[[2]int]*big.Rat{}
		for ij, x := range factors[0] {
			for kl, y := range factors[1] {
				key := [2]int{ij[0] + kl[0], ij[1] + kl[1]}
				if power[key] == nil {
					power[key] = new(big.Rat)
				}
				power[key].Add(power[key], new(big.Rat).Mul(x, y))
			}
		}
		k := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
		poles := make([]Vec3, 25)
		for i := 0; i < 5; i++ {
			for j := 0; j < 5; j++ {
				poles[5*i+j] = Vec3{float64(i) / 4, float64(j) / 4, 0}
			}
		}
		s, _ := NewBSplineSurface(4, 4, 5, 5, poles, k, k, nil)
		b, _ := NewSurfacePatch(s, SurfacePatchOptions{})
		for i := 0; i < 5; i++ {
			for j := 0; j < 5; j++ {
				z := new(big.Rat)
				for ij, q := range power {
					if ij[0] > i || ij[1] > j {
						continue
					}
					fu := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(ij[0])), new(big.Int).Binomial(4, int64(ij[0])))
					fv := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(j), int64(ij[1])), new(big.Int).Binomial(4, int64(ij[1])))
					z.Add(z, new(big.Rat).Mul(q, new(big.Rat).Mul(fu, fv)))
				}
				b.net[5*i+j][2] = z
			}
		}
		b.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384})
		r, e := DiscoverSplineGraphRadialPair(ctx, a, b, GraphPointContactOptions{})
		if e != nil || !r.Proven || r.Center[0].Cmp(center[0]) != 0 || r.Center[1].Cmp(center[1]) != 0 || r.Radius[0].Cmp(radius[0]) != 0 || r.Radius[1].Cmp(radius[1]) != 0 {
			t.Fatal(r, e)
		}
	}
}
