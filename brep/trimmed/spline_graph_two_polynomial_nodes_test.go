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
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func nativeTwoPolynomialNodeFixture(t *testing.T, axis int, firstDenominator int64) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	// g-f=(firstDenominator*u-1)(2*u-1)(1+u^2)/64. The independent
	// source roof builder uses the complete quadratic gap in the dependent axis.
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{big.NewRat(17, 64), big.NewRat(22-firstDenominator, 64), big.NewRat(2*firstDenominator-23, 64), big.NewRat(6-firstDenominator, 64), big.NewRat(2*firstDenominator, 64)}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func nativeWideTwoPolynomialNodeFixture(t *testing.T, axis int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	// g-f=(4u-1)(4u-3)(1+u^2)/32 for the independent OCCT comparison.
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{big.NewRat(11, 32), big.NewRat(-1, 8), big.NewRat(7, 32), big.NewRat(-3, 8), big.NewRat(1, 2)}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func nativeTwoPolynomialNodeVolume(op parametric.BooleanOp, wide bool) float64 {
	// Over each free slab the negative gap integral is 105|g-f|^3/768.
	// Integrate the original product independently over the full rectangle
	// for its net contribution, and add the common base volume 769/256.
	if wide {
		if op == parametric.Union {
			return 6637852629.0 / 2147483648
		}
		return 6450735147.0 / 2147483648
	}
	if op == parametric.Union {
		return 23331176626458563.0 / 7547047813054464
	}
	return 22670564921241661.0 / 7547047813054464
}

func TestNativeTwoPolynomialNodeEnvelopes(t *testing.T) {
	slowtest.Skip(t)
	t.Run("close nodes", func(t *testing.T) { testNativeTwoPolynomialNodeEnvelopes(t, false) })
	t.Run("wider separation", func(t *testing.T) { testNativeTwoPolynomialNodeEnvelopes(t, true) })
}

func testNativeTwoPolynomialNodeEnvelopes(t *testing.T, wide bool) {
	t.Helper()
	ctx := context.Background()
	build := func(t *testing.T, axis int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
		return nativeTwoPolynomialNodeFixture(t, axis, 4)
	}
	if wide {
		build = nativeWideTwoPolynomialNodeFixture
	}
	for _, axis := range []int{0, 1} {
		base, a, b := build(t, axis)
		o := SplineGraphBooleanOptions{TwoPolynomialNodes: &geom.GraphPointContactOptions{}, TwoPolynomialNodesAxis: axis}
		for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, roofs[0], roofs[1], op, 1e-9, o)
				if e != nil || h.TwoPolynomialNodes == nil || !h.TwoPolynomialNodes.Arrangement.Proven || len(h.TwoPolynomialNodes.Arrangement.CertifiedCells) != 4 {
					t.Fatal(axis, order, op, e)
				}
				for i, region := range h.TwoPolynomialNodes.Arrangement.Regions {
					owner := roofs[1]
					if (region.HeightSide > 0) == (op == parametric.Union) {
						owner = roofs[0]
					}
					if s.faces[i].Reversed || s.faces[i].Surface != owner || s.faces[h.Base].Surface != base {
						t.Fatal("original signed roof ownership", axis, order, op, i)
					}
				}
				checkNativeTwoPolynomialNodeTopology(t, s, h)
				mass, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				want := nativeTwoPolynomialNodeVolume(op, wide)
				if e != nil || math.Abs(mass.Volume-want) > 1e-9 {
					t.Fatal("analytical two-node graph volume", axis, order, op, mass.Volume, want, e)
				}
				var encoded bytes.Buffer
				if e = WriteJSON(&encoded, s); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadJSONContext(ctx, bytes.NewReader(encoded.Bytes()), JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				checkNativeTwoPolynomialNodeTopology(t, loaded, h)
				for i := 0; i <= h.Base; i++ {
					original := s.faces[i].Surface.(*geom.BSplineSurface)
					restored, ok := loaded.faces[i].Surface.(*geom.BSplineSurface)
					if !ok || !reflect.DeepEqual(original.Data(), restored.Data()) {
						t.Fatal("native original support and knot ledger changed", i)
					}
				}
				if e = validateExactGraphPartitionEdges(ctx, loaded, geom.GraphPointContactOptions{}); e != nil {
					t.Fatal("reloaded original exact edge lifts", e)
				}
				var rewritten bytes.Buffer
				if e = WriteJSON(&rewritten, loaded); e != nil || !bytes.Equal(rewritten.Bytes(), encoded.Bytes()) {
					t.Fatal("native two-node canonical round trip", e)
				}
				if axis == 0 && order == 0 && op == parametric.Intersection {
					checkNativePolynomialNodeRejectsChangedOwner(t, encoded.Bytes(), 2)
				}
			}
			s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, roofs[0], roofs[1], parametric.Difference, 1e-9, o)
			if !errors.Is(e, geom.ErrDegenerate) || len(s.faces) != 0 || h.TwoPolynomialNodes != nil {
				t.Fatal("two-node Cut requires separate contact admission", e)
			}
		}
	}
}

func TestNativeTwoPolynomialNodeOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_TWO_POLYNOMIAL_NODES")
	if exe == "" {
		t.Skip("optional independent original OCCT stocks")
	}
	data, e := exec.Command(exe, "node-pair").Output()
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Axis, Order, Operation, Solids int
		Valid                          bool
		Volume                         float64
	}
	if e = json.Unmarshal(data, &rows); e != nil || len(rows) != 8 {
		t.Fatal(string(data), e)
	}
	var seen [2][2][2]bool
	for _, r := range rows {
		if r.Axis < 0 || r.Axis > 1 || r.Order < 0 || r.Order > 1 || r.Operation < 0 || r.Operation > 1 || seen[r.Axis][r.Order][r.Operation] {
			t.Fatal(r)
		}
		seen[r.Axis][r.Order][r.Operation] = true
		op := [2]parametric.BooleanOp{parametric.Intersection, parametric.Union}[r.Operation]
		want := nativeTwoPolynomialNodeVolume(op, true)
		if !r.Valid || r.Solids != 1 || math.Abs(r.Volume-want) > 1e-9 {
			t.Fatal(r, want)
		}
	}
}

func checkNativeTwoPolynomialNodeTopology(t *testing.T, s Solid, h SplineGraphBooleanHistory) {
	t.Helper()
	topology := s.Topology()
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 || len(s.faces) != 14 || len(h.ContourEdges) != 8 {
		t.Fatal("closed two-node envelope", topology, h.ContourEdges)
	}
	mapping := h.TwoPolynomialNodes
	axis := mapping.Arrangement.FreeAxis
	uv, points, e := mapping.Arrangement.RepresentedVertices()
	if e != nil {
		t.Fatal(e)
	}
	degree := func(v int) int {
		count := 0
		for _, e := range s.edges {
			if e.Start == v || e.End == v {
				count++
			}
		}
		return count
	}
	for i, id := range mapping.Vertices {
		if s.vertices[id] != points[i] || (i >= 4 && degree(id) != 4) {
			t.Fatal("original exact node and terminal vertex identities", i, id)
		}
	}
	wantStation := 0.5 * (([2]float64{uv[4].X, uv[4].Y})[axis] + ([2]float64{uv[5].X, uv[5].Y})[axis])
	for _, id := range mapping.Midpoints {
		p := s.vertices[id]
		if ([2]float64{p.X, p.Y})[axis] != wantStation || degree(id) != 2 {
			t.Fatal("branch midpoint identity and station", id, p)
		}
	}
	if mapping.Midpoints[0] == mapping.Midpoints[1] || len(s.faces[2].Outer) != 4 {
		t.Fatal("central lens has four distinct boundary edges", mapping.Midpoints)
	}
	seen := make(map[int]bool, len(h.ContourEdges))
	for i, chain := range mapping.Edges {
		source := mapping.Arrangement.Edges[i]
		stations := []int{mapping.Vertices[source.Start], mapping.Vertices[source.End]}
		if i == 1 || i == 4 {
			stations = []int{stations[0], mapping.Midpoints[source.Branch], stations[1]}
		}
		if len(chain) != len(stations)-1 {
			t.Fatal("original branch split ledger", i, chain)
		}
		for j, id := range chain {
			edge := s.edges[id]
			ends := [2]int{edge.Start, edge.End}
			if seen[id] || (ends != [2]int{stations[j], stations[j+1]} && ends != [2]int{stations[j+1], stations[j]}) {
				t.Fatal("native edge identity and exact ancestry", i, j, id, ends, stations)
			}
			seen[id] = true
			curve := edge.Curve.(surfaceTrimCurve).curve.(ExactBezierUV)
			uv, _, e := curve.EvaluateRational(big.NewRat(1, 3))
			if e != nil {
				t.Fatal(e)
			}
			want := new(big.Rat)
			power := mapping.Arrangement.Power[source.Branch]
			for k := len(power) - 1; k >= 0; k-- {
				want.Mul(want, uv[axis])
				want.Add(want, power[k])
			}
			if uv[1-axis].Cmp(want) != 0 {
				t.Fatal("exact native curve restricted to original source branch", i, j, uv, want)
			}
		}
	}
	for _, id := range h.ContourEdges {
		if !seen[id] {
			t.Fatal("unbound native section edge", id)
		}
	}
}

func TestNativeTwoPolynomialNodeAtomicFailures(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativeTwoPolynomialNodeFixture(t, 0, 4)
	for _, o := range []SplineGraphBooleanOptions{
		{TwoPolynomialNodes: &geom.GraphPointContactOptions{MaxOperations: 1}},
		{TwoPolynomialNodes: &geom.GraphPointContactOptions{}, TwoPolynomialNodesAxis: 2},
		{TwoPolynomialNodes: &geom.GraphPointContactOptions{}, PolynomialNode: &geom.GraphPointContactOptions{}},
	} {
		s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Union, 1e-9, o)
		if e == nil || len(s.faces) != 0 || h.TwoPolynomialNodes != nil {
			t.Fatal("ambiguous or exhausted proof produced geometry", e)
		}
	}
	base, a, b = nativeTwoPolynomialNodeFixture(t, 0, 3)
	o := SplineGraphBooleanOptions{TwoPolynomialNodes: &geom.GraphPointContactOptions{}}
	s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Union, 1e-9, o)
	if !errors.Is(e, geom.ErrUnrepresentable) || len(s.faces) != 0 || h.TwoPolynomialNodes != nil {
		t.Fatal("rounded two-node output accepted", e)
	}
}
