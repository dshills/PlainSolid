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

func threeEllipseGraphFixture(t *testing.T, four bool) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	base, _, b, _ := disjointEllipseGraphFixture(t)
	us := []float64{0, .125, .25, .375, .5, .75, 1}
	terms := []float64{1, -1, 1, -1, 1, -3, 9}
	knots := []float64{0, 0, 0, .25, .25, .5, .5, 1, 1, 1}
	if four {
		us = []float64{0, .125, .25, .375, .5, .625, .75, .875, 1}
		terms = []float64{1, -1, 1, -1, 1, -1, 1, -1, 1}
		knots = []float64{0, 0, 0, .25, .25, .5, .5, .75, .75, 1, 1, 1}
	}
	var poles []geom.Vec3
	for i, u := range us {
		for j, v := range []float64{0, .5, 1} {
			z := 3 + u*v/64 + 1.0/1024 - terms[i]/64 - []float64{.25, -.25, .25}[j]
			poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
		}
	}
	a, e := geom.NewBSplineSurface(2, 2, len(us), 3, poles, knots, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot(.0625, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, e = p.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	return base, a, b
}

func TestAutomaticThreeEllipseGraphBooleans(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	base, a, b := threeEllipseGraphFixture(t, false)
	for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
		for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
			s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{ThreeEllipses: &geom.ThreeEllipseDiscoveryOptions{}})
			if e != nil {
				t.Fatal(order, op, e)
			}
			if h.ThreeEllipses == nil || !h.ThreeEllipses.Complete || h.DisjointEllipses != nil || h.ThreeEllipses.SplitU != ([2]float64{.25, .5}) || h.ThreeEllipses.TestedPartitions != 3 || len(h.ContourEdges) != 12 {
				t.Fatal("incomplete original discovery", h)
			}
			shells, euler := 1, 2
			if op == parametric.Difference {
				shells, euler = 3, 6
				if order == 1 {
					shells, euler = 1, -4
				}
			}
			top := s.Topology()
			if len(top.Shells) != shells || top.EulerCharacteristic != euler {
				t.Fatal(top)
			}
			for owner, ids := range [][]int{h.FacesA, h.FacesB} {
				for _, id := range ids {
					if s.faces[id].Surface != pair[owner] {
						t.Fatal("original owner lost")
					}
				}
			}
			m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if e != nil || math.Abs(m.Volume-threeEllipseVolume(order, op)) > 2e-8 {
				t.Fatal(order, op, m.Volume, threeEllipseVolume(order, op), e)
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
				t.Fatal("restored topology")
			}
			if e = validateExactGraphPartitionEdges(ctx, loaded, geom.GraphPointContactOptions{}); e != nil {
				t.Fatal(e)
			}
			t.Log(order, op, top, m.Volume)
		}
	}
}

func TestAutomaticThreeEllipseLimitsAndMissingBranch(t *testing.T) {
	ctx := context.Background()
	base, a, b := threeEllipseGraphFixture(t, false)
	ap, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	bp, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	proof, e := geom.DiscoverSplineGraphThreeEllipses(ctx, ap, bp, geom.ThreeEllipseDiscoveryOptions{})
	if e != nil || !proof.Complete {
		t.Fatal(e)
	}
	var hints [3]geom.GraphEllipseHint
	for i := range hints {
		hints[i] = proof.Section.Contours[i].Ellipse
	}
	wrong := hints
	wrong[2] = hints[1]
	for _, tc := range []struct {
		splits [2]float64
		hints  [3]geom.GraphEllipseHint
	}{{[2]float64{.5, .25}, hints}, {[2]float64{.25, .5}, wrong}} {
		if r, e := geom.CertifySplineGraphThreeEllipses(ctx, ap, bp, tc.splits, tc.hints, geom.GraphPointContactOptions{}); e == nil || r.Complete {
			t.Fatal("invalid complete partition", e)
		}
	}
	for _, o := range []geom.ThreeEllipseDiscoveryOptions{{MaxSeparators: 2}, {MaxSeparators: 17}, {GraphPointContactOptions: geom.GraphPointContactOptions{MaxOperations: proof.ExactOperations - 1}}, {GraphPointContactOptions: geom.GraphPointContactOptions{MaxCells: 2}}} {
		if q, e := geom.DiscoverSplineGraphThreeEllipses(ctx, ap, bp, o); e == nil || q.Complete || q.Section.Complete {
			t.Fatal("atomic limits", e)
		}
	}
	if q, e := geom.DiscoverSplineGraphThreeEllipses(ctx, ap, bp, geom.ThreeEllipseDiscoveryOptions{GraphPointContactOptions: geom.GraphPointContactOptions{MaxOperations: proof.ExactOperations}}); e != nil || !q.Complete {
		t.Fatal("exact quota", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := geom.DiscoverSplineGraphThreeEllipses(cancelled, ap, bp, geom.ThreeEllipseDiscoveryOptions{}); e == nil || q.Complete {
		t.Fatal("cancel", e)
	}
	if s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Difference, 1e-9, SplineGraphBooleanOptions{ThreeEllipses: &geom.ThreeEllipseDiscoveryOptions{}, DisjointEllipses: &geom.DisjointEllipseDiscoveryOptions{}}); e == nil || len(s.faces) != 0 || h.ThreeEllipses != nil {
		t.Fatal("ambiguous selector", e)
	}
	base, a, b = threeEllipseGraphFixture(t, true)
	ap, _ = geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	bp, _ = geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	for _, center := range []float64{.125, .375, .625, .875} {
		u, v := new(big.Rat).SetFloat64(center+1.0/32), big.NewRat(1, 2)
		pa, ea := ap.EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
		pb, eb := bp.EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
		if ea != nil || eb != nil {
			t.Fatal(ea, eb)
		}
		for axis := range pa.Point {
			if pa.Point[axis].Cmp(pb.Point[axis]) != 0 {
				t.Fatal("fourth original contact witness")
			}
		}
	}

	if s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Difference, 1e-9, SplineGraphBooleanOptions{ThreeEllipses: &geom.ThreeEllipseDiscoveryOptions{}}); e == nil || len(s.faces) != 0 || h.ThreeEllipses != nil {
		t.Fatal("omitted fourth loop", e)
	}
}
