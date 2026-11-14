package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestEncompassingTorusCylinderMaterial(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, op := range []TorusCylinderOperation{TorusCylinderCommon, TorusCylinderDifference, TorusCylinderUnion} {
		s, e := NewEncompassingTorusCylinderMaterial(5, 2, 8, -3, 4, op)
		if e != nil {
			t.Fatal(e)
		}
		gaps, _ := s.Clearances()
		for i, v := range []int64{1, 1, 2} {
			if gaps[i].Cmp(r(v, 1)) != 0 {
				t.Fatal(gaps)
			}
		}
		gaps[0].SetInt64(99)
		again, _ := s.Clearances()
		if again[0].Cmp(r(1, 1)) != 0 {
			t.Fatal("alias")
		}
		p := [3]*big.Rat{r(7, 1), r(0, 1), r(0, 1)}
		loc, mask, e := s.ClassifyRational(ctx, p, SurfacePatchOptions{})
		want, owner := BoreBoundary, TorusCylinderTorus
		if op == TorusCylinderDifference {
			want, owner = BoreOutside, 0
		}
		if op == TorusCylinderUnion {
			want, owner = BoreInside, 0
		}
		if e != nil || loc != want || mask != owner {
			t.Fatal(op, loc, mask, e)
		}
		normals, ne := s.BoundaryNormalsRational(ctx, p, SurfacePatchOptions{})
		if op == TorusCylinderCommon {
			if ne != nil || normals.Owners != TorusCylinderTorus || normals.Directions[0][0].Sign() <= 0 {
				t.Fatal("normal", ne)
			}
		} else if ne == nil {
			t.Fatal("buried normal")
		}
		line, e := s.ClipRationalLine(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-9, 1), r(9, 1), ToroidLineOptions{ParameterTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		ends := [][2]int64{{-7, -3}, {3, 7}}
		owner = TorusCylinderTorus
		if op == TorusCylinderDifference {
			ends = nil
		}
		if op == TorusCylinderUnion {
			ends = [][2]int64{{-8, 8}}
			owner = TorusCylinderWall
		}
		if len(line.Intervals) != len(ends) || len(line.Points) != 0 {
			t.Fatal(line)
		}
		for i, v := range line.Intervals {
			if v.StartBoundary != owner || v.EndBoundary != owner {
				t.Fatal("owner")
			}
			for j, root := range []*ExactAlgebraicRoot{v.Start, v.End} {
				sign, e := root.SignPolynomial(ctx, []*big.Rat{r(-ends[i][j], 1), r(1, 1)}, ExactCurvePlaneEventOptions{})
				if e != nil || sign.Sign != 0 {
					t.Fatal("root", e)
				}
			}
		}
		if _, _, e = s.ClassifyRational(ctx, p, SurfacePatchOptions{MaxOperations: 1}); e == nil {
			t.Fatal("budget")
		}
	}
	for _, args := range [][3]float64{{7, -3, 4}, {8, -2, 4}, {8, -3, 2}, {6, -3, 4}} {
		if _, e := NewEncompassingTorusCylinderMaterial(5, 2, args[0], args[1], args[2], TorusCylinderUnion); e == nil {
			t.Fatal("contact", args)
		}
	}
	if _, e := NewEncompassingTorusCylinderMaterial(5, 2, math.Nextafter(7, math.Inf(1)), -3, 4, TorusCylinderUnion); e != nil {
		t.Fatal("strict gap", e)
	}
}
