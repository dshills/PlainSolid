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

func TestOCCTLinearExtrusions(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_LINEAR_EXTRUSION")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_LINEAR_EXTRUSION to standalone helper")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	generated := filepath.Join(t.TempDir(), "occt.step")
	if b, e := exec.CommandContext(ctx, oracle, "write", generated).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	raw, e := os.ReadFile(generated)
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	found := 0
	for _, id := range f.Order {
		if _, ok := component(f.Entities[id], "RECTANGULAR_TRIMMED_SURFACE"); !ok {
			continue
		}
		s, e := DecodeLinearExtrusionSurface(f, id, 64)
		if e != nil {
			t.Fatal(e)
		}
		surface, e := s.Surface()
		if e != nil {
			t.Fatal(e)
		}
		p, e := surface.Eval(.25, .5)
		if e != nil || math.Abs(p.X-.5) > 1e-12 || math.Abs(p.Y-.75) > 1e-12 || math.Abs(p.Z-.5) > 1e-12 {
			t.Fatal(p, e)
		}
		found++
	}
	if found != 1 {
		t.Fatal("independent bounded support count", found)
	}
	m, e := ReadLinearExtrusions(bytes.NewReader(linearExtrusionFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	output := filepath.Join(t.TempDir(), "native.step")
	file, e := os.Create(output)
	if e != nil {
		t.Fatal(e)
	}
	e = WriteLinearExtrusions(file, m)
	ce := file.Close()
	if e != nil || ce != nil {
		t.Fatal(e, ce)
	}
	b, e := exec.CommandContext(ctx, oracle, "read", output).CombinedOutput()
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
	// x=2u,y=4u(1-u), length=sqrt(5)+asinh(2)/2; extrusion interval length3.
	want := 3 * (math.Sqrt(5) + math.Asinh(2)/2)
	if got.Faces != 1 || math.Abs(got.Area-want) > 1e-7 {
		t.Fatal(got, want)
	}
}
