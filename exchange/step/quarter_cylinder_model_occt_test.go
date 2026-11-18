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
	"time"
)

func TestQuarterCylinderModelOCCT(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_SPLIT_CYLINDER")
	if bin == "" {
		t.Skip("optional OCCT")
	}
	for _, native := range []bool{false, true} {
		p, e := filepath.Abs("testdata/quarter-cylinder-model.step")
		if e != nil {
			t.Fatal(e)
		}
		want := [3]float64{0, 0, 1.5}
		if native {
			var b bytes.Buffer
			if e = WriteQuarterCylinderModel(&b, splitWriterFixture(t)); e != nil {
				t.Fatal(e)
			}
			p = filepath.Join(t.TempDir(), "posed-quarter.step")
			if e = os.WriteFile(p, b.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			want = [3]float64{8.5, -4, 7}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		raw, e := exec.CommandContext(ctx, bin, p, "quarter").CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(raw))
		}
		var v struct {
			Faces        int
			Volume, Area float64
			Center       [3]float64
		}
		if e = json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e, string(raw))
		}
		if v.Faces != 6 || math.Abs(v.Volume-12*math.Pi) > 1e-10 || math.Abs(v.Area-20*math.Pi) > 1e-10 {
			t.Fatal(v)
		}
		for i, x := range want {
			if math.Abs(v.Center[i]-x) > 1e-10 {
				t.Fatal(v)
			}
		}
	}
}
