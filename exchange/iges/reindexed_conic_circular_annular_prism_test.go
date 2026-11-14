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

func TestReindexedConicCircularAnnularPrism(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/loop-start-conic-circular-annular-prism-topology.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadReindexedConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old canonical reader accepted")
	}
	for canonical, source := range m.SourceForCanonical {
		if source != 56-canonical {
			t.Fatal("source roles")
		}
	}
	if m.SourceVertexOrdinals[1] != 4 || m.SourceVertexOrdinals[2] != 3 || m.SourceUseOrdinals[23][0] != 4 {
		t.Fatal("source ordinals", m.SourceUseOrdinals)
	}
	mass, e := m.Material.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(mass.Volume-84*math.Pi) > 1e-7 || math.Abs(mass.Area-98*math.Pi) > 1e-7 {
		t.Fatal(mass, e)
	}
	var native, source bytes.Buffer
	if e = WriteReindexedConicCircularAnnularPrismMaterialJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	p, e := ReadReindexedConicCircularAnnularPrismMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteReindexedConicCircularAnnularPrismSource(ctx, &source, p); e != nil || !bytes.Equal(source.Bytes(), raw) {
		t.Fatal("lost source", e)
	}
	var again bytes.Buffer
	if e = WriteReindexedConicCircularAnnularPrismMaterialJSON(ctx, &again, p); e != nil || !bytes.Equal(native.Bytes(), again.Bytes()) {
		t.Fatal("unstable recipe", e)
	}
	forged := p
	forged.SourceVertexOrdinals = map[int]int{1: 1, 2: 2}
	var rejected bytes.Buffer
	if e = WriteReindexedConicCircularAnnularPrismSource(ctx, &rejected, forged); e == nil || rejected.Len() != 0 {
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
		if _, e = ReadReindexedConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("invalid original geometry or graph", mutation)
		}
	}
	wrongHole := append([]byte(nil), raw...)
	for _, line := range bytes.Split(wrongHole, []byte("\n")) {
		if len(line) != 80 || line[72] != 'P' {
			continue
		}
		owner, _ := field(string(line[64:72]))
		if owner != m.SourceForCanonical[39] {
			continue
		}
		v := strings.Split(strings.TrimSuffix(strings.TrimSpace(string(line[:64])), ";"), ",")
		v[5] = "1"
		text := strings.Join(v, ",") + ";"
		copy(line[:64], text+strings.Repeat(" ", 64-len(text)))
	}
	if _, e = ReadReindexedConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(wrongHole), Limits{}, 1e-9); e == nil {
		t.Fatal("reversed inner cap ownership")
	}
	for _, l := range []Limits{{MaxEntities: 27}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 8}} {
		if _, e = ReadReindexedConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
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
		if at < 0 || json.Unmarshal(b[at:], &q) != nil || q.Faces != 4 || q.Solids != 1 || math.Abs(q.Area-98*math.Pi) > 1e-7 || math.Abs(q.Volume-84*math.Pi) > 1e-7 || math.Abs(q.Center[2]-5) > 1e-9 {
			t.Fatalf("%s", b)
		}
	}
}
func FuzzReindexedConicCircularAnnularPrism(f *testing.F) {
	b, e := os.ReadFile("testdata/loop-start-conic-circular-annular-prism-topology.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadReindexedConicCircularAnnularPrismMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
