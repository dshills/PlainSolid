package trimmed

import (
	"context"
	"plainsolid/geom"
	"testing"
)

func makeCornerShearBox(t *testing.T, lo, hi [3]float64, warp float64) (Solid, []splineBoxFace) {
	t.Helper()
	var f []splineBoxFace
	var surfaces []*geom.BSplineSurface
	var d []geom.ParameterRectangle
	var rev []bool
	for axis := range 3 {
		u, v := (axis+1)%3, (axis+2)%3
		for side, x := range [2]float64{lo[axis], hi[axis]} {
			domain := geom.ParameterRectangle{UMin: lo[u], UMax: hi[u], VMin: lo[v], VMax: hi[v]}
			p := []geom.Vec3{}
			for _, a := range [2]float64{lo[u], hi[u]} {
				for _, b := range [2]float64{lo[v], hi[v]} {
					c := [3]float64{}
					c[axis], c[u], c[v] = x, a, b
					c[2] += .25 * (c[0] + c[1])
					p = append(p, geom.Vec3{X: c[0], Y: c[1], Z: c[2]})
				}
			}
			s, e := geom.NewBSplineSurface(1, 1, 2, 2, p, []float64{lo[u], lo[u], hi[u], hi[u]}, []float64{lo[v], lo[v], hi[v], hi[v]}, nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces, d, rev = append(surfaces, s), append(d, domain), append(rev, side == 0)
			f = append(f, splineBoxFace{axis, u, v, x, domain, s})
		}
	}
	s, _, e := SewSplineRectangles(context.Background(), surfaces, d, rev, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return s, f
}
