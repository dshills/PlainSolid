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

func TestBuildProjectiveSplineRoofContourSolidRetainsSweptOriginalKnot(t *testing.T) {
	ctx := context.Background()
	a, b := projectiveContourImprintFace(t, true, false, false), projectiveContourImprintFace(t, false, true, false)
	for _, axes := range [][2]int{{0, 1}, {1, 0}} {
		for _, tc := range []struct {
			op           parametric.BooleanOp
			volume, x, y float64
		}{
			{parametric.Intersection, 9.0 / 8, 31.0 / 54, 29.0 / 54},
			{parametric.Union, 11.0 / 8, 37.0 / 66, 35.0 / 66},
		} {
			q, err := BuildProjectiveSplineRoofContourSolid(ctx, a, b, axes, 0, tc.op, ProjectiveRoofSolidOptions{})
			if err != nil || !q.Complete || len(q.SectionEdges) != 2 || q.SectionEdges[0] == q.SectionEdges[1] {
				t.Fatal("both original seam pieces must form one closed material roof", axes, tc.op, err)
			}
			s := q.Solid
			if len(s.faces) != 10 || len(s.edges) != 23 || len(s.vertices) != 15 || len(q.Walls) != 7 || q.BaseFace != 9 {
				t.Fatal("one original knot must also subdivide the corresponding side wall and base wire", axes, tc.op, len(s.faces), len(s.edges), len(s.vertices))
			}
			want := geom.Vec3{X: .5, Y: .375, Z: 19.0 / 16}
			for _, eid := range q.SectionEdges {
				e := s.edges[eid]
				if s.vertices[e.Start] != want && s.vertices[e.End] != want {
					t.Fatal("each exact section edge must meet the original knot identity", eid, e)
				}
				for _, roof := range q.Roofs {
					n := 0
					for _, use := range s.faces[roof].Outer {
						if use.Edge == eid {
							n++
						}
					}
					if n != 1 {
						t.Fatal("both original roofs retain one use of every section edge", roof, eid, n)
					}
				}
			}
			splitWalls := 0
			for _, wall := range q.Walls {
				for _, station := range wall.Source.OriginalRange {
					if wall.Owner == 0 && (station.Cmp(big.NewRat(3, 8)) == 0 || station.Cmp(big.NewRat(5, 8)) == 0) {
						splitWalls++
					}
				}
			}
			if splitWalls != 2 {
				t.Fatal("two walls must retain the original split side's parameter interval", splitWalls)
			}
			p, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil || math.Abs(p.Volume-tc.volume) > 1e-10 || math.Abs(p.Centroid.X-tc.x) > 1e-10 || math.Abs(p.Centroid.Y-tc.y) > 1e-10 {
				t.Fatal("native subdivision must preserve independent volume and centroid", axes, tc.op, p, err)
			}
			var data, replay bytes.Buffer
			if err := WriteJSON(&data, s); err != nil {
				t.Fatal("write original roof with both sections", err)
			}
			reopened, err := ReadJSONContext(ctx, bytes.NewReader(data.Bytes()), JSONLimits{})
			if err != nil {
				t.Fatal("reopen original roof with both sections", err)
			}
			if err = WriteJSON(&replay, reopened); err != nil || !bytes.Equal(data.Bytes(), replay.Bytes()) {
				t.Fatal("both native section curves must persist byte-identically", err)
			}
		}
	}
	o := ProjectiveRoofSolidOptions{Sew: ExactSewOptions{MaxComparisons: 1}}
	r, err := BuildProjectiveSplineRoofContourSolid(ctx, a, b, [2]int{0, 1}, 0, parametric.Union, o)
	if !errors.Is(err, geom.ErrConvergence) || r.Complete || len(r.SectionEdges) != 0 {
		t.Fatal("an unpaired seam cannot publish a closed multispan roof", r, err)
	}
}

func TestBuildProjectiveSplineRoofContourDifferenceRetainsBothOriginalKnotWires(t *testing.T) {
	ctx := context.Background()
	a, b := projectiveContourImprintFace(t, true, true, false), projectiveContourImprintFace(t, true, false, false)
	for _, axes := range [][2]int{{0, 1}, {1, 0}} {
		for i, pair := range [][2]Face{{a, b}, {b, a}} {
			q, err := BuildProjectiveSplineRoofContourDifference(ctx, pair[0], pair[1], axes, 0, ProjectiveRoofSolidOptions{})
			if err != nil || !q.Complete || q.BaseFace != -1 || len(q.SectionEdges) != 2 || len(q.Walls) != 4 || len(q.FloorSources) != 4 {
				t.Fatal("paired original exterior knots and both section edges must close the result", axes, i, err)
			}
			s := q.Solid
			if len(s.faces) != 6 || len(s.edges) != 13 || len(s.vertices) != 9 || !s.faces[1].Reversed {
				t.Fatal("original floor orientation and complete paired wall topology", axes, i)
			}
			qx := 5.0 / 6
			if i == 1 {
				qx = 1.0 / 6
			}
			props, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil || math.Abs(props.Volume-1.0/8) > 1e-10 || math.Abs(props.Centroid.X-qx) > 1e-10 {
				t.Fatal("joint subdivision must preserve independent signed material", axes, i, props, err)
			}
		}
	}
	unmatched := projectiveContourImprintFace(t, false, false, false)
	q, err := BuildProjectiveSplineRoofContourDifference(ctx, a, unmatched, [2]int{0, 1}, 0, ProjectiveRoofSolidOptions{})
	if !errors.Is(err, geom.ErrConvergence) || q.Complete || len(q.SectionEdges) != 0 {
		t.Fatal("unmatched exterior partitions cannot publish a torn wall", q, err)
	}
}
