package geom

import (
	"context"
	"math/big"
	"testing"
)

func rotatedEllipseFixture(t *testing.T, higher bool, nonsquare ...bool) (*SurfacePatch, *SurfacePatch, GraphQuadraticEllipseHint) {
	hint := GraphQuadraticEllipseHint{Coefficients: [6]*big.Rat{big.NewRat(12, 1), big.NewRat(-24, 1), big.NewRat(16, 1), big.NewRat(-28, 1), big.NewRat(20, 1), big.NewRat(16, 1)}}
	if len(nonsquare) > 0 && nonsquare[0] {
		hint.Coefficients = [6]*big.Rat{big.NewRat(10, 1), big.NewRat(-20, 1), big.NewRat(16, 1), big.NewRat(-24, 1), big.NewRat(20, 1), big.NewRat(8, 1)}
	}
	indices := [][2]int{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {0, 2}, {1, 1}}
	power := map[[2]int]*big.Rat{}
	for i, ij := range indices {
		shifts := []int{0}
		if higher {
			shifts = append(shifts, 2)
		}
		for _, shift := range shifts {
			key := [2]int{ij[0] + shift, ij[1]}
			if power[key] == nil {
				power[key] = new(big.Rat)
			}
			power[key].Add(power[key], hint.Coefficients[i])
		}
	}
	a, b := graphContactFixture(t, false, false)
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			z := new(big.Rat)
			for ij, c := range power {
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
func TestRotatedGraphEllipseCertificateAndDiscovery(t *testing.T) {
	for _, higher := range []bool{false, true} {
		a, b, h := rotatedEllipseFixture(t, higher)
		for swap, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			r, e := CertifySplineGraphQuadraticEllipse(context.Background(), pair[0], pair[1], h, GraphPointContactOptions{})
			sign := 1
			if swap == 1 {
				sign = -1
			}
			if e != nil || !r.Proven || r.InsideSide != -sign || r.OutsideSide != sign || r.Center[0].Cmp(big.NewRat(1, 2)) != 0 || r.Center[1].Cmp(big.NewRat(1, 2)) != 0 || r.ProjectionRadiusSquared[0].Cmp(big.NewRat(5, 64)) != 0 || r.ProjectionRadiusSquared[1].Cmp(big.NewRat(1, 16)) != 0 {
				t.Fatal(r, e)
			}
			for _, item := range []struct {
				uv   [2]*big.Rat
				sign int
			}{{[2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, -sign}, {[2]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1)}, sign}, {[2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)}, 0}} {
				x, e := pair[0].EvaluateRational(context.Background(), item.uv[0], item.uv[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				y, e := pair[1].EvaluateRational(context.Background(), item.uv[0], item.uv[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if new(big.Rat).Sub(x.Point[2], y.Point[2]).Sign() != item.sign {
					t.Fatal("independent sign")
				}
			}
			if !higher {
				r, e = DiscoverSplineGraphQuadraticEllipse(context.Background(), pair[0], pair[1], GraphPointContactOptions{})
				if e != nil || !r.Proven || r.Center[0].Cmp(big.NewRat(1, 2)) != 0 || r.ProjectionRadiusSquared[0].Cmp(big.NewRat(5, 64)) != 0 {
					t.Fatal(r, e)
				}
			}
		}
	}
}
func TestRotatedGraphEllipseConservativeFailures(t *testing.T) {
	a, b, h := rotatedEllipseFixture(t, false)
	wrong := h
	wrong.Coefficients[5] = big.NewRat(100, 1)
	r, e := CertifySplineGraphQuadraticEllipse(context.Background(), a, b, wrong, GraphPointContactOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	wrong = h
	wrong.Coefficients[0] = big.NewRat(8, 1)
	r, e = CertifySplineGraphQuadraticEllipse(context.Background(), a, b, wrong, GraphPointContactOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	r, e = DiscoverSplineGraphQuadraticEllipse(context.Background(), a, b, GraphPointContactOptions{MaxOperations: 1})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	a.net[a.countV+1][2].Add(a.net[a.countV+1][2], tiny)
	r, e = CertifySplineGraphQuadraticEllipse(context.Background(), a, b, h, GraphPointContactOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
}
