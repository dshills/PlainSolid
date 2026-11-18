//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func twoPolynomialFixture(t *testing.T, axis int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{big.NewRat(5, 8), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func TestNativeTwoPolynomialGraphs(t *testing.T) {
	slowtest.Skip(t)
	for _, axis := range []int{0, 1} {
		base, a, b := twoPolynomialFixture(t, axis)
		for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
				s, h, e := CombineSplineGraphSolidsWithOptions(context.Background(), base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{TwoPolynomials: &geom.GraphPointContactOptions{}, TwoPolynomialsAxis: axis})
				if e != nil {
					t.Fatal(axis, order, op, e)
				}
				if h.TwoPolynomials == nil || !h.TwoPolynomials.Proven || len(h.ContourEdges) != 2 {
					t.Fatal("both original branches", h)
				}
				if op == parametric.Difference && order == 1 {
					if len(h.WallFaces) != 4 || len(h.DomainSides) != 4 {
						t.Fatal("original two-ended wall subdivision", h.WallFaces, h.DomainSides)
					}
					counts := map[int]int{}
					for _, side := range h.DomainSides {
						counts[side]++
					}
					wantSides := []int{1, 3}
					if axis == 1 {
						wantSides = []int{0, 2}
					}
					for _, side := range wantSides {
						if counts[side] != 2 {
							t.Fatal("original side ownership", counts)
						}
					}
				}
				shells := 1
				if op == parametric.Difference && order == 0 {
					shells = 2
				}
				if len(s.Topology().Shells) != shells || s.Topology().EulerCharacteristic != 2*shells {
					t.Fatal(s.Topology())
				}
				for owner, ids := range [][]int{h.FacesA, h.FacesB} {
					for _, id := range ids {
						if s.faces[id].Reversed != (owner == 1 && op == parametric.Difference) {
							t.Fatal("original signed roof ownership")
						}
						if s.faces[id].Surface != pair[owner] {
							t.Fatal("source replaced")
						}
					}
				}
				m, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
				if e != nil || math.Abs(m.Volume-twoPolynomialVolume(order, op)) > 2e-7 {
					t.Fatal(axis, order, op, m.Volume, twoPolynomialVolume(order, op), e)
				}
				var buf bytes.Buffer
				if e = WriteJSON(&buf, s); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadJSONContext(context.Background(), &buf, JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				if e = validateExactGraphPartitionEdges(context.Background(), loaded, geom.GraphPointContactOptions{}); e != nil {
					t.Fatal(e)
				}
			}
		}
	}
}

func TestNativeTwoPolynomialAtomicLimits(t *testing.T) {
	base, a, b := twoPolynomialFixture(t, 0)
	for _, o := range []SplineGraphBooleanOptions{
		{TwoPolynomials: &geom.GraphPointContactOptions{MaxOperations: 1}},
		{TwoPolynomials: &geom.GraphPointContactOptions{MaxCells: 1}},
		{TwoPolynomials: &geom.GraphPointContactOptions{}, TwoPolynomialsAxis: 2},
		{TwoPolynomials: &geom.GraphPointContactOptions{}, Polynomial: &geom.GraphPointContactOptions{}},
	} {
		s, h, e := CombineSplineGraphSolidsWithOptions(context.Background(), base, a, b, parametric.Union, 1e-9, o)
		if e == nil || len(s.faces) != 0 || h.TwoPolynomials != nil {
			t.Fatal("partial publication", e)
		}
	}
}
