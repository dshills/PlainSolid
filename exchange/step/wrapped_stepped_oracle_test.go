package step

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

// These binary-rational quarter weights represent an exact circle without
// irrational rounded sqrt weights. OCCT independently uses analytic cylinders.
func steppedOracleCylinder(t *testing.T, radius, lo, hi float64) trimmed.Solid {
	t.Helper()
	q := [][2]float64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	ws := []float64{1, 1, 2, 2, 4, 2, 2, 1, 1}
	var poles []geom.Vec3
	var weights []float64
	for i, p := range q {
		for _, z := range []float64{lo, hi} {
			poles = append(poles, geom.Vec3{X: radius * p[0], Y: radius * p[1], Z: z})
			weights = append(weights, ws[i])
		}
	}
	support, e := geom.NewBSplineSurface(2, 1, 9, 2, poles, []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}, []float64{0, 0, 1, 1}, weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(support, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(context.Background(), p, geom.WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, proof, e := trimmed.NewWrappedCylinder(context.Background(), w, 1e-10, geom.SurfacePatchOptions{})
	if e != nil || !proof.Circular {
		t.Fatal(proof, e)
	}
	return s
}
func TestWrappedSteppedIndependentOCCT(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := steppedOracleCylinder(t, 2, 0, 4)
	b := steppedOracleCylinder(t, 1, 2, 6)
	for _, tc := range []struct {
		name            string
		op              parametric.BooleanOp
		volume, area, z float64
		faces           int
	}{{"union", parametric.Union, 18 * math.Pi, 28 * math.Pi, 7.0 / 3, 5}, {"intersection", parametric.Intersection, 2 * math.Pi, 6 * math.Pi, 3, 3}, {"difference", parametric.Difference, 14 * math.Pi, 28 * math.Pi, 13.0 / 7, 5}} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, e := trimmed.CombineAxialWrappedCylinders(ctx, a, b, tc.op, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			top := s.Topology()
			if top.Faces != tc.faces || len(top.Shells) != 1 || top.EulerCharacteristic != 2 {
				t.Fatal(top)
			}
			mass, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(mass.Volume-tc.volume) > 1e-6 || math.Abs(mass.Area-tc.area) > 1e-6 || math.Abs(mass.Centroid.Z-tc.z) > 1e-7 {
				t.Fatal(mass, tc)
			}
			source := t.TempDir() + "/shape.step"
			stepOracle(t, source, "write-step-wrapped-stepped-"+tc.name)
			out := t.TempDir() + "/mass.json"
			stepOracle(t, out, "step-mass-precise", source)
			raw, e := os.ReadFile(out)
			if e != nil {
				t.Fatal(e)
			}
			var reference struct {
				Area, Volume float64
				Centroid     [3]float64
			}
			if e = json.Unmarshal(raw, &reference); e != nil {
				t.Fatal(e)
			}
			if math.Abs(mass.Volume-reference.Volume) > 1e-6 || math.Abs(mass.Area-reference.Area) > 1e-6 || mass.Centroid.Distance(geom.Vec3{X: reference.Centroid[0], Y: reference.Centroid[1], Z: reference.Centroid[2]}) > 1e-7 {
				t.Fatal(mass, reference)
			}
		})
	}
}
