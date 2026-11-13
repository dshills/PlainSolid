package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/exchange/step"
	"testing"
	"time"
)

func TestOCCTRevolutionSurfacesNative(t *testing.T) {
	testOCCTRevolutionNative(t, "write-oblique", WriteRevolutionSurfacesJSON, ReadRevolutionSurfacesJSON)
}
func TestOCCTRationalRevolutionSurfacesNative(t *testing.T) {
	testOCCTRevolutionNative(t, "write-rational", WriteRationalRevolutionSurfacesJSON, ReadRationalRevolutionSurfacesJSON)
}
func testOCCTRevolutionNative(t *testing.T, mode string, write func(io.Writer, step.RevolutionSurfaceModel) error, read func(io.Reader, RevolutionSurfaceJSONLimits) (step.RevolutionSurfaceModel, error)) {
	oracle := os.Getenv("PLAINSOLID_OCCT_REVOLUTION_SURFACE")
	if oracle == "" {
		t.Skip("optional revolution helper")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	source := filepath.Join(t.TempDir(), "source.step")
	if b, e := exec.CommandContext(ctx, oracle, mode, source).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	b, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	f, e := step.Parse(bytes.NewReader(b), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m := step.RevolutionSurfaceModel{LengthUnit: .001}
	for _, id := range f.Order {
		for _, r := range f.Entities[id].Records {
			if r.Type == "RECTANGULAR_TRIMMED_SURFACE" {
				s, e := step.DecodeRevolutionSurface(f, id, 64)
				if e != nil {
					t.Fatal(e)
				}
				m.Surfaces = append(m.Surfaces, s)
			}
		}
	}
	if len(m.Surfaces) != 1 {
		t.Fatal(len(m.Surfaces))
	}
	var native bytes.Buffer
	if e = write(&native, m); e != nil {
		t.Fatal(e)
	}
	loaded, e := read(&native, RevolutionSurfaceJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = step.WriteRevolutionSurfaces(&out, loaded); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(t.TempDir(), "native.step")
	if e = os.WriteFile(target, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	type measure struct {
		Faces         int
		Area          float64
		Point, Du, Dv [3]float64
	}
	var measures []measure
	for _, p := range []string{source, target} {
		b, e = exec.CommandContext(ctx, oracle, "read", p).Output()
		if e != nil {
			t.Fatal(e)
		}
		var v measure
		if e = json.Unmarshal(bytes.TrimSpace(b), &v); e != nil {
			t.Fatal(e, string(b))
		}
		if v.Faces != 1 {
			t.Fatal(v)
		}
		measures = append(measures, v)
	}
	if math.Abs(measures[0].Area-measures[1].Area) > 1e-10 {
		t.Fatal(measures)
	}
	for j := 0; j < 3; j++ {
		for _, pair := range [][2]float64{{measures[0].Point[j], measures[1].Point[j]}, {measures[0].Du[j], measures[1].Du[j]}, {measures[0].Dv[j], measures[1].Dv[j]}} {
			if math.Abs(pair[0]-pair[1]) > 1e-11 {
				t.Fatal(pair)
			}
		}
	}
}
