package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestWrappedCylinderUnionComplex(t *testing.T) {
	ctx := context.Background()
	w, _, err := NewWrappedSurfacePatch(ctx, wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	target, _, err := w.InsertKnot(ctx, 0, .125, 1, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	a := [2][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(10, 3)}, {big.NewRat(-1, 1), big.NewRat(2, 1)}}
	for _, reverse := range []bool{false, true} {
		other := target
		b := [2][2]*big.Rat{{big.NewRat(2, 1), big.NewRat(5, 1)}, {big.NewRat(0, 1), big.NewRat(3, 1)}}
		options := ExactSurfaceComparisonOptions{}
		if reverse {
			other, _, err = target.Reparameterize(ctx, ParameterRectangle{UMin: 2, UMax: 10, VMin: -3, VMax: 7}, true, false, SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			options = ExactSurfaceComparisonOptions{NormalizeDomains: true, ReverseBU: true}
			b = [2][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(6, 1)}, {big.NewRat(1, 1), big.NewRat(7, 1)}}
		}
		relation, _, err := CompareWrappedSurfaceCharts(ctx, w, other, options)
		if err != nil {
			t.Fatal(err)
		}
		r, err := relation.CertifyCylinderUnionComplex(ctx, a, b, WrappedCylinderTwoOverlapOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if !r.Complete() {
			t.Fatal("incomplete")
		}
		vertices, edges, faces := r.Vertices(), r.Edges(), r.Faces()
		if len(faces) != 10 || len(vertices)-len(edges)+len(faces) != 0 {
			t.Fatal("wrong annulus", len(vertices), len(edges), len(faces))
		}
		area := new(big.Rat)
		for _, f := range faces {
			area.Add(area, new(big.Rat).Mul(new(big.Rat).Sub(f.Bounds[0][1], f.Bounds[0][0]), new(big.Rat).Sub(f.Bounds[1][1], f.Bounds[1][0])))
		}
		if area.Cmp(big.NewRat(43, 3)) != 0 {
			t.Fatal("incorrect quotient area", area)
		}
		orientation := 1
		if reverse {
			orientation = -1
		}
		if r.OwnerOrientations() != [2]int{1, orientation} {
			t.Fatal("lost source orientation")
		}
		found, seam := false, false
		for _, v := range vertices {
			found = found || v.UV[0].Cmp(big.NewRat(10, 3)) == 0
		}
		if !found {
			t.Fatal("lost nonbinary cut")
		}
		for _, f := range faces {
			for i, u := range f.Uses {
				if u.PeriodOffset.Sign() != 0 {
					seam = true
				}
				for owner := 0; owner < 2; owner++ {
					if f.Owners&(1<<owner) == 0 {
						continue
					}
					support := w
					if owner == 1 {
						support = other
					}
					p := f.OwnerUV[owner][i]
					jet, e := support.EvaluateRational(ctx, p[0], p[1], SurfacePatchEvaluationOptions{})
					if e != nil {
						t.Fatal(e)
					}
					edge := edges[u.Edge]
					index := edge.Start
					if u.Reversed {
						index = edge.End
					}
					for axis := 0; axis < 3; axis++ {
						if jet.Point[axis].Cmp(vertices[index].Point[axis]) != 0 {
							t.Fatal("lost owner model identity")
						}
					}
				}
			}
		}
		if !seam {
			t.Fatal("no exact seam edge reuse")
		}
		for ci, cycle := range r.BoundaryCycles() {
			sum := new(big.Rat)
			first, last := -1, -1
			for _, u := range cycle {
				edge := edges[u.Edge]
				s, e := edge.Start, edge.End
				if u.Reversed {
					s, e = e, s
				}
				if first < 0 {
					first = s
				} else if last != s {
					t.Fatal("broken cycle")
				}
				last = e
				sum.Add(sum, new(big.Rat).Sub(u.UV[1][0], u.UV[0][0]))
			}
			want := big.NewRat(4, 1)
			if ci == 1 {
				want.Neg(want)
			}
			if first != last || sum.Cmp(want) != 0 {
				t.Fatal("wrong winding", sum)
			}
		}
		for i, e := range edges {
			for end := 0; end < 2; end++ {
				jet, er := r.EvaluateEdgeRational(ctx, i, big.NewRat(int64(end), 1), SurfacePatchEvaluationOptions{})
				if er != nil {
					t.Fatal(er)
				}
				id := e.Start
				if end == 1 {
					id = e.End
				}
				for axis := 0; axis < 3; axis++ {
					if jet.Point[axis].Cmp(vertices[id].Point[axis]) != 0 {
						t.Fatal("edge endpoint")
					}
				}
			}
		}
		vertices[0].Point[0].SetInt64(123)
		faces[0].OwnerUV[0][0][0].SetInt64(123)
		faces[0].Uses[0].PeriodOffset.SetInt64(123)
		if r.Vertices()[0].Point[0].Cmp(big.NewRat(123, 1)) == 0 || r.Faces()[0].OwnerUV[0][0][0].Cmp(big.NewRat(123, 1)) == 0 {
			t.Fatal("mutable snapshot")
		}
		limited := WrappedCylinderTwoOverlapOptions{WrappedRectangleOverlapOptions: WrappedRectangleOverlapOptions{MaxOperations: r.ExactOperations() - 1}}
		if q, e := relation.CertifyCylinderUnionComplex(ctx, a, b, limited); e == nil || q != nil {
			t.Fatal("non-atomic budget")
		}
		cancel, stop := context.WithCancel(ctx)
		stop()
		if q, e := relation.CertifyCylinderUnionComplex(cancel, a, b, WrappedCylinderTwoOverlapOptions{}); e == nil || q != nil {
			t.Fatal("non-atomic cancel")
		}
		forged := *r
		forged.faces = r.Faces()
		forged.faces[0].Uses[0].PeriodOffset.SetInt64(123)
		if _, e := validateCylinderQuotientIncidence(ctx, &forged, big.NewRat(4, 1), &polynomialBudget{maxOperations: 1000000, maxBits: 16384}); e == nil {
			t.Fatal("accepted false seam lift")
		}
		forged.faces = append(r.Faces(), r.Faces()[0])
		if _, e := validateCylinderQuotientIncidence(ctx, &forged, big.NewRat(4, 1), &polynomialBudget{maxOperations: 1000000, maxBits: 16384}); e == nil {
			t.Fatal("accepted third use")
		}
	}
}
