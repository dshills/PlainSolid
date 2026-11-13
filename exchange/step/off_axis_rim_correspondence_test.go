package step

import (
	"bytes"
	"os"
	"testing"
)

func TestOffAxisRimCorrespondence(t *testing.T) {
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	r, e := BoundOffAxisRimCorrespondence(bytes.NewReader(raw), Limits{}, 15, 1e-4, PolynomialWindingOptions{})
	if e != nil || !r.FullDomainCorrespondence || !r.WithinTolerance || r.Winding.Turns != 1 {
		t.Fatal(r, e)
	}
	t.Log("certified bound", r.HausdorffUpper.FloatString(15))
	tiny, e := BoundOffAxisRimCorrespondence(bytes.NewReader(raw), Limits{}, 15, 1e-20, PolynomialWindingOptions{})
	if e != nil || tiny.WithinTolerance {
		t.Fatal(tiny, e)
	}
	for _, pair := range [][2]string{{"SPHERICAL_SURFACE('',#198,5.)", "SPHERICAL_SURFACE('',#198,3.)"}, {"CYLINDRICAL_SURFACE('',#377,2.)", "CYLINDRICAL_SURFACE('',#377,4.)"}, {"#379 = DIRECTION('',(0.,0.,1.))", "#379 = DIRECTION('',(0.,1.,0.))"}, {"#199 = CARTESIAN_POINT('',(0.,0.,0.))", "#199 = CARTESIAN_POINT('',(0.,0.,5.))"}, {"#378 = CARTESIAN_POINT('',(1.,0.,-0.75))", "#378 = CARTESIAN_POINT('',(0.1,0.,-0.75))"}} {
		bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(raw, bad) {
			t.Fatal("stale mutation", pair)
		}
		if _, e := BoundOffAxisRimCorrespondence(bytes.NewReader(bad), Limits{}, 15, 1, PolynomialWindingOptions{}); e == nil {
			t.Fatal("accepted", pair)
		}
	}

	broken := bytes.Replace(raw, []byte("#195 = CARTESIAN_POINT('',(3.,-4.440892098501E-16,4.));"), []byte("#195 = CARTESIAN_POINT('',(3.,-4.440892098501E-16,4.01));"), 1)
	if bytes.Equal(broken, raw) {
		t.Fatal("stale closure mutation")
	}
	if _, e = BoundOffAxisRimCorrespondence(bytes.NewReader(broken), Limits{}, 15, 1, PolynomialWindingOptions{}); e == nil {
		t.Fatal("broken closure")
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	en := f.Entities[25]
	points := en.Records[0].Args[2].Items
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
	f.Entities[25] = en
	if _, e = BoundOffAxisRimCorrespondence(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, 15, 1, PolynomialWindingOptions{}); e == nil {
		t.Fatal("negative turn")
	}
	if _, e = BoundOffAxisRimCorrespondence(bytes.NewReader(raw), Limits{MaxBytes: 10}, 15, 1, PolynomialWindingOptions{}); e == nil {
		t.Fatal("limit")
	}
	if _, e = BoundOffAxisRimCorrespondence(bytes.NewReader(raw), Limits{}, 15, 1, PolynomialWindingOptions{MaxCells: 1, MaxDepth: 1}); e == nil {
		t.Fatal("winding budget")
	}
	if _, e = BoundOffAxisRimCorrespondence(bytes.NewReader(raw), Limits{}, 16, 1, PolynomialWindingOptions{}); e == nil {
		t.Fatal("foreign owner")
	}
}
