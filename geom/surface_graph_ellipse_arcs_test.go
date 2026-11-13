package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestRationalPointEllipseArcsNonsquareConjugate(t *testing.T) {
	a, b, h := rotatedEllipseFixture(t, true, true)
	point := [2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)}
	r, e := BuildSplineGraphQuadraticEllipseArcs(context.Background(), a, b, h, point, GraphPointContactOptions{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	for arc, curve := range r.Arcs {
		if curve.patch.degreeU != 2 {
			t.Fatal("degree")
		}
		for _, pole := range curve.patch.net {
			if pole[3].Sign() <= 0 {
				t.Fatal("weight")
			}
		}
		for _, parameter := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 7), big.NewRat(1, 2), big.NewRat(6, 7), big.NewRat(1, 1)} {
			jet, e := curve.patch.EvaluateRational(context.Background(), parameter, big.NewRat(0, 1), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			u, v := jet.Point[0], jet.Point[1]
			q := h.Coefficients
			residual := new(big.Rat).Set(q[0])
			for _, term := range []*big.Rat{new(big.Rat).Mul(q[1], u), new(big.Rat).Mul(q[2], new(big.Rat).Mul(u, u)), new(big.Rat).Mul(q[3], v), new(big.Rat).Mul(q[4], new(big.Rat).Mul(v, v)), new(big.Rat).Mul(q[5], new(big.Rat).Mul(u, v))} {
				residual.Add(residual, term)
			}
			if residual.Sign() != 0 || jet.Point[2].Sign() != 0 {
				t.Fatal("implicit identity")
			}
			// Counterclockwise radial determinant at every interior represented sample.
			if parameter.Sign() > 0 && parameter.Cmp(big.NewRat(1, 1)) < 0 {
				position, e := curve.Eval(.5)
				if e != nil {
					t.Fatal(e)
				}
				tangent, e := curve.Derivative(.5)
				if e != nil {
					t.Fatal(e)
				}
				if (position.X-.5)*tangent.Y-(position.Y-.5)*tangent.X <= 0 {
					t.Fatal("orientation")
				}
			}
			if parameter.Sign() == 0 {
				if u.Cmp(r.Junctions[arc][0]) != 0 || v.Cmp(r.Junctions[arc][1]) != 0 {
					t.Fatal("start")
				}
			}
			if parameter.Cmp(big.NewRat(1, 1)) == 0 {
				if u.Cmp(r.Junctions[(arc+1)%4][0]) != 0 || v.Cmp(r.Junctions[(arc+1)%4][1]) != 0 {
					t.Fatal("join")
				}
			}
		}
	}
	// Returned junctions own their data; mutation cannot alter exact curves.
	r.Junctions[0][0].SetInt64(99)
	start, e := r.Arcs[0].ExactPoint(0)
	if e != nil || start.X != .75 || start.Y != .5 {
		t.Fatal(start, e)
	}
}
func TestRationalPointEllipseArcsRejectsWrongPointAndBudget(t *testing.T) {
	a, b, h := rotatedEllipseFixture(t, false, true)
	for _, point := range [][2]*big.Rat{{big.NewRat(1, 2), big.NewRat(1, 2)}, {new(big.Rat).Add(big.NewRat(3, 4), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))), big.NewRat(1, 2)}} {
		r, e := BuildSplineGraphQuadraticEllipseArcs(context.Background(), a, b, h, point, GraphPointContactOptions{})
		if e == nil || r.Complete {
			t.Fatal(r, e)
		}
	}
	r, e := BuildSplineGraphQuadraticEllipseArcs(context.Background(), a, b, h, [2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)}, GraphPointContactOptions{MaxOperations: 1})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
}
