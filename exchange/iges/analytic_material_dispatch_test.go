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
	"reflect"
	"strings"
	"testing"
)

var analyticDispatchFixtures = []struct {
	name string
	kind AnalyticMaterialFamily
}{
	{"spherical-material", AnalyticSphere}, {"toroidal-material", AnalyticTorus}, {"conical45-frustum", AnalyticCone}, {"spherical-cavity-material", AnalyticSphereCavity}, {"conical45-cavity-material", AnalyticConeCavity}, {"cylinder-sphere-cavity", AnalyticCylinderSphereCavity}, {"conical45-cylinder-through", AnalyticConeCylinderThrough},
}

func TestAnalyticMaterialDispatch(t *testing.T) {
	ctx := context.Background()
	for _, fixture := range analyticDispatchFixtures {
		for _, prefix := range []string{"", "placed-", "loop-start-", "loop-start-placed-"} {
			t.Run(prefix+fixture.name, func(t *testing.T) {
				raw, e := os.ReadFile("testdata/" + prefix + fixture.name + ".igs")
				if e != nil {
					t.Fatal(e)
				}
				m, e := ReadAnalyticMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
				if e != nil {
					t.Fatal(e)
				}
				kind, e := m.Family()
				if e != nil || kind != fixture.kind {
					t.Fatal(kind, e)
				}
				unit, e := m.LengthUnit()
				if e != nil || unit <= 0 {
					t.Fatal("unit", e)
				}
				solid, e := m.Solid()
				if e != nil {
					t.Fatal(e)
				}
				mass, e := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
				if e != nil || mass.Volume <= 0 {
					t.Fatal(e)
				}
				roles := m.SourceRoles()
				faces := m.NativeFaceSourceIDs()
				if len(roles) == 0 || len(faces) != len(solid.Faces()) {
					t.Fatal("ancestry")
				}
				old := m.SourceRoles()
				for k := range roles {
					roles[k] = 0
				}
				faces[0] = 0
				if !reflect.DeepEqual(old, m.SourceRoles()) || m.NativeFaceSourceIDs()[0] == 0 {
					t.Fatal("mutable ancestry")
				}
				var out bytes.Buffer
				if e = WriteAnalyticMaterialSource(ctx, &out, m); e != nil || !bytes.Equal(raw, out.Bytes()) {
					t.Fatal("source", e)
				}
				if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID"); oracle != "" && prefix == "loop-start-placed-" {
					path := filepath.Join(t.TempDir(), "source.igs")
					if e := os.WriteFile(path, out.Bytes(), 0600); e != nil {
						t.Fatal(e)
					}
					b, e := exec.Command(oracle, path).CombinedOutput()
					if e != nil {
						t.Fatal(string(b), e)
					}
					at := bytes.LastIndex(b, []byte("{"))
					var q struct {
						Solids       int
						Area, Volume float64
					}
					if at < 0 || json.Unmarshal(b[at:], &q) != nil || q.Solids != 1 || math.Abs(q.Volume-mass.Volume) > 1e-6 || math.Abs(q.Area-mass.Area) > 1e-6 {
						t.Fatal(string(b))
					}
				}

				for _, l := range []Limits{{MaxBytes: 80}, {MaxEntities: 1}, {MaxRecords: 2}, {MaxValues: 2}} {
					v, e := ReadAnalyticMaterial(ctx, bytes.NewReader(raw), l, 1e-9)
					if e == nil || !reflect.DeepEqual(v, AnalyticMaterial{}) {
						t.Fatal("partial/limit")
					}
				}
				// Keep the signature, but invalidate a real root support relationship.
				bad := append([]byte(nil), raw...)
				changed := false
				for _, line := range bytes.Split(bad, []byte("\n")) {
					if len(line) != 80 || line[72] != 'P' || !bytes.HasPrefix(line, []byte("186,")) {
						continue
					}
					v := strings.Split(strings.TrimSpace(string(line[:64])), ",")
					v[1] = "999"
					text := strings.Join(v, ",")
					copy(line[:64], text+strings.Repeat(" ", 64-len(text)))
					changed = true
					break
				}
				if !changed {
					t.Fatal("root")
				}
				if v, e := ReadAnalyticMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil || !reflect.DeepEqual(v, AnalyticMaterial{}) {
					t.Fatal("invalid source admitted")
				}
			})
		}
	}
	var out bytes.Buffer
	if e := WriteAnalyticMaterialSource(ctx, &out, AnalyticMaterial{}); e == nil || out.Len() != 0 {
		t.Fatal("zero")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadAnalyticMaterial(ctx, bytes.NewReader(nil), Limits{}, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}
func TestAnalyticMaterialDispatchAmbiguity(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/conical45-cylinder-through.igs")
	if e != nil {
		t.Fatal(e)
	}
	// Duplicate root declaration and mixed analytic support signature never route.
	for _, replacement := range []int{186, 196, 198} {
		bad := append([]byte(nil), raw...)
		for _, line := range bytes.Split(bad, []byte("\n")) {
			if len(line) == 80 && line[72] == 'D' {
				kind, _ := field(string(line[:8]))
				if kind == 192 {
					copy(line[:8], []byte(fmt.Sprintf("%8d", replacement)))
				}
			}
		}
		if _, e := ReadAnalyticMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("mixed signature")
		}
	}
	a, e := ReadAnalyticMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	// Even package-internal corruption is rejected at the export boundary.
	a.sphere = &ReindexedSphericalMaterial{}
	var out bytes.Buffer
	if e := WriteAnalyticMaterialSource(ctx, &out, a); e == nil || out.Len() != 0 {
		t.Fatal("ambiguous union")
	}
}
func FuzzAnalyticMaterialDispatch(f *testing.F) {
	for _, fixture := range analyticDispatchFixtures {
		b, e := os.ReadFile("testdata/loop-start-placed-" + fixture.name + ".igs")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadAnalyticMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
