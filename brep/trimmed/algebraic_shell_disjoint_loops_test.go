package trimmed

import (
	"context"
	"testing"

	"plainsolid/geom"
)

// One connected rectangular shell has two quadratic roof lobes. The C0
// seam at u=1/2 is below the lower face of the other shell. Each positive
// lobe projects to a disjoint disk of radius 5/64; its geometry does not
// depend on the hints sent to the contact solver.
func disjointLoopRoof(t *testing.T) Solid {
	t.Helper()
	x := [5]float64{.125, .3125, .5, .6875, .875}
	y := [3]float64{.25, .5, .75}
	x2 := [5]float64{9.0 / 256, -9.0 / 256, 9.0 / 256, -9.0 / 256, 9.0 / 256}
	y2 := [3]float64{1.0 / 16, -1.0 / 16, 1.0 / 16}
	const floor = -.5
	height := func(i, j int) float64 { return 25.0/4096 - x2[i] - y2[j] }
	xk := []float64{x[0], x[0], x[0], x[2], x[2], x[4], x[4], x[4]}
	yk := []float64{y[0], y[0], y[0], y[2], y[2], y[2]}
	xl, yl, zk := []float64{x[0], x[0], x[4], x[4]}, []float64{y[0], y[0], y[2], y[2]}, []float64{0, 0, 1, 1}
	var supports []*geom.BSplineSurface
	var reversed []bool
	add := func(du, dv, nu, nv int, uk, vk []float64, rev bool, at func(int, int) geom.Vec3) {
		var p []geom.Vec3
		for i := range nu {
			for j := range nv {
				p = append(p, at(i, j))
			}
		}
		s, e := geom.NewBSplineSurface(du, dv, nu, nv, p, uk, vk, nil)
		if e != nil {
			t.Fatal(e)
		}
		supports, reversed = append(supports, s), append(reversed, rev)
	}
	add(1, 1, 2, 2, xl, yl, true, func(i, j int) geom.Vec3 { return geom.Vec3{X: x[4*i], Y: y[2*j], Z: floor} })
	add(2, 2, 5, 3, xk, yk, false, func(i, j int) geom.Vec3 { return geom.Vec3{X: x[i], Y: y[j], Z: height(i, j)} })
	for _, edge := range []int{0, 4} {
		add(2, 1, 3, 2, yk, zk, edge == 0, func(i, j int) geom.Vec3 {
			return geom.Vec3{X: x[edge], Y: y[i], Z: [2]float64{floor, height(edge, i)}[j]}
		})
	}
	for _, edge := range []int{0, 2} {
		add(2, 1, 5, 2, xk, zk, edge != 0, func(i, j int) geom.Vec3 {
			return geom.Vec3{X: x[i], Y: y[edge], Z: [2]float64{floor, height(i, edge)}[j]}
		})
	}
	s, _, e := SewSplineRectangles(context.Background(), supports, make([]geom.ParameterRectangle, len(supports)), reversed, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
