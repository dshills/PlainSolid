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

func TestOCCTRepairEllipseExtrusion(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_ELLIPSE_EXTRUSION")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw := extrusionFaceFixture(t)
	f, e := RepairRoundedEllipseExtrusionFace(bytes.NewReader(raw), Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteRepairedEllipseExtrusionFaceJSON(&native, f); e != nil {
		t.Fatal(e)
	}
	g, e := ReadRepairedEllipseExtrusionFaceJSON(&native, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	out, e := mappedPlanarSourceBytes(g.Source)
	if e != nil {
		t.Fatal(e)
	}
	for _, b := range [][]byte{raw, out} {
		path := filepath.Join(t.TempDir(), "side.step")
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		r, e := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(r))
		}
		at := bytes.LastIndex(r, []byte(`{"Area"`))
		if at < 0 {
			t.Fatal(string(r))
		}
		var q struct {
			Area, LibraryArea, Length float64
			Edges, Faces              int
			Center, Normal            [3]float64
		}
		if e = json.Unmarshal(r[at:], &q); e != nil {
			t.Fatal(e)
		}
		const n = 16384
		h := 2 * math.Pi / n
		sum := 0.
		for i := 0; i <= n; i++ {
			u := float64(i) * h
			w := 2.
			if i == 0 || i == n {
				w = 1
			} else if i%2 == 1 {
				w = 4
			}
			sum += w * math.Hypot(4*math.Sin(u), 2*math.Cos(u))
		}
		length := sum * h / 3
		if q.Faces != 1 || q.Edges != 4 || math.Abs(q.Area-3*length) > 1e-8 || math.Abs(q.Length-(2*length+6)) > 1e-8 || math.Abs(q.Center[0]-1) > 1e-9 || math.Abs(q.Center[1]-3.5) > 1e-9 || math.Abs(q.Center[2]-3) > 1e-9 || q.Normal != ([3]float64{0, 0, 1}) {
			t.Fatal(q, length)
		}
		t.Logf("OCCT parametric integration area %.17g; BRepGProp area %.17g (not used as proof)", q.Area, q.LibraryArea)
	}
}
func TestRepairEllipseExtrusionNative(t *testing.T) {
	f, e := RepairRoundedEllipseExtrusionFace(bytes.NewReader(extrusionFaceFixture(t)), Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteRepairedEllipseExtrusionFaceJSON(&b, f); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	g, e := ReadRepairedEllipseExtrusionFaceJSON(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if g.SourceFaceID != f.SourceFaceID {
		t.Fatal("ancestry")
	}
	f.Face.Outer[2].Reversed = false
	if WriteRepairedEllipseExtrusionFaceJSON(&bytes.Buffer{}, f) == nil {
		t.Fatal("stale use")
	}
	var x map[string]any
	if json.Unmarshal(raw, &x) != nil {
		t.Fatal("json")
	}
	for _, field := range []string{"version", "max_distance_model_units", "reconstructed_surface"} {
		y := map[string]any{}
		for k, v := range x {
			y[k] = v
		}
		y[field] = 0.
		bad, _ := json.Marshal(y)
		q, e := ReadRepairedEllipseExtrusionFaceJSON(bytes.NewReader(bad), Limits{})
		if e == nil || q.Source != nil {
			t.Fatal("tamper", field)
		}
	}
}
