package step

import (
	"bytes"
	"math"
	"math/big"
	"os"
	"plainsolid/geom"
	"reflect"
	"strings"
	"testing"
)

func TestPolynomialCylinderResidualIndependent(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		axis  geom.Vec3
		upper *big.Rat
	}{{geom.Vec3{X: 1}, big.NewRat(-1, 1)}, {geom.Vec3{Z: 1}, big.NewRat(0, 1)}, {geom.Vec3{X: 1, Y: 1}, big.NewRat(-1, 2)}} {
		r, e := BoundPolynomialSplineCylinderResidual(strings.NewReader(polynomialResidualFixture()), Limits{}, 3, geom.Vec3{}, test.axis, 1)
		if e != nil || len(r.Spans) != 1 || r.Spans[0].Lower.Cmp(big.NewRat(-1, 1)) != 0 || r.Spans[0].Upper.Cmp(test.upper) != 0 {
			t.Fatal(r, e)
		}
	}
	var expected PolynomialCylinderResidual
	for i, a := range []geom.Vec3{{X: 1, Y: 1}, {X: -2, Y: -2}, {X: 1e-200, Y: 1e-200}, {X: 1e200, Y: 1e200}} {
		r, e := BoundPolynomialSplineCylinderResidual(strings.NewReader(polynomialResidualFixture()), Limits{}, 3, geom.Vec3{}, a, 1)
		if e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			expected = r
		} else if !reflect.DeepEqual(r, expected) {
			t.Fatal("axis scaling changed exact projection")
		}
	}
	for _, a := range []geom.Vec3{{}, {X: math.Inf(1)}, {Z: math.NaN()}} {
		if _, e := BoundPolynomialSplineCylinderResidual(strings.NewReader(polynomialResidualFixture()), Limits{}, 3, geom.Vec3{}, a, 1); e == nil {
			t.Fatal("invalid axis accepted")
		}
	}
}
func TestPolynomialCylinderResidualOCCT(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	cylinder, e := BoundPolynomialSplineCylinderResidual(bytes.NewReader(raw), Limits{}, 25, geom.Vec3{X: 1, Z: -.75}, geom.Vec3{Z: 1}, 2)
	if e != nil {
		t.Fatal(e)
	}
	sphere, e := BoundPolynomialSplineSphereResidual(bytes.NewReader(raw), Limits{}, 25, geom.Vec3{}, 5)
	if e != nil {
		t.Fatal(e)
	}
	if len(cylinder.Spans) != 28 || cylinder.MaxAbsoluteResidual.Sign() <= 0 || cylinder.RadialDistanceUpper.Cmp(big.NewRat(8, 100000000)) >= 0 {
		t.Fatal("unexpected residual bound", cylinder.RadialDistanceUpper.FloatString(15))
	}
	for i, s := range sphere.Spans {
		if s.Start.Cmp(cylinder.Spans[i].Start) != 0 || s.End.Cmp(cylinder.Spans[i].End) != 0 {
			t.Fatal("different parameter coverage")
		}
	}
	t.Log("continuous cylinder distance upper bound", cylinder.RadialDistanceUpper.FloatString(15))
}
func FuzzPolynomialCylinderResidual(f *testing.F) {
	f.Add([]byte(polynomialResidualFixture()), 1., 1., 0.)
	f.Fuzz(func(t *testing.T, b []byte, x, y, z float64) {
		_, _ = BoundPolynomialSplineCylinderResidual(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 8192}, 3, geom.Vec3{}, geom.Vec3{X: x, Y: y, Z: z}, 1)
	})
}
