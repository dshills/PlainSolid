package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"testing"
	"time"
)

func TestOCCTRepairEllipticalPrism(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_ELLIPTICAL_PRISM")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw := ellipticalPrismFixture(t)
	o := EllipticalWireRepairOptions{MaxDistance: 1e-11}
	m, e := RepairRoundedEllipticalPrismShell(bytes.NewReader(raw), Limits{}, o)
	if e != nil {
		t.Fatal(e)
	}
	// Native surfaces preserve original analytic charts independently of source.
	for _, face := range m.Faces {
		b, e := parametric.EncodeSurface(face.Surface)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = parametric.DecodeSurface(b, 100); e != nil {
			t.Fatal(e)
		}
	}
	var out bytes.Buffer
	if e = WriteEllipticalPrismShellSource(&out, m.Source, Limits{}, o); e != nil {
		t.Fatal(e)
	}
	for _, b := range [][]byte{raw, out.Bytes()} {
		path := filepath.Join(t.TempDir(), "prism.step")
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
			Area, Volume, LibraryVolume float64
			Faces, Solids, EdgeUses     int
			SurfaceCenter               [3]float64
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
		area := 3*sum*h/3 + 16*math.Pi
		if q.Faces != 3 || q.Solids != 1 || q.EdgeUses != 6 || math.Abs(q.Area-area) > 1e-8 || math.Abs(q.Volume-24*math.Pi) > 1e-8 || math.Abs(q.LibraryVolume-24*math.Pi) > 1e-8 || math.Abs(q.SurfaceCenter[0]-1) > 1e-9 || math.Abs(q.SurfaceCenter[1]-3.5) > 1e-9 || math.Abs(q.SurfaceCenter[2]-3) > 1e-9 {
			t.Fatal(q, area)
		}
		t.Logf("OCCT area %.17g volume %.17g", q.Area, q.Volume)
	}
}
