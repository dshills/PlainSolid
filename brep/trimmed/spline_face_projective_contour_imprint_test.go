package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func projectiveContourImprintFace(t *testing.T, split, offset, projective bool) Face {
	t.Helper()
	var poles []geom.Vec3
	var weights []float64
	stations, vknots := []float64{0, 1}, []float64{0, 0, 1, 1}
	if split {
		stations, vknots = []float64{0, .375, 1}, []float64{0, 0, .375, 1, 1}
	}
	for i := range 2 {
		x := float64(i)
		for _, y := range stations {
			z, w := 1+x*y, 1.
			if offset {
				z += x - .5
			}
			if projective {
				w += y
			}
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: z})
			weights = append(weights, w)
		}
	}
	raw, err := geom.NewBSplineSurface(1, 1, 2, len(stations), poles, []float64{0, 0, 1, 1}, vknots, weights)
	if err != nil {
		t.Fatal(err)
	}
	corners := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	f := Face{Surface: raw}
	for i := range corners {
		f.Outer = append(f.Outer, Use{Edge: i + 10, UVStart: corners[i], UVEnd: corners[(i+1)%4]})
	}
	return f
}

func TestImprintProjectiveSplineFaceContourPreservesOriginalKnotUses(t *testing.T) {
	ctx := context.Background()
	a, b := projectiveContourImprintFace(t, true, false, false), projectiveContourImprintFace(t, false, true, false)
	want := geom.Vec3{X: .5, Y: .375, Z: 19.0 / 16}
	for _, axes := range [][2]int{{0, 1}, {1, 0}} {
		r, err := ImprintProjectiveSplineFaceContour(ctx, a, b, axes, ProjectiveSplineImprintOptions{})
		if err != nil || !r.Complete || len(r.Sections) != 2 || r.Section.Model != nil {
			t.Fatal("complete contour must retain both original model pieces", axes, err)
		}
		models := make(map[int]*geom.ExactSplineCurve)
		for i, piece := range r.Sections {
			models[piece.Arc] = piece.Spline.Model
			at := (1 - i)
			if piece.Reversed {
				at = 1 - at
			}
			if p, err := piece.Spline.Model.ExactPoint(float64(at)); err != nil || p != want {
				t.Fatal("source knot owner must join at the exact model point", p, err)
			}
		}
		for owner, fragments := range r.Fragments {
			n := 6
			if owner == 1 {
				n = 5
			}
			var signed [2]int
			for i, f := range fragments {
				if len(f.Face.Outer) != n {
					t.Fatal("original exterior or interior knot piece disappeared", owner, i, f.Face.Outer)
				}
				seen := make(map[int]bool)
				boundaryRepartition := false
				for j, use := range f.Face.Outer {
					s := r.Sources[owner][i][j]
					if s.Section {
						if seen[s.SectionArc] || f.Edges[use.Edge].Curve != models[s.SectionArc] {
							t.Fatal("each wire must use each same shared model object once", owner, i, s)
						}
						seen[s.SectionArc] = true
						sign := 1
						if use.Reversed {
							sign = -1
						}
						signed[s.SectionArc] += sign
					} else {
						for _, at := range s.OriginalRange {
							boundaryRepartition = boundaryRepartition || at.Cmp(big.NewRat(3, 8)) == 0 || at.Cmp(big.NewRat(5, 8)) == 0
						}
					}
				}
				if len(seen) != 2 || boundaryRepartition != (owner == 0) {
					t.Fatal("missing original coedge split or attributed it to wrong chart", owner, i)
				}
			}
			if signed != [2]int{} {
				t.Fatal("sibling section directions must oppose", owner, signed)
			}
		}
		short, err := ImprintProjectiveSplineFaceContour(ctx, a, b, axes,
			ProjectiveSplineImprintOptions{MaxEdgeOperations: r.ExactOperations - 1})
		if !errors.Is(err, geom.ErrConvergence) || short.Complete || len(short.Sections) != 0 {
			t.Fatal("one-short exact contour must not publish a partial split", short, err)
		}
	}
	fractional := projectiveContourImprintFace(t, false, true, true)
	r, err := ImprintProjectiveSplineFaceContour(ctx, a, fractional, [2]int{0, 1}, ProjectiveSplineImprintOptions{})
	if err == nil || r.Complete || len(r.Sections) != 0 || len(r.Fragments[0][0].Edges) != 0 {
		t.Fatal("the source knot at the nonbinary second-owner UV must refuse exact native storage", r, err)
	}
}
