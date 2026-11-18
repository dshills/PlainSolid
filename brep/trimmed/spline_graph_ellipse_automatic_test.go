package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/intersect"
	"testing"
)

func automaticEllipseGraphs(t *testing.T) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
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
					z += 13.0/256 - 2*term[i] - (u-.5)*(v-.5) - 3*term[j]
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
	// Different exact knot partitions remain the actual native original supports.
	for owner := 1; owner < 3; owner++ {
		p, e := geom.NewSurfacePatch(out[owner], geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if owner == 1 {
			p, e = p.InsertUKnot(.25, 1, geom.SurfacePatchOptions{})
		} else {
			p, e = p.InsertVKnot(.75, 1, geom.SurfacePatchOptions{})
		}
		if e != nil {
			t.Fatal(e)
		}
		out[owner], e = p.ToBSplineSurface()
		if e != nil {
			t.Fatal(e)
		}
	}
	return out[0], out[1], out[2]
}
func TestAutomaticRationalEllipseGraphBoolean(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	base, a, b := automaticEllipseGraphs(t)
	// Complete exact conic fallback remains available when generic tracing cannot
	// spend enough cells. This quota does not restrict the independent conic proof.
	opts := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	lens := 169 * math.Pi / (65536 * math.Sqrt(23))
	vb := 3 + 1.0/256
	delta := 13.0/256 - 5.0/12
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

func TestAutomaticRationalEllipseDefault(t *testing.T) {
	base, a, b := automaticEllipseGraphs(t)
	s, h, e := CombineSplineGraphSolids(context.Background(), base, a, b, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{})
	if e != nil || len(s.faces) != 2 || h.RationalEllipseArcs == nil {
		t.Fatal(h, e)
	}
	if strict, _, err := CombineQuadraticEllipseContactSplineGraphs(context.Background(), base, a, b, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}, h.RationalEllipseArcs.Certificate.Ellipse, geom.GraphPointContactOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || len(strict.faces) != 0 {
		t.Fatal("strict conjugate-scale route changed", err)
	}
	for _, o := range []intersect.SplinePlaneOptions{{Patch: geom.SurfacePatchOptions{MaxOperations: 1}}} {
		q, _, err := CombineSplineGraphSolids(context.Background(), base, a, b, parametric.Difference, 1e-9, o)
		if err == nil || len(q.faces) != 0 {
			t.Fatal("quota publication", err)
		}
	}

}

func TestAutomaticRationalEllipseOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_RATIONAL_ELLIPSE_ROOFS")
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
	lens := 169 * math.Pi / (65536 * math.Sqrt(23))
	vb := 3 + 1.0/256
	delta := 13.0/256 - 5.0/12
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
