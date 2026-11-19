//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBuildProjectiveSplineRoofAdjacentStepFuse(t *testing.T) {
	ctx := context.Background()
	a := projectiveAdjacentRoof(t, 0, 1, 1, 0, 1)
	b := projectiveAdjacentRoof(t, 1, 3, 1, .25, 2)
	for lower, pair := range [][2]Face{{a, b}, {b, a}} {
		for _, axes := range [][2]int{{0, 1}, {1, 0}} {
			r, err := BuildProjectiveSplineRoofAdjacentStepFuse(ctx, pair[0], pair[1], axes, 0, ProjectiveRoofSolidOptions{})
			if err != nil || !r.Complete || r.Empty || len(r.Walls) != 6 || len(r.Steps) != 1 || len(r.SectionEdges) != 2 {
				t.Fatal("only the positive gap between two complete neighboring rims should become a step", axes, lower, err)
			}
			s, step := r.Solid, r.Steps[0]
			if len(s.faces) != 10 || len(s.edges) != 22 || len(s.vertices) != 14 ||
				step.Owners != [2]int{lower, 1 - lower} || step.Edges != [2]int(r.SectionEdges) {
				t.Fatal("wrong closed step topology or original owner ledger", axes, lower, len(s.faces), len(s.edges), len(s.vertices), step)
			}
			for i, e := range step.Edges {
				edge := s.edges[e]
				for _, v := range [2]int{edge.Start, edge.End} {
					p := s.vertices[v]
					if p.X != 1 || p.Z != 1+p.Y+float64(i)/4 {
						t.Fatal("new wall must end exactly on both original rims", i, p)
					}
				}
				owner := step.Owners[i]
				if roof := s.faces[r.Roofs[owner]]; roof.Surface != pair[owner].Surface || roof.Outer[step.Uses[i].OriginalUse].Edge != e {
					t.Fatal("new wall must retain both original uses", owner, step.Uses[i], e)
				}
			}
			ha := 0
			for _, exterior := range r.Walls {
				wall := s.faces[exterior.Face]
				if len(wall.Outer) == 5 && exterior.Owner == 1-lower {
					ha++
				} else if len(wall.Outer) != 4 {
					t.Fatal("only the two tall neighboring exterior walls require one additional corner", exterior)
				}
			}
			if ha != 2 {
				t.Fatal("the high exterior walls must each retain the original lower step corner", ha)
			}
			m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil || math.Abs(m.Volume-23.0/4) > 1e-10 || math.Abs(m.Centroid.X-40.0/23) > 1e-10 || math.Abs(m.Centroid.Y-13.0/23) > 1e-10 {
				t.Fatal("independent physical integrals disagree", axes, lower, m, err)
			}
			var first, replay bytes.Buffer
			if err := WriteJSON(&first, s); err != nil {
				t.Fatal("write native step", err)
			}
			q, err := ReadJSONContext(ctx, bytes.NewReader(first.Bytes()), JSONLimits{})
			if err != nil {
				t.Fatal("reopen native step", err)
			}
			if err := WriteJSON(&replay, q); err != nil || !bytes.Equal(first.Bytes(), replay.Bytes()) {
				t.Fatal("native step replays byte-for-byte", err)
			}
		}
	}
	if r, err := BuildProjectiveSplineRoofBoolean(ctx, a, b, [2]int{0, 1}, 0, parametric.Union, ProjectiveRoofSolidOptions{}); err != nil || len(r.Steps) != 1 || !r.Complete {
		t.Fatal("automatic union must use the original stepped-seam proof", err)
	}
	for _, f := range []Face{
		projectiveAdjacentRoof(t, 1, 3, .5, .25, 2),
		projectiveAdjacentRoof(t, 1, 3, 1, 0, 2),
		projectiveAdjacentRoof(t, 2, 3, 1, .25, 2),
	} {
		if r, err := BuildProjectiveSplineRoofAdjacentStepFuse(ctx, a, f, [2]int{0, 1}, 0, ProjectiveRoofSolidOptions{}); !errors.Is(err, geom.ErrConvergence) || r.Complete || len(r.Solid.faces) != 0 {
			t.Fatal("partial, coincident or disjoint rims must not publish a positive step", err)
		}
	}
	o := ProjectiveRoofSolidOptions{Sew: ExactSewOptions{MaxFaces: 9}}
	if r, err := BuildProjectiveSplineRoofAdjacentStepFuse(ctx, a, b, [2]int{0, 1}, 0, o); err == nil || r.Complete || len(r.Solid.faces) != 0 {
		t.Fatal("bounded sewing must publish no partial step or owners", err)
	}
}

func TestBuildProjectiveSplineRoofAdjacentStepFuseCurvedRims(t *testing.T) {
	ctx := context.Background()
	chart := func(left, right, ratio, lowerShift, upperShift float64) Face {
		t.Helper()
		f := projectiveFootprintRoof(t, left, right, 0, 0)
		var poles []geom.Vec3
		for _, x := range [2]float64{left, right} {
			poles = append(poles, geom.Vec3{X: x, Z: 1 + lowerShift},
				geom.Vec3{X: x, Y: .5, Z: 1 + (x+lowerShift+upperShift)/2},
				geom.Vec3{X: x, Y: 1, Z: 2 + x + upperShift})
		}
		p, err := geom.NewBSplineSurface(1, 2, 2, 3, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1},
			[]float64{1, ratio, ratio * ratio, 1, ratio, ratio * ratio})
		if err != nil {
			t.Fatal(err)
		}
		f.Surface = p
		return f
	}
	a, b := chart(0, 1, 1, 0, 0), chart(1, 3, 2, .25, .25)
	r, err := BuildProjectiveSplineRoofAdjacentStepFuse(ctx, a, b, [2]int{1, 0}, 0, ProjectiveRoofSolidOptions{})
	if err != nil || !r.Complete || len(r.Steps) != 1 {
		t.Fatal("step must retain both original parabolic rims", err)
	}
	for i, edge := range r.Steps[0].Edges {
		e := r.Solid.edges[edge]
		s, _ := e.Curve.(*geom.ExactSplineCurve)
		if s == nil {
			t.Fatal("a linear substitute cannot represent the original parabolic rim", i)
		}
		jet, err := s.EvaluateRational(ctx, big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		// The independent clocks have Y = 1/2 and Y = 2/3 at t=1/2.
		y := big.NewRat(int64(1+i), int64(2+i))
		z := new(big.Rat).Mul(y, y)
		z.Add(z, y).Add(z, big.NewRat(int64(4+i), 4))
		if jet.Point[0].Cmp(big.NewRat(1, 1)) != 0 || jet.Point[1].Cmp(y) != 0 || jet.Point[2].Cmp(z) != 0 {
			t.Fatal("rim left its original parabola or changed its original clock", i, jet.Point)
		}
	}
	m, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil || math.Abs(m.Volume-27.0/4) > 1e-10 || math.Abs(m.Centroid.X-46.0/27) > 1e-10 || math.Abs(m.Centroid.Y-16.0/27) > 1e-10 {
		t.Fatal("step over an original parabolic rim must preserve exact physical material", m, err)
	}
	crossing := chart(1, 3, 2, -.5, .5)
	if r, err := BuildProjectiveSplineRoofBoolean(ctx, a, crossing, [2]int{0, 1}, 0, parametric.Union, ProjectiveRoofSolidOptions{}); !errors.Is(err, geom.ErrConvergence) || r.Complete {
		t.Fatal("a real step crossing must remain unresolved", err)
	}
}
