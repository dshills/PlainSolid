//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func nativeBivariatePolynomialFixture(t *testing.T, axis int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	base, _, _ := polynomialGraphFixture(t, 3, axis)
	var out [2]*geom.BSplineSurface
	fg := []float64{.25, 13.0 / 32, 9.0 / 16, 21.0 / 32, .75}
	f := []float64{.25, 11.0 / 32, 3.0 / 8, 3.0 / 8, 3.0 / 8}
	for owner := 0; owner < 2; owner++ {
		var poles []geom.Vec3
		for i := 0; i < 5; i++ {
			u := float64(i) / 4
			for j := 0; j < 3; j++ {
				v := float64(j) / 2
				z := 3 + u*v/64
				if owner == 0 {
					z += v*(1+u-f[i]) - fg[i]
					if j == 2 {
						z++
					}
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

func TestNativeBivariatePolynomialBooleans(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []int{0, 1} {
		base, a, b := nativeBivariatePolynomialFixture(t, axis)
		for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
				s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{BivariatePolynomial: &geom.GraphPointContactOptions{}, BivariatePolynomialAxis: axis})
				if e != nil {
					t.Fatal(axis, order, op, e)
				}
				if h.BivariatePolynomial == nil || !h.BivariatePolynomial.Proven || len(h.BivariatePolynomial.Factors) != 4 || len(h.ContourEdges) != 1 || h.Polynomial != nil {
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
				if e != nil || math.Abs(m.Volume-bivariatePolynomialVolume(order, op)) > 3e-8 {
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

func TestNativeBivariatePolynomialAtomicRejects(t *testing.T) {
	base, a, b := nativeBivariatePolynomialFixture(t, 0)
	for _, proof := range []geom.GraphPointContactOptions{{MaxOperations: 1}, {MaxCells: 7}} {
		s, h, e := CombineSplineGraphSolidsWithOptions(context.Background(), base, a, b, parametric.Union, 1e-9, SplineGraphBooleanOptions{BivariatePolynomial: &proof})
		if e == nil || len(s.faces) > 0 || h.BivariatePolynomial != nil {
			t.Fatal("partial native output", e)
		}
	}
	d := a.Data()
	d.Poles[len(d.Poles)-1].Z += 1.0 / 1024
	changed, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	s, h, e := CombineSplineGraphSolidsWithOptions(context.Background(), base, changed, b, parametric.Union, 1e-9, SplineGraphBooleanOptions{BivariatePolynomial: &geom.GraphPointContactOptions{}})
	if e == nil || len(s.faces) > 0 || h.BivariatePolynomial != nil {
		t.Fatal("altered original tensor accepted", e)
	}
}
