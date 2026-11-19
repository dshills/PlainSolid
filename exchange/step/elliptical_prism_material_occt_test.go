package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"testing"
	"time"
)

func TestOCCTEllipticalPrismMaterial(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_ELLIPTICAL_PRISM")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	o := EllipticalWireRepairOptions{MaxDistance: 1e-11}
	shell, e := RepairRoundedEllipticalPrismShell(bytes.NewReader(ellipticalPrismFixture(t)), Limits{}, o)
	if e != nil {
		t.Fatal(e)
	}
	m, e := PromoteRepairedEllipticalPrism(context.Background(), shell.Source, Limits{}, o, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteEllipticalPrismMaterialJSON(context.Background(), &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadEllipticalPrismMaterialJSON(context.Background(), &native, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	props, e := m.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteEllipticalPrismShellSource(&out, m.Shell.Source, Limits{}, o); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "roundtrip.step")
	if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	b, e := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
	cancel()
	if e != nil {
		t.Fatal(e, string(b))
	}
	at := bytes.LastIndex(b, []byte(`{"Area"`))
	if at < 0 {
		t.Fatal(string(b))
	}
	var q struct {
		Area, Volume  float64
		Faces, Solids int
	}
	if e = json.Unmarshal(b[at:], &q); e != nil {
		t.Fatal(e)
	}
	if q.Solids != 1 || q.Faces != 3 || math.Abs(props.Volume-q.Volume) > 1e-7 || math.Abs(props.Area-q.Area) > 1e-7 {
		t.Fatal(props, q)
	}
	t.Logf("native and OCCT: V %.17g / %.17g A %.17g / %.17g", props.Volume, q.Volume, props.Area, q.Area)
}
