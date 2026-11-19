package step

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSplitCylinderModelOCCT(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_SPLIT_CYLINDER")
	if bin == "" {
		t.Skip("optional independent OCCT helper")
	}
	for _, name := range []string{"split-cylinder-model.step", "split-cylinder-model-posed.step"} {
		p, e := filepath.Abs("testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		b, e := exec.CommandContext(ctx, bin, p).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(b))
		}
		var v struct {
			Faces        int
			Volume, Area float64
			Center       [3]float64
		}
		if e = json.Unmarshal(b, &v); e != nil {
			t.Fatal(e, string(b))
		}
		want := [3]float64{0, 0, 1.5}
		if name == "split-cylinder-model-posed.step" {
			want = [3]float64{8.5, -4, 7}
		}
		for i := range want {
			if math.Abs(v.Center[i]-want[i]) > 1e-10 {
				t.Fatal("center", v.Center, want)
			}
		}
		if v.Faces != 4 || math.Abs(v.Volume-12*math.Pi) > 1e-10 || math.Abs(v.Area-20*math.Pi) > 1e-10 {
			t.Fatal(v)
		}
	}
}
