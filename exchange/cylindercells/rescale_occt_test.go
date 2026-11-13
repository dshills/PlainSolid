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

func TestCylinderAssetRescaleOCCTPhysicalMeasures(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_SPLIT_CYLINDER")
	if bin == "" {
		t.Skip("optional OCCT")
	}
	for _, dialect := range []string{Half, Quarter} {
		a, e := FromNative(native(t), "physical units", SIUnit{0}, dialect)
		if e != nil {
			t.Fatal(e)
		}
		scaled, e := RescaleSI(a, SIUnit{-3})
		if e != nil {
			t.Fatal(e)
		}
		type measures struct {
			Faces        int
			Volume, Area float64
			Center       [3]float64
		}
		got := []measures{}
		for _, asset := range []Asset{a, scaled} {
			var b bytes.Buffer
			if e = WriteCanonicalSTEP(&b, asset); e != nil {
				t.Fatal(e)
			}
			p := filepath.Join(t.TempDir(), "units.step")
			if e = os.WriteFile(p, b.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			mode := "roots"
			if dialect == Quarter {
				mode = "quarter-roots"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			raw, e := exec.CommandContext(ctx, bin, p, mode).CombinedOutput()
			cancel()
			if e != nil {
				t.Fatal(e, string(raw))
			}
			var m measures
			if e = json.Unmarshal(raw, &m); e != nil {
				t.Fatal(e, string(raw))
			}
			got = append(got, m)
		}
		if got[0].Faces != got[1].Faces {
			t.Fatal(got)
		}
		x := []float64{got[0].Volume, got[0].Area, got[0].Center[0], got[0].Center[1], got[0].Center[2]}
		y := []float64{got[1].Volume, got[1].Area, got[1].Center[0], got[1].Center[1], got[1].Center[2]}
		for i := range x {
			if math.Abs(x[i]-y[i]) > 1e-12*math.Max(1, math.Abs(x[i])) {
				t.Fatal("physical OCCT measure changed", got)
			}
		}
	}
}
