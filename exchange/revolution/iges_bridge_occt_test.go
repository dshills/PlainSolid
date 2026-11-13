package revolution

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"testing"
	"time"
)

func TestOCCTIGESBridge(t *testing.T) {
	ig := os.Getenv("PLAINSOLID_OCCT_IGES_REVOLUTION")
	st := os.Getenv("PLAINSOLID_OCCT_REVOLUTION_SURFACE")
	if ig == "" || st == "" {
		t.Skip("set both independent revolution helpers")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	sourceIG := filepath.Join(dir, "source.igs")
	if b, e := exec.CommandContext(ctx, ig, "write", sourceIG).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	raw, e := os.ReadFile(sourceIG)
	if e != nil {
		t.Fatal(e)
	}
	im, e := iges.ReadRevolutionSurfaces(bytes.NewReader(raw), iges.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := FromIGES(im)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteRationalRevolutionSurfacesJSON(&native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadRationalRevolutionSurfacesJSON(&native, RevolutionSurfaceJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	targetST := filepath.Join(dir, "native.step")
	var b bytes.Buffer
	if e = step.WriteRevolutionSurfaces(&b, m); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(targetST, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	sourceST := filepath.Join(dir, "source.step")
	if out, e := exec.CommandContext(ctx, st, "write-rational", sourceST).CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	raw, e = os.ReadFile(sourceST)
	if e != nil {
		t.Fatal(e)
	}
	f, e := step.Parse(bytes.NewReader(raw), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m = step.RevolutionSurfaceModel{LengthUnit: .001}
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
	if _, e = ToIGES(m); e == nil {
		t.Fatal("long original STEP name silently lost")
	}
	// Explicit caller-selected label change; the strict bridge itself never truncates.
	m.Surfaces[0].Name = "REVOLVE"
	m.Surfaces[0].Profile.Name = "PROFILE"
	native.Reset()
	if e = WriteRationalRevolutionSurfacesJSON(&native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadRationalRevolutionSurfacesJSON(&native, RevolutionSurfaceJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	im, e = ToIGES(m)
	if e != nil {
		t.Fatal(e)
	}
	targetIG := filepath.Join(dir, "native.igs")
	b.Reset()
	if e = iges.WriteRevolutionSurfaces(&b, im); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(targetIG, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	type measure struct {
		Faces         int
		Area          float64
		Point, Du, Dv [3]float64
	}
	var first *measure
	for _, tc := range [][2]string{{ig, sourceIG}, {st, targetST}, {st, sourceST}, {ig, targetIG}} {
		raw, e = exec.CommandContext(ctx, tc[0], "read", tc[1]).Output()
		if e != nil {
			t.Fatal(e, string(raw))
		}
		start := bytes.LastIndexByte(raw, '{')
		if start < 0 {
			t.Fatal(string(raw))
		}
		var v measure
		if e = json.Unmarshal(bytes.TrimSpace(raw[start:]), &v); e != nil {
			t.Fatal(e)
		}
		if v.Faces != 1 {
			t.Fatal(v)
		}
		if first == nil {
			first = &v
			continue
		}
		if math.Abs(first.Area-v.Area) > 1e-9 {
			t.Fatal(first, v)
		}
		for i := 0; i < 3; i++ {
			for _, q := range [][2]float64{{first.Point[i], v.Point[i]}, {first.Du[i], v.Du[i]}, {first.Dv[i], v.Dv[i]}} {
				if math.Abs(q[0]-q[1]) > 1e-10 {
					t.Fatal(q)
				}
			}
		}
	}
}
