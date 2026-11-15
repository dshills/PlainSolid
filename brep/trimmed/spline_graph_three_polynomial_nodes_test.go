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
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func nativeThreePolynomialNodeFixture(t *testing.T, axis int, firstDenominator, gapDenominator int64) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	// g-f=(1-firstDenominator*u)(1-2u)(3-4u)(1+u)/gapDenominator.
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{
		big.NewRat(3, gapDenominator), big.NewRat(-3*firstDenominator-7, gapDenominator),
		big.NewRat(7*firstDenominator-2, gapDenominator), big.NewRat(2*firstDenominator+8, gapDenominator),
		big.NewRat(-8*firstDenominator, gapDenominator),
	}
	for i, c := range f {
		g[i].Add(g[i], c)
	}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func nativeThreePolynomialNodeVolume(op parametric.BooleanOp, halfGap bool) float64 {
	// Integrate the negative fiber gap -105|g-f|^3/768 on the four
	// analytical slabs for Common. The signed integral of the full
	// quadratic gap gives Fuse by subtracting that negative contribution.
	// These values also include the common 769/256 base contribution.
	if halfGap {
		if op == parametric.Union {
			return 27209155362505.0 / 8796093022208
		}
		return 26422638133559.0 / 8796093022208
	}
	if op == parametric.Union {
		return 3401145007817.0 / 1099511627776
	}
	return 3302829179191.0 / 1099511627776
}

func TestNativeThreePolynomialNodeEnvelopes(t *testing.T) {
	ctx := context.Background()
	for axis := 0; axis < 2; axis++ {
		base, a, b := nativeThreePolynomialNodeFixture(t, axis, 4, 128)
		o := SplineGraphBooleanOptions{ThreePolynomialNodes: &geom.GraphPointContactOptions{}, ThreePolynomialNodesAxis: axis}
		for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			var patches [2]*geom.SurfacePatch
			for i, roof := range roofs {
				var e error
				patches[i], e = geom.NewSurfacePatch(roof, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
			}
			source, e := geom.DiscoverSplineGraphThreePolynomialNodes(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
			if e != nil || !source.Proven || len(source.CertifiedCells) != 4 {
				t.Fatal("original source arrangement", axis, order, e)
			}
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, roofs[0], roofs[1], op, 1e-9, o)
				if e != nil || h.ThreePolynomialNodes == nil || !h.ThreePolynomialNodes.Arrangement.Proven {
					t.Fatal(axis, order, op, e)
				}
				mass, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				want := nativeThreePolynomialNodeVolume(op, false)
				if e != nil || math.Abs(mass.Volume-want) > 1e-9 {
					t.Fatal("analytical three-node graph volume", axis, order, op, mass.Volume, want, e)
				}
				var encoded bytes.Buffer
				if e = WriteJSON(&encoded, s); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadJSONContext(ctx, bytes.NewReader(encoded.Bytes()), JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				checkNativeThreePolynomialNodeTopology(t, loaded, h, source)
				checkNativeThreePolynomialNodeOwners(t, s, loaded, h, source, base, roofs, patches, op)
				if e = validateExactGraphPartitionEdges(ctx, loaded, geom.GraphPointContactOptions{}); e != nil {
					t.Fatal("reloaded exact coedge lifts", e)
				}
				var rewritten bytes.Buffer
				if e = WriteJSON(&rewritten, loaded); e != nil || !bytes.Equal(rewritten.Bytes(), encoded.Bytes()) {
					t.Fatal("native three-node canonical round trip", e)
				}
				if axis == 0 && order == 0 && op == parametric.Intersection {
					checkNativePolynomialNodeRejectsChangedOwner(t, encoded.Bytes(), 2)
					checkNativePolynomialNodeRejectsChangedOwner(t, encoded.Bytes(), 3)
				}
			}
		}
	}
}

func TestNativeThreePolynomialNodeOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_THREE_POLYNOMIAL_NODES")
	if exe == "" {
		t.Skip("optional independent original OCCT stocks")
	}
	data, e := exec.Command(exe, "node-triple").Output()
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
	var base, a, b [2]*geom.BSplineSurface
	for axis := range base {
		base[axis], a[axis], b[axis] = nativeThreePolynomialNodeFixture(t, axis, 4, 256)
	}
	ctx := context.Background()
	for _, r := range rows {
		if r.Axis < 0 || r.Axis > 1 || r.Order < 0 || r.Order > 1 || r.Operation < 0 || r.Operation > 1 || seen[r.Axis][r.Order][r.Operation] {
			t.Fatal(r)
		}
		seen[r.Axis][r.Order][r.Operation] = true
		op := [2]parametric.BooleanOp{parametric.Intersection, parametric.Union}[r.Operation]
		want := nativeThreePolynomialNodeVolume(op, true)
		if !r.Valid || r.Solids != 1 || math.Abs(r.Volume-want) > 1e-9 {
			t.Fatal(r, want)
		}
		roofs := [2]*geom.BSplineSurface{a[r.Axis], b[r.Axis]}
		o := SplineGraphBooleanOptions{ThreePolynomialNodes: &geom.GraphPointContactOptions{}, ThreePolynomialNodesAxis: r.Axis}
		s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base[r.Axis], roofs[r.Order], roofs[1-r.Order], op, 1e-9, o)
		if e != nil || h.ThreePolynomialNodes == nil || len(s.Topology().Shells) != 1 {
			t.Fatal("native half-gap envelope", r, e)
		}
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil || math.Abs(m.Volume-want) > 1e-9 || math.Abs(m.Volume-r.Volume) > 1e-9 {
			t.Fatal("native and OCCT half-gap volumes", r, m.Volume, want, e)
		}
	}
}

func checkNativeThreePolynomialNodeOwners(t *testing.T, s, loaded Solid, h SplineGraphBooleanHistory, source geom.SplineGraphThreePolynomialNodes, base *geom.BSplineSurface, roofs [2]*geom.BSplineSurface, patches [2]*geom.SurfacePatch, op parametric.BooleanOp) {
	t.Helper()
	ctx := context.Background()
	if h.Base != len(source.Regions) || s.faces[h.Base].Surface != base || !geom.SameSplineSurface(loaded.faces[h.Base].Surface.(*geom.BSplineSurface), base) {
		t.Fatal("original shared base", h.Base)
	}
	for i, region := range source.Regions {
		var heights [2]*big.Rat
		for j, patch := range patches {
			p, e := patch.EvaluateRational(ctx, region.Witness[0], region.Witness[1], geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			heights[j] = p.Point[2]
		}
		sign := heights[0].Cmp(heights[1])
		if sign == 0 || sign != region.HeightSide {
			t.Fatal("original strict height order at rational witness", i, sign)
		}
		owner := roofs[1]
		if (sign > 0) == (op == parametric.Union) {
			owner = roofs[0]
		}
		f := loaded.faces[i]
		got, ok := f.Surface.(*geom.BSplineSurface)
		location, proof, e := LocateRationalInFaceTrim(ctx, f, region.Witness, trimvalidate.Options{})
		if s.faces[i].Surface != owner || f.Reversed || !ok || !geom.SameSplineSurface(got, owner) || e != nil || location != trimvalidate.Inside || proof.Status != trimvalidate.Certified {
			t.Fatal("selected original roof and its reloaded trim", i, location, proof, e)
		}
	}
}

func checkNativeThreePolynomialNodeTopology(t *testing.T, s Solid, h SplineGraphBooleanHistory, source geom.SplineGraphThreePolynomialNodes) {
	t.Helper()
	topology := s.Topology()
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 || len(s.faces) != 15 || len(h.ContourEdges) != 12 {
		t.Fatal("closed three-node envelope", topology, h.ContourEdges)
	}
	mapping := h.ThreePolynomialNodes
	_, points, e := source.RepresentedVertices()
	if e != nil {
		t.Fatal(e)
	}
	axis := source.FreeAxis
	degrees := make([]int, len(s.vertices))
	for _, edge := range s.edges {
		degrees[edge.Start]++
		degrees[edge.End]++
	}
	vertices := make(map[int]bool)
	for i, id := range mapping.Vertices {
		if vertices[id] || s.vertices[id] != points[i] || (i >= 4 && degrees[id] != 4) {
			t.Fatal("original exact node and terminal vertex identities", i, id)
		}
		vertices[id] = true
	}
	for lens, mids := range mapping.Midpoints {
		if len(s.faces[lens+2].Outer) != 4 {
			t.Fatal("interior lens rim needs four distinct edges", lens)
		}
		for _, id := range mids {
			p := s.vertices[id]
			if vertices[id] || degrees[id] != 2 || ([2]float64{p.X, p.Y})[axis] != float64(3+2*lens)/8 {
				t.Fatal("branch midpoint and free station", lens, id, p)
			}
			vertices[id] = true
		}
	}
	seen := make(map[int]bool, len(h.ContourEdges))
	for i, chain := range mapping.Edges {
		edge := source.Edges[i]
		stations := []int{mapping.Vertices[edge.Start], mapping.Vertices[edge.End]}
		piece := i % 4
		if piece == 1 || piece == 2 {
			stations = []int{stations[0], mapping.Midpoints[piece-1][edge.Branch], stations[1]}
		}
		if len(chain) != len(stations)-1 {
			t.Fatal("original branch piece subdivision", i, chain)
		}
		for j, id := range chain {
			native := s.edges[id]
			ends := [2]int{native.Start, native.End}
			if seen[id] || (ends != [2]int{stations[j], stations[j+1]} && ends != [2]int{stations[j+1], stations[j]}) {
				t.Fatal("distinct reloaded edge and original vertex ancestry", i, j, id, ends, stations)
			}
			seen[id] = true
			model, ok := native.Curve.(surfaceTrimCurve)
			if !ok {
				t.Fatal("native model curve must lift the original exact branch", id)
			}
			checkNativeThreePolynomialBranch(t, model.curve, axis, source.Power[edge.Branch])
			var expectedUses [6]bool
			for f, region := range source.Regions {
				for _, use := range region.Edges {
					if use.Edge == i {
						expectedUses[f] = true
					}
				}
			}
			useCount := 0
			for f, face := range s.faces {
				for _, use := range face.Outer {
					if use.Edge != id {
						continue
					}
					if f >= h.Base || !expectedUses[f] {
						t.Fatal("native coedge off its original sector", f, id)
					}
					expectedUses[f] = false
					checkNativeThreePolynomialBranch(t, use.Curve, axis, source.Power[edge.Branch])
					useCount++
				}
			}
			if useCount != 2 {
				t.Fatal("an original branch subdivision needs two exact native coedges", id, useCount)
			}
		}
	}
	for _, id := range h.ContourEdges {
		if !seen[id] {
			t.Fatal("unbound native section edge", id)
		}
	}
}

func checkNativeThreePolynomialBranch(t *testing.T, c PCurve, axis int, power []*big.Rat) {
	t.Helper()
	q, ok := c.(ExactBezierUV)
	if !ok || len(q.control) != len(power) {
		t.Fatal("original cubic or quartic degree and exact native curve")
	}
	left := q.control[0][axis]
	interval := new(big.Rat).Sub(q.control[len(power)-1][axis], left)
	for i, p := range q.control {
		want := new(big.Rat).Add(left, new(big.Rat).Mul(interval, big.NewRat(int64(i), int64(len(power)-1))))
		if p[2].Cmp(big.NewRat(1, 1)) != 0 || p[axis].Cmp(want) != 0 {
			t.Fatal("original affine free coordinate on the branch", i, p, want)
		}
	}
	// With unit weights and an affine free parameter, the difference from
	// the source branch is a polynomial of at most the original degree.
	for i := range power {
		uv, _, e := q.EvaluateRational(big.NewRat(int64(i), int64(len(power)-1)))
		if e != nil {
			t.Fatal(e)
		}
		want := new(big.Rat)
		for j := len(power) - 1; j >= 0; j-- {
			want.Mul(want, uv[axis]).Add(want, power[j])
		}
		if uv[1-axis].Cmp(want) != 0 {
			t.Fatal("reloaded curve differs from its original source polynomial", i, uv, want)
		}
	}
}

func TestNativeThreePolynomialNodeAtomicFailures(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativeThreePolynomialNodeFixture(t, 0, 4, 128)
	proof := &geom.GraphPointContactOptions{}
	for _, tc := range []struct {
		op   parametric.BooleanOp
		o    SplineGraphBooleanOptions
		want error
	}{
		{parametric.Union, SplineGraphBooleanOptions{ThreePolynomialNodes: &geom.GraphPointContactOptions{MaxOperations: 1}}, geom.ErrConvergence},
		{parametric.Union, SplineGraphBooleanOptions{ThreePolynomialNodes: proof, ThreePolynomialNodesAxis: 2}, geom.ErrInvalid},
		{parametric.Union, SplineGraphBooleanOptions{ThreePolynomialNodes: proof, TwoPolynomialNodes: proof}, geom.ErrInvalid},
		{parametric.Difference, SplineGraphBooleanOptions{ThreePolynomialNodes: proof}, geom.ErrDegenerate},
	} {
		s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, tc.op, 1e-9, tc.o)
		if !errors.Is(e, tc.want) || len(s.faces) != 0 || h.ThreePolynomialNodes != nil || h.Base != -1 {
			t.Fatal("ambiguous, exhausted or singular proof produced geometry", e)
		}
	}
	base, a, b = nativeThreePolynomialNodeFixture(t, 0, 3, 128)
	o := SplineGraphBooleanOptions{ThreePolynomialNodes: proof}
	s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Union, 1e-9, o)
	if !errors.Is(e, geom.ErrUnrepresentable) || len(s.faces) != 0 || h.ThreePolynomialNodes != nil || h.Base != -1 {
		t.Fatal("rounded three-node output accepted", e)
	}
}
