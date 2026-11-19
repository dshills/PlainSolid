package step

import (
	"bytes"
	"math/big"
	"os"
	"plainsolid/geom"
	"strings"
	"testing"
)

func polynomialResidualFixture() string {
	return "ISO-10303-21;HEADER;FILE_DESCRIPTION(('test'),'2;1');FILE_NAME('','','',(''),'','','');FILE_SCHEMA(('AUTOMOTIVE_DESIGN'));ENDSEC;DATA;#1=CARTESIAN_POINT('',(0.,0.,0.));#2=CARTESIAN_POINT('',(1.,0.,0.));#3=B_SPLINE_CURVE_WITH_KNOTS('',1,(#1,#2),.UNSPECIFIED.,.F.,.F.,(2,2),(0.,1.),.UNSPECIFIED.);ENDSEC;END-ISO-10303-21;"
}
func TestPolynomialSphereResidualIndependent(t *testing.T) {
	t.Parallel()
	raw := polynomialResidualFixture()
	r, e := BoundPolynomialSplineSphereResidual(strings.NewReader(raw), Limits{}, 3, geom.Vec3{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Spans) != 1 || r.Spans[0].Lower.Cmp(big.NewRat(-1, 1)) != 0 || r.Spans[0].Upper.Sign() != 0 || r.RadialDistanceUpper.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(r)
	}
	// The line t -> (t,0,0) gives exactly t²-1, Bernstein controls [-1,-1,0].
	r, e = BoundPolynomialSplineSphereResidual(strings.NewReader(raw), Limits{}, 3, geom.Vec3{X: 2}, 1)
	if e != nil || r.Spans[0].Lower.Sign() != 0 || r.Spans[0].Upper.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal(r, e)
	}
	for _, bad := range []string{strings.Replace(raw, "(2,2)", "(1,2)", 1), strings.Replace(raw, "(0.,1.)", "(1.,0.)", 1), strings.Replace(raw, "(1.,0.,0.)", "(1.E999999999,0.,0.)", 1)} {
		if _, e = BoundPolynomialSplineSphereResidual(strings.NewReader(bad), Limits{}, 3, geom.Vec3{}, 1); e == nil {
			t.Fatal("accepted invalid source")
		}
	}
}
func TestPolynomialSphereResidualPartitioned(t *testing.T) {
	t.Parallel()
	raw := polynomialResidualFixture()
	raw = strings.Replace(raw, "#3=B_SPLINE", "#4=CARTESIAN_POINT('',(0.,0.,0.));#3=B_SPLINE", 1)
	raw = strings.Replace(raw, "(#1,#2)", "(#1,#2,#4)", 1)
	raw = strings.Replace(raw, "(2,2),(0.,1.)", "(2,1,2),(0.,0.5,1.)", 1)
	r, e := BoundPolynomialSplineSphereResidual(strings.NewReader(raw), Limits{}, 3, geom.Vec3{}, 1)
	if e != nil || len(r.Spans) != 2 {
		t.Fatal(r, e)
	}
	for _, span := range r.Spans {
		if span.Lower.Cmp(big.NewRat(-1, 1)) != 0 || span.Upper.Sign() != 0 {
			t.Fatal(span)
		}
	}
	raw = polynomialResidualFixture()
	raw = strings.Replace(raw, "#3=B_SPLINE", "#4=CARTESIAN_POINT('',(0.5,0.,0.));#5=CARTESIAN_POINT('',(1.,1.,0.));#3=B_SPLINE", 1)
	raw = strings.Replace(raw, "('',1,(#1,#2)", "('',2,(#1,#4,#5)", 1)
	raw = strings.Replace(raw, "(2,2),(0.,1.)", "(3,3),(0.,1.)", 1)
	r, e = BoundPolynomialSplineSphereResidual(strings.NewReader(raw), Limits{}, 3, geom.Vec3{}, 1)
	if e != nil || r.Spans[0].Lower.Cmp(big.NewRat(-1, 1)) != 0 || r.Spans[0].Upper.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(r, e)
	}
}

func TestPolynomialSphereResidualOCCT(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	r, e := BoundPolynomialSplineSphereResidual(bytes.NewReader(raw), Limits{}, 25, geom.Vec3{}, 5)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Spans) != 28 || r.MaxAbsoluteResidual.Sign() <= 0 || r.RadialDistanceUpper.Cmp(big.NewRat(3, 100000000)) >= 0 {
		t.Fatal("unexpected continuous bound", len(r.Spans), r.RadialDistanceUpper.FloatString(15))
	}
	t.Log("exact Bernstein radial-distance upper bound", r.RadialDistanceUpper.FloatString(15))
}
func FuzzPolynomialSphereResidual(f *testing.F) {
	f.Add([]byte(polynomialResidualFixture()))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = BoundPolynomialSplineSphereResidual(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 8192}, 3, geom.Vec3{}, 1)
	})
}
