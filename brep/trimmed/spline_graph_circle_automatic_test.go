package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func automaticCircleGraphs(t *testing.T, radiusSquared float64) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	var out [3]*geom.BSplineSurface
	for owner := range out {
		var poles []geom.Vec3
		term := []float64{.25, -.25, .25}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				u, v := float64(i)/2, float64(j)/2
				z := 0.0
				if owner > 0 {
					z = 3 + u*v/64
				}
				if owner == 1 {
					z += radiusSquared - term[i] - term[j]
				}
				poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		var e error
		out[owner], e = geom.NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
	}
	return out[0], out[1], out[2]
}
func TestAutomaticRationalCircleGraphBoolean(t *testing.T) {
	ctx := context.Background()
	base, a, b := automaticCircleGraphs(t, 5.0/64)
	// Complete exact conic fallback remains available when generic tracing cannot
	// spend enough cells. This quota does not restrict the independent conic proof.
	opts := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	lens := 25 * math.Pi / 8192
	vb := 3 + 1.0/256
	delta := 5.0/64 - 1.0/6
	for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
		for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
			s, h, e := CombineSplineGraphSolids(ctx, base, pair[0], pair[1], op, 1e-9, opts)
			if e != nil || h.RationalEllipseArcs == nil || !h.RationalEllipseArcs.Complete {
				t.Fatal(order, op, h, e)
			}
			want := vb + delta - lens
			if op == parametric.Union {
				want = vb + lens
			}
			if op == parametric.Difference {
				want = lens
				if order == 1 {
					want = lens - delta
				}
			}
			euler := 2
			if op == parametric.Difference && order == 1 {
				euler = 0
			}
			if s.Topology().EulerCharacteristic != euler || len(s.Topology().Shells) != 1 || len(h.ContourEdges) != 4 {
				t.Fatal("native region topology", s.Topology())
			}
			for owner, indices := range [][]int{h.FacesA, h.FacesB} {
				for _, fi := range indices {
					if s.faces[fi].Surface != pair[owner] {
						t.Fatal("original roof support lost")
					}
				}
			}
			mass, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if e != nil || math.Abs(mass.Volume-want) > 3e-8 {
				t.Fatal(order, op, mass.Volume, want, e)
			}
			var data bytes.Buffer
			if e = WriteJSON(&data, s); e != nil {
				t.Fatal(e)
			}
			q, e := ReadJSONContext(ctx, &data, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			if e = validateExactGraphPartitionEdges(ctx, q, geom.GraphPointContactOptions{}); e != nil {
				t.Fatal("restored original conic", e)
			}
			t.Log(order, op, s.Topology(), mass.Volume)
		}
	}
}

func TestAutomaticRationalCircleDefaultAndRejects(t *testing.T) {
	ctx := context.Background()
	base, a, b := automaticCircleGraphs(t, 5.0/64)
	s, h, e := CombineSplineGraphSolids(ctx, base, a, b, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{})
	if e != nil || len(s.faces) != 2 || h.RationalEllipseArcs == nil {
		t.Fatal("default normal discovery", h, e)
	}
	hint := geom.GraphEllipseHint{Center: [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, RadiusSquared: [2]*big.Rat{big.NewRat(5, 64), big.NewRat(5, 64)}}
	if strict, _, err := CombineEllipticalContactSplineGraphs(ctx, base, a, b, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}, hint, geom.GraphPointContactOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || len(strict.faces) != 0 {
		t.Fatal("strict principal-radius path changed", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if s, h, e := CombineSplineGraphSolids(canceled, base, a, b, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}); e == nil || len(s.faces) != 0 || h.RationalEllipseArcs != nil {
		t.Fatal("canceled partial publication")
	}
	if s, h, e := CombineSplineGraphSolids(ctx, base, a, b, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{Patch: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil || len(s.faces) != 0 || h.RationalEllipseArcs != nil {
		t.Fatal("quota partial publication")
	}
	base, a, b = automaticCircleGraphs(t, 3.0/64)
	// No exact rational point was proved: the new automatic route must not
	// publish a guessed conic, even with a small generic trace budget.
	opts := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	if s, h, e := CombineSplineGraphSolids(ctx, base, a, b, parametric.Difference, 1e-9, opts); e == nil || len(s.faces) != 0 || h.RationalEllipseArcs != nil {
		t.Fatal("unproved rational point accepted", e)
	}
}

func TestAutomaticRationalCircleOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_RATIONAL_CIRCLE_ROOFS")
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
	lens := 25 * math.Pi / 8192
	vb := 3 + 1.0/256
	delta := 5.0/64 - 1.0/6
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
		if !row.Valid || row.Solids != 1 || math.Abs(row.Volume-want) > 2e-8 {
			t.Fatal("OCCT original Boolean", row, want)
		}
	}
}
