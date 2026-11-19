//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func nativeFactoredPolynomialFixture(t *testing.T, axis int, extraBranch ...bool) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	base, _, _ := polynomialGraphFixture(t, 3, axis)
	var out [2]*geom.BSplineSurface
	fg := []float64{.25, 17.0 / 32, 15.0 / 16, 39.0 / 32, 1.5}
	g0, g1 := 1.0, 3.0
	if len(extraBranch) > 0 && extraBranch[0] {
		g0, g1 = -.5, 1
		fg = []float64{-.125, -7.0 / 64, 0, 3.0 / 32, 3.0 / 16}
	}
	for owner := 0; owner < 2; owner++ {
		var poles []geom.Vec3
		for i := 0; i < 5; i++ {
			u := float64(i) / 4
			for j := 0; j < 3; j++ {
				v := float64(j) / 2
				z := 3 + u*v/64
				if owner == 0 {
					z += v*(g0+g1*u) - fg[i]
				}
				poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(4, 2, 5, 3, poles, []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		if axis == 1 {
			d := s.Data()
			poles = nil
			for j := 0; j < 3; j++ {
				for i := 0; i < 5; i++ {
					p := d.Poles[i*3+j]
					p.X, p.Y = p.Y, p.X
					poles = append(poles, p)
				}
			}
			s, e = geom.NewBSplineSurface(2, 4, 3, 5, poles, d.KnotsV, d.KnotsU, nil)
			if e != nil {
				t.Fatal(e)
			}
		}
		p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		k := .5
		if owner == 1 {
			k = .3125
		}
		if (owner == 0) != (axis == 1) {
			p, e = p.InsertUKnot(k, 1, geom.SurfacePatchOptions{})
		} else {
			p, e = p.InsertVKnot(k, 1, geom.SurfacePatchOptions{})
		}
		if e != nil {
			t.Fatal(e)
		}
		out[owner], e = p.ToBSplineSurface()
		if e != nil {
			t.Fatal(e)
		}
	}
	return base, out[0], out[1]
}

func TestNativeFactoredPolynomialBooleans(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, axis := range []int{0, 1} {
		base, a, b := nativeFactoredPolynomialFixture(t, axis)
		for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
				s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{FactoredPolynomial: &geom.GraphPointContactOptions{}, FactoredPolynomialAxis: axis})
				if e != nil {
					t.Fatal(axis, order, op, e)
				}
				if h.FactoredPolynomial == nil || !h.FactoredPolynomial.Proven || len(h.FactoredPolynomial.Factors) != 4 || len(h.ContourEdges) != 1 || h.Polynomial != nil {
					t.Fatal("factored ownership", h)
				}
				if len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
					t.Fatal(s.Topology())
				}
				for owner, ids := range [][]int{h.FacesA, h.FacesB} {
					for _, id := range ids {
						if s.faces[id].Surface != pair[owner] {
							t.Fatal("source replaced")
						}
					}
				}
				m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
				if e != nil || math.Abs(m.Volume-factoredPolynomialVolume(order, op)) > 3e-8 {
					t.Fatal(axis, order, op, m.Volume, e)
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
			}
		}
	}
}

func TestNativeFactoredPolynomialRejectsExtraBranch(t *testing.T) {
	for _, axis := range []int{0, 1} {
		base, a, b := nativeFactoredPolynomialFixture(t, axis, true)
		s, h, e := CombineSplineGraphSolidsWithOptions(context.Background(), base, a, b, parametric.Union, 1e-9, SplineGraphBooleanOptions{FactoredPolynomial: &geom.GraphPointContactOptions{MaxDepth: 4}, FactoredPolynomialAxis: axis})
		if e == nil || len(s.faces) > 0 || h.FactoredPolynomial != nil {
			t.Fatal("extra vertical branch published", axis, e)
		}
	}
}
