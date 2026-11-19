package step

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestOCCTObliqueCylinderImport(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "oblique.step")
	stepOracle(t, path, "write-step-oblique-cylinder")
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	checkOblique(t, string(b))
	// These mutations retain valid Part 21 syntax and break independent proofs.
	for _, m := range [][2]string{
		{"2.061552812809,2.", "2.161552812809,2."},
		{"#21,.F.", "#21,.T."},
		{"#30,.T.", "#38,.T."},
		{"(3.,2.,7.5)", "(3.,2.,7.6)"},
		{"#63,.F.", "#63,.T."},
		{"#38,.F.", "#21,.F."},
	} {
		bad := strings.Replace(string(b), m[0], m[1], 1)
		if bad == string(b) {
			t.Fatalf("fixture changed: %s", m[0])
		}
		if _, e := ReadTrimmedBodies(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatalf("accepted %v", m)
		}
	}
	for _, l := range []Limits{{MaxSolids: 1, MaxFaces: 1}, {MaxVertices: 1}, {MaxEdgeUses: 1}} {
		if _, e := ReadTrimmedBodies(strings.NewReader(string(b)), l); e == nil {
			t.Fatal("resource limit ignored", l)
		}
	}
}
func checkOblique(t *testing.T, s string) {
	t.Helper()
	m, e := ReadTrimmedBodies(strings.NewReader(s), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(p.Volume-16*math.Pi) > 1e-6 || math.Abs(p.Area-(20+math.Sqrt(17))*math.Pi) > 1e-6 || p.Centroid.Distance(geom.Vec3{X: 1.0625, Y: 2, Z: 5.0078125}) > 1e-7 {
		t.Fatal(p)
	}
}
func TestTrimmedAnalyticBodies(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	if e := WriteCylinders(&b, cylinderModel(t)); e != nil {
		t.Fatal(e)
	}
	m, e := ReadTrimmedBodies(strings.NewReader(b.String()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 {
		t.Fatal(m)
	}
	if _, e := ReadTrimmedBodies(nil, Limits{}); e == nil {
		t.Fatal("nil input")
	}
}

// The persisted independently generated OCCT fixture also runs without OCCT.
// Its volume and centroid checks use closed-form cylinder integrals.
func TestObliqueCylinderFixture(t *testing.T) {
	t.Parallel()
	b, e := os.ReadFile("testdata/oblique-cylinder.step")
	if e != nil {
		t.Fatal(e)
	}
	checkOblique(t, string(b))
}

func TestObliqueRejectsFalseLocus(t *testing.T) {
	t.Parallel()
	b, e := os.ReadFile("testdata/oblique-cylinder.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range [][2]string{{"2.061552812809,2.", "2.061552812809,2.1"}, {"#63,.F.", "#63,.T."}, {"#30,.T.", "#38,.T."}} {
		bad := strings.Replace(string(b), change[0], change[1], 1)
		if bad == string(b) {
			t.Fatal("stale fixture mutation")
		}
		if _, e := ReadTrimmedBodies(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal("false locus accepted", change)
		}
	}
}

func FuzzTrimmedSTEP(f *testing.F) {
	b, e := os.ReadFile("testdata/oblique-cylinder.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(string(b))
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ReadTrimmedBodies(strings.NewReader(s), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 8192, MaxDepth: 24, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 512})
	})
}
