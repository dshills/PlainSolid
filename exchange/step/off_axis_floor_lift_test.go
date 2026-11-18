package step

import (
	"math/big"
	"os"
	"strings"
	"testing"
)

func TestOffAxisFloorCircleLift(t *testing.T) {
	t.Parallel()
	b, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	check := func(s string) FloorCircleLiftEvidence {
		t.Helper()
		r, e := InspectOffAxisFloorCircleLift(strings.NewReader(s), Limits{}, 15)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	r := check(string(b))
	if !r.IdenticalParameterization || r.EdgeID != 607 || r.ModelCircleID != 609 || r.ParameterCircleID != 628 || r.PlaneID != 622 {
		t.Fatal(r)
	}
	changed := strings.Replace(string(b), "#630 = CARTESIAN_POINT('',(0.,0.));", "#630 = CARTESIAN_POINT('',(0.01,0.));", 1)
	q := check(changed)
	if q.IdenticalParameterization || q.CenterDifference[0].Cmp(big.NewRat(1, 100)) != 0 {
		t.Fatal(q)
	}
	changed = strings.Replace(string(b), "#631 = DIRECTION('',(1.,0.));", "#631 = DIRECTION('',(0.,1.));", 1)
	q = check(changed)
	if q.IdenticalParameterization || q.CosineDifference[0].Cmp(big.NewRat(-2, 1)) != 0 || q.CosineDifference[1].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("phase change hidden", q)
	}
	changed = strings.Replace(string(b), "#628 = CIRCLE('',#629,2.);", "#628 = CIRCLE('',#629,2.5);", 1)
	q = check(changed)
	if q.IdenticalParameterization || q.CosineDifference[0].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(q)
	}
	changed = strings.Replace(string(b), "#631 = DIRECTION('',(1.,0.));", "#631 = DIRECTION('',(1.,1.));", 1)
	if _, e := InspectOffAxisFloorCircleLift(strings.NewReader(changed), Limits{}, 15); e == nil {
		t.Fatal("unsupported normalization")
	}
}
