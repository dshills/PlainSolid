//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func nativePiecewiseParabolaFixture(t *testing.T) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	base, _, _ := openParabolicGraphFixture(t, [3]float64{.25, .5, .25}, false)
	var out [2]*geom.BSplineSurface
	for owner := 0; owner < 2; owner++ {
		xs := []float64{0, .25, .5, .75, 1}
		knots := []float64{0, 0, 0, .5, .5, 1, 1, 1}
		fs := []float64{.25, .5, .375, .375, .25}
		if owner == 1 {
			xs = []float64{0, .5, 1}
			knots = []float64{0, 0, 0, 1, 1, 1}
		}
		var poles []geom.Vec3
		for i, u := range xs {
			for j := 0; j < 3; j++ {
				v := float64(j) / 2
				z := 3 + u*v/64
				if owner == 0 {
					z += v - fs[i]
				}
				poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, len(xs), 3, poles, knots, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		if owner == 1 {
			p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			p, e = p.InsertUKnot(.75, 1, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			s, e = p.ToBSplineSurface()
			if e != nil {
				t.Fatal(e)
			}
		}
		out[owner] = s
	}
	return base, out[0], out[1]
}

func TestNativePiecewiseParabolaBooleans(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativePiecewiseParabolaFixture(t)
	for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
		for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
			s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{PiecewiseParabola: &geom.PiecewiseParabolaOptions{}})
			if e != nil {
				t.Fatal(order, op, e)
			}
			if h.PiecewiseParabola == nil || !h.PiecewiseParabola.Proven || len(h.ContourEdges) != 3 {
				t.Fatal("complete piece ownership", h)
			}
			if len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
				t.Fatal(s.Topology())
			}
			for owner, ids := range [][]int{h.FacesA, h.FacesB} {
				for _, id := range ids {
					if s.faces[id].Surface != pair[owner] {
						t.Fatal("replaced original chart")
					}
				}
			}
			// The three contour edges form one unbranched path with two actual shared
			// vertices. They are not three approximate disconnected model curves.
			incidence := map[int]int{}
			for _, id := range h.ContourEdges {
				edge := s.edges[id]
				incidence[edge.Start]++
				incidence[edge.End]++
			}
			ends, joins := 0, 0
			for id, n := range incidence {
				if n == 1 {
					ends++
				} else if n == 2 {
					point := s.vertices[id]
					if point.X == .5 {
						if point.Y != .375 || point.Z != 3075.0/1024 {
							t.Fatal("original C0 point", point)
						}
					} else if point.X == .75 {
						if point.Y != 11.0/32 || point.Z != 24609.0/8192 {
							t.Fatal("original refinement point", point)
						}
					} else {
						t.Fatal("invented contour vertex", point)
					}
					joins++
				} else {
					t.Fatal("bad contour incidence", incidence)
				}
			}
			if ends != 2 || joins != 2 {
				t.Fatal(incidence)
			}
			if e = validateExactGraphPartitionEdges(ctx, s, geom.GraphPointContactOptions{}); e != nil {
				t.Fatal(e)
			}
			m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if e != nil || math.Abs(m.Volume-piecewiseParabolaVolume(order, op)) > 2e-8 {
				t.Fatal(order, op, m.Volume, e)
			}
			var encoded bytes.Buffer
			if e = WriteJSON(&encoded, s); e != nil {
				t.Fatal(order, op, e)
			}
			loaded, e := ReadJSONContext(ctx, &encoded, JSONLimits{})
			if e != nil {
				t.Fatal(order, op, e)
			}
			if e = validateExactGraphPartitionEdges(ctx, loaded, geom.GraphPointContactOptions{}); e != nil {
				t.Fatal(e)
			}
			t.Log(order, op, s.Topology(), m.Volume)
		}
	}
}

func TestNativePiecewiseParabolaAtomicLimits(t *testing.T) {
	base, a, b := nativePiecewiseParabolaFixture(t)
	for _, o := range []SplineGraphBooleanOptions{{PiecewiseParabola: &geom.PiecewiseParabolaOptions{MaxPieces: 2}}, {PiecewiseParabola: &geom.PiecewiseParabolaOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}}, {PiecewiseParabola: &geom.PiecewiseParabolaOptions{}, Parabola: &geom.GraphPointContactOptions{}}} {
		s, h, e := CombineSplineGraphSolidsWithOptions(context.Background(), base, a, b, parametric.Union, 1e-9, o)
		if e == nil || len(s.faces) != 0 || h.PiecewiseParabola != nil {
			t.Fatal("partial native output", e)
		}
	}
}
