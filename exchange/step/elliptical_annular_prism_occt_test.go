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

func TestOCCTEllipticalAnnularPrism(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_ELLIPTICAL_ANNULAR_PRISM")
	if bin == "" {
		t.Skip("optional pinned OCCT helper")
	}
	d := EllipticalAnnularPrismDefinition{Name: "exact rational", Origin: [3]*big.Rat{big.NewRat(1, 10), big.NewRat(2, 1), big.NewRat(3, 1)}, Reference: geom.Vec3{X: 1}, Axis: geom.Vec3{Y: 3, Z: 4}, MajorRadius: 5, MinorRadius: 2, InnerMajorRadius: 2.5, InnerMinorRadius: 1, Height: 5}
	var b bytes.Buffer
	if e := WriteEllipticalAnnularPrismSTEP(context.Background(), &b, d, 1e-9); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "native.step")
	if e := os.WriteFile(out, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"testdata/occt-elliptical-annular-prism.step", out} {
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
		if result.Faces != 4 || result.Solids != 1 || result.EdgeUses != 12 || math.Abs(result.Volume-37.5*math.Pi) > 1e-8 || math.Abs(result.Area-219.72223427133798) > 1e-7 || math.Abs(result.SurfaceCenter[0]-.1) > 1e-9 || math.Abs(result.SurfaceCenter[1]-3.5) > 1e-9 || math.Abs(result.SurfaceCenter[2]-5) > 1e-9 {
			t.Fatal(result)
		}
	}
}
