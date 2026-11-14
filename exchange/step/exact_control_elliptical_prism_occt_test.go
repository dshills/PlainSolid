package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func TestOCCTExactControlEllipticalPrism(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_EXACT_CONTROL_ELLIPTICAL_PRISM")
	if bin == "" {
		t.Skip("optional pinned OCCT helper")
	}
	d := ExactControlEllipticalPrismDefinition{Name: "exact rational", Origin: [3]*big.Rat{big.NewRat(1, 10), big.NewRat(2, 1), big.NewRat(3, 1)}, Reference: geom.Vec3{X: 3, Y: 4}, Axis: geom.Vec3{Z: 1}, MajorRadius: 5, MinorRadius: 2, Height: 3}
	var b bytes.Buffer
	if e := WriteExactControlEllipticalPrismSTEP(context.Background(), &b, d, 1e-9); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "native.step")
	if e := os.WriteFile(out, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"testdata/occt-exact-control-elliptical-prism.step", out} {
		data, e := exec.Command(bin, "read", p).CombinedOutput()
		if e != nil {
			t.Fatalf("%v: %s", e, data)
		}
		var result struct {
			Area, Volume            float64
			Faces, Solids, EdgeUses int
			SurfaceCenter           [3]float64
		}
		if e = json.Unmarshal(bytes.TrimSpace(data), &result); e != nil {
			t.Fatal(e, string(data))
		}
		if result.Faces != 3 || result.Solids != 1 || result.EdgeUses != 6 || math.Abs(result.Volume-30*math.Pi) > 1e-8 || math.Abs(result.Area-131.87119085879) > 1e-7 || math.Abs(result.SurfaceCenter[0]-.1) > 1e-9 {
			t.Fatal(result)
		}
	}
}
