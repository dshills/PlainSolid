package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSplineGeneratrixCylinder(t *testing.T) {
	b, e := os.ReadFile("testdata/spline-generatrix-cylinder.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineGeneratrixCylinderProduct(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Product.Model.Radius != 2 || m.Product.Model.Height != 3 || len(m.Curves) != 2 || m.SameSense[28] {
		t.Fatal(m)
	}
	for _, c := range m.Curves {
		a, z := c.Geometry.Domain()
		if a != 3 || z != 7 || c.Geometry.Weights()[0] != .5 {
			t.Fatal(c)
		}
	}
	if _, e = ReadSplitCylinderProduct(bytes.NewReader(b), Limits{}); e == nil {
		t.Fatal("old acceptance widened")
	}
}

func TestSplineGeneratrixCylinderRejects(t *testing.T) {
	raw, e := os.ReadFile("testdata/spline-generatrix-cylinder.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]string{
		{"(#12,#14)", "(#12,#15)"}, {"(0.5,1.)", "(0.,1.)"},
		{"(3.,7.)", "(3.,3.)"}, {"#28=EDGE_CURVE('',#17,#19,#22,.F.)", "#28=EDGE_CURVE('',#17,#19,#22,.T.)"},
		{"CYLINDRICAL_SURFACE('',#4,2.)", "CYLINDRICAL_SURFACE('',#4,3.)"},
		{"#28=EDGE_CURVE('',#17,#19,#22,.F.)", "#28=EDGE_CURVE('',#17,#19,#21,.F.)"},
		{".MILLI.", ".BOGUS."}, {"PRODUCT('split-cylinder','split'", "PRODUCT('foreign','split'"},
		{"(2.,0.,3.)", "(2.,0.,3.0000000000000001)"},
		{"ENDSEC;END-ISO-10303-21;", "#99=CARTESIAN_POINT('',(0.,0.,0.));ENDSEC;END-ISO-10303-21;"},
	} {
		bad := strings.Replace(string(raw), pair[0], pair[1], 1)
		if bad == string(raw) {
			t.Fatal("stale mutation", pair)
		}
		m, e := ReadSplineGeneratrixCylinderProduct(strings.NewReader(bad), Limits{})
		if e == nil || m.Product.Model.Body != 0 || len(m.Curves) != 0 {
			t.Fatal(pair, e)
		}
	}
	for _, l := range []Limits{{MaxEntities: 20}, {MaxVertices: 3}, {MaxFaces: 3}, {MaxBytes: 100}} {
		if m, e := ReadSplineGeneratrixCylinderProduct(bytes.NewReader(raw), l); e == nil || len(m.Curves) != 0 {
			t.Fatal(l, e)
		}
	}
}
func TestOCCTSplineGeneratrixCylinder(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_SPLIT_CYLINDER")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	path, e := filepath.Abs("testdata/spline-generatrix-cylinder.step")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	b, e := exec.CommandContext(ctx, bin, path, "roots").CombinedOutput()
	if e != nil {
		t.Fatal(e, string(b))
	}
	var got struct {
		Faces        int
		Volume, Area float64
		Center       [3]float64
	}
	if e = json.Unmarshal(b, &got); e != nil {
		t.Fatal(e, string(b))
	}
	// OCCT's rational-edge numerical quadrature is independent of the exact Go proof.
	if got.Faces != 4 || math.Abs(got.Volume-12*math.Pi) > 3e-6 || math.Abs(got.Area-20*math.Pi) > 3e-6 || math.Abs(got.Center[2]-1.5) > 1e-10 {
		t.Fatal(got)
	}
}
func FuzzSplineGeneratrixCylinder(f *testing.F) {
	raw, e := os.ReadFile("testdata/spline-generatrix-cylinder.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("invalid"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		m, e := ReadSplineGeneratrixCylinderProduct(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 240, MaxValues: 4096, MaxDepth: 12, MaxVertices: 32, MaxFaces: 8})
		if e != nil && (len(m.Curves) != 0 || m.Product.Model.Body != 0) {
			t.Fatal("partial")
		}
	})
}
