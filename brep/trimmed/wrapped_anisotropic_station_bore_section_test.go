package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestAnisotropicBoreAnnularSections(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s := anisotropicMeasuredBore(t)
	r := big.NewRat
	cuts := []float64{-2, -1, 1, 4, 8}
	for _, h := range []*big.Rat{r(-4, 1), r(-11, 3), r(-3, 1), r(-2, 1), r(-1, 1)} {
		q, e := s.SectionWrappedAnisotropicStationBoreRational(ctx, h, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(h, e)
		}
		if !q.InnerReversed {
			t.Fatal("hole orientation")
		}
		for side, loop := range []WrappedAnisotropicStationSection{q.Outer, q.Inner} {
			for j := 0; j < 4; j++ {
				piece, e := loop.Profile.Curve.Restrict(cuts[j], cuts[j+1], geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				net, _, e := piece.BezierControlNet(ctx, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for _, p := range net.Control {
					if p[2].Cmp(new(big.Rat).Mul(h, p[3])) != 0 {
						t.Fatal("height")
					}
				}
				if len(loop.Rim) > 0 {
					id := loop.Rim[j]
					edge := s.edges[id]
					if edge.Range.Min != cuts[j] || edge.Range.Max != cuts[j+1] {
						t.Fatal("original parameter range")
					}
					original, reversed, e := exactEdgeSplineWithOptions(edge, geom.SurfacePatchOptions{})
					if e != nil {
						t.Fatal(e)
					}
					equal, e := geom.CompareExactCurvesContext(ctx, piece, original, geom.ExactCurveComparisonOptions{NormalizeDomains: true, ReverseB: reversed})
					if e != nil || !equal.Complete || !equal.Equal {
						t.Fatal("exact original rim", side, h, e)
					}
				}
			}
			for _, id := range loop.Faces {
				if _, ok := s.faces[id].Surface.(*geom.WrappedSurfacePatch); ok && s.faces[id].Reversed != (side == 1) {
					t.Fatal("source side")
				}
			}
		}
		if h.Cmp(r(-3, 1)) == 0 {
			if len(q.Outer.Rim) != 4 || len(q.Outer.Faces) != 2 || len(q.Inner.Rim) != 0 || len(q.Inner.Faces) != 1 {
				t.Fatal("outer-only crease", q)
			}
		}
		if h.Cmp(r(-2, 1)) == 0 {
			if len(q.Inner.Rim) != 4 || len(q.Inner.Faces) != 2 || len(q.Outer.Rim) != 0 || len(q.Outer.Faces) != 1 {
				t.Fatal("inner-only crease", q)
			}
		}
		if h.Cmp(r(-4, 1)) == 0 || h.Cmp(r(-1, 1)) == 0 {
			if q.Outer.Cap < 0 || q.Outer.Cap != q.Inner.Cap || len(q.Outer.Rim) != 4 || len(q.Inner.Rim) != 4 || len(s.faces[q.Outer.Cap].Holes) != 1 {
				t.Fatal("actual annular cap", q)
			}
		}
		if h.Cmp(r(-11, 3)) == 0 {
			if q.Outer.Profile.ParameterV.Cmp(q.Inner.Profile.ParameterV) == 0 {
				t.Fatal("independent original V parameters lost")
			}
			if q.Outer.Profile.Scale[0].Cmp(r(7, 3)) != 0 || q.Inner.Profile.Scale[0].Cmp(r(11, 24)) != 0 {
				t.Fatal("exact distinct scale interpolation")
			}
		}
	}
	for _, h := range []*big.Rat{nil, r(-5, 1), r(0, 1)} {
		q, e := s.SectionWrappedAnisotropicStationBoreRational(ctx, h, geom.SurfacePatchOptions{})
		if e == nil || q.Outer.Profile.Curve != nil || q.InnerReversed {
			t.Fatal("atomic domain", e)
		}
	}
	q, e := s.SectionWrappedAnisotropicStationBoreRational(ctx, r(-3, 1), geom.SurfacePatchOptions{MaxOperations: 1})
	if e == nil || q.Outer.Profile.Curve != nil || q.Inner.Profile.Curve != nil {
		t.Fatal("atomic source budget", e)
	}
}
