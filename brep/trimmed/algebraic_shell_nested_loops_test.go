package trimmed

import (
	"context"
	"testing"

	"plainsolid/geom"
)

func nestedQuarticRoofSolid(t *testing.T) Solid {
	t.Helper()
	return quarticRoofSolid(t, 25.0/1024, 25.0/256)
}

func quarticRoofSolid(t *testing.T, ri, ro float64) Solid {
	t.Helper()
	x2 := [5]float64{3, 0, -1, 0, 3}
	x4 := [5]float64{1, -1, 1, -1, 1}
	height := func(i, j int) float64 {
		return (9*(x4[i]+x4[j]) + 2*x2[i]*x2[j] - 12*(ri+ro)*(x2[i]+x2[j]) + 144*ri*ro) / 256
	}
	return quarticHeightRoofSolid(t, height)
}

func quarticHeightRoofSolid(t *testing.T, height func(i, j int) float64) Solid {
	t.Helper()
	knots := func(n int) []float64 {
		k := make([]float64, 2*n)
		for i := n; i < len(k); i++ {
			k[i] = 1
		}
		return k
	}
	var supports []*geom.BSplineSurface
	var reversed []bool
	add := func(nu, nv int, rev bool, at func(int, int) geom.Vec3) {
		var p []geom.Vec3
		for i := range nu {
			for j := range nv {
				p = append(p, at(i, j))
			}
		}
		s, e := geom.NewBSplineSurface(nu-1, nv-1, nu, nv, p, knots(nu), knots(nv), nil)
		if e != nil {
			t.Fatal(e)
		}
		supports, reversed = append(supports, s), append(reversed, rev)
	}
	add(2, 2, true, func(i, j int) geom.Vec3 { return geom.Vec3{X: float64(i), Y: float64(j), Z: -1} })
	add(5, 5, false, func(i, j int) geom.Vec3 { return geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: -height(i, j)} })
	for _, edge := range []int{0, 4} {
		add(5, 2, edge == 0, func(i, j int) geom.Vec3 {
			return geom.Vec3{X: float64(edge) / 4, Y: float64(i) / 4, Z: [2]float64{-1, -height(edge, i)}[j]}
		})
	}
	for _, edge := range []int{0, 4} {
		add(5, 2, edge != 0, func(i, j int) geom.Vec3 {
			return geom.Vec3{X: float64(i) / 4, Y: float64(edge) / 4, Z: [2]float64{-1, -height(i, edge)}[j]}
		})
	}
	s, _, e := SewSplineRectangles(context.Background(), supports, make([]geom.ParameterRectangle, len(supports)), reversed, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
