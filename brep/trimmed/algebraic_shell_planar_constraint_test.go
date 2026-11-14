package trimmed

import (
	"context"
	"errors"
	"plainsolid/geom"
	"testing"
)

func TestNonAffinePlanarShellConstraint(t *testing.T) {
	ctx := context.Background()
	makePatch := func(bump float64) *geom.SurfacePatch {
		var poles []geom.Vec3
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				u, v := float64(i)/2, float64(j)/2
				z := 0.
				if i == 1 && j == 1 {
					z = bump
				}
				poles = append(poles, geom.Vec3{X: u, Y: v * (1 + u/2), Z: z})
			}
		}
		k := []float64{0, 0, 0, 1, 1, 1}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	p := makePatch(0)
	if _, e := geom.NewAffineSurfaceChart(ctx, p, geom.AffineSurfaceChartOptions{}); e == nil {
		t.Fatal("fixture is affine")
	}
	plane, e := exactPlanarShellConstraint(ctx, p, geom.SurfaceCurveAgreementOptions{})
	if e != nil || plane[0].Sign() != 0 || plane[1].Sign() != 0 || plane[2].Sign() == 0 || plane[3].Sign() != 0 {
		t.Fatal("nonlinear plane", e)
	}
	// The same three corner witnesses do not certify an interior perturbation.
	if _, e := exactPlanarShellConstraint(ctx, makePatch(1.0/1024), geom.SurfaceCurveAgreementOptions{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("corner-only plane accepted", e)
	}
	if _, e := exactPlanarShellConstraint(ctx, p, geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := exactPlanarShellConstraint(canceled, p, geom.SurfaceCurveAgreementOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	k := []float64{0, 0, 1, 1}
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, []float64{1, 1, 2, 2})
	if e != nil {
		t.Fatal(e)
	}
	weighted, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	weighted, e = weighted.Trim(geom.ParameterRectangle{UMin: .5, UMax: 1, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := exactPlanarShellConstraint(ctx, weighted, geom.SurfaceCurveAgreementOptions{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("rounded nonbinary plane proposal", e)
	}
}
