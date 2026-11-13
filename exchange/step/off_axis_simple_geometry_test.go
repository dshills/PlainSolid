package step

import (
	"bytes"
	"math/big"
	"os"
	"strings"
	"testing"
)

func TestOffAxisSimpleGeometry(t *testing.T) {
	b, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	check := func(raw []byte) OffAxisSimpleGeometryEvidence {
		t.Helper()
		r, e := InspectOffAxisSimpleGeometry(bytes.NewReader(raw), Limits{}, 15)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	r := check(b)
	if r.CircleID != 609 || r.LineID != 588 || r.FloorCylinderSquaredRadiusResidual.Sign() != 0 || r.FloorPlaneSignedDistance.Sign() != 0 || r.SeamCylinderSquaredRadiusResidual.Sign() <= 0 || r.SeamEndpointLineSquaredDistances[0].Sign() != 0 || r.SeamEndpointLineSquaredDistances[1].Sign() <= 0 {
		t.Fatal(r)
	}
	if len(r.SourceUncertainties) != 1 || r.SourceUncertainties[0].UnitID != 639 || r.SourceUncertainties[0].Value.Cmp(big.NewRat(1, 10000000)) != 0 {
		t.Fatal(r.SourceUncertainties)
	}
	// Exact decimal mismatch is retained rather than hidden by source tolerance.
	changed := strings.Replace(string(b), "#609 = CIRCLE('',#610,2.);", "#609 = CIRCLE('',#610,2.1);", 1)
	q := check([]byte(changed))
	if q.FloorCylinderSquaredRadiusResidual.Cmp(big.NewRat(41, 100)) != 0 {
		t.Fatal(q)
	}
	changed = strings.Replace(string(b), "#624 = CARTESIAN_POINT('',(1.,0.,-0.75));", "#624 = CARTESIAN_POINT('',(1.,0.,-0.5));", 1)
	q = check([]byte(changed))
	if q.FloorPlaneSignedDistance.Cmp(big.NewRat(-1, 4)) != 0 {
		t.Fatal(q)
	}
	for _, pair := range [][2]string{{"#611 = CARTESIAN_POINT('',(1.,0.,-0.75));", "#611 = CARTESIAN_POINT('',(1.1,0.,-0.75));"}, {"#591 = DIRECTION('',(0.,0.,1.));", "#591 = DIRECTION('',(1.,0.,1.));"}, {"LENGTH_MEASURE(1.E-07)", "LENGTH_MEASURE(-1.E-07)"}} {
		raw := strings.Replace(string(b), pair[0], pair[1], 1)
		if raw == string(b) {
			t.Fatal("stale mutation")
		}
		if _, e := InspectOffAxisSimpleGeometry(strings.NewReader(raw), Limits{}, 15); e == nil {
			t.Fatal("accepted unsupported source")
		}
	}
}
