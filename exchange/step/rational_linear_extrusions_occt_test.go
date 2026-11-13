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
	"time"
)

func checkRationalExtrusionOCCTSource(t *testing.T, raw []byte) {
	t.Helper()
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	found := 0
	for _, id := range f.Order {
		if _, ok := component(f.Entities[id], "RECTANGULAR_TRIMMED_SURFACE"); !ok {
			continue
		}
		s, e := DecodeRationalLinearExtrusionSurface(f, id, 64)
		if e != nil {
			t.Fatal(e)
		}
		ratios := s.DirectionRatios()
		if ratios[0].Cmp(big.NewRat(3, 5)) != 0 || ratios[1].Cmp(big.NewRat(4, 5)) != 0 {
			t.Fatal("source ratios lost")
		}
		g, e := s.Surface()
		if e != nil {
			t.Fatal(e)
		}
		p, e := g.Eval(.5, 2.5)
		if e != nil || p.Distance(geom.Vec3{X: -.5, Y: 3.5, Z: 1}) > 1e-12 {
			t.Fatal(p, e)
		}
		if _, e = DecodeLinearExtrusionSurface(f, id, 64); e == nil {
			t.Fatal("old API broadened")
		}
		found++
	}
	if found != 1 {
		t.Fatal("source count", found)
	}
}
func TestRationalLinearExtrusionRetainedOCCT(t *testing.T) {
	raw, e := os.ReadFile("testdata/rational-linear-extrusion-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	checkRationalExtrusionOCCTSource(t, raw)
}
func TestOCCTRationalLinearExtrusions(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_RATIONAL_EXTRUSION")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_RATIONAL_EXTRUSION to updated standalone helper")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	source := filepath.Join(t.TempDir(), "source.step")
	if b, e := exec.CommandContext(ctx, oracle, "write-oblique", source).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	checkRationalExtrusionOCCTSource(t, raw)
	path := filepath.Join(t.TempDir(), "native.step")
	file, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	e = WriteRationalLinearExtrusions(file, RationalLinearExtrusionModel{Surfaces: []RationalLinearExtrusionSurface{rationalExtrusionFixture(t, false)}, LengthUnit: .001})
	ce := file.Close()
	if e != nil || ce != nil {
		t.Fatal(e, ce)
	}
	b, e := exec.CommandContext(ctx, oracle, "read", path).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(b))
	}
	var got struct {
		Faces int
		Area  float64
	}
	if e = json.Unmarshal(bytes.TrimSpace(b), &got); e != nil {
		t.Fatal(e, string(b))
	}
	// Profile (-8u(1-u),6u(1-u),2u) is perpendicular to extrusion.
	// Its exact arc-length integral is sqrt(26)+asinh(5)/5; height is5.
	want := 5*math.Sqrt(26) + math.Asinh(5)
	if got.Faces != 1 || math.Abs(got.Area-want) > 1e-7 {
		t.Fatal(got, want)
	}
}
