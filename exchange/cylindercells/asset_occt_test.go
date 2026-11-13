package cylindercells

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

func TestCylinderAssetOCCTChain(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_SPLIT_CYLINDER")
	if bin == "" {
		t.Skip("optional OCCT")
	}
	for _, dialect := range []string{Half, Quarter} {
		a, e := FromNative(native(t), "stored named part", SIUnit{-3}, dialect)
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		_ = WriteSTEP(&b, a)
		a, e = FromSTEP(&b, dialect)
		if e != nil {
			t.Fatal(e)
		}
		if e = WriteNative(&b, a); e != nil {
			t.Fatal(e)
		}
		a, e = ReadNative(&b)
		if e != nil {
			t.Fatal(e)
		}
		if e = WriteSTEP(&b, a); e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(t.TempDir(), "asset.step")
		if e = os.WriteFile(p, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		mode := "roots"
		faces := 4
		if dialect == Quarter {
			mode = "quarter-roots"
			faces = 6
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		raw, e := exec.CommandContext(ctx, bin, p, mode).CombinedOutput()
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
		if v.Faces != faces || math.Abs(v.Volume-12*math.Pi) > 1e-10 || math.Abs(v.Area-20*math.Pi) > 1e-10 {
			t.Fatal(v)
		}
		for i, x := range []float64{-3, 5, .5} {
			if math.Abs(v.Center[i]-x) > 1e-10 {
				t.Fatal(v)
			}
		}
	}
}
