package step

import (
	"bytes"
	"fmt"
	"os"
	"plainsolid/geom"
	"strings"
	"testing"
)

func windingPolygon(turns int) string {
	n := turns
	if n < 0 {
		n = -n
	}
	points := [][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}
	if turns < 0 {
		points = [][2]int{{1, 0}, {0, -1}, {-1, 0}, {0, 1}}
	}
	var b strings.Builder
	b.WriteString(strings.Split(polynomialResidualFixture(), "DATA;")[0] + "DATA;")
	refs, knots, mults := []string{}, []string{}, []string{}
	for i := 0; i <= 4*n; i++ {
		p := points[i%4]
		fmt.Fprintf(&b, "#%d=CARTESIAN_POINT('',(%d.,%d.,0.));", i+1, p[0], p[1])
		refs = append(refs, fmt.Sprintf("#%d", i+1))
		knots = append(knots, fmt.Sprint(i))
		m := "1"
		if i == 0 || i == 4*n {
			m = "2"
		}
		mults = append(mults, m)
	}
	fmt.Fprintf(&b, "#1000=B_SPLINE_CURVE_WITH_KNOTS('',1,(%s),.UNSPECIFIED.,.T.,.F.,(%s),(%s),.UNSPECIFIED.);ENDSEC;END-ISO-10303-21;", strings.Join(refs, ","), strings.Join(mults, ","), strings.Join(knots, ","))
	return b.String()
}
func TestPolynomialWindingIndependent(t *testing.T) {
	for _, turns := range []int{1, 2, -1, -2} {
		r, e := BoundPolynomialSplineWinding(strings.NewReader(windingPolygon(turns)), Limits{}, 1000, geom.Vec3{}, PolynomialWindingOptions{})
		if e != nil || r.Turns != turns {
			t.Fatal(turns, r, e)
		}
	}
	if _, e := BoundPolynomialSplineWinding(strings.NewReader(windingPolygon(2)), Limits{}, 1000, geom.Vec3{}, PolynomialWindingOptions{MaxCells: 1}); e == nil {
		t.Fatal("ignored cell budget")
	}
	if _, e := BoundPolynomialSplineWinding(strings.NewReader(polynomialResidualFixture()), Limits{}, 3, geom.Vec3{}, PolynomialWindingOptions{}); e == nil {
		t.Fatal("accepted nonclosed/radial source")
	}
}
func TestPolynomialWindingOCCT(t *testing.T) {
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	r, e := BoundPolynomialSplineWinding(bytes.NewReader(raw), Limits{}, 25, geom.Vec3{X: 1}, PolynomialWindingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r.Turns != 1 || len(r.Crossings) != 1 {
		t.Fatal("not one turn", r)
	}
	t.Log("certified cells", r.Cells, "crossing", r.Crossings[0][0].RatString(), r.Crossings[0][1].RatString())
}
func FuzzPolynomialWinding(f *testing.F) {
	f.Add([]byte(windingPolygon(1)))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = BoundPolynomialSplineWinding(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 8192}, 1000, geom.Vec3{}, PolynomialWindingOptions{MaxDepth: 16, MaxCells: 1024})
	})
}

func TestPolynomialWindingSubdivisionBudget(t *testing.T) {
	var b strings.Builder
	b.WriteString(strings.Split(polynomialResidualFixture(), "DATA;")[0] + "DATA;")
	points := [][2]int{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	refs := []string{}
	for i, p := range points {
		fmt.Fprintf(&b, "#%d=CARTESIAN_POINT('',(%d.,%d.,0.));", i+1, p[0], p[1])
		refs = append(refs, fmt.Sprintf("#%d", i+1))
	}
	fmt.Fprintf(&b, "#1000=B_SPLINE_CURVE_WITH_KNOTS('',2,(%s),.UNSPECIFIED.,.T.,.F.,(3,2,2,2,3),(0.,1.,2.,3.,4.),.UNSPECIFIED.);ENDSEC;END-ISO-10303-21;", strings.Join(refs, ","))
	// This origin places the positive-ray root exactly at a subdivision
	// midpoint, exercising duplicate endpoint exclusion between child cells.
	exact, e := BoundPolynomialSplineWinding(strings.NewReader(b.String()), Limits{}, 1000, geom.Vec3{X: .25, Y: .75}, PolynomialWindingOptions{})
	if e != nil || exact.Turns != 1 || len(exact.Crossings) != 1 || exact.Crossings[0][0].Cmp(exact.Crossings[0][1]) != 0 {
		t.Fatal("midpoint crossing", exact, e)
	}
	origin := geom.Vec3{X: .9, Y: .1}
	r, e := BoundPolynomialSplineWinding(strings.NewReader(b.String()), Limits{}, 1000, origin, PolynomialWindingOptions{})
	if e != nil || r.Turns != 1 || r.Cells <= 4 {
		t.Fatal(r, e)
	}
	if _, e = BoundPolynomialSplineWinding(strings.NewReader(b.String()), Limits{}, 1000, origin, PolynomialWindingOptions{MaxDepth: 1}); e == nil {
		t.Fatal("ignored isolation depth")
	}
}
