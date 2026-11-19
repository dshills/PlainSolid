package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func rationalGraphClipStock(t *testing.T, dependentChange float64) Solid {
	t.Helper()
	var faces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	k := []float64{0, 0, 0, 1, 1, 1}
	for axis := range 3 {
		u, v := (axis+1)%3, (axis+2)%3
		for side := range 2 {
			var poles []geom.Vec3
			for i := range 3 {
				for j := range 3 {
					p := [3]float64{}
					p[axis], p[u], p[v] = float64(side), float64(i)/2, float64(j)/2
					x2 := p[0] * p[0]
					if u == 0 && i == 1 || v == 0 && j == 1 {
						x2 = 0
					}
					z := p[2] + p[0]/16 + x2/32 + (1+dependentChange*p[0])*p[1]/4
					poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: z})
				}
			}
			s, err := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
			if err != nil {
				t.Fatal(err)
			}
			faces = append(faces, s)
			domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
			reversed = append(reversed, side == 0)
		}
	}
	s, _, err := SewSplineRectangles(context.Background(), faces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRationalGraphPlaneRejectsSingularDenominator(t *testing.T) {
	stock := rationalGraphClipStock(t, -2)
	ctx := context.Background()
	face, _, err := stock.splineEmbeddingFace(ctx, stock.faces[4], SplineRectangleEmbeddingOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = rationalGraphPlaneTrim(face.patch, horizontalPolynomialPlane(big.NewRat(3, 16)), 0); !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("accepted an interior denominator zero", err)
	}
}
