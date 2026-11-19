//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestAutomaticOpenParabolicGraphBooleans(t *testing.T) {
	ctx := context.Background()
	base, a, b := openParabolicGraphFixture(t, [3]float64{.25, .5, .25}, false)
	for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
		for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
			s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{Parabola: &geom.GraphPointContactOptions{}})
			if e != nil {
				t.Fatal(order, op, e)
			}
			if h.Parabola == nil || !h.Parabola.Proven || h.Parabola.TerminalMasks != ([2]uint8{1, 2}) || len(h.Parabola.CertifiedCells) < 4 || len(h.ContourEdges) != 1 {
				t.Fatal("complete original open section", h)
			}
			for _, control := range []int{0, 2} {
				uv := h.Parabola.Controls[control]
				if uv[0].Cmp(big.NewRat(int64(control/2), 1)) != 0 || uv[1].Cmp(big.NewRat(1, 4)) != 0 {
					t.Fatal("original side parameter", uv)
				}
			}
			sides := map[int]int{}
			for _, side := range h.DomainSides {
				sides[side]++
			}
			if op != parametric.Difference {
				if sides[0] != 1 || sides[1] != 2 || sides[2] != 1 || sides[3] != 2 {
					t.Fatal("split original wall intervals", sides)
				}
			} else {
				horizontal := 2
				if order == 1 {
					horizontal = 0
				}
				if len(h.WallFaces) != 3 || sides[1] != 1 || sides[3] != 1 || sides[horizontal] != 1 {
					t.Fatal("retained original side intervals", sides)
				}
			}
			for owner, ids := range [][]int{h.FacesA, h.FacesB} {
				for _, id := range ids {
					if s.faces[id].Surface != pair[owner] {
						t.Fatal("original source replaced")
					}
				}
			}
			if len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
				t.Fatal(s.Topology())
			}
			m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if e != nil || math.Abs(m.Volume-parabolicGraphVolume(order, op)) > 2e-8 {
				t.Fatal(order, op, m.Volume, parabolicGraphVolume(order, op), e)
			}
			var data bytes.Buffer
			if e = WriteJSON(&data, s); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadJSONContext(ctx, &data, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			if e = validateExactGraphPartitionEdges(ctx, loaded, geom.GraphPointContactOptions{}); e != nil {
				t.Fatal(e)
			}
			t.Log(order, op, s.Topology(), m.Volume)
		}
	}
}

func TestOpenParabolicGraphCoverageAndLimits(t *testing.T) {
	ctx := context.Background()
	base, a, b := openParabolicGraphFixture(t, [3]float64{.25, .5, .25}, false)
	ap, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	bp, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	r, e := geom.DiscoverSplineGraphParabola(ctx, ap, bp, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Change only the final U control row: first-cell inference is unchanged,
	// but the original second cell no longer has the same complete contact.
	d := a.Data()
	for j := 0; j < d.CountV; j++ {
		d.Poles[(d.CountU-1)*d.CountV+j].Z += 1.0 / 1024
	}
	changed, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	if s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, changed, b, parametric.Difference, 1e-9, SplineGraphBooleanOptions{Parabola: &geom.GraphPointContactOptions{}}); e == nil || len(s.faces) != 0 || h.Parabola != nil {
		t.Fatal("unproved later original cell", e)
	}
	if s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Difference, 1e-9, SplineGraphBooleanOptions{Parabola: &geom.GraphPointContactOptions{MaxOperations: 1}}); e == nil || len(s.faces) != 0 || h.Parabola != nil {
		t.Fatal("quota publication", e)
	}
	for _, limit := range []int{r.ExactOperations, r.ExactOperations - 1} {
		q, e := geom.DiscoverSplineGraphParabola(ctx, ap, bp, geom.GraphPointContactOptions{MaxOperations: limit})
		if (e == nil) != (limit == r.ExactOperations) || q.Proven != (e == nil) {
			t.Fatal("exact cumulative quota", e)
		}
	}
	for _, tc := range []struct {
		f       [3]float64
		perturb bool
	}{{[3]float64{0, .5, .25}, false}, {[3]float64{.25, 1.25, .25}, false}, {[3]float64{.25, .25, .25}, false}, {[3]float64{.25, .5, .25}, true}} {
		base, a, b = openParabolicGraphFixture(t, tc.f, tc.perturb)
		s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Difference, 1e-9, SplineGraphBooleanOptions{Parabola: &geom.GraphPointContactOptions{}})
		if e == nil || len(s.faces) != 0 || h.Parabola != nil {
			t.Fatal("unsupported original arrangement", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := geom.DiscoverSplineGraphParabola(cancelled, ap, bp, geom.GraphPointContactOptions{}); e == nil || q.Proven {
		t.Fatal("cancel", e)
	}
}

func TestOpenParabolicGraphDoesNotRoundTerminalOwners(t *testing.T) {
	ctx := context.Background()
	base, a, b := openParabolicGraphFixture(t, [3]float64{1, 1.5, 1}, false)
	d := a.Data()
	for i := range d.Poles {
		d.Poles[i].Z += 2 * d.Poles[i].Y
	}
	a, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	ap, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	bp, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	proof, e := geom.DiscoverSplineGraphParabola(ctx, ap, bp, geom.GraphPointContactOptions{})
	if e != nil || !proof.Proven || proof.Controls[0][1].Cmp(big.NewRat(1, 3)) != 0 || proof.Controls[2][1].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("exact nonbinary original terminal", proof, e)
	}
	s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Difference, 1e-9, SplineGraphBooleanOptions{Parabola: &geom.GraphPointContactOptions{}})
	if e == nil || len(s.faces) != 0 || h.Parabola != nil {
		t.Fatal("native terminal was rounded", e)
	}
}

func TestAutomaticOpenParabolicVGraphBooleans(t *testing.T) {
	ctx := context.Background()
	base, a, b := openParabolicGraphFixture(t, [3]float64{.25, .5, .25}, false, 1)
	for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
		for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
			s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{Parabola: &geom.GraphPointContactOptions{}, ParabolaAxis: 1})
			if e != nil {
				t.Fatal(order, op, e)
			}
			if h.Parabola == nil || !h.Parabola.Proven || h.Parabola.TerminalMasks != ([2]uint8{4, 8}) || len(h.Parabola.CertifiedCells) < 4 || len(h.ContourEdges) != 1 {
				t.Fatal("complete original open section", h)
			}
			for _, control := range []int{0, 2} {
				uv := h.Parabola.Controls[control]
				if uv[1].Cmp(big.NewRat(int64(control/2), 1)) != 0 || uv[0].Cmp(big.NewRat(1, 4)) != 0 {
					t.Fatal("original side parameter", uv)
				}
			}
			sides := map[int]int{}
			for _, side := range h.DomainSides {
				sides[map[int]int{0: 3, 1: 2, 2: 1, 3: 0}[side]]++
			}
			if op != parametric.Difference {
				if sides[0] != 1 || sides[1] != 2 || sides[2] != 1 || sides[3] != 2 {
					t.Fatal("split original wall intervals", sides)
				}
			} else {
				horizontal := 2
				if order == 1 {
					horizontal = 0
				}
				if len(h.WallFaces) != 3 || sides[1] != 1 || sides[3] != 1 || sides[horizontal] != 1 {
					t.Fatal("retained original side intervals", sides)
				}
			}
			for owner, ids := range [][]int{h.FacesA, h.FacesB} {
				for _, id := range ids {
					if s.faces[id].Surface != pair[owner] {
						t.Fatal("original source replaced")
					}
				}
			}
			if len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
				t.Fatal(s.Topology())
			}
			m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if e != nil || math.Abs(m.Volume-parabolicGraphVolume(order, op)) > 2e-8 {
				t.Fatal(order, op, m.Volume, parabolicGraphVolume(order, op), e)
			}
			var data bytes.Buffer
			if e = WriteJSON(&data, s); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadJSONContext(ctx, &data, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			if e = validateExactGraphPartitionEdges(ctx, loaded, geom.GraphPointContactOptions{}); e != nil {
				t.Fatal(e)
			}
			t.Log(order, op, s.Topology(), m.Volume)
		}
	}
}

func TestOpenParabolicVGraphScope(t *testing.T) {
	ctx := context.Background()
	base, a, b := openParabolicGraphFixture(t, [3]float64{.25, .5, .25}, false, 1)
	ap, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	bp, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	r, e := geom.DiscoverSplineGraphParabolaOnAxis(ctx, ap, bp, 1, geom.GraphPointContactOptions{})
	if e != nil || r.FreeAxis != 1 {
		t.Fatal(r, e)
	}
	for _, axis := range []int{-1, 0, 2} {
		q, e := geom.DiscoverSplineGraphParabolaOnAxis(ctx, ap, bp, axis, geom.GraphPointContactOptions{})
		if e == nil || q.Proven || len(q.CertifiedCells) != 0 {
			t.Fatal("wrong axis accepted", axis, e)
		}
	}
	for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
		q, e := geom.DiscoverSplineGraphParabolaOnAxis(ctx, ap, bp, 1, geom.GraphPointContactOptions{MaxOperations: limit})
		if (e == nil) != (limit == r.ExactOperations) || (e != nil && q.Proven) {
			t.Fatal("exact quota", limit, e)
		}
	}
	d := a.Data()
	for i := 0; i < d.CountU; i++ {
		d.Poles[i*d.CountV+d.CountV-1].Z += 1.0 / 1024
	}
	changed, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, changed, b, parametric.Union, 1e-9, SplineGraphBooleanOptions{Parabola: &geom.GraphPointContactOptions{}, ParabolaAxis: 1})
	if e == nil || len(s.faces) != 0 || h.Parabola != nil {
		t.Fatal("later original V cell omitted", e)
	}
}
