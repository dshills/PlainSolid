//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"testing"

	"plainsolid/geom"
)

func TestImprintProjectiveSplineFacePairRetainsNativeSeamAndOriginalFragments(t *testing.T) {
	ctx := context.Background()
	a := projectiveTrimTestSurface(t, 1, []float64{-.5, -.5, .5, 1.5})
	knots := []float64{0, 0, 1, 1}
	b, err := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{
		{X: 1}, {}, {X: 1, Y: 1, Z: 1}, {Y: 1},
	}, knots, knots, []float64{1, 1, 2, 2})
	if err != nil {
		t.Fatal(err)
	}
	whole := geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}
	fa, fb := projectiveTrimTestRectangle(a, whole, 10), projectiveTrimTestRectangle(b, whole, 20)
	fb.Reversed = true
	r, err := ImprintProjectiveSplineFacePair(ctx, fa, fb, [2]int{0, 1}, ProjectiveSplineImprintOptions{})
	if err != nil || !r.Complete || r.Section.Model == nil {
		t.Fatal("failed to imprint exact shared section on both original faces", r, err)
	}
	m, err := r.Section.Model.ExactPoint(.5)
	if err != nil || m != (geom.Vec3{X: .5, Y: .5, Z: .25}) {
		t.Fatal("shared original model curve has the wrong independent center", m, err)
	}
	for owner, fragments := range r.Fragments {
		type interval [2]*big.Rat
		ranges := make(map[int][]interval)
		flags := [2]bool{}
		for i, f := range fragments {
			if len(f.Face.Outer) != 4 || len(f.Vertices) != 4 || len(f.Edges) != 4 || f.Face.Reversed != (owner == 1) {
				t.Fatal("each complete owner must be split into two original native rectangles", owner, i, f)
			}
			for k, history := range r.Sources[owner][i] {
				e := f.Edges[k]
				if history.Section {
					u := f.Face.Outer[k]
					if e.Curve != r.Section.Model || history.OriginalEdge != -1 {
						t.Fatal("reconstructed faces must share one exact model seam", owner, i, history)
					}
					flags[i] = u.Reversed
					q, ok := u.Curve.(ExactQuadraticUV)
					if !ok {
						t.Fatal("lost native owner pcurve", u)
					}
					p, _, err := q.EvaluateRational(big.NewRat(1, 2))
					expected := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
					if owner == 1 {
						expected[0] = big.NewRat(1, 3)
					}
					if err != nil || p[0].Cmp(expected[0]) != 0 || p[1].Cmp(expected[1]) != 0 {
						t.Fatal("exact original fractional pcurve station changed", owner, p, err)
					}
					continue
				}
				if history.OriginalEdge != 10+10*owner+history.OriginalUse {
					t.Fatal("lost an original trim edge ID", history)
				}
				ranges[history.OriginalUse] = append(ranges[history.OriginalUse], history.OriginalRange)
			}
		}
		if flags[0] == flags[1] || len(ranges) != 4 {
			t.Fatal("the two native fragments must oppose the shared seam and cover the same four original edges", owner, flags, ranges)
		}
		split := 0
		for _, parts := range ranges {
			sort.Slice(parts, func(i, j int) bool { return parts[i][0].Cmp(parts[j][0]) < 0 })
			if parts[0][0].Sign() != 0 || parts[len(parts)-1][1].Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal("a reconstructed boundary does not cover the complete original use", owner, parts)
			}
			if len(parts) == 2 {
				if parts[0][1].Cmp(big.NewRat(1, 2)) != 0 || parts[1][0].Cmp(big.NewRat(1, 2)) != 0 {
					t.Fatal("wrong original split station", owner, parts)
				}
				split++
			}
		}
		if split != 2 {
			t.Fatal("one section must split two original edges per face", owner, split)
		}
	}
	r, err = ImprintProjectiveSplineFacePair(ctx, fa, fb, [2]int{0, 1}, ProjectiveSplineImprintOptions{MaxEdgeOperations: 1})
	if !errors.Is(err, geom.ErrConvergence) || r.Complete || r.Section.Model != nil || len(r.Fragments[0][0].Edges) != 0 {
		t.Fatal("a partial native imprint must not escape", r, err)
	}
	partial := projectiveTrimTestRectangle(b, geom.ParameterRectangle{UMin: .125, UMax: .875, VMin: 0, VMax: 1}, 20)
	r, err = ImprintProjectiveSplineFacePair(ctx, fa, partial, [2]int{0, 1}, ProjectiveSplineImprintOptions{})
	if !errors.Is(err, geom.ErrConvergence) || r.Complete {
		t.Fatal("a terminal inside the other native face cannot split its complete outer boundary", r, err)
	}
}
