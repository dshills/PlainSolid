package geom

import (
	"context"
	"math/big"
	"testing"
)

func ellipseGraphFixture(t *testing.T, repeated ...bool) (*SurfacePatch, *SurfacePatch, GraphEllipseHint) {
	hint := GraphEllipseHint{Center: [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, RadiusSquared: [2]*big.Rat{big.NewRat(1, 16), big.NewRat(1, 25)}}
	power := map[[2]int]*big.Rat{{0, 0}: big.NewRat(-1, 1)}
	for axis := 0; axis < 2; axis++ {
		center, r2 := hint.Center[axis], hint.RadiusSquared[axis]
		power[[2]int{0, 0}].Add(power[[2]int{0, 0}], new(big.Rat).Quo(new(big.Rat).Mul(center, center), r2))
		linear, quad := [2]int{}, [2]int{}
		linear[axis] = 1
		quad[axis] = 2
		power[linear] = new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(-2, 1), center), r2)
		power[quad] = new(big.Rat).Inv(r2)
	}
	product := map[[2]int]*big.Rat{}
	for ij, c := range power {
		for _, shift := range []int{0, 2} {
			key := [2]int{ij[0] + shift, ij[1]}
			if product[key] == nil {
				product[key] = new(big.Rat)
			}
			product[key].Add(product[key], c)
		}
	}
	if len(repeated) > 0 && repeated[0] {
		product = map[[2]int]*big.Rat{}
		for ij, c := range power {
			for kl, d := range power {
				key := [2]int{ij[0] + kl[0], ij[1] + kl[1]}
				if product[key] == nil {
					product[key] = new(big.Rat)
				}
				product[key].Add(product[key], new(big.Rat).Mul(c, d))
			}
		}
	}
	a, b := graphContactFixture(t, false, false)
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			z := new(big.Rat)
			for ij, c := range product {
				if ij[0] > i || ij[1] > j {
					continue
				}
				fu := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(ij[0])), new(big.Int).Binomial(int64(a.degreeU), int64(ij[0])))
				fv := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(j), int64(ij[1])), new(big.Int).Binomial(int64(a.degreeV), int64(ij[1])))
				z.Add(z, new(big.Rat).Mul(c, new(big.Rat).Mul(fu, fv)))
			}
			a.net[i*a.countV+j][2].Add(b.net[i*a.countV+j][2], z)
		}
	}
	a, _ = a.InsertUKnot(.5, 1, SurfacePatchOptions{})
	b, _ = b.InsertVKnot(.25, 1, SurfacePatchOptions{})
	return a, b, hint
}
func TestGraphEllipseFactorComplete(t *testing.T) {
	a, b, hint := ellipseGraphFixture(t)
	for swap, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
		r, e := CertifySplineGraphEllipse(context.Background(), pair[0], pair[1], hint, GraphPointContactOptions{})
		sign := 1
		if swap == 1 {
			sign = -1
		}
		if e != nil || !r.Proven || r.InsideSide != -sign || r.OutsideSide != sign || len(r.CertifiedCells) < 2 {
			t.Fatal(r, e)
		}
		for _, item := range []struct {
			uv   [2]*big.Rat
			sign int
		}{{[2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, -sign}, {[2]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1)}, sign}, {[2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)}, 0}, {[2]*big.Rat{big.NewRat(1, 2), big.NewRat(7, 10)}, 0}} {
			x, e := pair[0].EvaluateRational(context.Background(), item.uv[0], item.uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			y, e := pair[1].EvaluateRational(context.Background(), item.uv[0], item.uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if new(big.Rat).Sub(x.Point[2], y.Point[2]).Sign() != item.sign {
				t.Fatal("exact independent sign")
			}
		}
	}
}
func TestGraphEllipseRejectsWrongHintPerturbationAndBounds(t *testing.T) {
	a, b, hint := ellipseGraphFixture(t)
	wrong := hint
	wrong.Center[0] = big.NewRat(1, 3)
	r, e := CertifySplineGraphEllipse(context.Background(), a, b, wrong, GraphPointContactOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	wrong = hint
	wrong.RadiusSquared[0] = big.NewRat(1, 4)
	r, e = CertifySplineGraphEllipse(context.Background(), a, b, wrong, GraphPointContactOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	r, e = CertifySplineGraphEllipse(context.Background(), a, b, hint, GraphPointContactOptions{MaxOperations: 1})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	a.net[a.countV+1][2].Add(a.net[a.countV+1][2], tiny)
	r, e = CertifySplineGraphEllipse(context.Background(), a, b, hint, GraphPointContactOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = CertifySplineGraphEllipse(ctx, a, b, hint, GraphPointContactOptions{})
	if e != context.Canceled {
		t.Fatal(e)
	}
}

func TestGraphEllipseRepeatedContactUnresolved(t *testing.T) {
	a, b, hint := ellipseGraphFixture(t, true)
	r, e := CertifySplineGraphEllipse(context.Background(), a, b, hint, GraphPointContactOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
}

func TestCertifiedRepeatedGraphEllipse(t *testing.T) {
	ctx := context.Background()
	a, b, hint := ellipseGraphFixture(t, true)
	r, e := CertifySplineGraphRepeatedEllipse(ctx, a, b, hint, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.InsideSide != 1 || r.OutsideSide != 1 || len(r.CertifiedCells) < 2 {
		t.Fatal(r, e)
	}
	r, e = CertifySplineGraphRepeatedEllipse(ctx, b, a, hint, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.InsideSide != -1 || r.OutsideSide != -1 {
		t.Fatal("same strict reverse side inside and outside", r, e)
	}
	r, e = DiscoverSplineGraphRepeatedEllipse(ctx, a, b, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.InsideSide != 1 || r.OutsideSide != 1 {
		t.Fatal("independent multi-cell repeated ellipse discovery", r, e)
	}
	for i := range hint.Center {
		if hint.Center[i].Cmp(r.Ellipse.Center[i]) != 0 || hint.RadiusSquared[i].Cmp(r.Ellipse.RadiusSquared[i]) != 0 {
			t.Fatal("wrong original conic recovery", r.Ellipse, hint)
		}
	}
	a, b, hint = ellipseGraphFixture(t, false)
	if r, e := CertifySplineGraphRepeatedEllipse(ctx, a, b, hint, GraphPointContactOptions{}); e == nil || r.Proven {
		t.Fatal("transverse contact accepted as double", r, e)
	}
}
