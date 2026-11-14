package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestIndependentToroidalUnits(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-toroidal-units.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadNormalizedIndependentToroidalMaterials(ctx, bytes.NewReader(raw), IndependentMillimetre, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Members) != 4 || m.Members[3].Torus == nil || m.OriginalUnits[3] != IndependentMetre {
		t.Fatal("family")
	}
	q := m.Members[3].Torus
	if q.MajorRadius != 3000 || q.MinorRadius != 1000 || q.Frame.Origin().X != 1000 || q.Frame.Origin().Z != 3000 {
		t.Fatal("scaled")
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Members[3].Solid); e != nil {
		t.Fatal(e)
	}
	if _, e = trimmed.ReadJSON(&native, trimmed.JSONLimits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadNormalizedIndependentMaterials(ctx, bytes.NewReader(raw), IndependentMillimetre, Limits{}, 1e-9); e == nil {
		t.Fatal("old mixed widened")
	}
	var forbidden bytes.Buffer
	if e = WriteNormalizedIndependentMaterialsJSON(ctx, &forbidden, m); e == nil || forbidden.Len() != 0 {
		t.Fatal("old persistence widened")
	}
	var sources []*File
	for _, p := range m.Members {
		sources = append(sources, p.Source)
		if _, ok := m.OriginalSource.Entities[p.SourceBodyID]; !ok {
			t.Fatal("source body")
		}
	}
	var out bytes.Buffer
	if e = WriteIndependentToroidalMaterials(ctx, &out, sources, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if v, e := ReadNormalizedIndependentToroidalMaterials(ctx, bytes.NewReader(raw), IndependentMetre, Limits{}, 1e-9); !errors.Is(e, geom.ErrUnrepresentable) || len(v.Members) != 0 {
		t.Fatal("rounding", e)
	}
	solo, e := os.ReadFile("testdata/independent-model-metre-torus.step")
	if e != nil {
		t.Fatal(e)
	}
	up, e := ReadNormalizedIndependentToroidalMaterials(ctx, bytes.NewReader(solo), IndependentMillimetre, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	b, e := mappedPlanarSourceBytes(up.NormalizedSource)
	if e != nil {
		t.Fatal(e)
	}
	down, e := ReadNormalizedIndependentToroidalMaterials(ctx, bytes.NewReader(b), IndependentMetre, Limits{}, 1e-9)
	if e != nil || down.Members[0].Torus.MajorRadius != 3 || down.Members[0].Torus.MinorRadius != 1 {
		t.Fatal("reverse", e)
	}
	mmTorus, e := os.ReadFile("testdata/independent-model-torus.step")
	if e != nil {
		t.Fatal(e)
	}
	if v, e := ReadNormalizedIndependentToroidalMaterials(ctx, bytes.NewReader(mmTorus), IndependentMetre, Limits{}, 1e-9); !errors.Is(e, geom.ErrUnrepresentable) || len(v.Members) != 0 {
		t.Fatal("torus radius rounding", e)
	}
	vendor, e := os.ReadFile("testdata/occt-independent-unsupported-torus.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadNormalizedIndependentToroidalMaterials(ctx, bytes.NewReader(vendor), IndependentMillimetre, Limits{}, 1e-9); e == nil {
		t.Fatal("rounded vendor")
	}
	for _, kind := range []string{"minor", "major", "seam", "underflow"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		id := q.SourceSurfaceID
		if kind == "seam" {
			id = q.SourceCircleIDs[1]
		}
		en := f.Entities[id]
		r := &en.Records[0]
		switch kind {
		case "minor":
			r.Args[3].Text = "2."
		case "major":
			r.Args[2].Text = "1."
		case "seam":
			r.Args[2].Text = "2."
		case "underflow":
			r.Args[3].Text = "1.E-999999999"
		}
		f.Entities[id] = en
		b, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		if v, e := ReadNormalizedIndependentToroidalMaterials(ctx, bytes.NewReader(b), IndependentMillimetre, Limits{}, 1e-9); e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}
	for _, l := range []Limits{{MaxBytes: 1}, {MaxSolids: 3}, {MaxEntities: 20}, {MaxFaces: 1}} {
		if v, e := ReadNormalizedIndependentToroidalMaterials(ctx, bytes.NewReader(raw), IndependentMillimetre, l, 1e-9); e == nil || len(v.Members) != 0 {
			t.Fatal("budget")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadNormalizedIndependentToroidalMaterials(cancelled, bytes.NewReader(raw), IndependentMillimetre, Limits{}, 1e-9); e == nil {
		t.Fatal("cancel")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TOROIDAL_UNITS"); oracle != "" {
		for _, b := range [][]byte{raw, out.Bytes()} {
			p := filepath.Join(t.TempDir(), "model.step")
			if e = os.WriteFile(p, b, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := exec.Command(oracle, "read", p).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			at := bytes.LastIndex(data, []byte("{"))
			var q struct {
				Solids       int
				Volume, Area float64
			}
			if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Solids != 4 || math.Abs(q.Volume-(24e9+12*math.Pi+32e9*math.Pi/3+6e9*math.Pi*math.Pi)) > 1e-3 || math.Abs(q.Area-(52e6+(16e6+20)*math.Pi+12e6*math.Pi*math.Pi)) > 1e-3 {
				t.Fatal(string(data))
			}
		}
	}
}
func FuzzIndependentToroidalUnits(f *testing.F) {
	b, e := os.ReadFile("testdata/independent-toroidal-units.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		v, e := ReadNormalizedIndependentToroidalMaterials(context.Background(), bytes.NewReader(b), IndependentMillimetre, Limits{MaxEntities: 1024, MaxValues: 16384, MaxSolids: 4}, 1e-9)
		if e != nil && len(v.Members) != 0 {
			t.Fatal("partial")
		}
	})
}
