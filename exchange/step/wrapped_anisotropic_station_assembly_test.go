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

func anisotropicAssemblyFixture(t testing.TB, bore bool) []byte {
	t.Helper()
	name := "occt-anisotropic-assembly.step"
	if bore {
		name = "occt-anisotropic-bore-assembly.step"
	}
	b, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func anisotropicAssemblyUses(a Assembly) []AssemblyInstance {
	var out []AssemblyInstance
	for _, u := range a.Occurrences {
		out = append(out, AssemblyInstance{ID: fmt.Sprint(u.ID), Name: u.Name, Parent: fmt.Sprint(u.Parent), Child: fmt.Sprint(u.Child), Transform: u.Transform})
	}
	return out
}
func regenerateAnisotropicAssembly(t testing.TB, source WrappedAnisotropicAssemblySource, filled *WrappedAnisotropicStationLoftBody, bore *WrappedAnisotropicStationBoreBody) []byte {
	t.Helper()
	ctx := context.Background()
	var out bytes.Buffer
	if filled != nil {
		m := WrappedAnisotropicStationAssemblyModel{LengthUnit: source.Assembly.LengthUnit, Occurrences: anisotropicAssemblyUses(source.Assembly)}
		for _, p := range source.Assembly.Products {
			q := WrappedAnisotropicStationAssemblyPart{ID: fmt.Sprint(p.DefinitionID), Name: p.Name}
			if p.DefinitionID == source.Product.DefinitionID {
				q.Body = filled
			}
			m.Products = append(m.Products, q)
		}
		var e error
		if len(source.Assembly.Products) == 3 {
			e = WriteNestedWrappedAnisotropicStationAssembly(ctx, &out, m, Limits{}, WrappedSurfaceOptions{})
		} else {
			e = WriteWrappedAnisotropicStationAssembly(ctx, &out, m, Limits{}, WrappedSurfaceOptions{})
		}
		if e != nil {
			t.Fatal(e)
		}
	} else {
		m := WrappedAnisotropicStationBoreAssemblyModel{LengthUnit: source.Assembly.LengthUnit, Occurrences: anisotropicAssemblyUses(source.Assembly)}
		for _, p := range source.Assembly.Products {
			q := WrappedAnisotropicStationBoreAssemblyPart{ID: fmt.Sprint(p.DefinitionID), Name: p.Name}
			if p.DefinitionID == source.Product.DefinitionID {
				q.Body = bore
			}
			m.Products = append(m.Products, q)
		}
		var e error
		if len(source.Assembly.Products) == 3 {
			e = WriteNestedWrappedAnisotropicStationBoreAssembly(ctx, &out, m, Limits{}, WrappedSurfaceOptions{})
		} else {
			e = WriteWrappedAnisotropicStationBoreAssembly(ctx, &out, m, Limits{}, WrappedSurfaceOptions{})
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	return out.Bytes()
}
func TestWrappedAnisotropicStationAssemblies(t *testing.T) {
	for _, bore := range []bool{false, true} {
		t.Run(fmt.Sprint(bore), func(t *testing.T) {
			ctx := context.Background()
			raw := anisotropicAssemblyFixture(t, bore)
			var source WrappedAnisotropicAssemblySource
			var solid trimmed.Solid
			var generated []byte
			if !bore {
				m, e := ReadWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
				if e != nil {
					t.Fatal(e)
				}
				source, solid = m.WrappedAnisotropicAssemblySource, m.Body.Solid
				generated = regenerateAnisotropicAssembly(t, source, &m.Body, nil)
				if _, e = ReadWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
					t.Fatal("filled as bore")
				}
			} else {
				m, e := ReadWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
				if e != nil {
					t.Fatal(e)
				}
				source, solid = m.WrappedAnisotropicAssemblySource, m.Body.Solid
				generated = regenerateAnisotropicAssembly(t, source, nil, &m.Body)
				if _, e = ReadWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
					t.Fatal("bore as filled")
				}
			}
			if len(source.Assembly.Products) != 2 || len(source.Occurrences) != 2 || source.Occurrences[0].ChildID != source.Occurrences[1].ChildID || source.SourceBodyID == 0 || len(source.SourceFaceIDs) < 4 {
				t.Fatal("source ownership")
			}
			for _, id := range source.SourceFaceIDs {
				if _, e := simple(source.Assembly.Source, id, "ADVANCED_FACE", 4); e != nil {
					t.Fatal(e)
				}
			}
			if _, e := ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("homothetic widened")
			}
			var native bytes.Buffer
			if e := trimmed.WriteJSON(&native, solid); e != nil {
				t.Fatal(e)
			}
			s, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			if bore {
				if _, e = s.ValidateWrappedAnisotropicStationBore(ctx, geom.SurfacePatchOptions{}); e != nil {
					t.Fatal(e)
				}
				if _, e = ReadWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(generated), Limits{}, WrappedSurfaceOptions{}); e != nil {
					t.Fatal(e)
				}
			} else {
				if _, e = s.ValidateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{}); e != nil {
					t.Fatal(e)
				}
				if _, e = ReadWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(generated), Limits{}, WrappedSurfaceOptions{}); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}
func TestWrappedAnisotropicStationAssemblyReject(t *testing.T) {
	ctx := context.Background()
	for _, bore := range []bool{false, true} {
		raw := anisotropicAssemblyFixture(t, bore)
		read := func(b []byte, l Limits) error {
			if bore {
				m, e := ReadWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(b), l, WrappedSurfaceOptions{})
				if e != nil && m.Assembly.Source != nil {
					t.Fatal("partial")
				}
				return e
			}
			m, e := ReadWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(b), l, WrappedSurfaceOptions{})
			if e != nil && m.Assembly.Source != nil {
				t.Fatal("partial")
			}
			return e
		}
		for _, edit := range []func(*File){func(f *File) {
			sideDiskEditRecord(f, "NEXT_ASSEMBLY_USAGE_OCCURRENCE", func(r *Record) { r.Args[4] = r.Args[3] })
		}, func(f *File) { sideDiskEditRecord(f, "PCURVE", func(r *Record) { r.Args[1] = r.Args[2] }) }, func(f *File) {
			sideDiskEditRecord(f, "RATIONAL_B_SPLINE_SURFACE", func(r *Record) { r.Args[0].Items[1].Items[0] = Value{Kind: Number, Text: "0.50000000000000001"} })
		}} {
			if e := read(sideDiskMutateSource(t, raw, edit), Limits{}); e == nil {
				t.Fatal("bad graph")
			}
		}
		for _, l := range []Limits{{MaxBytes: 20}, {MaxFaces: 2}, {MaxEntities: 10}} {
			if e := read(raw, l); e == nil {
				t.Fatal("limit")
			}
		}
		c, cancel := context.WithCancel(ctx)
		cancel()
		if _, e := ReadWrappedAnisotropicStationAssembly(c, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	}
}
func TestWrappedAnisotropicStationAssemblyLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_ANISOTROPIC_STATION")
	if bin == "" {
		t.Skip("optional independent XCAF")
	}
	for _, variant := range []string{"filled", "bore", "negative"} {
		bore := variant == "bore"
		t.Run(variant, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			dir := t.TempDir()
			path, report := filepath.Join(dir, "original.step"), filepath.Join(dir, "report.json")
			mode := "filled-assembly"
			if bore {
				mode = "bore-assembly"
			}
			if variant == "negative" {
				mode = "filled-assembly-negative"
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
			var source bytes.Buffer
			var generated []byte
			if !bore {
				m, e := ReadWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
				if e != nil {
					t.Fatal(e)
				}
				var envelope bytes.Buffer
				if e = WriteWrappedAnisotropicStationAssemblyJSON(ctx, &envelope, m); e != nil {
					t.Fatal(e)
				}
				m, e = ReadWrappedAnisotropicStationAssemblyJSON(ctx, &envelope, IndependentMaterialJSONLimits{})
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
				if e = WriteWrappedAnisotropicStationAssemblySource(ctx, &source, m.Assembly.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
					t.Fatal(e)
				}
			} else {
				m, e := ReadWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
				if e != nil {
					t.Fatal(e)
				}
				var envelope bytes.Buffer
				if e = WriteWrappedAnisotropicStationBoreAssemblyJSON(ctx, &envelope, m); e != nil {
					t.Fatal(e)
				}
				m, e = ReadWrappedAnisotropicStationBoreAssemblyJSON(ctx, &envelope, IndependentMaterialJSONLimits{})
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
				body, e := WrappedAnisotropicStationBoreFromSolid(ctx, s, m.Body.Name, m.Body.LengthUnit, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				generated = regenerateAnisotropicAssembly(t, m.WrappedAnisotropicAssemblySource, nil, &body)
				if e = WriteWrappedAnisotropicStationBoreAssemblySource(ctx, &source, m.Assembly.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
					t.Fatal(e)
				}
			}
			for i, b := range [][]byte{source.Bytes(), generated} {
				p := filepath.Join(dir, fmt.Sprint(i)+".step")
				if e = os.WriteFile(p, b, 0600); e != nil {
					t.Fatal(e)
				}
				if reply, e := exec.CommandContext(ctx, bin, p, report, "read-assembly").CombinedOutput(); e != nil {
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

func TestWrappedAnisotropicStationAssemblyAtomic(t *testing.T) {
	ctx := context.Background()
	m, e := ReadWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(anisotropicAssemblyFixture(t, false)), Limits{}, WrappedSurfaceOptions{})
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
	model.Occurrences[0].Transform = geom.Affine{{-1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}
	var out bytes.Buffer
	if e = WriteWrappedAnisotropicStationAssembly(ctx, &out, model, Limits{}, WrappedSurfaceOptions{}); e == nil || out.Len() != 0 {
		t.Fatal("improper frame output")
	}
	model.Occurrences[0].Transform = geom.Identity()
	model.LengthUnit = 1
	if e = WriteWrappedAnisotropicStationAssembly(ctx, &out, model, Limits{}, WrappedSurfaceOptions{}); e == nil || out.Len() != 0 {
		t.Fatal("unit mismatch output")
	}
}
func FuzzWrappedAnisotropicStationAssembly(f *testing.F) {
	f.Add(anisotropicAssemblyFixture(f, false), false)
	f.Add(anisotropicAssemblyFixture(f, true), true)
	f.Fuzz(func(t *testing.T, b []byte, bore bool) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		l := Limits{MaxBytes: 1 << 20, MaxEntities: 4096, MaxValues: 65536, MaxDepth: 16}
		o := WrappedSurfaceOptions{MaxSurfaces: 32, MaxOperations: 4000000}
		if bore {
			m, e := ReadWrappedAnisotropicStationBoreAssembly(context.Background(), bytes.NewReader(b), l, o)
			if e != nil && m.Assembly.Source != nil {
				t.Fatal("partial")
			}
		} else {
			m, e := ReadWrappedAnisotropicStationAssembly(context.Background(), bytes.NewReader(b), l, o)
			if e != nil && m.Assembly.Source != nil {
				t.Fatal("partial")
			}
		}
	})
}
