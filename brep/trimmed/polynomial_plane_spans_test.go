package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func creasedPolynomialClipStock(t *testing.T) Solid {
	t.Helper()
	original, surfaces := polynomialClipStock(t)
	var patched []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	for id, s := range surfaces {
		var e error
		if id >= 2 {
			s, e = s.InsertUKnot(.5, 4)
			if e != nil {
				t.Fatal(e)
			}
		}
		d := s.Data()
		for i := range d.Poles {
			d.Poles[i].Z += math.Abs(d.Poles[i].X-.5) / 32
		}
		s, e = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		patched = append(patched, s)
		domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
		reversed = append(reversed, original.faces[id].Reversed)
	}
	result, _, e := SewSplineRectangles(context.Background(), patched, domains, reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return result
}
func TestPiecewisePolynomialPlaneFaceSection(t *testing.T) {
	ctx := context.Background()
	s := creasedPolynomialClipStock(t)
	for _, face := range []int{2, 4} {
		proof, e := CertifyPiecewisePolynomialPlaneFaceSection(ctx, s, face, 3./16, 0, PolynomialPlaneSpanOptions{})
		if e != nil {
			t.Fatal(face, e)
		}
		if len(proof.Pieces()) != 2 || len(proof.Knots()) != 1 {
			t.Fatal("piece count")
		}
		k := proof.Knots()[0]
		if k.Face != face || k.Pieces != [2]int{0, 1} || k.Knot.Cmp(ratInset(.5)) != 0 || k.Point[2].Cmp(ratInset(3./16)) != 0 {
			t.Fatal("knot ancestry")
		}
		// Original source actually has a nonzero derivative jump, not just a
		// removable knot partition of the previous polynomial.
		a, e := proof.patch.Trim(proof.domains[0])
		if e != nil {
			t.Fatal(e)
		}
		b, e := proof.patch.Trim(proof.domains[1])
		if e != nil {
			t.Fatal(e)
		}
		l, e := a.EvaluateRational(ctx, k.UV[0], k.UV[1], geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		r, e := b.EvaluateRational(ctx, k.UV[0], k.UV[1], geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if new(big.Rat).Sub(r.Du[2], l.Du[2]).Cmp(big.NewRat(1, 16)) != 0 {
			t.Fatal("wrong independent derivative jump", l.Du[2], r.Du[2])
		}
		k.Point[0].SetInt64(99)
		if proof.Knots()[0].Point[0].Cmp(ratInset(.5)) != 0 {
			t.Fatal("mutable evidence")
		}
		if _, e = CertifyPiecewisePolynomialPlaneFaceSection(ctx, s, face, 3./16, 0, PolynomialPlaneSpanOptions{MaxSpans: 1}); e == nil {
			t.Fatal("span budget")
		}
	}
	if r, e := s.ValidateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{}); e != nil || !r.Complete {
		t.Fatal("whole original embedding", r, e)
	}
}
