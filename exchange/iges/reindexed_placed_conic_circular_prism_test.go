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
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestReindexedPlacedConicCircularPrism(t *testing.T) {
	ctx := context.Background()
	for _, hollow := range []bool{false, true} {
		name := "placed-conic-circular-prism-topology"
		count, faces := 21, 3
		volume, area := 100*math.Pi, 90*math.Pi
		if hollow {
			name = "placed-conic-circular-annular-prism-topology"
			count, faces = 29, 4
			volume, area = 84*math.Pi, 98*math.Pi
		}
		raw, e := os.ReadFile("testdata/loop-start-" + name + ".igs")
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadReindexedPlacedConicCircularPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		for role, id := range m.SourceForCanonical {
			if id != 2*count-role {
				t.Fatal("source role", role, id)
			}
		}
		if m.SourceForCanonical[m.Material.SourceTransformID] != 1 {
			t.Fatal("transform provenance")
		}
		if (m.Material.Annular != nil) != hollow {
			t.Fatal("wrong family")
		}
		mass, e := m.Material.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil || math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 || mass.Centroid.Distance(geom.Vec3{X: 12.5, Y: -4.25, Z: 2.125}) > 1e-9 {
			t.Fatal(mass, e)
		}
		var native, source bytes.Buffer
		if e = WriteReindexedPlacedConicCircularPrismMaterialJSON(ctx, &native, m); e != nil {
			t.Fatal(e)
		}
		p, e := ReadReindexedPlacedConicCircularPrismMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if e = WriteReindexedPlacedConicCircularPrismSource(ctx, &source, p); e != nil || !bytes.Equal(source.Bytes(), raw) {
			t.Fatal("source changed", e)
		}
		if _, e = ReadPlacedConicCircularPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
			t.Fatal("canonical reader widened")
		}
		for _, mutation := range []struct {
			prefix string
			field  int
			value  string
		}{{"124,", 1, "1"}, {"122,", 2, "9"}, {"504,", 4, "9"}, {"104,", 3, "9"}} {
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
			if _, e = ReadReindexedPlacedConicCircularPrismMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
				t.Fatal("bad source placement or geometry")
			}
		}
		forged := p
		forged.SourceForCanonical = map[int]int{1: 1}
		var rejected bytes.Buffer
		if e = WriteReindexedPlacedConicCircularPrismSource(ctx, &rejected, forged); e == nil || rejected.Len() != 0 {
			t.Fatal("stale identity")
		}
		for _, l := range []Limits{{MaxEntities: count - 1}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 8}} {
			if _, e = ReadReindexedPlacedConicCircularPrismMaterial(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
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
			if at < 0 || json.Unmarshal(b[at:], &q) != nil || q.Faces != faces || q.Solids != 1 || math.Abs(q.Area-area) > 1e-7 || math.Abs(q.Volume-volume) > 1e-7 || (geom.Vec3{X: q.Center[0], Y: q.Center[1], Z: q.Center[2]}).Distance(mass.Centroid) > 1e-9 {
				t.Fatalf("%s", b)
			}
		}
	}
}
func FuzzReindexedPlacedConicCircularPrism(f *testing.F) {
	for _, name := range []string{"placed-conic-circular-prism-topology", "placed-conic-circular-annular-prism-topology"} {
		b, e := os.ReadFile("testdata/loop-start-" + name + ".igs")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadReindexedPlacedConicCircularPrismMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
