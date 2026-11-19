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

func TestOCCTExactEllipticalPrismCollection(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_EXACT_ELLIPSE_COLLECTION")
	if bin == "" {
		t.Skip("optional pinned OCCT helper")
	}
	source := "testdata/occt-exact-ellipse-collection.step"
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadExactEllipticalPrismCollection(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteExactEllipticalPrismCollection(context.Background(), &out, []*File{m.Members[0].Source, m.Members[1].Source, m.Members[2].Source}, Limits{}, 1e-9); e != nil {
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
		if v.Faces != 10 || v.Solids != 3 || v.EdgeUses != 24 || math.Abs(v.Volume-111.25*math.Pi) > 1e-8 || math.Abs(v.Area-470.16869819555723) > 1e-7 || math.Abs(v.SurfaceCenter[0]-16.177008342760743) > 1e-8 || math.Abs(v.SurfaceCenter[1]-3.5) > 1e-8 || math.Abs(v.SurfaceCenter[2]-5) > 1e-8 {
			t.Fatal(v)
		}
	}
}
