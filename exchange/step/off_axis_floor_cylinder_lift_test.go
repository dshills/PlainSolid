package step

import (
	"math/big"
	"os"
	"strings"
	"testing"
)

func TestOffAxisFloorCylinderLift(t *testing.T) {
	t.Parallel()
	b, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	check := func(s string) FloorCylinderLiftEvidence {
		t.Helper()
		r, e := InspectOffAxisFloorCylinderLift(strings.NewReader(s), Limits{}, 15)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	r := check(string(b))
	if !r.SupportCircleEqual || !r.SameParameterization || r.AuthoredTrimIntervalsKnown || r.EdgeID != 607 || r.ParameterLineID != 616 {
		t.Fatal(r)
	}
	for _, pair := range [][2]string{{"#619 = DIRECTION('',(1.,0.));", "#619 = DIRECTION('',(-1.,0.));"}, {"#618 = VECTOR('',#619,1.);", "#618 = VECTOR('',#619,2.);"}, {"#617 = CARTESIAN_POINT('',(0.,0.));", "#617 = CARTESIAN_POINT('',(0.1,0.));"}} {
		s := strings.Replace(string(b), pair[0], pair[1], 1)
		if s == string(b) {
			t.Fatal("stale")
		}
		q := check(s)
		if !q.SupportCircleEqual || q.SameParameterization {
			t.Fatal(q)
		}
	}
	s := strings.Replace(string(b), "#617 = CARTESIAN_POINT('',(0.,0.));", "#617 = CARTESIAN_POINT('',(0.,0.01));", 1)
	q := check(s)
	if q.SupportCircleEqual || q.SameParameterization || q.CenterDifference[2].Cmp(big.NewRat(1, 100)) != 0 {
		t.Fatal(q)
	}
}
