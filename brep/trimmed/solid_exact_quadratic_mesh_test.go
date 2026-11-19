package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func solidEllipticalLensFixture(t *testing.T) Solid {
	t.Helper()
	graph := func(z, f float64) *geom.BSplineSurface {
		var poles []geom.Vec3
		term := []float64{.25, -.25, .25}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z + f*(1./16-term[i]-4*term[j])})
			}
		}
		knots := []float64{0, 0, 0, 1, 1, 1}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	hint := geom.GraphEllipseHint{Center: [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, RadiusSquared: [2]*big.Rat{big.NewRat(1, 16), big.NewRat(1, 64)}}
	s, _, e := CombineEllipticalContactSplineGraphs(context.Background(), graph(0, 0), graph(2, 1), graph(2, 0), parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}, hint, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
