package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSeparatedTorusCylinderMaterial(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, op := range []TorusCylinderOperation{TorusCylinderCommon, TorusCylinderDifference, TorusCylinderUnion} {
		s, e := NewSeparatedTorusCylinderMaterial(5, 2, 2, -3, 4, op)
		if e != nil {
			t.Fatal(e)
		}
		gap, _ := s.RadialClearance()
		if gap.Cmp(r(1, 1)) != 0 {
			t.Fatal(gap)
		}
		gap.SetInt64(99)
		again, _ := s.RadialClearance()
		if again.Cmp(r(1, 1)) != 0 {
			t.Fatal("alias")
		}
		for _, sample := range []struct {
			p     [3]*big.Rat
			loc   BorePointLocation
			owner BoreBoundaryMask
		}{
			{[3]*big.Rat{r(3, 1), r(0, 1), r(0, 1)}, BoreBoundary, TorusCylinderTorus},
			{[3]*big.Rat{r(5, 1), r(0, 1), r(0, 1)}, BoreInside, 0},
			{[3]*big.Rat{r(2, 1), r(0, 1), r(-3, 1)}, BoreBoundary, TorusCylinderWall | TorusCylinderLowerCap},
			{[3]*big.Rat{r(0, 1), r(0, 1), r(4, 1)}, BoreBoundary, TorusCylinderUpperCap},
			{[3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, BoreInside, 0},
		} {
			want, owner := sample.loc, sample.owner
			if op == TorusCylinderCommon || (op == TorusCylinderDifference && sample.p[0].Cmp(r(3, 1)) < 0) {
				want, owner = BoreOutside, 0
			}
			loc, mask, e := s.ClassifyRational(ctx, sample.p, SurfacePatchOptions{})
			if e != nil || loc != want || mask != owner {
				t.Fatal(op, sample.p, loc, mask, want, owner, e)
			}
		}
		p, d := [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}
		line, e := s.ClipRationalLine(ctx, p, d, r(-8, 1), r(8, 1), ToroidLineOptions{ParameterTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		want := 0
		if op == TorusCylinderDifference {
			want = 2
		}
		if op == TorusCylinderUnion {
			want = 3
		}
		if len(line.Intervals) != want || len(line.Points) != 0 {
			t.Fatal(op, line)
		}
		expected := [][2]int64{{-7, -3}, {3, 7}}
		if op == TorusCylinderUnion {
			expected = [][2]int64{{-7, -3}, {-2, 2}, {3, 7}}
		}
		if op != TorusCylinderCommon {
			for i, interval := range line.Intervals {
				for j, root := range []*ExactAlgebraicRoot{interval.Start, interval.End} {
					sign, e := root.SignPolynomial(ctx, []*big.Rat{r(-expected[i][j], 1), r(1, 1)}, ExactCurvePlaneEventOptions{})
					if e != nil || sign.Sign != 0 {
						t.Fatal("exact section", e)
					}
				}
			}
		}
		// The original cylinder generator is exposed only in Union, with both cap
		// owner masks at its finite endpoints; it is not a fabricated common rim.
		generator, e := s.ClipRationalLine(ctx, [3]*big.Rat{r(2, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}, r(-5, 1), r(5, 1), ToroidLineOptions{ParameterTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		if op == TorusCylinderUnion {
			if len(generator.Intervals) != 1 || generator.Intervals[0].Boundary != TorusCylinderWall || generator.Intervals[0].StartBoundary != 6 || generator.Intervals[0].EndBoundary != 10 {
				t.Fatal(generator)
			}
		} else if len(generator.Intervals) != 0 || len(generator.Points) != 0 {
			t.Fatal("unretained tool support")
		}
		tangent, e := s.ClipRationalLine(ctx, [3]*big.Rat{r(0, 1), r(7, 1), r(0, 1)}, d, r(-1, 1), r(1, 1), ToroidLineOptions{ParameterTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		count := 1
		if op == TorusCylinderCommon {
			count = 0
		}
		if len(tangent.Points) != count || len(tangent.Intervals) != 0 {
			t.Fatal("isolated touch", tangent)
		}
		normals, ne := s.BoundaryNormalsRational(ctx, [3]*big.Rat{r(2, 1), r(0, 1), r(-3, 1)}, SurfacePatchOptions{})
		if op == TorusCylinderUnion {
			if ne != nil || normals.Owners != 6 || normals.Directions[1][0].Cmp(r(2, 1)) != 0 || normals.Directions[2][2].Cmp(r(-1, 1)) != 0 {
				t.Fatal("exposed cap rim normals", normals, ne)
			}
		} else if ne == nil {
			t.Fatal("unretained normal")
		}
		if _, _, e = s.ClassifyRational(ctx, p, SurfacePatchOptions{MaxOperations: 1}); e == nil {
			t.Fatal("work budget")
		}
	}
	if _, e := NewSeparatedTorusCylinderMaterial(5, 2, 3, -3, 4, TorusCylinderUnion); e == nil {
		t.Fatal("contact")
	}
	if _, e := NewSeparatedTorusCylinderMaterial(5, 2, 4, -3, 4, TorusCylinderUnion); e == nil {
		t.Fatal("overlap")
	}
	near, e := NewSeparatedTorusCylinderMaterial(5, 2, math.Nextafter(3, 0), -3, 4, TorusCylinderUnion)
	if e != nil {
		t.Fatal("strict sub-ULP gap", e)
	}
	g, _ := near.RadialClearance()
	if g.Sign() <= 0 {
		t.Fatal("gap")
	}
	s, _ := NewSeparatedTorusCylinderMaterial(5, 2, 2, -3, 4, TorusCylinderUnion)
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for _, sign := range []int64{-1, 1} {
		x := new(big.Rat).Add(r(2, 1), new(big.Rat).Mul(r(sign, 1), eps))
		loc, mask, e := s.ClassifyRational(ctx, [3]*big.Rat{x, r(0, 1), r(0, 1)}, SurfacePatchOptions{})
		want := BoreOutside
		if sign < 0 {
			want = BoreInside
		}
		if e != nil || loc != want || mask != 0 {
			t.Fatal("exact side", loc, mask, e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if line, e := s.ClipRationalLine(canceled, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-8, 1), r(8, 1), ToroidLineOptions{ParameterTolerance: 1e-9}); e == nil || len(line.Intervals) != 0 || len(line.Points) != 0 {
		t.Fatal("cancel")
	}
}
