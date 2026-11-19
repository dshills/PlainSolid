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

func TestNativePolynomialGraphBooleans(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, degree := range []int{3, 4} {
		for _, axis := range []int{0, 1} {
			base, a, b := polynomialGraphFixture(t, degree, axis)
			for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
					s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{Polynomial: &geom.GraphPointContactOptions{}, PolynomialAxis: axis})
					if e != nil {
						t.Fatal(degree, axis, order, op, e)
					}
					if h.Polynomial == nil || !h.Polynomial.Proven || h.Polynomial.Degree != degree || h.Polynomial.FreeAxis != axis || len(h.ContourEdges) != 1 {
						t.Fatal(h)
					}
					if degree == 3 && h.Polynomial.Controls[1][axis].Cmp(big.NewRat(1, 3)) != 0 {
						t.Fatal("nonbinary exact cubic control lost")
					}
					found := 0
					for _, v := range s.vertices {
						free, dependent := v.X, v.Y
						if axis == 1 {
							free, dependent = dependent, free
						}
						if free == 1 && dependent == .375 && v.Z > 0 {
							found++
							if v.Z != 1539.0/512 {
								t.Fatal("rounded original wall endpoint", v)
							}
						}
					}
					if found != 1 {
						t.Fatal("shared exact terminal ownership", found)
					}
					if len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
						t.Fatal(s.Topology())
					}
					for owner, ids := range [][]int{h.FacesA, h.FacesB} {
						for _, id := range ids {
							if s.faces[id].Surface != pair[owner] {
								t.Fatal("original support replaced")
							}
						}
					}
					m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
					if e != nil || math.Abs(m.Volume-polynomialGraphVolume(degree, order, op)) > 3e-8 {
						t.Fatal(degree, axis, order, op, m.Volume, e)
					}
					var buf bytes.Buffer
					if e = WriteJSON(&buf, s); e != nil {
						t.Fatal(e)
					}
					loaded, e := ReadJSONContext(ctx, &buf, JSONLimits{})
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
}

func TestPolynomialGraphScopeAndQuota(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []int{0, 1} {
		base, a, b := polynomialGraphFixture(t, 3, axis)
		ap, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
		bp, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
		r, e := geom.DiscoverSplineGraphPolynomialContact(ctx, ap, bp, axis, geom.GraphPointContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
			q, e := geom.DiscoverSplineGraphPolynomialContact(ctx, ap, bp, axis, geom.GraphPointContactOptions{MaxOperations: limit})
			if (e == nil) != (limit == r.ExactOperations) || (e != nil && (q.Proven || len(q.Controls) > 0)) {
				t.Fatal("atomic quota", e)
			}
		}
		if q, e := geom.DiscoverSplineGraphParabolaOnAxis(ctx, ap, bp, axis, geom.GraphPointContactOptions{}); e == nil || q.Proven {
			t.Fatal("parabola scope widened")
		}
		if q, e := geom.DiscoverSplineGraphPolynomialContact(ctx, ap, bp, 1-axis, geom.GraphPointContactOptions{}); e == nil || q.Proven {
			t.Fatal("wrong axis")
		}
		d := a.Data()
		d.Poles[len(d.Poles)-1].Z += 1.0 / 1024
		changed, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, changed, b, parametric.Union, 1e-9, SplineGraphBooleanOptions{Polynomial: &geom.GraphPointContactOptions{}, PolynomialAxis: axis})
		if e == nil || len(s.faces) > 0 || h.Polynomial != nil {
			t.Fatal("later mixed term omitted")
		}
	}
	_, a, b := openParabolicGraphFixture(t, [3]float64{.25, .5, .25}, false)
	ap, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	bp, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if q, e := geom.DiscoverSplineGraphPolynomialContact(ctx, ap, bp, 0, geom.GraphPointContactOptions{}); e == nil || q.Proven {
		t.Fatal("not genuine cubic/quartic")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := geom.DiscoverSplineGraphPolynomialContact(canceled, ap, bp, 0, geom.GraphPointContactOptions{}); e == nil || q.Proven {
		t.Fatal("canceled proof")
	}
}
