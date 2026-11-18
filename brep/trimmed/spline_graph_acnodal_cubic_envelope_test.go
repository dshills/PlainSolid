package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func acnodalCubicStockFixture(t *testing.T, separateArc bool) [3]*geom.BSplineSurface {
	t.Helper()
	// A−B=(3/16)*((v-1/2)^2-(u-13/16)^2*(u-13/16-gap)).
	// These are the exact Bernstein controls of the free-coordinate term.
	if separateArc {
		return nodalCubicStockFixture(t, [4]float64{-7605.0 / 65536, 1339.0 / 65536, -213.0 / 65536, 27.0 / 65536}, 3.0/64)
	}
	return nodalCubicStockFixture(t, [4]float64{-8619.0 / 65536, 1157.0 / 65536, -75.0 / 65536, -27.0 / 65536}, 3.0/64)
}

func TestNativeAcnodalCubicEnvelopeOtherStockSides(t *testing.T) {
	ctx := context.Background()
	stock := acnodalCubicStockFixture(t, true)
	for name, place := range map[string]func(geom.Vec3) geom.Vec3{
		"left":  func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: 3 - p.X, Y: p.Y, Z: p.Z} },
		"upper": func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: p.Y, Y: p.X, Z: p.Z} },
		"lower": func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: p.Y, Y: 3 - p.X, Z: p.Z} },
	} {
		t.Run(name, func(t *testing.T) {
			var source [3]*geom.BSplineSurface
			for i, patch := range stock {
				d := patch.Data()
				for j, p := range d.Poles {
					d.Poles[j] = place(p)
				}
				var err error
				source[i], err = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
				if err != nil {
					t.Fatal(err)
				}
			}
			out, err := CommonAcnodalCubicSplineGraphs(ctx, source[0], source[1], source[2], 0, 1e-9, AcnodalCubicEnvelopeOptions{})
			if err != nil {
				t.Fatal(err)
			}
			s, h := out.Boundary(), out.History()
			for i, vertex := range [3]geom.Vec3{
				{X: 45.0 / 16, Y: 1, Z: 2273.0 / 1024},
				{X: 3, Y: 29.0 / 32, Z: 4567.0 / 2048},
				{X: 3, Y: 35.0 / 32, Z: 4585.0 / 2048},
			} {
				if s.vertices[h.ArcVertices[i]] != place(vertex) {
					t.Fatal("placement changed original branch occurrence", i, s.vertices[h.ArcVertices[i]], place(vertex))
				}
			}
			m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil || math.Abs(m.Volume-19907538337.0/1614807040) > 1e-9 {
				t.Fatal("axis permutation or reflection changed signed material", m, err)
			}
		})
	}
}
