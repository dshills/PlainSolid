package step

import (
	"bytes"
	"math/big"
	"os"
	"testing"
)

func TestOffAxisSphericalRephase(t *testing.T) {
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	r, e := CertifyOffAxisSphericalRephase(bytes.NewReader(raw), Limits{}, 15, 1e-7)
	if e != nil {
		t.Fatal(e)
	}
	if !r.RepairedDiskCoverage || !r.AssumesFullRimAndCornerSegments || r.ModelMeridianAgreementProven || r.SourceMaterialEquivalenceProven || r.DownwardSeamPCurveID != 571 || r.UpwardSeamPCurveID != 564 || r.PoleVertexID != 556 {
		t.Fatal(r)
	}
	if r.LongitudeDisplacementUpper.Sign() <= 0 || r.LongitudeDisplacementUpper.Cmp(big.NewRat(3, 1000000000000)) >= 0 || r.PoleDisplacementSquared.Sign() <= 0 {
		t.Fatal("correction bounds", r)
	}
	for _, pair := range [][2]string{
		{`#205 = CARTESIAN_POINT('',(4.58811104823E-02,0.927295218002));`, `#205 = CARTESIAN_POINT('',(-1.,1.));`},
		{"#204 = CARTESIAN_POINT('',(0.,0.927295218002));", "#204 = CARTESIAN_POINT('',(1.,0.927295218002));"},
		{"#204 = CARTESIAN_POINT('',(0.,0.927295218002));", "#204 = CARTESIAN_POINT('',(0.,2.));"},
		{"#373 = CARTESIAN_POINT('',(6.28318530718,0.927295218002));", "#373 = CARTESIAN_POINT('',(6.28318530718,0.927295218003));"},
		{"#19 = EDGE_LOOP('',(#20,#554,#578));", "#19 = EDGE_LOOP('',(#20,#578,#554));"},
		{"#564 = PCURVE('',#197,#565);", "#564 = PCURVE('',#376,#565);"},
		{"#567 = CARTESIAN_POINT('',(6.28318530718,-6.28318530718));", "#567 = CARTESIAN_POINT('',(6.28318530717,-6.28318530718));"},
		{"#17 = ADVANCED_FACE('',(#18),#197,.T.);", "#17 = ADVANCED_FACE('',(#18),#197,.F.);"},
		{"#557 = CARTESIAN_POINT('',(3.061616997868E-16,-7.498798913309E-32,-5.));", "#557 = CARTESIAN_POINT('',(0.,0.,5.));"},
	} {
		bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(raw, bad) {
			t.Fatal("stale mutation", pair)
		}
		if _, e := CertifyOffAxisSphericalRephase(bytes.NewReader(bad), Limits{}, 15, 1e-7); e == nil {
			t.Fatal("accepted mutation", pair)
		}
	}
	// Preserve both branches while allowing arbitrary association list ordering.
	swapped := bytes.Replace(raw, []byte("#558 = SEAM_CURVE('',#559,(#564,#571),.PCURVE_S1.);"), []byte("#558 = SEAM_CURVE('',#559,(#571,#564),.PCURVE_S1.);"), 1)
	if bytes.Equal(raw, swapped) {
		t.Fatal("stale reorder")
	}
	q, e := CertifyOffAxisSphericalRephase(bytes.NewReader(swapped), Limits{}, 15, 1e-7)
	if e != nil || q.DownwardSeamPCurveID != 571 {
		t.Fatal(q, e)
	}
	for _, tol := range []float64{0, -1, 1e-14} {
		if _, e := CertifyOffAxisSphericalRephase(bytes.NewReader(raw), Limits{}, 15, tol); e == nil {
			t.Fatal("tolerance", tol)
		}
	}
	if _, e := CertifyOffAxisSphericalRephase(bytes.NewReader(raw), Limits{MaxBytes: 16}, 15, 1e-7); e == nil {
		t.Fatal("budget")
	}
}
