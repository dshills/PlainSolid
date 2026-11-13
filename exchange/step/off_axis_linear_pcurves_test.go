package step

import (
	"bytes"
	"math/big"
	"os"
	"strings"
	"testing"
)

func TestOffAxisLinearPCurves(t *testing.T) {
	b, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	check := func(raw string) OffAxisLinearPCurveEvidence {
		t.Helper()
		r, e := InspectOffAxisLinearPCurves(strings.NewReader(raw), Limits{}, 15)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	r := check(string(b))
	if r.AuthoredTrimIntervalsKnown || r.FloorPlaneCircleID != 628 || r.FloorVertexID != 585 || r.CylinderSeams[0].CurveID != 594 || r.CylinderSeams[1].CurveID != 601 || r.FloorCylinder.CurveID != 616 {
		t.Fatal(r)
	}
	if r.SeamAngleTurnDefectLower.Sign() <= 0 || r.SeamAngleTurnDefectUpper.Cmp(big.NewRat(1, 1000000000000)) >= 0 || r.FloorVertexZeroAngleSquaredGap.Sign() <= 0 {
		t.Fatal("source discrepancy lost")
	}
	if r.FloorVertexCircleCoordinates[0].Cmp(big.NewRat(2, 1)) != 0 || r.FloorVertexCircleCoordinates[2].Sign() != 0 || r.FloorCylinder.Origin[1].Sign() != 0 || r.FloorPlaneCircleRadius.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal(r)
	}
	changed := strings.Replace(string(b), "#617 = CARTESIAN_POINT('',(0.,0.));", "#617 = CARTESIAN_POINT('',(0.,0.01));", 1)
	if check(changed).FloorCylinder.Origin[1].Cmp(big.NewRat(1, 100)) != 0 {
		t.Fatal("offset hidden")
	}
	changed = strings.Replace(string(b), "#597 = DIRECTION('',(0.,1.));", "#597 = DIRECTION('',(1.,1.));", 1)
	if _, e := InspectOffAxisLinearPCurves(strings.NewReader(changed), Limits{}, 15); e == nil {
		t.Fatal("nonconstant seam angle accepted")
	}
	if _, e := InspectOffAxisLinearPCurves(bytes.NewReader(b), Limits{MaxBytes: 100}, 15); e == nil {
		t.Fatal("limit")
	}
}
