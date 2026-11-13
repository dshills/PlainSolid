package step

import (
	"bytes"
	"os"
	"testing"
)

func TestOffAxisCylinderChartBoundary(t *testing.T) {
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	r, e := CertifyOffAxisCylinderChartBoundary(bytes.NewReader(raw), Limits{}, 15, 1e-7)
	if e != nil || !r.ChartJordan || !r.AssumesFullRimDomain || !r.AssumesCornerSelectedSegments || !r.PeriodicMismatchProven || r.LiftedClosureProven || r.Orientation != -1 {
		t.Fatal(r, e)
	}
	if r.SeamAssignments[0].CoedgeID != 633 || r.SeamAssignments[0].PCurveID != 592 || r.SeamAssignments[1].CoedgeID != 583 || r.SeamAssignments[1].PCurveID != 599 || r.PeriodicWidthDefect[0].Sign() <= 0 {
		t.Fatal(r)
	}
	for _, pair := range [][2]string{
		{"#619 = DIRECTION('',(1.,0.));", "#619 = DIRECTION('',(-1.,0.));"},
		{"#595 = CARTESIAN_POINT('',(6.28318530718,-0.));", "#595 = CARTESIAN_POINT('',(6.28318530717,-0.));"},
		{"#552 = CARTESIAN_POINT('',(6.28318530718,4.75));", "#552 = CARTESIAN_POINT('',(6.28318530718,4.76));"},
		{"#617 = CARTESIAN_POINT('',(0.,0.));", "#617 = CARTESIAN_POINT('',(0.,1.));"},
		{"#581 = EDGE_LOOP('',(#582,#583,#606,#633));", "#581 = EDGE_LOOP('',(#582,#633,#606,#583));"},
		{"#383 = CARTESIAN_POINT('',(0.,4.75));", "#383 = CARTESIAN_POINT('',(8.,4.75));"},
		{"#580 = FACE_BOUND('',#581,.F.);", "#580 = FACE_BOUND('',#581,.T.);"},
	} {
		bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(raw, bad) {
			t.Fatal("stale", pair)
		}
		if _, e := CertifyOffAxisCylinderChartBoundary(bytes.NewReader(bad), Limits{}, 15, 1e-7); e == nil {
			t.Fatal("accepted", pair)
		}
	}
	swapped := bytes.Replace(raw, []byte("#587 = SEAM_CURVE('',#588,(#592,#599),.PCURVE_S1.);"), []byte("#587 = SEAM_CURVE('',#588,(#599,#592),.PCURVE_S1.);"), 1)
	if bytes.Equal(raw, swapped) {
		t.Fatal("stale seam reorder mutation")
	}
	r, e = CertifyOffAxisCylinderChartBoundary(bytes.NewReader(swapped), Limits{}, 15, 1e-7)
	if e != nil || r.SeamAssignments[0].PCurveID != 592 {
		t.Fatal("list order guessed", r, e)
	}
	if _, e := CertifyOffAxisCylinderChartBoundary(bytes.NewReader(raw), Limits{MaxBytes: 16}, 15, 1e-7); e == nil {
		t.Fatal("byte budget")
	}
	if _, e := CertifyOffAxisCylinderChartBoundary(bytes.NewReader(raw), Limits{}, 15, 1e-20); e == nil {
		t.Fatal("endpoint tolerance")
	}
}
