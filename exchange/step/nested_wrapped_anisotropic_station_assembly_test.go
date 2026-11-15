package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
	"time"
)

func nestedAnisotropicFixture(t testing.TB, bore bool) []byte {
	t.Helper()
	name := "occt-anisotropic-nested-assembly.step"
	if bore {
		name = "occt-anisotropic-bore-nested-assembly.step"
	}
	b, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestNestedWrappedAnisotropicAssemblies(t *testing.T) {
	ctx := context.Background()
	for _, bore := range []bool{false, true} {
		raw := nestedAnisotropicFixture(t, bore)
		var source WrappedAnisotropicAssemblySource
		var group int64
		var uses []WrappedAnisotropicAssemblyInstance
		var generated []byte
		if bore {
			m, e := ReadNestedWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			source, group, uses = m.WrappedAnisotropicAssemblySource, m.GroupID, m.Instances
			generated = regenerateAnisotropicAssembly(t, source, nil, &m.Body)
			if _, e = ReadWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("flat widened")
			}
			if _, e = ReadNestedWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(generated), Limits{}, WrappedSurfaceOptions{}); e != nil {
				t.Fatal(e)
			}
		} else {
			m, e := ReadNestedWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			source, group, uses = m.WrappedAnisotropicAssemblySource, m.GroupID, m.Instances
			generated = regenerateAnisotropicAssembly(t, source, &m.Body, nil)
			if _, e = ReadWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("flat widened")
			}
			if _, e = ReadNestedWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(generated), Limits{}, WrappedSurfaceOptions{}); e != nil {
				t.Fatal(e)
			}
		}
		if group == 0 || len(source.Assembly.Products) != 3 || len(source.Occurrences) != 3 || len(uses) != 2 || uses[0].Path[1] != uses[1].Path[1] || uses[0].Path[0] == uses[1].Path[0] {
			t.Fatal("source paths")
		}
		want := []geom.Affine{{{1, 0, 0, 1}, {0, 1, 0, 2}, {0, 0, 1, 0}}, {{0, 0, 1, 10}, {1, 0, 0, -1}, {0, 1, 0, 5}}}
		for _, u := range uses {
			if u.ProductID != source.Product.DefinitionID || (u.Transform != want[0] && u.Transform != want[1]) {
				t.Fatal("exact world pose", u)
			}
		}
	}
}
func TestNestedWrappedAnisotropicReject(t *testing.T) {
	ctx := context.Background()
	for _, bore := range []bool{false, true} {
		raw := nestedAnisotropicFixture(t, bore)
		for _, edit := range []func(*File){func(f *File) {
			sideDiskEditRecord(f, "NEXT_ASSEMBLY_USAGE_OCCURRENCE", func(r *Record) { r.Args[4] = r.Args[3] })
		}, func(f *File) { sideDiskEditRecord(f, "PCURVE", func(r *Record) { r.Args[1] = r.Args[2] }) }} {
			bad := sideDiskMutateSource(t, raw, edit)
			if bore {
				m, e := ReadNestedWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(bad), Limits{}, WrappedSurfaceOptions{})
				if e == nil || m.Assembly.Source != nil {
					t.Fatal("bad nested")
				}
			} else {
				m, e := ReadNestedWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(bad), Limits{}, WrappedSurfaceOptions{})
				if e == nil || m.Assembly.Source != nil {
					t.Fatal("bad nested")
				}
			}
		}
		if _, e := ReadNestedWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{MaxEntities: 10}, WrappedSurfaceOptions{}); e == nil {
			t.Fatal("limit")
		}
		c, cancel := context.WithCancel(ctx)
		cancel()
		if _, e := ReadNestedWrappedAnisotropicStationAssembly(c, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	}
	// Both authored offsets are exactly represented; only their sum is not.
	m, e := ReadNestedWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(nestedAnisotropicFixture(t, false)), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	model := WrappedAnisotropicStationAssemblyModel{LengthUnit: m.Assembly.LengthUnit, Occurrences: anisotropicAssemblyUses(m.Assembly)}
	for _, p := range m.Assembly.Products {
		q := WrappedAnisotropicStationAssemblyPart{ID: fmt.Sprint(p.DefinitionID), Name: p.Name}
		if p.DefinitionID == m.Product.DefinitionID {
			q.Body = &m.Body
		}
		model.Products = append(model.Products, q)
	}
	for i, u := range model.Occurrences {
		if u.Parent == fmt.Sprint(m.GroupID) {
			model.Occurrences[i].Transform = geom.Affine{{1, 0, 0, .125}, {0, 1, 0, 0}, {0, 0, 1, 0}}
		} else {
			model.Occurrences[i].Transform = geom.Affine{{1, 0, 0, 9007199254740992}, {0, 1, 0, 0}, {0, 0, 1, 0}}
		}
	}
	var authored bytes.Buffer
	if e = writeWrappedAnisotropicStationAssembly(ctx, &authored, model, Limits{}, WrappedSurfaceOptions{}, true); e != nil {
		t.Fatal("represented individual frames", e)
	}
	if _, e = ReadNestedWrappedAnisotropicStationAssembly(ctx, &authored, Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("rounded composed frame")
	}
	var out bytes.Buffer
	if e = WriteNestedWrappedAnisotropicStationAssembly(ctx, &out, model, Limits{}, WrappedSurfaceOptions{}); e == nil || out.Len() != 0 {
		t.Fatal("partial output")
	}
}
func TestNestedWrappedAnisotropicLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_ANISOTROPIC_STATION")
	if bin == "" {
		t.Skip("optional independent XCAF")
	}
	for _, bore := range []bool{false, true} {
		t.Run(fmt.Sprint(bore), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			dir := t.TempDir()
			path, report := filepath.Join(dir, "original.step"), filepath.Join(dir, "report.json")
			mode := "filled-nested-assembly"
			if bore {
				mode = "bore-nested-assembly"
			}
			if b, e := exec.CommandContext(ctx, bin, path, report, mode).CombinedOutput(); e != nil {
				t.Fatal(e, string(b))
			}
			raw, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			type mass struct {
				Volume, Area   float64
				Centroid       [3]float64
				Solids, Shells int
				Shared         bool
			}
			data, e := os.ReadFile(report)
			if e != nil {
				t.Fatal(e)
			}
			var original mass
			if e = json.Unmarshal(data, &original); e != nil {
				t.Fatal(e)
			}
			var preserved bytes.Buffer
			var generated []byte
			if bore {
				m, e := ReadNestedWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
				if e != nil {
					t.Fatal(e)
				}
				var envelope bytes.Buffer
				if e = WriteNestedWrappedAnisotropicStationBoreAssemblyJSON(ctx, &envelope, m); e != nil {
					t.Fatal(e)
				}
				m, e = ReadNestedWrappedAnisotropicStationBoreAssemblyJSON(ctx, &envelope, IndependentMaterialJSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				var native bytes.Buffer // full assembly reloaded
				if e = trimmed.WriteJSON(&native, m.Body.Solid); e != nil {
					t.Fatal(e)
				}
				s, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				body, e := WrappedAnisotropicStationBoreFromSolid(ctx, s, m.Body.Name, m.Body.LengthUnit, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				generated = regenerateAnisotropicAssembly(t, m.WrappedAnisotropicAssemblySource, nil, &body)
				if e = WriteNestedWrappedAnisotropicStationBoreAssemblySource(ctx, &preserved, m.Assembly.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
					t.Fatal(e)
				}
			} else {
				m, e := ReadNestedWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
				if e != nil {
					t.Fatal(e)
				}
				var envelope bytes.Buffer
				if e = WriteNestedWrappedAnisotropicStationAssemblyJSON(ctx, &envelope, m); e != nil {
					t.Fatal(e)
				}
				m, e = ReadNestedWrappedAnisotropicStationAssemblyJSON(ctx, &envelope, IndependentMaterialJSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				var native bytes.Buffer
				if e = trimmed.WriteJSON(&native, m.Body.Solid); e != nil {
					t.Fatal(e)
				}
				s, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				body, e := WrappedAnisotropicStationLoftFromSolid(ctx, s, m.Body.Name, m.Body.LengthUnit, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				generated = regenerateAnisotropicAssembly(t, m.WrappedAnisotropicAssemblySource, &body, nil)
				if e = WriteNestedWrappedAnisotropicStationAssemblySource(ctx, &preserved, m.Assembly.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
					t.Fatal(e)
				}
			}
			for i, b := range [][]byte{preserved.Bytes(), generated} {
				p := filepath.Join(dir, fmt.Sprint(i)+".step")
				if e = os.WriteFile(p, b, 0600); e != nil {
					t.Fatal(e)
				}
				if reply, e := exec.CommandContext(ctx, bin, p, report, "read-nested-assembly").CombinedOutput(); e != nil {
					t.Fatal(e, string(reply))
				}
				data, e = os.ReadFile(report)
				if e != nil {
					t.Fatal(e)
				}
				var q mass
				if e = json.Unmarshal(data, &q); e != nil {
					t.Fatal(e)
				}
				if !q.Shared || q.Solids != 2 || q.Shells != 2 || math.Abs(q.Volume-original.Volume) > 1e-8 || math.Abs(q.Area-original.Area) > 1e-8 {
					t.Fatal(q, original)
				}
				for j := range q.Centroid {
					if math.Abs(q.Centroid[j]-original.Centroid[j]) > 1e-8 {
						t.Fatal(q, original)
					}
				}
			}
		})
	}
}

func FuzzNestedWrappedAnisotropicStationAssembly(f *testing.F) {
	f.Add(nestedAnisotropicFixture(f, false), false)
	f.Add(nestedAnisotropicFixture(f, true), true)
	f.Fuzz(func(t *testing.T, b []byte, bore bool) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		l := Limits{MaxBytes: 1 << 20, MaxEntities: 4096, MaxValues: 65536, MaxDepth: 16}
		o := WrappedSurfaceOptions{MaxSurfaces: 32, MaxOperations: 4000000}
		if bore {
			m, e := ReadNestedWrappedAnisotropicStationBoreAssembly(context.Background(), bytes.NewReader(b), l, o)
			if e != nil && m.Assembly.Source != nil {
				t.Fatal("partial")
			}
		} else {
			m, e := ReadNestedWrappedAnisotropicStationAssembly(context.Background(), bytes.NewReader(b), l, o)
			if e != nil && m.Assembly.Source != nil {
				t.Fatal("partial")
			}
		}
	})
}
