package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestIndependentPlanarUnits(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-planar-units.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadNormalizedIndependentPlanarMaterials(ctx, bytes.NewReader(raw), PlanarMillimetre, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	for _, member := range m.Members {
		var native bytes.Buffer
		if e = trimmed.WriteJSON(&native, member.Solid); e != nil {
			t.Fatal(e)
		}
		if _, e = trimmed.ReadJSON(&native, trimmed.JSONLimits{}); e != nil {
			t.Fatal(e)
		}
	}
	if len(m.Members) != 2 || m.OriginalUnits[0] != PlanarMillimetre || m.OriginalUnits[1] != PlanarMetre {
		t.Fatal("units")
	}
	if _, e = ReadIndependentExactMaterials(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old widened")
	}
	// Independent exact scalar check covers each authored curve's origin and speed,
	// including every 2D pcurve; source directions remain byte-for-byte unchanged.
	points2, points3, vectors := 0, 0, 0
	for _, id := range m.Members[1].Source.Order {
		a, b := m.OriginalSource.Entities[id], m.NormalizedSource.Entities[id]
		for j, r := range a.Records {
			scaled := []Value{}
			original := []Value{}
			switch r.Type {
			case "CARTESIAN_POINT":
				original = r.Args[1].Items
				scaled = b.Records[j].Args[1].Items
				if len(original) == 2 {
					points2++
				} else {
					points3++
				}
			case "VECTOR":
				original = []Value{r.Args[2]}
				scaled = []Value{b.Records[j].Args[2]}
				vectors++
			case "DIRECTION":
				if !reflect.DeepEqual(r, b.Records[j]) {
					t.Fatal("direction changed")
				}
			}
			for k, v := range original {
				x, ok := new(big.Rat).SetString(v.Text)
				if !ok {
					t.Fatal("original")
				}
				y, ok := new(big.Rat).SetString(scaled[k].Text)
				if !ok || new(big.Rat).Mul(x, big.NewRat(1000, 1)).Cmp(y) != 0 {
					t.Fatal("scalar", id)
				}
			}
		}
	}
	if points2 == 0 || points3 == 0 || vectors == 0 {
		t.Fatal("roles")
	}
	normalized, e := mappedPlanarSourceBytes(m.NormalizedSource)
	if e != nil {
		t.Fatal(e)
	}
	if n, e := ReadIndependentExactMaterials(ctx, bytes.NewReader(normalized), Limits{}, 1e-9); e != nil || len(n.Members) != 2 {
		t.Fatal(e)
	}
	if v, e := ReadNormalizedIndependentPlanarMaterials(ctx, bytes.NewReader(raw), PlanarMetre, Limits{}, 1e-9); !errors.Is(e, geom.ErrUnrepresentable) || len(v.Members) != 0 {
		t.Fatal("rounded", e)
	}
	solo, e := os.ReadFile("testdata/occt-independent-metre-box.step")
	if e != nil {
		t.Fatal(e)
	}
	up, e := ReadNormalizedIndependentPlanarMaterials(ctx, bytes.NewReader(solo), PlanarMillimetre, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	b, e := mappedPlanarSourceBytes(up.NormalizedSource)
	if e != nil {
		t.Fatal(e)
	}
	down, e := ReadNormalizedIndependentPlanarMaterials(ctx, bytes.NewReader(b), PlanarMetre, Limits{}, 1e-9)
	if e != nil || len(down.Members) != 1 {
		t.Fatal("reverse", e)
	}
	for _, kind := range []string{"pcurve", "curve-type", "unit", "zero-speed", "negative-speed", "exponent"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		changed := false
		for _, id := range f.Order {
			en := f.Entities[id]
			for j, r := range en.Records {
				switch {
				case (kind == "zero-speed" || kind == "negative-speed" || kind == "exponent") && r.Type == "VECTOR":
					text := "0."
					if kind == "negative-speed" {
						text = "-1."
					}
					if kind == "exponent" {
						text = "1.E-999999999"
					}
					en.Records[j].Args[2].Text = text
					changed = true

				case kind == "pcurve" && r.Type == "CARTESIAN_POINT" && len(r.Args[1].Items) == 2:
					en.Records[j].Args[1].Items[1].Text = "1."
					changed = true
				case kind == "curve-type" && r.Type == "LINE":
					en.Records[j].Type = "TRIMMED_CURVE"
					changed = true
				case kind == "unit" && r.Type == "SI_UNIT" && r.Args[1].Text == "METRE":
					en.Records[j].Args[0] = Value{Kind: Enumeration, Text: "CENTI"}
					changed = true
				}
				if changed {
					break
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
		if v, e := ReadNormalizedIndependentPlanarMaterials(ctx, bytes.NewReader(b), PlanarMillimetre, Limits{}, 1e-9); e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}
	round, e := os.ReadFile("testdata/independent-round-units.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadNormalizedIndependentPlanarMaterials(ctx, bytes.NewReader(round), PlanarMillimetre, Limits{}, 1e-9); e == nil {
		t.Fatal("family")
	}
	if _, e = ReadNormalizedIndependentRoundMaterials(ctx, bytes.NewReader(raw), RoundMillimetre, Limits{}, 1e-9); e == nil {
		t.Fatal("round widened")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadNormalizedIndependentPlanarMaterials(cancelled, bytes.NewReader(raw), PlanarMillimetre, Limits{}, 1e-9); e == nil {
		t.Fatal("cancel")
	}
	for _, limit := range []Limits{{MaxBytes: 1}, {MaxSolids: 1}, {MaxEntities: 20}, {MaxFaces: 1}} {
		if v, e := ReadNormalizedIndependentPlanarMaterials(ctx, bytes.NewReader(raw), PlanarMillimetre, limit, 1e-9); e == nil || len(v.Members) != 0 {
			t.Fatal("limit")
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_PLANAR_UNITS"); oracle != "" {
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
				Solids       int
				Volume, Area float64
			}
			if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Solids != 2 || math.Abs(q.Volume-(24+24e9)) > 1e-3 || math.Abs(q.Area-(52+52e6)) > 1e-3 {
				t.Fatal(string(data))
			}
		}
	}
}
func FuzzIndependentPlanarUnits(f *testing.F) {
	b, e := os.ReadFile("testdata/independent-planar-units.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		v, e := ReadNormalizedIndependentPlanarMaterials(context.Background(), bytes.NewReader(b), PlanarMillimetre, Limits{MaxEntities: 1024, MaxValues: 16384, MaxSolids: 4}, 1e-9)
		if e != nil && len(v.Members) != 0 {
			t.Fatal("partial")
		}
	})
}
