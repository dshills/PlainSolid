package step

import (
	"os"
	"strings"
	"testing"
)

func TestCylinderPCurveEvidenceRequiresRadianContext(t *testing.T) {
	b, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = InspectCylinderPCurve(strings.NewReader(string(b)), Limits{}, 21); e != nil {
		t.Fatal(e)
	}
	for _, p := range [][2]string{{"SI_UNIT($,.RADIAN.)", "SI_UNIT(.MILLI.,.RADIAN.)"}, {"SI_UNIT($,.RADIAN.)", "SI_UNIT($,.STERADIAN.)"}, {"(#639,#640,#641)", "(#639,#641)"}} {
		s := strings.Replace(string(b), p[0], p[1], 1)
		if s == string(b) {
			t.Fatal("stale mutation")
		}
		if _, e := InspectCylinderPCurve(strings.NewReader(s), Limits{}, 21); e == nil {
			t.Fatal("radian claim accepted incompatible context")
		}
	}
}
