//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestTraceProjectiveSplineFaceContactsClipsOriginalRectangles(t *testing.T) {
	ctx := context.Background()
	aSource := projectiveTrimTestSurface(t, 1, []float64{0, 0, 0, 0})
	bSource := projectiveTrimTestSurface(t, 1, []float64{-.375, -.375, .625, .625})
	a := projectiveTrimTestRectangle(aSource, geom.ParameterRectangle{UMin: .25, UMax: .75, VMin: .125, VMax: .625}, 21)
	b := projectiveTrimTestRectangle(bSource, geom.ParameterRectangle{UMin: .125, UMax: .75, VMin: .25, VMax: .875}, 31)
	a.Outer[2].Reversed = true
	// Rotate the actual owner order. Boundary IDs must not be reassigned.
	b.Outer = append(b.Outer[1:], b.Outer[0])
	axes := [2]int{0, 1}
	r, err := TraceProjectiveSplineFaceContacts(ctx, a, b, axes, ProjectiveSplineFaceContactOptions{})
	if err != nil || !r.Complete || len(r.Graph.Contours) != 1 || len(r.Vertices) != 2 || r.Graph.Contours[0].Closed {
		t.Fatal("trimmed faces should have exactly one open original trace", r, err)
	}
	count := [2]int{}
	for _, v := range r.Vertices {
		point := r.Graph.Vertices[v.Vertex]
		for i, owner := range v.Uses {
			testProjectiveTrimRoot(t, point.OriginalUV[i][0], big.NewRat(3, 8))
			if len(owner) == 0 {
				continue
			}
			if len(owner) != 1 || len(v.Uses[1-i]) != 0 {
				t.Fatal("each clipped end is owned by exactly one original wire", v)
			}
			got := owner[0]
			count[i]++
			if i == 0 {
				if got.Use != 2 || got.Edge != 23 {
					t.Fatal("wrong original reversed upper use", got)
				}
				testProjectiveTrimRoot(t, point.OriginalUV[0][1], big.NewRat(5, 8))
				testProjectiveTrimRoot(t, got.UseParameter, big.NewRat(3, 4))
				testProjectiveTrimRoot(t, got.EdgeParameter, big.NewRat(1, 4))
			} else {
				if got.Use != 3 || got.Edge != 31 {
					t.Fatal("rotated wire storage lost original lower use", got)
				}
				testProjectiveTrimRoot(t, point.OriginalUV[1][1], big.NewRat(1, 4))
				testProjectiveTrimRoot(t, got.UseParameter, big.NewRat(2, 5))
				testProjectiveTrimRoot(t, got.EdgeParameter, big.NewRat(2, 5))
			}
		}
	}
	if count != [2]int{1, 1} {
		t.Fatal("overlap must clip at independently selected original sides", count)
	}
	for _, o := range []ProjectiveSplineFaceContactOptions{{MaxPreparationOperations: 1}, {MaxBoundaryOperations: 1}} {
		r, err := TraceProjectiveSplineFaceContacts(ctx, a, b, axes, o)
		if !errors.Is(err, geom.ErrConvergence) || r.Complete || len(r.Vertices)+len(r.Graph.Vertices) != 0 {
			t.Fatal("trim work exhaustion cannot claim a partially attached graph", r, err)
		}
	}
	a.Holes = [][]Use{append([]Use(nil), a.Outer...)}
	r, err = TraceProjectiveSplineFaceContacts(ctx, a, b, axes, ProjectiveSplineFaceContactOptions{})
	if !errors.Is(err, geom.ErrConvergence) || r.Complete {
		t.Fatal("a wire with a hole cannot be silently reduced to its rectangle", r, err)
	}
}

func TestTraceProjectiveSplineFaceContactsRetainsBothUsesAtAnOriginalCorner(t *testing.T) {
	a := projectiveTrimTestSurface(t, 1, []float64{0, 0, 0, 0})
	var heights []float64
	for _, u := range []float64{.25, -.25, .25} {
		for _, v := range []float64{.25, -.25, .25} {
			heights = append(heights, v-u)
		}
	}
	b := projectiveTrimTestSurface(t, 2, heights)
	faceA := projectiveTrimTestRectangle(a, geom.ParameterRectangle{UMin: .25, UMax: .75, VMin: .25, VMax: .75}, 41)
	faceB := projectiveTrimTestRectangle(b, geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}, 51)
	r, err := TraceProjectiveSplineFaceContacts(context.Background(), faceA, faceB, [2]int{0, 1}, ProjectiveSplineFaceContactOptions{
		Graph: geom.ProjectiveSurfaceContactGraphOptions{Cover: geom.ProjectiveSurfaceContactCoverOptions{IsolateQuadraticSaddles: true}},
	})
	if err != nil || !r.Complete || len(r.Graph.SaddleVertices) != 1 || len(r.Graph.Contours) != 4 {
		t.Fatal("one analytic crossing must preserve the four original trim corners", r, err)
	}
	seen := make(map[[2]int]bool)
	for _, point := range r.Vertices {
		if point.Vertex == r.Graph.SaddleVertices[0] {
			if len(point.Uses[0])+len(point.Uses[1]) != 0 {
				t.Fatal("interior saddle must not acquire an original edge use", point)
			}
			continue
		}
		if len(point.Uses[0]) != 2 || len(point.Uses[1]) != 0 {
			t.Fatal("each terminal has the two original incident A coedges", point)
		}
		x, y := point.Uses[0][0], point.Uses[0][1]
		key := [2]int{x.Edge, y.Edge}
		if seen[key] {
			t.Fatal("duplicated corner", point)
		}
		seen[key] = true
		want := [2]*big.Rat{big.NewRat(1, 1), new(big.Rat)}
		if key == [2]int{41, 44} {
			want[0], want[1] = want[1], want[0]
		}
		testProjectiveTrimRoot(t, x.UseParameter, want[0])
		testProjectiveTrimRoot(t, y.UseParameter, want[1])
	}
	if len(seen) != 4 {
		t.Fatal("missing original corner", seen)
	}
}

func projectiveTrimTestSurface(t *testing.T, degree int, heights []float64) *geom.BSplineSurface {
	t.Helper()
	count := degree + 1
	knots := make([]float64, 2*count)
	for i := count; i < len(knots); i++ {
		knots[i] = 1
	}
	points := make([]geom.Vec3, len(heights))
	for i, z := range heights {
		points[i] = geom.Vec3{X: float64(i/count) / float64(degree), Y: float64(i%count) / float64(degree), Z: z}
	}
	s, err := geom.NewBSplineSurface(degree, degree, count, count, points, knots, knots, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func projectiveTrimTestRectangle(s geom.Surface, d geom.ParameterRectangle, edge int) Face {
	pts := [4]geom.Vec2{{X: d.UMin, Y: d.VMin}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}, {X: d.UMin, Y: d.VMax}}
	uses := make([]Use, 4)
	for i := range uses {
		uses[i] = Use{Edge: edge + i, UVStart: pts[i], UVEnd: pts[(i+1)%4]}
	}
	return Face{Surface: s, Outer: uses}
}

func testProjectiveTrimRoot(t *testing.T, root *geom.ExactAlgebraicRoot, want *big.Rat) {
	t.Helper()
	sign, err := root.SignPolynomial(context.Background(), []*big.Rat{new(big.Rat).Neg(want), big.NewRat(1, 1)}, geom.ExactCurvePlaneEventOptions{})
	if err != nil || sign.Sign != 0 {
		t.Fatal("wrong original trim root", root.Interval(), want, sign, err)
	}
}

func TestTraceProjectiveSplineFaceContactsClipsAtOriginalInnerUses(t *testing.T) {
	ctx := context.Background()
	s0 := projectiveTrimTestSurface(t, 1, []float64{0, 0, 0, 0})
	s1 := projectiveTrimTestSurface(t, 1, []float64{-.375, -.375, .625, .625})
	outer := geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}
	a, b := projectiveTrimTestRectangle(s0, outer, 21), projectiveTrimTestRectangle(s1, outer, 31)
	hole := func(s geom.Surface, d geom.ParameterRectangle, edge int) []Use {
		ccw := projectiveTrimTestRectangle(s, d, edge).Outer
		cw := make([]Use, 4)
		for i := range cw {
			u := ccw[3-i]
			u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
			cw[i] = u
		}
		return cw
	}
	a.Holes = [][]Use{
		hole(s0, geom.ParameterRectangle{UMin: .25, UMax: .5, VMin: .25, VMax: .375}, 41),
		// This second hole has collinear edge extensions but is spatially
		// separate: no terminal at U=3/8 is on either original side.
		hole(s0, geom.ParameterRectangle{UMin: .625, UMax: .75, VMin: .25, VMax: .375}, 51),
	}
	b.Holes = [][]Use{hole(s1, geom.ParameterRectangle{UMin: .25, UMax: .5, VMin: .625, VMax: .75}, 61)}
	result, err := TraceProjectiveSplineFaceContacts(ctx, a, b, [2]int{0, 1}, ProjectiveSplineFaceContactOptions{})
	if err != nil || !result.Complete || len(result.Graph.Contours) != 3 || len(result.Vertices) != 6 {
		t.Fatal("the original holes must cut the contact into three open sections", result, err)
	}
	seen := make(map[int]bool)
	for _, v := range result.Vertices {
		for owner, source := range v.Uses {
			if result.Graph.Vertices[v.Vertex].OnOverlapBoundary {
				if len(source) != 1 || source[0].Loop != 0 {
					t.Fatal("external terminal does not belong to the original outer side", v)
				}
				continue
			}
			if len(source) == 0 {
				continue
			}
			if len(source) != 1 || len(v.Uses[1-owner]) != 0 || source[0].Loop != 1 {
				t.Fatal("hole terminal lost its sole original coedge", v)
			}
			q := source[0]
			if q.Edge != 41+20*owner && q.Edge != 43+20*owner || seen[q.Edge] {
				t.Fatal("manufactured an extra binding on a hole edge extension", q)
			}
			seen[q.Edge] = true
			testProjectiveTrimRoot(t, q.UseParameter, big.NewRat(1, 2))
		}
	}
	if len(seen) != 4 {
		t.Fatal("lost an original hole terminal", seen)
	}
	b.Holes[0] = projectiveTrimTestRectangle(s1, geom.ParameterRectangle{UMin: .25, UMax: .5, VMin: .625, VMax: .75}, 61).Outer
	r, err := TraceProjectiveSplineFaceContacts(ctx, a, b, [2]int{0, 1}, ProjectiveSplineFaceContactOptions{})
	if !errors.Is(err, geom.ErrConvergence) || r.Complete {
		t.Fatal("an original hole needs the clockwise source winding", r, err)
	}
}
