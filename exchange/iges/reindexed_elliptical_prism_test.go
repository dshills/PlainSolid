package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
	"testing"
)

func reindexedFixtureNames() []string {
	return []string{"elliptical-prism-topology", "elliptical-annular-prism-topology", "placed-elliptical-prism-topology", "placed-elliptical-annular-prism-topology"}
}
func TestReindexedEllipticalPrism(t *testing.T)  { testReindexedEllipsePrefix(t, "reindexed-") }
func TestEllipticalLoopStart(t *testing.T)       { testReindexedEllipsePrefix(t, "loop-start-") }
func TestEllipticalVertexListOrder(t *testing.T) { testReindexedEllipsePrefix(t, "vertex-order-") }
func TestEllipticalEdgeListOrder(t *testing.T)   { testReindexedEllipsePrefix(t, "edge-order-") }
func TestEllipticalShellFaceOrder(t *testing.T)  { testReindexedEllipsePrefix(t, "face-order-") }
func testReindexedEllipsePrefix(t *testing.T, prefix string) {
	ctx := context.Background()
	for _, name := range reindexedFixtureNames() {
		t.Run(name, func(t *testing.T) {
			raw, e := os.ReadFile("testdata/" + prefix + name + ".igs")
			if e != nil {
				t.Fatal(e)
			}
			m, e := ReadReindexedEllipticalPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			placed := strings.HasPrefix(name, "placed-")
			hollow := strings.Contains(name, "annular")
			count := 20
			if hollow {
				count = 28
			}
			if placed {
				count++
			}
			if len(m.SourceForCanonical) != count {
				t.Fatal(m.SourceForCanonical)
			}
			for canonical, source := range m.SourceForCanonical {
				if source != 2*count-canonical {
					t.Fatal("lost source identity", canonical, source)
				}
			}
			if _, e = ReadPlacedEllipticalPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
				t.Fatal("old fixed order accepted")
			}
			vertexCount := 2
			if hollow {
				vertexCount = 4
			}
			for role, original := range m.SourceVertexOrdinals {
				want := role
				if prefix == "vertex-order-" || prefix == "loop-start-" {
					want = vertexCount + 1 - role
				}
				if original != want {
					t.Fatal("wrong source vertex role", role, original)
				}
			}
			if len(m.SourceVertexOrdinals) != vertexCount {
				t.Fatal("missing original vertex roles")
			}
			for _, ordinals := range m.SourceUseOrdinals {
				for j, original := range ordinals {
					want := j + 1
					if prefix == "loop-start-" && len(ordinals) == 4 {
						want = (j+3)%4 + 1
					}
					if original != want {
						t.Fatal("lost loop use source ordinal", ordinals)
					}
				}
			}
			volume := 40 * math.Pi
			area := 154.88430345445522
			faces := 3
			if hollow {
				volume = 32 * math.Pi
				area = 181.07172572228598
				faces = 4
			}
			center := geom.Vec3{Z: 5}
			if placed {
				center = geom.Vec3{X: 12.5, Y: -4.25, Z: 2.125}
			}
			mass, e := m.Solid().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if e != nil || math.Abs(mass.Volume-volume) > 1e-6 || mass.Centroid.Distance(center) > 1e-7 {
				t.Fatal(mass, e)
			}
			var native bytes.Buffer
			if e = WriteReindexedEllipticalPrismMaterialJSON(ctx, &native, m); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadReindexedEllipticalPrismMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			var source bytes.Buffer
			if e = WriteReindexedEllipticalPrismSource(ctx, &source, loaded); e != nil || !bytes.Equal(raw, source.Bytes()) {
				t.Fatal("source changed", e)
			}
			var again bytes.Buffer
			if e = WriteReindexedEllipticalPrismMaterialJSON(ctx, &again, loaded); e != nil || !bytes.Equal(native.Bytes(), again.Bytes()) {
				t.Fatal("unstable native", e)
			}
			for _, target := range []int{m.SourceSolidID, 99999} {
				bad := append([]byte(nil), raw...)
				for _, line := range bytes.Split(bad, []byte("\n")) {
					if len(line) == 80 && line[72] == 'P' && strings.HasPrefix(string(line), "186,") {
						text := fmt.Sprintf("186,%d,1,0;", target)
						copy(line[:64], []byte(text+strings.Repeat(" ", 64-len(text))))
					}
				}
				if _, e = ReadReindexedEllipticalPrismMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
					t.Fatal("recursive/foreign owner accepted")
				}
			}
			if hollow {
				bad := append([]byte(nil), raw...)
				owner := m.SourceForCanonical[11]
				for _, line := range bytes.Split(bad, []byte("\n")) {
					if len(line) != 80 || line[72] != 'P' {
						continue
					}
					id, _ := field(string(line[64:72]))
					if id == owner {
						v := strings.Split(strings.TrimSuffix(strings.TrimSpace(string(line[:64])), ";"), ",")
						v[1] = fmt.Sprint(m.SourceForCanonical[23])
						text := strings.Join(v, ",") + ";"
						copy(line[:64], []byte(text+strings.Repeat(" ", 64-len(text))))
					}
				}
				if _, e = ReadReindexedEllipticalPrismMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
					t.Fatal("aliased wall owner")
				}
			}
			// A face permutation must preserve its own shell orientation.
			badSense := append([]byte(nil), raw...)
			for _, line := range bytes.Split(badSense, []byte("\n")) {
				if len(line) == 80 && line[72] == 'P' && bytes.HasPrefix(line, []byte("514,")) {
					v := strings.Split(strings.TrimSuffix(strings.TrimSpace(string(line[:64])), ";"), ",")
					if v[3] == "0" {
						v[3] = "1"
					} else {
						v[3] = "0"
					}
					text := strings.Join(v, ",") + ";"
					copy(line[:64], text+strings.Repeat(" ", 64-len(text)))
				}
			}
			if _, e = ReadReindexedEllipticalPrismMaterial(ctx, bytes.NewReader(badSense), Limits{}, 1e-9); e == nil {
				t.Fatal("wrong shell face orientation")
			}
			badEdge := append([]byte(nil), raw...)
			for _, line := range bytes.Split(badEdge, []byte("\n")) {
				if len(line) == 80 && line[72] == 'P' && bytes.HasPrefix(line, []byte("508,")) {
					v := strings.Split(strings.TrimSpace(string(line[:64])), ",")
					v[4] = "999"
					text := strings.Join(v, ",")
					if len(text) <= 64 {
						copy(line[:64], text+strings.Repeat(" ", 64-len(text)))
					}
				}
			}
			if _, e = ReadReindexedEllipticalPrismMaterial(ctx, bytes.NewReader(badEdge), Limits{}, 1e-9); e == nil {
				t.Fatal("foreign edge ordinal")
			}
			for _, mutation := range []struct {
				kind, field int
				value       string
			}{{122, 2, "9"}, {502, 2, "9"}, {504, 4, "9"}} {
				bad := append([]byte(nil), raw...)
				changed := false
				for _, line := range bytes.Split(bad, []byte("\n")) {
					if len(line) != 80 || line[72] != 'P' || !bytes.HasPrefix(line, []byte(fmt.Sprint(mutation.kind)+",")) {
						continue
					}
					v := strings.Split(strings.TrimSpace(string(line[:64])), ",")
					if len(v) <= mutation.field {
						t.Fatal("mutation missing field")
					}
					v[mutation.field] = mutation.value
					text := strings.Join(v, ",")
					if len(text) > 64 {
						t.Fatal("mutation record overflow")
					}
					copy(line[:64], text+strings.Repeat(" ", 64-len(text)))
					changed = true
					break
				}
				if !changed {
					t.Fatal("missing mutation entity")
				}
				if _, e = ReadReindexedEllipticalPrismMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
					t.Fatal("wrong source terminal/vertex/incidence", mutation)
				}
			}
			forged := m
			forged.SourceVertexOrdinals = map[int]int{1: 999}
			var rejected bytes.Buffer
			if e = WriteReindexedEllipticalPrismSource(ctx, &rejected, forged); e == nil || rejected.Len() != 0 {
				t.Fatal("stale vertex source roles")
			}
			forgedUses := m
			forgedUses.SourceUseOrdinals = map[int][]int{999: {1, 2, 3, 4}}
			if e = WriteReindexedEllipticalPrismSource(ctx, &rejected, forgedUses); e == nil || rejected.Len() != 0 {
				t.Fatal("stale source use ordinals")
			}
			var env reindexedEllipseEnvelope
			if json.Unmarshal(native.Bytes(), &env) != nil {
				t.Fatal("json")
			}
			env.Identity.Roles[1] = env.Identity.Solid
			b, _ := json.Marshal(env)
			if _, e = ReadReindexedEllipticalPrismMaterialJSON(ctx, bytes.NewReader(b), Limits{}); e == nil {
				t.Fatal("forged source map")
			}
			for _, l := range []Limits{{MaxEntities: count - 1}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 4}} {
				if _, e = ReadReindexedEllipticalPrismMaterial(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
					t.Fatal("limit")
				}
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_ELLIPTICAL_PRISM_TOPOLOGY"); oracle != "" {
				path := filepath.Join(t.TempDir(), "source.igs")
				if e = os.WriteFile(path, source.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				out, e := exec.Command(oracle, "read", path).CombinedOutput()
				if e != nil {
					t.Fatalf("%s %v", out, e)
				}
				var v struct {
					Faces, Solids int
					Volume, Area  float64
					Center        [3]float64
				}
				at := bytes.LastIndex(out, []byte("{"))
				if at < 0 || json.Unmarshal(bytes.TrimSpace(out[at:]), &v) != nil || v.Faces != faces || v.Solids != 1 || math.Abs(v.Volume-volume) > 1e-8 || math.Abs(v.Area-area) > 1e-7 || (geom.Vec3{X: v.Center[0], Y: v.Center[1], Z: v.Center[2]}).Distance(center) > 1e-10 {
					t.Fatalf("%s", out)
				}
			}
		})
	}
}
func FuzzReindexedEllipticalPrism(f *testing.F) {
	for _, name := range reindexedFixtureNames() {
		b, e := os.ReadFile("testdata/reindexed-" + name + ".igs")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b)
		c, e := os.ReadFile("testdata/face-order-" + name + ".igs")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(c)
		d, e := os.ReadFile("testdata/edge-order-" + name + ".igs")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(d)
		eb, e := os.ReadFile("testdata/vertex-order-" + name + ".igs")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(eb)
		lb, e := os.ReadFile("testdata/loop-start-" + name + ".igs")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(lb)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadReindexedEllipticalPrismMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
