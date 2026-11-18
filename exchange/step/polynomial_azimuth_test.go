package step

import (
	"bytes"
	"math/big"
	"os"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestPolynomialAzimuthIndependent(t *testing.T) {
	t.Parallel()
	raw := strings.Replace(polynomialResidualFixture(), "(0.,0.,0.)", "(0.,1.,0.)", 1)
	for _, sign := range []int{-1, 1} {
		source := raw
		if sign == 1 {
			source = strings.Replace(source, "(0.,1.,0.)", "(0.,-1.,0.)", 1)
		}
		r, e := BoundPolynomialSplineAzimuth(strings.NewReader(source), Limits{}, 3, geom.Vec3{})
		if e != nil {
			t.Fatal(e)
		}
		if r.Positive != (sign > 0) || r.Negative != (sign < 0) || r.Closed || r.ProjectedClosed || r.Spans[0].Lower.Cmp(big.NewRat(int64(sign), 1)) != 0 || r.Spans[0].Upper.Cmp(big.NewRat(int64(sign), 1)) != 0 {
			t.Fatal(r)
		}
	}
	r, e := BoundPolynomialSplineAzimuth(strings.NewReader(polynomialResidualFixture()), Limits{}, 3, geom.Vec3{})
	if e != nil || r.Positive || r.Negative {
		t.Fatal("radial line certified angular", r, e)
	}
	raw = strings.Replace(polynomialResidualFixture(), "(1.,0.,0.)", "(0.,0.,0.)", 1)
	r, e = BoundPolynomialSplineAzimuth(strings.NewReader(raw), Limits{}, 3, geom.Vec3{})
	if e != nil || !r.Closed || !r.ProjectedClosed || r.EndpointDistanceSquared.Sign() != 0 {
		t.Fatal(r, e)
	}
}
func TestPolynomialAzimuthOCCT(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	r, e := BoundPolynomialSplineAzimuth(bytes.NewReader(raw), Limits{}, 25, geom.Vec3{X: 1, Z: -.75})
	if e != nil {
		t.Fatal(e)
	}
	if !r.Positive || r.Negative || len(r.Spans) != 28 || !r.Closed || !r.ProjectedClosed || r.EndpointDistanceSquared.Sign() != 0 {
		t.Fatal("no positive angular proof", r.Positive, r.Negative)
	}
	t.Log("closure", r.Closed, "projected", r.ProjectedClosed, "endpoint squared distance", r.EndpointDistanceSquared.RatString())
}
func FuzzPolynomialAzimuth(f *testing.F) {
	f.Add([]byte(polynomialResidualFixture()))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = BoundPolynomialSplineAzimuth(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 8192}, 3, geom.Vec3{})
	})
}
