package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

// Independently author vertical loops: transpose the tensor basis AND exchange
// represented physical X/Y poles, preserving the identity XY source chart.
// Runtime discovery never changes these authored supports or owner coordinates.
func verticalGraphSource(t *testing.T, s *geom.BSplineSurface) *geom.BSplineSurface {
	t.Helper()
	q, e := s.Transposed()
	if e != nil {
		t.Fatal(e)
	}
	d := q.Data()
	for i := range d.Poles {
		d.Poles[i].X, d.Poles[i].Y = d.Poles[i].Y, d.Poles[i].X
	}
	q, e = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	return q
}
func verticalGraphFixture(t *testing.T, n int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	var base, a, b *geom.BSplineSurface
	if n == 3 {
		base, a, b = threeEllipseGraphFixture(t, false)
	} else {
		base, a, b, _ = disjointEllipseGraphFixture(t)
		p, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = p.InsertUKnot(.125, 1, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		a, e = p.ToBSplineSurface()
		if e != nil {
			t.Fatal(e)
		}
	}
	return verticalGraphSource(t, base), verticalGraphSource(t, a), verticalGraphSource(t, b)
}
func verticalGraphOptions(n, axis int) SplineGraphBooleanOptions {
	if n == 3 {
		return SplineGraphBooleanOptions{ThreeEllipses: &geom.ThreeEllipseDiscoveryOptions{PartitionAxis: axis}}
	}
	return SplineGraphBooleanOptions{DisjointEllipses: &geom.DisjointEllipseDiscoveryOptions{PartitionAxis: axis}}
}
func verticalGraphVolume(n, order int, op parametric.BooleanOp) float64 {
	if n == 3 {
		return threeEllipseVolume(order, op)
	}
	lens := math.Pi / 4096
	delta := 1.0/64 - 5.0/48
	if op == parametric.Union {
		return 3 + 1.0/256 + lens
	}
	if op == parametric.Difference {
		if order == 0 {
			return lens
		}
		return lens - delta
	}
	return 3 + 1.0/256 + delta - lens
}
func TestOriginalVPartitionGraphBooleans(t *testing.T) {
	ctx := context.Background()
	for _, n := range []int{2, 3} {
		base, a, b := verticalGraphFixture(t, n)
		for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
				s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, verticalGraphOptions(n, 1))
				if e != nil {
					t.Fatal(n, order, op, e)
				}
				if n == 2 {
					q := h.DisjointEllipses
					if q == nil || !q.Complete || q.PartitionAxis != 1 || q.Section.PartitionAxis != 1 || q.Separator != .5 || q.SplitU != 0 || q.TestedSeparators != 2 {
						t.Fatal("original V separator", q)
					}
				} else {
					q := h.ThreeEllipses
					if q == nil || !q.Complete || q.PartitionAxis != 1 || q.Section.PartitionAxis != 1 || q.Separators != ([2]float64{.25, .5}) || q.SplitU != ([2]float64{}) || q.TestedPartitions != 3 {
						t.Fatal("original V partitions", q)
					}
				}
				if len(h.ContourEdges) != 4*n {
					t.Fatal("missing source branch")
				}
				for owner, ids := range [][]int{h.FacesA, h.FacesB} {
					for _, id := range ids {
						if s.faces[id].Surface != pair[owner] {
							t.Fatal("replaced original support")
						}
						if s.faces[id].Reversed != (owner == 1 && op == parametric.Difference) {
							t.Fatal("original face orientation")
						}
					}
				}
				shells, euler := 1, 2
				if op == parametric.Difference {
					shells, euler = n, 2*n
					if order == 1 {
						shells, euler = 1, 2-2*n
					}
				}
				if len(s.Topology().Shells) != shells || s.Topology().EulerCharacteristic != euler {
					t.Fatal(s.Topology())
				}
				m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
				if e != nil || math.Abs(m.Volume-verticalGraphVolume(n, order, op)) > 2e-8 {
					t.Fatal(n, order, op, m.Volume, e)
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
				if loaded.Topology().EulerCharacteristic != euler || len(loaded.Topology().Shells) != shells {
					t.Fatal("restored topology")
				}
			}
		}
	}
}
func TestOriginalVPartitionLimitsAndStrictU(t *testing.T) {
	ctx := context.Background()
	for _, n := range []int{2, 3} {
		base, a, b := verticalGraphFixture(t, n)
		for _, axis := range []int{0, -1, 2} {
			s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Difference, 1e-9, verticalGraphOptions(n, axis))
			if e == nil || len(s.faces) != 0 || h.DisjointEllipses != nil || h.ThreeEllipses != nil {
				t.Fatal("unsupported axis or omitted branches", n, axis, e)
			}
		}
		// More actual loops than requested cannot be silently omitted in V.
		var moreBase, moreA, moreB *geom.BSplineSurface
		if n == 2 {
			moreBase, moreA, moreB = verticalGraphFixture(t, 3)
		} else {
			moreBase, moreA, moreB = threeEllipseGraphFixture(t, true)
			moreBase, moreA, moreB = verticalGraphSource(t, moreBase), verticalGraphSource(t, moreA), verticalGraphSource(t, moreB)
		}
		if s, h, e := CombineSplineGraphSolidsWithOptions(ctx, moreBase, moreA, moreB, parametric.Difference, 1e-9, verticalGraphOptions(n, 1)); e == nil || len(s.faces) != 0 || h.DisjointEllipses != nil || h.ThreeEllipses != nil {
			t.Fatal("omitted V component", n, e)
		}
		ap, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
		bp, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
		if n == 2 {
			r, e := geom.DiscoverSplineGraphDisjointEllipses(ctx, ap, bp, geom.DisjointEllipseDiscoveryOptions{PartitionAxis: 1})
			if e != nil {
				t.Fatal(e)
			}
			for _, budget := range []int{r.ExactOperations, r.ExactOperations - 1} {
				q, e := geom.DiscoverSplineGraphDisjointEllipses(ctx, ap, bp, geom.DisjointEllipseDiscoveryOptions{PartitionAxis: 1, GraphPointContactOptions: geom.GraphPointContactOptions{MaxOperations: budget}})
				if (e == nil) != (budget == r.ExactOperations) || q.Complete != (e == nil) {
					t.Fatal("V cumulative work", e)
				}
			}
		} else {
			r, e := geom.DiscoverSplineGraphThreeEllipses(ctx, ap, bp, geom.ThreeEllipseDiscoveryOptions{PartitionAxis: 1})
			if e != nil {
				t.Fatal(e)
			}
			for _, budget := range []int{r.ExactOperations, r.ExactOperations - 1} {
				q, e := geom.DiscoverSplineGraphThreeEllipses(ctx, ap, bp, geom.ThreeEllipseDiscoveryOptions{PartitionAxis: 1, GraphPointContactOptions: geom.GraphPointContactOptions{MaxOperations: budget}})
				if (e == nil) != (budget == r.ExactOperations) || q.Complete != (e == nil) {
					t.Fatal("V cumulative work", e)
				}
			}
		}
	}
}
func TestOriginalVPartitionOCCT(t *testing.T) {
	for _, n := range []int{2, 3} {
		key := "PLAINSOLID_OCCT_DISJOINT_ELLIPSE_ROOFS"
		if n == 3 {
			key = "PLAINSOLID_OCCT_THREE_ELLIPSE_ROOFS"
		}
		exe := os.Getenv(key)
		if exe == "" {
			t.Skip("optional independently authored vertical OCCT stocks")
		}
		data, e := exec.Command(exe, "vertical").Output()
		if e != nil {
			t.Fatal(e)
		}
		var rows []struct {
			Order, Operation, Solids int
			Valid, Vertical          bool
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
			solids := 1
			if op == parametric.Difference && r.Order == 0 {
				solids = n
			}
			if !r.Valid || !r.Vertical || r.Solids != solids || math.Abs(r.Volume-verticalGraphVolume(n, r.Order, op)) > 2e-8 {
				t.Fatal(n, r)
			}
		}
	}
}
