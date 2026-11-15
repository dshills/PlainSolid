package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func openParabolicGraphFixture(t *testing.T, controls [3]float64, perturb bool, axes ...int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	var out [3]*geom.BSplineSurface
	for owner := range out {
		var poles []geom.Vec3
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				u, v := float64(i)/2, float64(j)/2
				z := 0.0
				if owner > 0 {
					z = 3 + u*v/64
				}
				if owner == 1 {
					if len(axes) > 0 && axes[0] == 1 {
						z += u - controls[j]
					} else {
						z += v - controls[i]
					}
					if perturb {
						z += u * v / 32
					}
				}
				poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		if owner > 0 {
			p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			knot := .3125
			if owner == 1 {
				knot = .375
			}
			if (owner == 1) != (len(axes) > 0 && axes[0] == 1) {
				p, e = p.InsertUKnot(knot, 1, geom.SurfacePatchOptions{})
			} else {
				p, e = p.InsertVKnot(knot, 1, geom.SurfacePatchOptions{})
			}
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
	return out[0], out[1], out[2]
}
func parabolicGraphVolume(order int, op parametric.BooleanOp) float64 {
	if op == parametric.Union {
		return 3 + 1.0/256 + 107.0/480
	}
	if op == parametric.Difference {
		if order == 0 {
			return 107.0 / 480
		}
		return 9.0 / 160
	}
	return 3 + 1.0/256 - 9.0/160
}
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
func TestOpenParabolicGraphOCCT(t *testing.T)  { testParabolicOCCT(t, false) }
func TestOpenParabolicVGraphOCCT(t *testing.T) { testParabolicOCCT(t, true) }
func testParabolicOCCT(t *testing.T, vertical bool) {
	exe := os.Getenv("PLAINSOLID_OCCT_PARABOLIC_ROOFS")
	if exe == "" {
		t.Skip("optional independent original OCCT stocks")
	}
	args := []string{}
	if vertical {
		args = append(args, "vertical")
	}
	data, e := exec.Command(exe, args...).Output()
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Order, Operation, Solids int
		Valid                    bool
		Vertical                 bool
		Volume                   float64
	}
	if e = json.Unmarshal(data, &rows); e != nil || len(rows) != 6 {
		t.Fatal(e)
	}
	seen := map[[2]int]bool{}
	for _, r := range rows {
		k := [2]int{r.Order, r.Operation}
		if seen[k] || r.Order < 0 || r.Order > 1 || r.Operation < 0 || r.Operation > 2 {
			t.Fatal("oracle identity")
		}
		seen[k] = true
		op := []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}[r.Operation]
		if r.Vertical != vertical || !r.Valid || r.Solids != 1 || math.Abs(r.Volume-parabolicGraphVolume(r.Order, op)) > 2e-8 {
			t.Fatal(r)
		}
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
