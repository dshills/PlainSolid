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
	"plainsolid/intersect"
	"testing"
)

func disjointEllipseGraphFixture(t *testing.T) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface, [2]geom.GraphEllipseHint) {
	t.Helper()
	var out [3]*geom.BSplineSurface
	for owner := range out {
		us := []float64{0, .5, 1}
		ku := []float64{0, 0, 0, 1, 1, 1}
		if owner == 1 {
			us = []float64{0, .25, .5, .75, 1}
			ku = []float64{0, 0, 0, .5, .5, 1, 1, 1}
		}
		var poles []geom.Vec3
		for i, u := range us {
			for j, v := range []float64{0, .5, 1} {
				z := 0.0
				if owner > 0 {
					z = 3 + u*v/64
				}
				if owner == 1 {
					z += 1.0/64 - []float64{.0625, -.0625, .0625, -.0625, .0625}[i] - []float64{.25, -.25, .25}[j]
				}
				poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, len(us), 3, poles, ku, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		if owner == 2 {
			p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			p, e = p.InsertVKnot(.375, 1, geom.SurfacePatchOptions{})
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
	var hints [2]geom.GraphEllipseHint
	for i := range hints {
		hints[i] = geom.GraphEllipseHint{Center: [2]*big.Rat{big.NewRat(int64(1+2*i), 4), big.NewRat(1, 2)}, RadiusSquared: [2]*big.Rat{big.NewRat(1, 64), big.NewRat(1, 64)}}
	}
	return out[0], out[1], out[2], hints
}
func TestDisjointEllipseOriginalGraphBooleans(t *testing.T) {
	checkDisjointEllipseGraphBooleans(t, false)
}
func TestAutomaticDisjointEllipseGraphBooleans(t *testing.T) {
	checkDisjointEllipseGraphBooleans(t, true)
}
func checkDisjointEllipseGraphBooleans(t *testing.T, automatic bool) {
	ctx := context.Background()
	base, a, b, hints := disjointEllipseGraphFixture(t)
	if automatic {
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

	lens := math.Pi / 4096
	delta := 1.0/64 - 5.0/48
	vb := 3 + 1.0/256
	for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
		for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
			var s Solid
			var h DisjointEllipseGraphHistory
			var e error
			if automatic {
				var history SplineGraphBooleanHistory
				s, history, e = CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{DisjointEllipses: &geom.DisjointEllipseDiscoveryOptions{}})
				if e == nil {
					if history.DisjointEllipses == nil || !history.DisjointEllipses.Complete || history.DisjointEllipses.SplitU != .5 || history.DisjointEllipses.TestedSeparators != 2 {
						t.Fatal("automatic original knot proof", history)
					}
					h = DisjointEllipseGraphHistory{Section: history.DisjointEllipses.Section, Material: history}
				}
			} else {
				s, h, e = CombineDisjointEllipseContactSplineGraphs(ctx, base, pair[0], pair[1], op, 1e-9, intersect.SplinePlaneOptions{}, .5, hints, geom.GraphPointContactOptions{})
			}

			if e != nil {
				t.Fatal(order, op, e)
			}
			if !h.Section.Complete || len(h.Material.ContourEdges) != 8 {
				t.Fatal("incomplete original section", h)
			}
			want := vb + delta - lens
			shells, euler := 1, 2
			if op == parametric.Union {
				want = vb + lens
			}
			if op == parametric.Difference {
				want = lens
				shells, euler = 2, 4
				if order == 1 {
					want = lens - delta
					shells, euler = 1, -2
				}
			}
			topology := s.Topology()
			if len(topology.Shells) != shells || topology.EulerCharacteristic != euler {
				t.Fatal(order, op, topology)
			}
			for owner, ids := range [][]int{h.Material.FacesA, h.Material.FacesB} {
				for _, id := range ids {
					if s.faces[id].Surface != pair[owner] {
						t.Fatal("original ownership")
					}
				}
			}
			m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if e != nil || math.Abs(m.Volume-want) > 2e-8 {
				t.Fatal(order, op, m.Volume, want, e)
			}
			var data bytes.Buffer
			if e = WriteJSON(&data, s); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadJSONContext(ctx, &data, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			if loaded.Topology().EulerCharacteristic != euler || len(loaded.Topology().Shells) != shells {
				t.Fatal("restored material topology")
			}
			if e = validateExactGraphPartitionEdges(ctx, loaded, geom.GraphPointContactOptions{}); e != nil {
				t.Fatal(e)
			}
			t.Log(order, op, topology, m.Volume)
		}
	}
}
func TestDisjointEllipseCompleteCoverageRejects(t *testing.T) {
	ctx := context.Background()
	base, a, b, hints := disjointEllipseGraphFixture(t)
	ap, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	bp, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	proof, e := geom.CertifySplineGraphDisjointEllipses(ctx, ap, bp, .5, hints, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if reversed, e := geom.CertifySplineGraphDisjointEllipses(ctx, bp, ap, .5, hints, geom.GraphPointContactOptions{}); e != nil || !reversed.Complete || reversed.Contours[0].InsideSide != -proof.Contours[0].InsideSide {
		t.Fatal("reversed complete source", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := geom.CertifySplineGraphDisjointEllipses(cancelled, ap, bp, .5, hints, geom.GraphPointContactOptions{}); e == nil || r.Complete {
		t.Fatal("cancelled section", e)
	}
	if r, e := geom.CertifySplineGraphDisjointEllipses(ctx, ap, bp, .5, hints, geom.GraphPointContactOptions{MaxCells: 1}); e == nil || r.Complete {
		t.Fatal("cumulative cells", e)
	}
	for _, budget := range []int{1, proof.ExactOperations - 1} {
		r, e := geom.CertifySplineGraphDisjointEllipses(ctx, ap, bp, .5, hints, geom.GraphPointContactOptions{MaxOperations: budget})
		if e == nil || r.Complete {
			t.Fatal("atomic budget", e)
		}
	}
	if r, e := geom.CertifySplineGraphDisjointEllipses(ctx, ap, bp, .5, hints, geom.GraphPointContactOptions{MaxOperations: proof.ExactOperations}); e != nil || !r.Complete {
		t.Fatal("exact quota", e)
	}
	wrong := hints
	wrong[1] = hints[0]
	for _, tc := range []struct {
		split float64
		h     [2]geom.GraphEllipseHint
	}{{.5, wrong}, {.25, hints}, {.375, hints}} {
		s, h, e := CombineDisjointEllipseContactSplineGraphs(ctx, base, a, b, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}, tc.split, tc.h, geom.GraphPointContactOptions{})
		if e == nil || len(s.faces) != 0 || h.Section.Complete {
			t.Fatal("omitted or separator-touch branch", e)
		}
	}
}

func TestDisjointEllipseOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_DISJOINT_ELLIPSE_ROOFS")
	if exe == "" {
		t.Skip("optional independent original OCCT stocks")
	}
	data, e := exec.Command(exe).Output()
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Order, Operation, Solids int
		Valid                    bool
		Volume                   float64
	}
	if e = json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 6 {
		t.Fatal("incomplete oracle")
	}
	lens := math.Pi / 4096
	vb := 3 + 1.0/256
	delta := 1.0/64 - 5.0/48
	seen := map[[2]int]bool{}
	for _, row := range rows {
		id := [2]int{row.Order, row.Operation}
		if seen[id] || row.Order < 0 || row.Order > 1 || row.Operation < 0 || row.Operation > 2 {
			t.Fatal("oracle identity")
		}
		seen[id] = true
		want := vb + delta - lens
		if row.Operation == 1 {
			want = vb + lens
		}
		if row.Operation == 2 {
			want = lens
			if row.Order == 1 {
				want = lens - delta
			}
		}
		solids := 1
		if row.Order == 0 && row.Operation == 2 {
			solids = 2
		}
		if !row.Valid || row.Solids != solids || math.Abs(row.Volume-want) > 2e-8 {
			t.Fatal("OCCT original Boolean", row, want)
		}
	}
}
