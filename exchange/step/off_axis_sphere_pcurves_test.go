package step

import (
	"math/big"
	"os"
	"strings"
	"testing"
)

func TestSphericalSeamPCurves(t *testing.T) {
	t.Parallel()
	b, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	check := func(s string) SphericalSeamPCurveEvidence {
		t.Helper()
		r, e := InspectSphericalSeamPCurves(strings.NewReader(s), Limits{}, 15)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	r := check(string(b))
	if r.EdgeID != 555 || r.SphereID != 197 || r.Branches[0].PCurveID != 564 || r.Branches[1].PCurveID != 571 || r.AuthoredTrimIntervalsKnown || !r.SameMagnitude || !r.SameVDirection || r.VOriginDifference.Sign() != 0 || r.TurnDefectLower.Sign() <= 0 || r.TurnDefectUpper.Cmp(big.NewRat(1, 1000000000000)) >= 0 {
		t.Fatal(r)
	}
	if r.Branches[0].Origin[1].Sign() >= 0 {
		t.Fatal("source origin lost")
	}
	changed := strings.Replace(string(b), "#576 = DIRECTION('',(0.,1.));", "#576 = DIRECTION('',(0.,-1.));", 1)
	if check(changed).SameVDirection {
		t.Fatal("reversed branch hidden")
	}
	changed = strings.Replace(string(b), "#574 = CARTESIAN_POINT('',(0.,-6.28318530718));", "#574 = CARTESIAN_POINT('',(0.1,-6.));", 1)
	q := check(changed)
	if q.VOriginDifference.Sign() == 0 || q.TurnDefectUpper.Sign() >= 0 {
		t.Fatal("source discrepancies hidden")
	}
	changed = strings.Replace(string(b), "#575 = VECTOR('',#576,1.);", "#575 = VECTOR('',#576,2.);", 1)
	if check(changed).SameMagnitude {
		t.Fatal("scale hidden")
	}
	changed = strings.Replace(string(b), "#576 = DIRECTION('',(0.,1.));", "#576 = DIRECTION('',(1.,1.));", 1)
	if _, e := InspectSphericalSeamPCurves(strings.NewReader(changed), Limits{}, 15); e == nil {
		t.Fatal("nonmeridian accepted")
	}
}
