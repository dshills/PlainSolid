package step

import (
	"bytes"
	"math/big"
	"os"
	"testing"
)

func TestOffAxisSeamCoverage(t *testing.T) {
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	r, e := BoundOffAxisCylinderSeamCoverage(bytes.NewReader(raw), Limits{}, 15, 1e-7)
	if e != nil || !r.EndpointAdmissible || !r.OrderedCoverage || r.AuthoredTrimIntervalsKnown {
		t.Fatal(r, e)
	}
	if r.EdgeID != 584 || r.ProjectionParameters[0].Sign() != 0 || r.ProjectionParameters[1].Cmp(big.NewRat(19, 4)) != 0 || r.Branches[0].ParameterSlope.Cmp(big.NewRat(1, 1)) != 0 || len(r.Uses) != 2 {
		t.Fatal(r)
	}
	for _, tol := range []float64{1e-20, 3} {
		r, e := BoundOffAxisCylinderSeamCoverage(bytes.NewReader(raw), Limits{}, 15, tol)
		if e != nil || r.OrderedCoverage {
			t.Fatal("incompatible tolerance", tol, r, e)
		}
	}
	for _, pair := range [][2]string{{"#584 = EDGE_CURVE('',#585,#22,#587,.T.);", "#584 = EDGE_CURVE('',#22,#585,#587,.T.);"}, {"#591 = DIRECTION('',(0.,0.,1.));", "#591 = DIRECTION('',(0.,0.,-1.));"}, {"#584 = EDGE_CURVE('',#585,#22,#587,.T.);", "#584 = EDGE_CURVE('',#585,#22,#587,.F.);"}} {
		bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(raw, bad) {
			t.Fatal("stale")
		}
		if _, e := BoundOffAxisCylinderSeamCoverage(bytes.NewReader(bad), Limits{}, 15, 1e-7); e == nil {
			t.Fatal("invalid source", pair)
		}
	}

	signed := bytes.Replace(raw, []byte("#597 = DIRECTION('',(0.,1.));"), []byte("#597 = DIRECTION('',(0.,-1.));"), 1)
	signed = bytes.Replace(signed, []byte("#596 = VECTOR('',#597,1.);"), []byte("#596 = VECTOR('',#597,2.);"), 1)
	signed = bytes.Replace(signed, []byte("#595 = CARTESIAN_POINT('',(6.28318530718,-0.));"), []byte("#595 = CARTESIAN_POINT('',(6.28318530718,3.));"), 1)
	changed, e := BoundOffAxisCylinderSeamCoverage(bytes.NewReader(signed), Limits{}, 15, 1e-7)
	if e != nil || !changed.OrderedCoverage || changed.Branches[0].ParameterSlope.Cmp(big.NewRat(-1, 2)) != 0 || changed.Branches[0].ParameterOffset.Cmp(big.NewRat(3, 2)) != 0 {
		t.Fatal("lost signed branch relation", changed, e)
	}
	badBranch := bytes.Replace(raw, []byte("#597 = DIRECTION('',(0.,1.));"), []byte("#597 = DIRECTION('',(1.,0.));"), 1)
	if _, e := BoundOffAxisCylinderSeamCoverage(bytes.NewReader(badBranch), Limits{}, 15, 1e-7); e == nil {
		t.Fatal("nonmeridian branch")
	}
	if _, e := BoundOffAxisCylinderSeamCoverage(bytes.NewReader(raw), Limits{MaxBytes: 10}, 15, 1e-7); e == nil {
		t.Fatal("budget")
	}
}
