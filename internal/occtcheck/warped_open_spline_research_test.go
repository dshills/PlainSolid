//go:build research

package occtcheck

import (
	"context"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/intersect"
)

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
