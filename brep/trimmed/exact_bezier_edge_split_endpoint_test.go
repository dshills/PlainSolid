package trimmed

import (
	"errors"
	"plainsolid/geom"
	"testing"
)

func TestExactSplineEndpointLimits(t *testing.T) {
	c, e := geom.NewBSpline(1, []geom.Vec3{{}, {X: 1}}, []float64{0, 0, 1, 1}, []float64{1, 2})
	if e != nil {
		t.Fatal(e)
	}
	q := SplineUV{Curve: c, Range: UVInterval{0, .5}}
	if _, e = q.WithExactEndpoints(); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("rounded nonbinary endpoint", e)
	}
	for _, degree := range []int{4, 5} {
		poles := make([]geom.Vec3, degree+1)
		knots := make([]float64, 2*(degree+1))
		for i := range poles {
			poles[i].X = float64(i)
			knots[i+degree+1] = 1
		}
		c, e := geom.NewBSpline(degree, poles, knots, nil)
		if e != nil {
			t.Fatal(e)
		}
		q := SplineUV{Curve: c, Range: UVInterval{0, 1}}
		_, e = q.WithExactEndpoints()
		if (e != nil) != (degree > 4) {
			t.Fatal("degree gate", degree, e)
		}
	}
}
