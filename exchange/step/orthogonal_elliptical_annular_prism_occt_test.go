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

func TestOCCTOrthogonalEllipticalAnnularPrism(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_ORTHOGONAL_ELLIPTICAL_ANNULAR_PRISM")
	if bin == "" {
		t.Skip("optional pinned OCCT helper")
	}
	d := OrthogonalEllipticalAnnularPrismDefinition{Name: "exact rational", Origin: [3]*big.Rat{big.NewRat(1, 10), big.NewRat(2, 1), big.NewRat(3, 1)}, Reference: geom.Vec3{X: 1}, Axis: geom.Vec3{Y: 3, Z: 4}, MajorRadius: 5, MinorRadius: 2, InnerMajorRadius: 1.5, InnerMinorRadius: .5, Height: 5}
	var b bytes.Buffer
	if e := WriteOrthogonalEllipticalAnnularPrismSTEP(context.Background(), &b, d, 1e-9); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "native.step")
	if e := os.WriteFile(out, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"testdata/occt-orthogonal-elliptical-annular-prism.step", out} {
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
		if result.Faces != 4 || result.Solids != 1 || result.EdgeUses != 12 || math.Abs(result.Volume-46.25*math.Pi) > 1e-8 || math.Abs(result.Area-206.59726012112046) > 1e-7 || math.Abs(result.SurfaceCenter[0]-.1) > 1e-9 || math.Abs(result.SurfaceCenter[1]-3.5) > 1e-9 || math.Abs(result.SurfaceCenter[2]-5) > 1e-9 {
			t.Fatal(result)
		}
	}
}
