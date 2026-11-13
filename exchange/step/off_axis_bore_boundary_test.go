package step

import (
	"bytes"
	"math/big"
	"os"
	"testing"
)

// Independently authored with OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9:
// Cut(MakeSphere(5),MakeCylinder(Ax2((1,0,-.75),+Z),2,7)), STEPControl_AsIs.
// The retained approximate intersection must not enter an exact body importer.
func TestOffAxisSphereBoreExactExchangeBoundary(t *testing.T) {
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	edge, e := simple(f, 21, "EDGE_CURVE", 5)
	if e != nil {
		t.Fatal(e)
	}
	curveID, e := ref(edge.Args[3])
	if e != nil {
		t.Fatal(e)
	}
	sc, e := simple(f, curveID, "SURFACE_CURVE", 4)
	if e != nil {
		t.Fatal(e)
	}
	model, e := ref(sc.Args[1])
	if e != nil {
		t.Fatal(e)
	}
	spline, e := simple(f, model, "B_SPLINE_CURVE_WITH_KNOTS", 9)
	if e != nil {
		t.Fatal(e)
	}
	degree, e := numeric(spline.Args[1])
	if e != nil || degree != 7 {
		t.Fatal("unexpected independently authored model", e)
	}
	poles, e := listRefs(spline.Args[2], 2)
	if e != nil || len(poles) < 3 {
		t.Fatal(e)
	}
	a, e := simple(f, poles[0], "CARTESIAN_POINT", 2)
	if e != nil {
		t.Fatal(e)
	}
	b, e := simple(f, poles[1], "CARTESIAN_POINT", 2)
	if e != nil {
		t.Fatal(e)
	}
	if a.Args[1].Items[1].Text == b.Args[1].Items[1].Text {
		t.Fatal("constant curve fixture")
	}
	// The source's clamped first endpoint itself misses x²+y²+z²=25 in
	// exact STEP decimal arithmetic. No float tolerance is used for this check.
	residual := new(big.Rat).SetInt64(-25)
	for _, v := range a.Args[1].Items {
		q, ok := new(big.Rat).SetString(v.Text)
		if !ok {
			t.Fatal(v)
		}
		residual.Add(residual, new(big.Rat).Mul(q, q))
	}
	if residual.Sign() <= 0 {
		t.Fatal("fixture no longer exhibits exact endpoint mismatch", residual)
	}
	if _, e = ReadAnalyticBodies(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("accepted approximate sphere-cylinder material as exact analytic body")
	}
}
