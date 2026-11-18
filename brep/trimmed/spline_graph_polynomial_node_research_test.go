//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativePolynomialNodeGraphBoolean(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, axis := range []int{0, 1} {
		base, a, b := nativePolynomialNodeFixture(t, axis, 2)
		o := SplineGraphBooleanOptions{PolynomialNode: &geom.GraphPointContactOptions{}, PolynomialNodeAxis: axis}
		for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, o)
				if e != nil || h.PolynomialNode == nil || !h.PolynomialNode.Proven || len(h.PolynomialNode.CertifiedCells) != 4 {
					t.Fatal(axis, order, op, e)
				}
				if s.faces[h.Base].Surface != base || len(h.FacesA) != 2 || len(h.FacesB) != 2 || len(h.WallFaces) != 8 {
					t.Fatal("original sector and wall ledger", h)
				}
				for owner, ids := range [][]int{h.FacesA, h.FacesB} {
					for _, id := range ids {
						if s.faces[id].Surface != pair[owner] || s.faces[id].Reversed {
							t.Fatal("original roof owner", owner, id)
						}
					}
				}
				checkNativePolynomialNodeTopology(t, s, h)
				want := nativePolynomialNodeVolume(op)
				m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if e != nil || math.Abs(m.Volume-want) > 1e-9 {
					t.Fatal("analytical volume", m.Volume, want, e)
				}
				var native bytes.Buffer
				if e = WriteJSON(&native, s); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadJSONContext(ctx, bytes.NewReader(native.Bytes()), JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				checkNativePolynomialNodeTopology(t, loaded, h)
				if e = validateExactGraphPartitionEdges(ctx, loaded, geom.GraphPointContactOptions{}); e != nil {
					t.Fatal("native exact owner agreement", e)
				}
				var rewritten bytes.Buffer
				if e = WriteJSON(&rewritten, loaded); e != nil || !bytes.Equal(rewritten.Bytes(), native.Bytes()) {
					t.Fatal("native canonical round trip", e)
				}
				if axis == 0 && order == 0 && op == parametric.Intersection {
					checkNativePolynomialNodeRejectsChangedOwner(t, native.Bytes(), h.FacesA[0])
				}
			}
			s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], parametric.Difference, 1e-9, o)
			if !errors.Is(e, geom.ErrDegenerate) || len(s.faces) != 0 || h.PolynomialNode != nil {
				t.Fatal("point-contacting Cut components", e)
			}
		}
	}
}

func checkNativePolynomialNodeTopology(t *testing.T, s Solid, h SplineGraphBooleanHistory) {
	t.Helper()
	topology := s.Topology()
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 || len(h.ContourEdges) != 4 {
		t.Fatal("closed one-shell four-ray envelope", topology, h.ContourEdges)
	}
	center, e := geom.RepresentedExactPoint(h.PolynomialNode.Point)
	if e != nil {
		t.Fatal(e)
	}
	vertex := -1
	for i, point := range s.vertices {
		if point == center {
			if vertex >= 0 {
				t.Fatal("duplicate node identity", vertex, i)
			}
			vertex = i
		}
	}
	if vertex < 0 {
		t.Fatal("missing exact shared node")
	}
	for _, id := range h.ContourEdges {
		edge := s.edges[id]
		if (edge.Start == vertex) == (edge.End == vertex) {
			t.Fatal("ray incidence", id, edge)
		}
		model, ok := edge.Curve.(surfaceTrimCurve)
		if !ok {
			t.Fatal("missing ancestral model lift", edge)
		}
		if _, ok := model.curve.(ExactBezierUV); !ok {
			t.Fatal("ray missing original exact restriction", id, model.curve)
		}
		var forward, reversed int
		for _, face := range s.faces {
			for _, use := range face.Outer {
				if use.Edge == id {
					if _, ok := use.Curve.(ExactBezierUV); !ok {
						t.Fatal("missing exact owner curve", use)
					}
					if use.Reversed {
						reversed++
					} else {
						forward++
					}
				}
			}
		}
		if forward != 1 || reversed != 1 {
			t.Fatal("opposite owner uses", id, forward, reversed)
		}
	}
	incident := 0
	for _, e := range s.edges {
		if e.Start == vertex || e.End == vertex {
			incident++
		}
	}
	if incident != 4 {
		t.Fatal("node vertex degree", incident)
	}
}

func checkNativePolynomialNodeRejectsChangedOwner(t *testing.T, raw []byte, face int) {
	t.Helper()
	var native nativeSolid
	if e := json.Unmarshal(raw, &native); e != nil {
		t.Fatal(e)
	}
	for _, u := range native.Faces[face].Outer {
		if len(u.ExactBezier) == 0 {
			continue
		}
		v, ok := new(big.Rat).SetString(u.ExactBezier[1][1])
		if !ok {
			t.Fatal("native owner coordinate", u)
		}
		u.ExactBezier[1][1] = v.Add(v, big.NewRat(1, 1<<40)).RatString()
		changed, e := json.Marshal(native)
		if e != nil {
			t.Fatal(e)
		}
		if s, e := ReadJSONContext(context.Background(), bytes.NewReader(changed), JSONLimits{}); e == nil || len(s.faces) != 0 {
			t.Fatal("changed native owner interior accepted", e)
		}
		return
	}
	t.Fatal("native owner trim not encoded")
}

func TestNativePolynomialNodeAtomicRejections(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativePolynomialNodeFixture(t, 0, 2)
	for _, opt := range []SplineGraphBooleanOptions{
		{PolynomialNode: &geom.GraphPointContactOptions{MaxOperations: 1}},
		{PolynomialNode: &geom.GraphPointContactOptions{MaxCells: 1}},
		{PolynomialNode: &geom.GraphPointContactOptions{}, PolynomialNodeAxis: 2},
		{PolynomialNode: &geom.GraphPointContactOptions{}, TwoPolynomials: &geom.GraphPointContactOptions{}},
	} {
		s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Union, 1e-9, opt)
		if e == nil || len(s.faces) != 0 || h.PolynomialNode != nil {
			t.Fatal("partial or ambiguous node result", e)
		}
	}
	base, a, b = nativePolynomialNodeFixture(t, 0, 3)
	o := SplineGraphBooleanOptions{PolynomialNode: &geom.GraphPointContactOptions{}}
	s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Union, 1e-9, o)
	if !errors.Is(e, geom.ErrUnrepresentable) || len(s.faces) != 0 || h.PolynomialNode != nil {
		t.Fatal("nonbinary native vertex accepted", e)
	}
	base, a, b = twoPolynomialFixture(t, 0)
	s, h, e = CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Union, 1e-9, o)
	if !errors.Is(e, geom.ErrConvergence) || len(s.faces) != 0 || h.PolynomialNode != nil {
		t.Fatal("ordered contacts accepted by the node mode", e)
	}
}
