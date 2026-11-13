package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestQuadraticSaddleSectorSignsAndOrientation(t *testing.T) {
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	rectangle := [2][2]*big.Rat{{new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 1)}}
	for _, reflected := range []bool{false, true} {
		for _, swapped := range []bool{false, true} {
			a, b := graphContactFixture(t, false, true)
			if reflected {
				for _, p := range []*SurfacePatch{a, b} {
					for i := range p.net {
						p.net[i][0] = new(big.Rat).Neg(p.net[i][0])
					}
				}
			}
			if swapped {
				a, b = b, a
			}
			r, e := CertifyQuadraticGraphSaddleSectors(context.Background(), a, b, uv, rectangle, GraphPointContactOptions{})
			if e != nil || !r.Complete {
				t.Fatal(r, e)
			}
			for index, sector := range r.Sectors {
				q, e := r.ClassifyRational(context.Background(), sector.Witness, SurfacePatchEvaluationOptions{})
				if e != nil || q.Sector != index || q.HeightSide != sector.HeightSide || q.Ray != -1 {
					t.Fatal(index, q, e)
				}
				x, e := a.EvaluateRational(context.Background(), sector.Witness[0], sector.Witness[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				y, e := b.EvaluateRational(context.Background(), sector.Witness[0], sector.Witness[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if new(big.Rat).Sub(x.Point[2], y.Point[2]).Sign() != sector.HeightSide {
					t.Fatal("independent height side")
				}
				start, end := r.Section.Terminals[sector.PositiveBoundary.StartRay], r.Section.Terminals[sector.PositiveBoundary.EndRay]
				x0, y0 := new(big.Rat).Sub(start.Lower[0], uv[0]), new(big.Rat).Sub(start.Lower[1], uv[1])
				x1, y1 := new(big.Rat).Sub(end.Lower[0], uv[0]), new(big.Rat).Sub(end.Lower[1], uv[1])
				cross := new(big.Rat).Sub(new(big.Rat).Mul(x0, y1), new(big.Rat).Mul(y0, x1))
				orientation := 1
				if reflected {
					orientation = -1
				}
				if cross.Sign()*orientation != 1 || sector.PositiveBoundary.ParameterDirection != orientation || sector.NegativeBoundary.StartRay != sector.PositiveBoundary.EndRay {
					t.Fatal(sector)
				}
			}
			center, e := r.ClassifyRational(context.Background(), uv, SurfacePatchEvaluationOptions{})
			if e != nil || !center.Saddle || center.Sector != -1 {
				t.Fatal(center, e)
			}
			for _, terminal := range r.Section.Terminals {
				q, e := r.ClassifyRational(context.Background(), terminal.Lower, SurfacePatchEvaluationOptions{})
				if e != nil || q.Ray != terminal.Ray || !q.Boundary || q.HeightSide != 0 {
					t.Fatal(q, e)
				}
			}
			// Mutating display metadata cannot affect the private proof.
			query := [2]*big.Rat{new(big.Rat).Set(r.Sectors[0].Witness[0]), new(big.Rat).Set(r.Sectors[0].Witness[1])}
			r.Section.Certificate.Critical.Hessian[0][0].SetInt64(0)
			r.RayCycle = [4]int{}
			q, e := r.ClassifyRational(context.Background(), query, SurfacePatchEvaluationOptions{})
			if e != nil || q.Sector != 0 {
				t.Fatal(q, e)
			}
		}
	}
}
func TestSaddleIrrationalSectorPointClassification(t *testing.T) {
	a, b := graphContactFixture(t, false, true)
	n := a.degreeU
	q := func(i int) *big.Rat {
		return new(big.Rat).Add(big.NewRat(1, 4), new(big.Rat).Sub(big.NewRat(int64(i*(i-1)), int64(n*(n-1))), big.NewRat(int64(i), int64(n))))
	}
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			k := i*a.countV + j
			a.net[k][2] = new(big.Rat).Add(b.net[k][2], new(big.Rat).Sub(q(i), new(big.Rat).Mul(big.NewRat(2, 1), q(j))))
		}
	}
	r, e := CertifyQuadraticGraphSaddleSectors(context.Background(), a, b, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, [2][2]*big.Rat{{new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 1)}}, GraphPointContactOptions{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	for i, sector := range r.Sectors {
		point, e := r.ClassifyRational(context.Background(), sector.Witness, SurfacePatchEvaluationOptions{})
		if e != nil || point.Sector != i || point.HeightSide != sector.HeightSide {
			t.Fatal(point, e)
		}
	}
	outside, e := r.ClassifyRational(context.Background(), [2]*big.Rat{big.NewRat(2, 1), new(big.Rat)}, SurfacePatchEvaluationOptions{})
	if e != nil || !outside.Outside {
		t.Fatal(outside, e)
	}
}
func TestVerticalSaddleSectorClassification(t *testing.T) {
	a, b := graphContactFixture(t, false, true)
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			k := i*a.countV + j
			u := big.NewRat(int64(2*i-(a.countU-1)), int64(2*(a.countU-1)))
			v := big.NewRat(int64(2*j-(a.countV-1)), int64(2*(a.countV-1)))
			a.net[k][2] = new(big.Rat).Add(b.net[k][2], new(big.Rat).Mul(u, v))
		}
	}
	r, e := CertifyQuadraticGraphSaddleSectors(context.Background(), a, b, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, [2][2]*big.Rat{{new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 1)}}, GraphPointContactOptions{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	for i, sector := range r.Sectors {
		p, e := r.ClassifyRational(context.Background(), sector.Witness, SurfacePatchEvaluationOptions{})
		if e != nil || p.Sector != i || p.HeightSide != sector.HeightSide {
			t.Fatal(i, p, e)
		}
	}
	for _, terminal := range r.Section.Terminals {
		p, e := r.ClassifyRational(context.Background(), terminal.Lower, SurfacePatchEvaluationOptions{})
		if e != nil || p.Ray != terminal.Ray {
			t.Fatal(p, e)
		}
	}
}
