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
	"plainsolid/geom"
	"testing"
)

func TestIndependentRoundUnits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-round-units.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadNormalizedIndependentRoundMaterials(ctx, bytes.NewReader(raw), RoundMillimetre, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Members) != 2 || m.OriginalUnits[0] != RoundMillimetre || m.OriginalUnits[1] != RoundMetre || m.Members[0].Cylinder.Radius != 2 || m.Members[1].Sphere.Radius != 2000 || m.Members[1].Sphere.Frame.Origin().Z != 3000 {
		t.Fatal("scaled")
	}
	original, e := mappedPlanarSourceBytes(m.OriginalSource)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadIndependentSphereMaterials(ctx, bytes.NewReader(original), Limits{}, 1e-9); e == nil {
		t.Fatal("old unit contract widened")
	}
	normalized, e := mappedPlanarSourceBytes(m.NormalizedSource)
	if e != nil {
		t.Fatal(e)
	}
	n, e := ReadNormalizedIndependentRoundMaterials(ctx, bytes.NewReader(normalized), RoundMillimetre, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	same, e := mappedPlanarSourceBytes(n.NormalizedSource)
	if e != nil || !bytes.Equal(same, normalized) {
		t.Fatal("not stable", e)
	}
	// The millimetre cylinder's radius 2/1000 is not binary64 exact in metres.
	if v, e := ReadNormalizedIndependentRoundMaterials(ctx, bytes.NewReader(raw), RoundMetre, Limits{}, 1e-9); !errors.Is(e, geom.ErrUnrepresentable) || len(v.Members) != 0 {
		t.Fatal("rounded", e)
	}
	for _, target := range []RoundMaterialSIUnit{"", "inch"} {
		if _, e = ReadNormalizedIndependentRoundMaterials(ctx, bytes.NewReader(raw), target, Limits{}, 1e-9); e == nil {
			t.Fatal("target")
		}
	}
	for _, kind := range []string{"unit", "pole", "radius"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		changed := false
		for id, en := range f.Entities {
			for j, r := range en.Records {
				if kind == "unit" && r.Type == "SI_UNIT" && r.Args[1].Text == "METRE" {
					en.Records[j].Args[0] = Value{Kind: Enumeration, Text: "CENTI"}
					changed = true
				}
				if kind == "radius" && r.Type == "SPHERICAL_SURFACE" {
					en.Records[j].Args[2].Text = "2.0000000000000004"
					changed = true
				}
				if kind == "pole" && id == m.Members[1].Sphere.SourcePointID {
					en.Records[j].Args[1].Items[2].Text = "1.0000000000000002"
					changed = true
				}
			}
			f.Entities[id] = en
			if changed {
				break
			}
		}
		if !changed {
			t.Fatal("mutation")
		}
		b, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		if v, e := ReadNormalizedIndependentRoundMaterials(ctx, bytes.NewReader(b), RoundMillimetre, Limits{}, 1e-9); e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}
	solo, e := os.ReadFile("testdata/occt-independent-metre-sphere.step")
	if e != nil {
		t.Fatal(e)
	}
	up, e := ReadNormalizedIndependentRoundMaterials(ctx, bytes.NewReader(solo), RoundMillimetre, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	upBytes, e := mappedPlanarSourceBytes(up.NormalizedSource)
	if e != nil {
		t.Fatal(e)
	}
	down, e := ReadNormalizedIndependentRoundMaterials(ctx, bytes.NewReader(upBytes), RoundMetre, Limits{}, 1e-9)
	if e != nil || down.Members[0].Sphere.Radius != 2 || down.Members[0].Sphere.Frame.Origin().Z != 3 {
		t.Fatal("exact reverse", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadNormalizedIndependentRoundMaterials(cancelled, bytes.NewReader(raw), RoundMillimetre, Limits{}, 1e-9); e == nil {
		t.Fatal("cancel")
	}
	for _, limit := range []Limits{{MaxBytes: 1}, {MaxSolids: 1}, {MaxEntities: 20}, {MaxFaces: 1}} {
		if v, e := ReadNormalizedIndependentRoundMaterials(ctx, bytes.NewReader(raw), RoundMillimetre, limit, 1e-9); e == nil || len(v.Members) != 0 {
			t.Fatal("limit")
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_ROUND_UNITS"); oracle != "" {
		for _, b := range [][]byte{raw, normalized} {
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
				Solids int
				Volume float64
			}
			if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Solids != 2 || math.Abs(q.Volume-(12*math.Pi+32e9*math.Pi/3)) > 1e-3 {
				t.Fatal(string(data))
			}
		}
	}
}

func FuzzIndependentRoundUnits(f *testing.F) {
	raw, e := os.ReadFile("testdata/independent-round-units.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		v, e := ReadNormalizedIndependentRoundMaterials(context.Background(), bytes.NewReader(b), RoundMillimetre, Limits{MaxEntities: 1024, MaxValues: 16384, MaxSolids: 4}, 1e-9)
		if e != nil && len(v.Members) != 0 {
			t.Fatal("partial")
		}
	})
}
