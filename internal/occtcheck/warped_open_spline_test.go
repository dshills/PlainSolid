package occtcheck

import (
	"context"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func oracleWarpedOpenBox(t *testing.T, lo, hi [3]float64) (trimmed.Solid, []oracleOpenFace) {
	t.Helper()
	var f []oracleOpenFace
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
					p = append(p, geom.Vec3{X: c[0], Y: c[1], Z: c[2] + c[0]*c[1]/8})
				}
			}
			s, e := geom.NewBSplineSurface(1, 1, 2, 2, p, []float64{lo[u], lo[u], hi[u], hi[u]}, []float64{lo[v], lo[v], hi[v], hi[v]}, nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces, d, rev = append(surfaces, s), append(d, domain), append(rev, side == 0)
			f = append(f, oracleOpenFace{axis, u, v, x, domain, s})
		}
	}
	s, _, e := trimmed.SewSplineRectangles(context.Background(), surfaces, d, rev, trimmed.SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return s, f
}

func TestOCCTWarpedOpenPlaneShells(t *testing.T) {
	refs := runOracle(t, "warped-open-spline")
	ctx := context.Background()
	al, ah := [3]float64{0, 0, 0}, [3]float64{1, 1, 1}
	bl, bh := [3]float64{.5, .5, -.5}, [3]float64{1.5, 1.5, .5}
	a, af := oracleWarpedOpenBox(t, al, ah)
	b, bf := oracleWarpedOpenBox(t, bl, bh)
	var pairs [][2]int
	for i, x := range af {
		for j, y := range bf {
			if x.axis != y.axis && x.fixed > bl[x.axis] && x.fixed < bh[x.axis] && y.fixed > al[y.axis] && y.fixed < ah[y.axis] {
				pairs = append(pairs, [2]int{i, j})
			}
		}
	}
	for _, tc := range []struct {
		name string
		op   parametric.BooleanOp
	}{{"warped-open-union", parametric.Union}, {"warped-open-common", parametric.Intersection}, {"warped-open-cut", parametric.Difference}} {
		t.Run(tc.name, func(t *testing.T) {
			s, history, e := trimmed.CombineOpenPlaneSplineShells(ctx, a, b, tc.op, pairs, intersect.SplinePlanePairDiscoveryOptions{}, trimmed.SplineShellOpenOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if len(history.Pairs) != 6 || s.Topology().EulerCharacteristic != 2 {
				t.Fatal("topology/ancestry")
			}
			mass, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			compare(t, refs[tc.name], mass.Area, mass.Volume, mass.Centroid)
		})
	}
}
