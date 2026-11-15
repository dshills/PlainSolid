package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestTwoPolynomialOriginalGraphs(t *testing.T) {
	for _, degree := range []int{3, 4} {
		for _, axis := range []int{0, 1} {
			upper := []*big.Rat{big.NewRat(-5, 8), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(-1, 8)}
			a, b, lower := bivariateGraphFixture(t, degree, axis, []*big.Rat{big.NewRat(1, 1)}, upper)
			for order, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
				r, e := DiscoverSplineGraphTwoPolynomials(context.Background(), pair[0], pair[1], axis, GraphPointContactOptions{})
				if e != nil || !r.Proven || len(r.CertifiedCells) != 4 {
					t.Fatal(degree, axis, order, r, e)
				}
				for i, x := range lower {
					if r.Power[0][i].Cmp(x) != 0 {
						t.Fatal("original lower")
					}
				}
				for i, x := range upper {
					if r.Power[1][i].Cmp(new(big.Rat).Neg(x)) != 0 {
						t.Fatal("original upper")
					}
				}
				sign := 1
				if order == 1 {
					sign = -1
				}
				if r.StripSigns != ([3]int{sign, -sign, sign}) {
					t.Fatal(r.StripSigns)
				}
				for _, u := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 1)} {
					for k := range r.Power {
						v := new(big.Rat)
						for i := len(r.Power[k]) - 1; i >= 0; i-- {
							v.Mul(v, u)
							v.Add(v, r.Power[k][i])
						}
						uv := [2]*big.Rat{u, v}
						if axis == 1 {
							uv = [2]*big.Rat{v, u}
						}
						ja, e := a.EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
						if e != nil {
							t.Fatal(e)
						}
						jb, e := b.EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
						if e != nil {
							t.Fatal(e)
						}
						for j := 0; j < 3; j++ {
							if ja.Point[j].Cmp(jb.Point[j]) != 0 {
								t.Fatal("original model root")
							}
						}
					}
				}
				for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
					q, e := DiscoverSplineGraphTwoPolynomials(context.Background(), pair[0], pair[1], axis, GraphPointContactOptions{MaxOperations: limit})
					if (e == nil) != (limit == r.ExactOperations) || (e != nil && (q.Proven || len(q.Power[0]) > 0 || len(q.CertifiedCells) > 0)) {
						t.Fatal("atomic quota", e)
					}
				}
				if q, e := DiscoverSplineGraphBivariatePolynomialContact(context.Background(), pair[0], pair[1], axis, GraphPointContactOptions{}); e == nil || q.Proven {
					t.Fatal("old unique-root proof widened")
				}
			}
		}
	}
}
func TestTwoPolynomialRejectsContactAndMissingCoverage(t *testing.T) {
	for _, upper := range [][]*big.Rat{
		{big.NewRat(-1, 4), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(-1, 8)}, // identical
		{big.NewRat(-1, 4), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(-1, 4)}, // touch at original endpoint
		{big.NewRat(-3, 8), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 4)},  // cross
		{big.NewRat(-1, 1), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(-1, 8)}, // outside
	} {
		a, b, _ := bivariateGraphFixture(t, 4, 0, []*big.Rat{big.NewRat(1, 1)}, upper)
		r, e := DiscoverSplineGraphTwoPolynomials(context.Background(), a, b, 0, GraphPointContactOptions{MaxDepth: 3})
		if e == nil || r.Proven || len(r.Controls[0]) > 0 {
			t.Fatal("unproved two interior roots", e)
		}
	}
	a, b, _ := bivariateGraphFixture(t, 4, 0, []*big.Rat{big.NewRat(1, 1)}, []*big.Rat{big.NewRat(-5, 8), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(-1, 8)})
	for _, o := range []GraphPointContactOptions{{MaxCells: 1}, {MaxOperations: 1}} {
		r, e := DiscoverSplineGraphTwoPolynomials(context.Background(), a, b, 0, o)
		if e == nil || r.Proven || len(r.CertifiedCells) > 0 {
			t.Fatal("partial proof")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := DiscoverSplineGraphTwoPolynomials(ctx, a, b, 0, GraphPointContactOptions{}); e == nil || r.Proven {
		t.Fatal("cancel")
	}
}
