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
	bin := os.Getenv("PLAINSOLID_OCCT_SPLIT_CYLINDER")
	if bin == "" {
		t.Skip("optional independent OCCT helper")
	}
	p, e := filepath.Abs("testdata/split-cylinder-model.step")
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
	}
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e, string(b))
	}
	if v.Faces != 4 || math.Abs(v.Volume-12*math.Pi) > 1e-10 || math.Abs(v.Area-20*math.Pi) > 1e-10 {
		t.Fatal(v)
	}
}
