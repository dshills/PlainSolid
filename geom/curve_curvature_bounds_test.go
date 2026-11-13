package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestCertifiedCurveCurvatureCircleAndParameterScale(t *testing.T) {
	for _, domain := range [][2]float64{{0, 1}, {2, 6}} {
		c, _ := NewBSpline(2, []Vec3{{2, 0, 0}, {2, 2, 0}, {0, 2, 0}}, []float64{domain[0], domain[0], domain[0], domain[1], domain[1], domain[1]}, []float64{1, math.Sqrt(.5), 1})
		r, e := BoundCurveCurvature(c, CurveDerivativeBoundOptions{MaxParameterWidth: (domain[1] - domain[0]) / 16})
		if e != nil || !r.Complete || !r.AllRegular {
			t.Fatal(r, e)
		}
		for _, cell := range r.Cells {
			if cell.CurvatureLower > .5+1e-12 || cell.CurvatureUpper < .5-1e-12 || !finite(cell.CurvatureUpper) || cell.SpeedLower <= 0 {
				t.Fatal(cell)
			}
		}
	}
}
func TestCertifiedCurveCurvatureLinearStationaryAndUnresolved(t *testing.T) {
	c, _ := NewBezier([]Vec3{{}, {3, 4, 0}}, nil)
	r, e := BoundCurveCurvature(c, CurveDerivativeBoundOptions{})
	if e != nil || !r.AllRegular || r.Cells[0].CurvatureLower != 0 || r.Cells[0].CurvatureUpper != 0 || r.Cells[0].SpeedLower != 5 {
		t.Fatal(r, e)
	}
	c, _ = NewBezier([]Vec3{{1, 2, 3}, {1, 2, 3}}, nil)
	r, e = BoundCurveCurvature(c, CurveDerivativeBoundOptions{})
	if e != nil || !r.Complete || r.AllRegular || !r.Cells[0].IdenticallyStationary || !math.IsInf(r.Cells[0].CurvatureUpper, 1) {
		t.Fatal(r, e)
	}
	c, _ = NewBezier([]Vec3{{1, 0, 0}, {-1, 0, 0}, {1, 0, 0}}, nil)
	r, e = BoundCurveCurvature(c, CurveDerivativeBoundOptions{})
	if e != nil || !r.Complete || r.AllRegular || r.Cells[0].ProvenRegular || r.Cells[0].IdenticallyStationary {
		t.Fatal(r, e)
	}
}
func TestCertifiedCurveCurvatureLimitsAndIso(t *testing.T) {
	c, _ := NewBezier([]Vec3{{}, {1, 2, 0}, {2, 0, 0}}, nil)
	_, e := BoundCurveCurvature(c, CurveDerivativeBoundOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = BoundCurveCurvatureContext(ctx, c, CurveDerivativeBoundOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	iso := isoIntersectionFixture(t, c)
	r, e := iso.BoundCurvature(CurveDerivativeBoundOptions{MaxParameterWidth: .125})
	if e != nil || !r.AllRegular {
		t.Fatal(r, e)
	}
}
