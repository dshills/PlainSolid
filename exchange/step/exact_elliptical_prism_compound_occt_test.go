package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOCCTExactEllipticalPrismCompound(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_EXACT_ELLIPSE_COMPOUND")
	if bin == "" {
		t.Skip("optional pinned OCCT helper")
	}
	source := "testdata/occt-exact-ellipse-compound.step"
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadExactEllipticalPrismCompound(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteExactEllipticalPrismCompound(context.Background(), &out, [2]*File{m.Members[0].Source, m.Members[1].Source}, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "compound.step")
	if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{source, path} {
		data, e := exec.Command(bin, "read", p).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(data))
		}
		var v struct {
			Area, Volume            float64
			Faces, Solids, EdgeUses int
			SurfaceCenter           [3]float64
		}
		if e = json.Unmarshal(bytes.TrimSpace(data), &v); e != nil {
			t.Fatal(e, string(data))
		}
		if v.Faces != 7 || v.Solids != 2 || v.EdgeUses != 18 || math.Abs(v.Volume-96.25*math.Pi) > 1e-8 || math.Abs(v.Area-384.49467617125) > 1e-7 || math.Abs(v.SurfaceCenter[0]-10.846430206961) > 1e-8 || math.Abs(v.SurfaceCenter[1]-3.5) > 1e-8 || math.Abs(v.SurfaceCenter[2]-5) > 1e-8 {
			t.Fatal(v)
		}
	}
}
