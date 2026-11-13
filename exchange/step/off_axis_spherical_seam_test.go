package step

import (
	"bytes"
	"math/big"
	"os"
	"strings"
	"testing"
)

func TestOffAxisSphericalSeam(t *testing.T) {
	b, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	check := func(raw []byte) OffAxisSphericalSeamEvidence {
		t.Helper()
		r, e := InspectOffAxisSphericalSeam(bytes.NewReader(raw), Limits{}, 15)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	r := check(b)
	if r.EdgeID != 555 || r.CircleID != 559 || r.VertexIDs != [2]int64{556, 22} || !r.MeridianContainsSphereAxis || r.FullCircleSphereSquaredRadiusResidual.Sign() != 0 {
		t.Fatal(r)
	}
	if r.DirectionNormSquared.Cmp(big.NewRat(1, 1)) <= 0 || r.DirectionNormSquared.Cmp(r.ReferenceNormSquared) != 0 {
		t.Fatal("source normalization was lost")
	}
	for i := 0; i < 2; i++ {
		if r.EndpointSphereResiduals[i].Sign() <= 0 || r.EndpointPlaneSquaredDistances[i].Sign() <= 0 || r.EndpointAxisSquaredDistances[i].Sign() <= 0 {
			t.Fatal("source endpoint snapped", i)
		}
	}
	changed := strings.Replace(string(b), "#559 = CIRCLE('',#560,5.);", "#559 = CIRCLE('',#560,5.1);", 1)
	q := check([]byte(changed))
	if q.FullCircleSphereSquaredRadiusResidual.Cmp(big.NewRat(101, 100)) != 0 {
		t.Fatal(q)
	}
	tilted := strings.Replace(string(b), "#562 = DIRECTION('',(-2.449293598295E-16,-1.,0.));", "#562 = DIRECTION('',(0.,0.,1.));", 1)
	tilted = strings.Replace(tilted, "#563 = DIRECTION('',(1.,-2.449293598295E-16,0.));", "#563 = DIRECTION('',(1.,0.,0.));", 1)
	if check([]byte(tilted)).MeridianContainsSphereAxis {
		t.Fatal("equatorial plane called meridian")
	}
	for _, pair := range [][2]string{{"#561 = CARTESIAN_POINT('',(0.,0.,0.));", "#561 = CARTESIAN_POINT('',(0.,0.,0.1));"}, {"#563 = DIRECTION('',(1.,-2.449293598295E-16,0.));", "#563 = DIRECTION('',(1.,0.,0.));"}, {"#562 = DIRECTION('',(-2.449293598295E-16,-1.,0.));", "#562 = DIRECTION('',(0.,0.,0.));"}} {
		raw := strings.Replace(string(b), pair[0], pair[1], 1)
		if raw == string(b) {
			t.Fatal("stale mutation")
		}
		if _, e := InspectOffAxisSphericalSeam(strings.NewReader(raw), Limits{}, 15); e == nil {
			t.Fatal("unsupported accepted")
		}
	}
}
