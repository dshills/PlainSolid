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

func projectiveNativeBandFaces(t *testing.T) [2]Face {
	t.Helper()
	var f [2]Face
	for i := range f {
		x, z, degree, knots := []float64{0, 3}, []float64{3, 3}, 1, []float64{0, 0, 3, 3}
		if i == 1 {
			x, z, degree, knots = []float64{0, 1.5, 3}, []float64{5, .5, 5}, 2, []float64{0, 0, 0, 3, 3, 3}
		}
		var poles []geom.Vec3
		var weights []float64
		for u, value := range x {
			for v := range 2 {
				poles = append(poles, geom.Vec3{X: value, Y: float64(v), Z: z[u]})
				weights = append(weights, 1+float64(i*v))
			}
		}
		raw, err := geom.NewBSplineSurface(degree, 1, len(x), 2, poles, knots, []float64{0, 0, 1, 1}, weights)
		if err != nil {
			t.Fatal(err)
		}
		f[i].Surface = raw
		q := [4]geom.Vec2{{}, {X: 3}, {X: 3, Y: 1}, {Y: 1}}
		for j := range q {
			f[i].Outer = append(f[i].Outer, Use{Edge: 4*i + j, UVStart: q[j], UVEnd: q[(j+1)%4]})
		}
	}
	return f
}

func TestBuildProjectiveSplineRoofBandSolidKeepsThreeOriginalRoofFragments(t *testing.T) {
	ctx := context.Background()
	f := projectiveNativeBandFaces(t)
	for order, original := range [][2]Face{f, {f[1], f[0]}} {
		for _, axes := range [][2]int{{0, 1}, {1, 0}} {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				r, err := BuildProjectiveSplineRoofBandSolid(ctx, original[0], original[1], axes, 0, op, ProjectiveRoofSolidOptions{})
				if err != nil || !r.Complete || r.Empty || len(r.SectionEdges) != 2 || len(r.Walls) != 8 || r.BaseFace != 11 {
					t.Fatal("both original seams and three roof fragments must close", order, axes, op, err)
				}
				if order == 0 && axes == [2]int{0, 1} {
					q, err := BuildProjectiveSplineRoofBoolean(ctx, original[0], original[1], axes, 0, op, ProjectiveRoofSolidOptions{})
					if err != nil || !q.Complete || len(q.SectionEdges) != 2 ||
						len(q.RoofFragments[0])+len(q.RoofFragments[1]) != 3 {
						t.Fatal("automatic Boolean must dispatch to the proved original bands", op, err)
					}
				}
				s := r.Solid
				if len(s.faces) != 12 || len(s.edges) != 26 || len(s.vertices) != 16 {
					t.Fatal("the two section boundaries must be shared in one closed native shell", len(s.faces), len(s.edges), len(s.vertices))
				}
				middleOwner, wantVolume := 1-order, 53.0/6
				if op == parametric.Union {
					middleOwner, wantVolume = 1-middleOwner, 32.0/3
				}
				if len(r.RoofFragments[middleOwner]) != 1 || r.RoofFragments[middleOwner][0] != 1 ||
					len(r.RoofFragments[1-middleOwner]) != 2 || r.RoofFragments[1-middleOwner][0] != 0 || r.RoofFragments[1-middleOwner][1] != 2 {
					t.Fatal("each original support owns the independently predicted signed bands", order, axes, op, r.RoofFragments)
				}
				for owner, roofs := range r.RoofFragments {
					for _, index := range roofs {
						if s.faces[index].Surface != original[owner].Surface {
							t.Fatal("the band replaced its original support", index)
						}
					}
				}
				for i, id := range r.SectionEdges {
					e := s.edges[id]
					a, b := s.vertices[e.Start], s.vertices[e.End]
					if a.X != float64(i+1) || b.X != a.X || a.Z != 3 || b.Z != 3 || a.Y+b.Y != 1 {
						t.Fatal("native seam must preserve the complete original analytic line", id, a, b)
					}
				}
				m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
				if err != nil || math.Abs(m.Volume-wantVolume) > 1e-10 ||
					m.Centroid.Sub(geom.Vec3{X: 1.5, Y: .5, Z: m.Centroid.Z}).Length() > 1e-10 {
					t.Fatal("native selected material differs from the independent strip integrals", order, axes, op, m, err)
				}
				var data, replay bytes.Buffer
				if err := WriteJSON(&data, s); err != nil {
					t.Fatal("write complete native band solid", err)
				}
				restored, err := ReadJSONContext(ctx, bytes.NewReader(data.Bytes()), JSONLimits{})
				if err != nil {
					t.Fatal("reopen complete native band solid", err)
				}
				if err := WriteJSON(&replay, restored); err != nil || !bytes.Equal(data.Bytes(), replay.Bytes()) {
					t.Fatal("both analytic seams and all original roofs must persist identically", err)
				}
			}
		}
	}
	opts := ProjectiveRoofSolidOptions{Sew: ExactSewOptions{MaxComparisons: 1}}
	r, err := BuildProjectiveSplineRoofBandSolid(ctx, f[0], f[1], [2]int{0, 1}, 0, parametric.Union, opts)
	if !errors.Is(err, geom.ErrConvergence) || r.Complete || len(r.Solid.faces) != 0 {
		t.Fatal("unpaired native bands cannot publish a partial shell", r, err)
	}
}
