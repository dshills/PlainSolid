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

func TestTrimmedGeneratrixCylinder(t *testing.T) {
	raw, e := os.ReadFile("testdata/trimmed-generatrix-cylinder.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadTrimmedGeneratrixCylinderProduct(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Cylinder.Product.Model.Radius != 2 || m.Cylinder.Product.Model.Height != 3 || len(m.Trims) != 2 {
		t.Fatal(m)
	}
	a, b := m.Trims[27], m.Trims[28]
	if a.SenseAgreement || a.EdgeSameSense || !b.SenseAgreement || b.EdgeSameSense || a.Parameters != [2]float64{3, 7} || a.Basis != 21 || a.Entity != 80 || !m.Cylinder.SameSense[27] || m.Cylinder.SameSense[28] {
		t.Fatal(a, b)
	}
	if _, e = ReadSplineGeneratrixCylinderProduct(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("earlier API widened")
	}
	for _, pair := range [][2]string{
		{"PARAMETER_VALUE(3.)", "PARAMETER_VALUE(4.)"}, {"PARAMETER_VALUE(7.)", "PARAMETER_VALUE(7.0000000000000001)"},
		{".F.,.PARAMETER.", ".T.,.PARAMETER."}, {".PARAMETER.", ".UNSPECIFIED."},
		{"(PARAMETER_VALUE(3.))", "(#12)"}, {"(PARAMETER_VALUE(3.))", "(PARAMETER_VALUE(3.),#12)"},
		{"'forward full axial trim',#22", "'forward full axial trim',#21"},
		{"#16,#18,#80,.F.", "#16,#18,#80,.T."}, {"'reversed full axial trim',#21", "'reversed full axial trim',#81"},
		{"ENDSEC;END-ISO-10303-21;", "#99=CARTESIAN_POINT('',(0.,0.,0.));ENDSEC;END-ISO-10303-21;"},
	} {
		bad := strings.Replace(string(raw), pair[0], pair[1], 1)
		if bad == string(raw) {
			t.Fatal("stale", pair)
		}
		r, e := ReadTrimmedGeneratrixCylinderProduct(strings.NewReader(bad), Limits{})
		if e == nil || len(r.Trims) != 0 || r.Cylinder.Product.Model.Body != 0 {
			t.Fatal(pair, e)
		}
	}
	for _, l := range []Limits{{MaxEntities: 20}, {MaxVertices: 3}, {MaxFaces: 3}, {MaxBytes: 100}} {
		if _, e = ReadTrimmedGeneratrixCylinderProduct(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
}
func TestOCCTTrimmedGeneratrixCylinder(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_SPLIT_CYLINDER")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	path, e := filepath.Abs("testdata/trimmed-generatrix-cylinder.step")
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
	if got.Faces != 4 || math.Abs(got.Volume-12*math.Pi) > 3e-6 || math.Abs(got.Area-20*math.Pi) > 3e-6 || math.Abs(got.Center[2]-1.5) > 1e-10 {
		t.Fatal(got)
	}
}
func FuzzTrimmedGeneratrixCylinder(f *testing.F) {
	raw, e := os.ReadFile("testdata/trimmed-generatrix-cylinder.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("invalid"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		m, e := ReadTrimmedGeneratrixCylinderProduct(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 240, MaxValues: 4096, MaxDepth: 12, MaxVertices: 32, MaxFaces: 8})
		if e != nil && (len(m.Trims) != 0 || m.Cylinder.Product.Model.Body != 0) {
			t.Fatal("partial")
		}
	})
}
