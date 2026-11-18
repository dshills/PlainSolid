//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func projectiveAdjacentRoof(t *testing.T, left, right, upperY, offset, weight float64) Face {
	t.Helper()
	f := projectiveFootprintRoof(t, left, right, offset, 0)
	var poles []geom.Vec3
	var weights []float64
	for _, x := range []float64{left, right} {
		for v, y := range []float64{0, upperY} {
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: 1 + x*y + offset})
			weights = append(weights, 1+float64(v)*(weight-1))
		}
	}
	k := []float64{0, 0, 1, 1}
	raw, err := geom.NewBSplineSurface(1, 1, 2, 2, poles, k, k, weights)
	if err != nil {
		t.Fatal(err)
	}
	f.Surface = raw
	return f
}

func TestBuildProjectiveSplineRoofAdjacentFuseKeepsExactSharedWall(t *testing.T) {
	ctx := context.Background()
	a := projectiveAdjacentRoof(t, 0, 1, 1, 0, 1)
	b := projectiveAdjacentRoof(t, 1, 3, 1, 0, 2)
	for order, pair := range [][2]Face{{a, b}, {b, a}} {
		for _, axes := range [][2]int{{0, 1}, {1, 0}} {
			r, err := BuildProjectiveSplineRoofAdjacentFuse(ctx, pair[0], pair[1], axes, 0, ProjectiveRoofSolidOptions{})
			if err != nil || !r.Complete || r.Empty || len(r.Walls) != 6 || len(r.SectionEdges) != 1 || r.BaseFace != 8 {
				t.Fatal("both independent original clocks must fuse along their complete common wall", axes, order, err)
			}
			s := r.Solid
			if len(s.faces) != 9 || len(s.edges) != 19 || len(s.vertices) != 12 {
				t.Fatal("closed adjacent Fuse must cancel exactly the common interior wall", axes, order, len(s.faces))
			}
			e := s.edges[r.Section]
			for _, i := range [2]int{e.Start, e.End} {
				p := s.vertices[i]
				if p.X != 1 || p.Z != 1+p.Y {
					t.Fatal("shared original section geometry changed", axes, order, p)
				}
			}
			for owner, id := range r.Roofs {
				if s.faces[id].Surface != pair[owner].Surface {
					t.Fatal("original support was replaced", axes, order, owner)
				}
			}
			m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil || math.Abs(m.Volume-21.0/4) > 1e-10 || math.Abs(m.Centroid.X-12.0/7) > 1e-10 ||
				math.Abs(m.Centroid.Y-4.0/7) > 1e-10 {
				t.Fatal("independent original union integrals differ", axes, order, m, err)
			}
			var native, replay bytes.Buffer
			if err := WriteJSON(&native, s); err != nil {
				t.Fatal("write exact shared wall roof representation", err)
			}
			q, err := ReadJSONContext(ctx, bytes.NewReader(native.Bytes()), JSONLimits{})
			if err != nil {
				t.Fatal("reopen exact adjacent roof representation", err)
			}
			if err := WriteJSON(&replay, q); err != nil || !bytes.Equal(native.Bytes(), replay.Bytes()) {
				t.Fatal("adjacent original clocks must reopen byte-identically", err)
			}
		}
	}
	q, err := BuildProjectiveSplineRoofBoolean(ctx, a, b, [2]int{0, 1}, 0, parametric.Union, ProjectiveRoofSolidOptions{})
	if err != nil || !q.Complete || len(q.SectionEdges) != 1 {
		t.Fatal("automatic Fuse should find the complete original neighboring wall", err)
	}
	for _, b := range []Face{
		projectiveAdjacentRoof(t, 1, 3, 1, .25, 2),
		projectiveAdjacentRoof(t, 1, 3, .5, 0, 2),
		projectiveAdjacentRoof(t, 2, 4, 1, 0, 2),
	} {
		q, err := BuildProjectiveSplineRoofAdjacentFuse(ctx, a, b, [2]int{0, 1}, 0, ProjectiveRoofSolidOptions{})
		if !errors.Is(err, geom.ErrConvergence) || q.Complete || len(q.Solid.faces) != 0 {
			t.Fatal("unequal lift, partial side or disjoint footprint cannot publish a shared-wall Fuse", q, err)
		}
	}
}
