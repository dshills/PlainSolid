package step

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOCCTSTEPSurfaceExport(t *testing.T) {
	m, e := ReadSurfaces(strings.NewReader(independentSurface), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "surface.step")
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	e = WriteSurfaces(f, m)
	ce := f.Close()
	if e != nil || ce != nil {
		t.Fatal(e, ce)
	}
	result := filepath.Join(t.TempDir(), "result.json")
	stepOracle(t, result, "step-surface", path)
	data, e := os.ReadFile(result)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Faces int
		Area  float64
	}
	if e = json.Unmarshal(data, &got); e != nil {
		t.Fatal(e)
	}
	if got.Faces != 1 || math.Abs(got.Area-math.Pi) > 1e-6 {
		t.Fatal(got)
	}
}

// OCCT exports a topological face graph. Decode the exact support surface,
// without claiming that ReadSurfaces imports that face or its trimming graph.
func TestOCCTSTEPSurfaceDecode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "occt.step")
	stepOracle(t, path, "write-step-surface")
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	parsed, e := Parse(f, Limits{})
	_ = f.Close()
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for id, ent := range parsed.Entities {
		if _, ok := component(ent, "B_SPLINE_SURFACE_WITH_KNOTS"); !ok {
			continue
		}
		s, e := DecodeBSplineSurface(parsed, id, 100)
		if e != nil {
			t.Fatal(e)
		}
		count++
		for _, u := range []float64{0, .25, .5, 1} {
			for _, v := range []float64{0, .3, 1} {
				p, e := s.Geometry.Eval(u, v)
				if e != nil || math.Abs(math.Hypot(p.X, p.Y)-1) > 1e-9 || math.Abs(p.Z-2*v) > 1e-9 {
					t.Fatal(p, e)
				}
			}
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
}
