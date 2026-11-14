package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"strings"
	"testing"
)

func TestReindexedConicCircularPrism(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/loop-start-conic-circular-prism-topology.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadReindexedConicCircularPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadConicCircularPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old canonical reader accepted")
	}
	for canonical, source := range m.SourceForCanonical {
		if source != 40-canonical {
			t.Fatal("source roles")
		}
	}
	if m.SourceVertexOrdinals[1] != 2 || m.SourceVertexOrdinals[2] != 1 || m.SourceUseOrdinals[15][0] != 4 {
		t.Fatal("source ordinals", m.SourceUseOrdinals)
	}
	mass, e := m.Material.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(mass.Volume-100*math.Pi) > 1e-7 || math.Abs(mass.Area-90*math.Pi) > 1e-7 {
		t.Fatal(mass, e)
	}
	var native, source bytes.Buffer
	if e = WriteReindexedConicCircularPrismMaterialJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	p, e := ReadReindexedConicCircularPrismMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteReindexedConicCircularPrismSource(ctx, &source, p); e != nil || !bytes.Equal(source.Bytes(), raw) {
		t.Fatal("lost source", e)
	}
	var again bytes.Buffer
	if e = WriteReindexedConicCircularPrismMaterialJSON(ctx, &again, p); e != nil || !bytes.Equal(native.Bytes(), again.Bytes()) {
		t.Fatal("unstable recipe", e)
	}
	forged := p
	forged.SourceVertexOrdinals = map[int]int{1: 1, 2: 2}
	var rejected bytes.Buffer
	if e = WriteReindexedConicCircularPrismSource(ctx, &rejected, forged); e == nil || rejected.Len() != 0 {
		t.Fatal("stale identity")
	}
	for _, mutation := range []struct {
		prefix string
		field  int
		value  string
	}{{"104,", 3, "5"}, {"122,", 3, "1"}, {"504,", 4, "9"}, {"186,", 1, "1"}} {
		bad := append([]byte(nil), raw...)
		changed := false
		for _, line := range bytes.Split(bad, []byte("\n")) {
			if len(line) != 80 || line[72] != 'P' || !bytes.HasPrefix(line, []byte(mutation.prefix)) {
				continue
			}
			v := strings.Split(strings.TrimSpace(string(line[:64])), ",")
			v[mutation.field] = mutation.value
			text := strings.Join(v, ",")
			if len(text) > 64 {
				t.Fatal("mutation overflow")
			}
			copy(line[:64], text+strings.Repeat(" ", 64-len(text)))
			changed = true
			break
		}
		if !changed {
			t.Fatal("missing mutation")
		}
		if _, e = ReadReindexedConicCircularPrismMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("invalid original geometry or graph", mutation)
		}
	}
	for _, l := range []Limits{{MaxEntities: 19}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 8}} {
		if _, e = ReadReindexedConicCircularPrismMaterial(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
			t.Fatal("limit")
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_ELLIPTICAL_PRISM_TOPOLOGY"); oracle != "" {
		path := filepath.Join(t.TempDir(), "source.igs")
		if e = os.WriteFile(path, source.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		b, e := exec.Command(oracle, "read", path).CombinedOutput()
		if e != nil {
			t.Fatalf("%s %v", b, e)
		}
		var q struct {
			Faces, Solids int
			Area, Volume  float64
			Center        [3]float64
		}
		at := bytes.LastIndex(b, []byte("{"))
		if at < 0 || json.Unmarshal(b[at:], &q) != nil || q.Faces != 3 || q.Solids != 1 || math.Abs(q.Area-90*math.Pi) > 1e-7 || math.Abs(q.Volume-100*math.Pi) > 1e-7 || math.Abs(q.Center[2]-5) > 1e-9 {
			t.Fatalf("%s", b)
		}
	}
}
func FuzzReindexedConicCircularPrism(f *testing.F) {
	b, e := os.ReadFile("testdata/loop-start-conic-circular-prism-topology.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadReindexedConicCircularPrismMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
