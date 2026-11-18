package trimmed

import (
	"context"
	"errors"
	"testing"

	"plainsolid/geom"
)

func TestRoofPowerRejectsGenuinelyPiecewiseChart(t *testing.T) {
	// X=u,Y=v is exact; the two spans have different height slopes.
	ctx := context.Background()
	var poles []geom.Vec3
	for i, x := range []float64{0, .5, 1} {
		for _, y := range []float64{0, 1} {
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: []float64{1, 2, 1}[i]})
		}
	}
	s, err := geom.NewBSplineSurface(1, 1, 3, 2, poles, []float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if coeff, weight, work, err := mixedRoofWholePower(ctx, p, p.Domain(), 10000); !errors.Is(err, geom.ErrUnrepresentable) || coeff != nil || weight != nil || work == 0 {
		t.Fatal("distinct source spans cannot use a single polynomial", coeff, weight, work, err)
	}
}
