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

	"plainsolid/brep"
)

func advancedFixture(t testing.TB) string {
	t.Helper()
	b, e := os.ReadFile("testdata/linear_tetra.step")
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestIndependentAdvancedTetra(t *testing.T) {
	m, e := Read(strings.NewReader(advancedFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	s := m.Bodies[0].Solid
	if s.Topology() != (brep.Topology{Vertices: 4, Edges: 6, Faces: 4, Shells: 1, EulerCharacteristic: 2}) {
		t.Fatal(s.Topology())
	}
	if math.Abs(s.Tessellate().SignedVolume()-1.0/6) > 1e-14 {
		t.Fatal("volume")
	}
	var b bytes.Buffer
	if e = Write(&b, m); e != nil {
		t.Fatal(e)
	}
	out, e := Read(&b, Limits{})
	if e != nil || out.Bodies[0].Solid.Topology() != s.Topology() {
		t.Fatal(out, e)
	}
}
func TestAdvancedGraphGeometryFailures(t *testing.T) {
	valid := advancedFixture(t)
	for _, kind := range []string{"EDGE_CURVE", "ORIENTED_EDGE", "ADVANCED_FACE"} {
		lines := strings.Split(valid, "\n")
		for i, s := range lines {
			if strings.Contains(s, "="+kind+"(") {
				lines[i] = strings.Replace(s, ".T.", ".F.", 1)
				break
			}
		}
		if _, e := Read(strings.NewReader(strings.Join(lines, "\n")), Limits{}); e == nil {
			t.Fatal("accepted reversed", kind)
		}
	}
	for _, bad := range []string{strings.Replace(valid, "=LINE(", "=CIRCLE(", 1), strings.Replace(valid, "VECTOR('',#9,1.)", "VECTOR('',#9,0.)", 1), strings.Replace(valid, "ORIENTED_EDGE('',*,*", "ORIENTED_EDGE('',#1,*", 1), strings.Replace(valid, "CARTESIAN_POINT('',(1.,0.,0.))", "CARTESIAN_POINT('',(1.,0.25,0.))", 1)} {
		if bad == valid {
			t.Fatal("test mutation missed")
		}
		if _, e := Read(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal("accepted malformed graph")
		}
	}
}
func TestAdvancedLimits(t *testing.T) {
	for _, l := range []Limits{{MaxVertices: 3}, {MaxFaces: 3}, {MaxEdgeUses: 11}} {
		if _, e := Read(strings.NewReader(advancedFixture(t)), l); e == nil {
			t.Fatal(l)
		}
	}
}
func stepOracle(t *testing.T, out string, args ...string) {
	t.Helper()
	program := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if program == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("set PLAINSOLID_OCCT_ORACLE and PLAINSOLID_OCCT_STEP=1 for independent STEP qualification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if b, e := exec.CommandContext(ctx, program, append([]string{out}, args...)...).CombinedOutput(); e != nil {
		t.Fatalf("OCCT: %v\n%s", e, b)
	}
}
func TestOCCTAdvancedSTEPImport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "occt.step")
	stepOracle(t, path, "write-step")
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(f, Limits{})
	_ = f.Close()
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	s := m.Bodies[0].Solid
	props, e := s.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(props.Volume-24) > 1e-9 || math.Abs(props.SurfaceArea-52) > 1e-9 || math.Abs(props.Centroid.X) > 1e-9 || math.Abs(props.Centroid.Y-3.5) > 1e-9 || math.Abs(props.Centroid.Z-5) > 1e-9 || s.Topology() != (brep.Topology{Vertices: 8, Edges: 12, Faces: 6, Shells: 1, EulerCharacteristic: 2}) {
		t.Fatal(props, s.Topology())
	}
	out := filepath.Join(t.TempDir(), "go.step")
	file, e := os.Create(out)
	if e != nil {
		t.Fatal(e)
	}
	e = Write(file, m)
	closeErr := file.Close()
	if e != nil || closeErr != nil {
		t.Fatal(e, closeErr)
	}
	metrics := filepath.Join(t.TempDir(), "metrics.json")
	stepOracle(t, metrics, out)
	b, e := os.ReadFile(metrics)
	if e != nil {
		t.Fatal(e)
	}
	var r struct{ Area, Volume float64 }
	if e = json.Unmarshal(b, &r); e != nil {
		t.Fatal(e)
	}
	if math.Abs(r.Area-52) > 1e-6 || math.Abs(r.Volume-24) > 1e-6 {
		t.Fatal(r)
	}
}
func FuzzReadAdvanced(f *testing.F) {
	f.Add(advancedFixture(f))
	f.Fuzz(func(t *testing.T, s string) {
		m, e := Read(strings.NewReader(s), Limits{MaxBytes: 1 << 16, MaxEntities: 300, MaxValues: 3000, MaxDepth: 12, MaxVertices: 100, MaxFaces: 100, MaxEdgeUses: 300, MaxSolids: 5})
		if e == nil {
			for _, b := range m.Bodies {
				if e = b.Solid.Validate(); e != nil {
					t.Fatal(e)
				}
			}
		}
	})
}
