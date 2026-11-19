package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
	"time"
)

func diskAssemblyFixture() DiskHoleAssemblyModel {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	outer := trimmed.ExtrusionCircle{Radius: 3}
	inner := trimmed.ExtrusionCircle{Radius: 1}
	return DiskHoleAssemblyModel{LengthUnit: .001, Products: []DiskHoleAssemblyPart{{ID: "root", Name: "Bored assembly"}, {ID: "blind", Name: "Blind stock", Bodies: []DiskHoleExtrusionBody{{Name: "Blind body", Frame: f, Outer: outer, Height: 4, Blind: []trimmed.ExtrusionBlindHole{{ExtrusionCircle: inner, Depth: 2}}}}}, {ID: "through", Name: "Through stock", Bodies: []DiskHoleExtrusionBody{{Name: "Through body", Frame: f, Outer: outer, Height: 4, Holes: []trimmed.ExtrusionCircle{inner}}}}}, Occurrences: []AssemblyInstance{{ID: "first", Name: "Blind vertical", Parent: "root", Child: "blind", Transform: geom.Translation(geom.Vec3{X: 10})}, {ID: "second", Name: "Blind horizontal", Parent: "root", Child: "blind", Transform: geom.Affine{{0, 0, 1, 0}, {1, 0, 0, 20}, {0, 1, 0, 0}}}, {ID: "third", Name: "Through vertical", Parent: "root", Child: "through", Transform: geom.Translation(geom.Vec3{X: -10})}}}
}
func TestDiskHoleAssembly(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if e := WriteDiskHoleAssembly(&out, diskAssemblyFixture()); e != nil {
		t.Fatal(e)
	}
	a, e := ReadDiskHoleAssembly(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Products) != 3 || len(a.Assembly.Occurrences) != 3 {
		t.Fatal(a)
	}
	ids := map[int64]string{}
	m := DiskHoleAssemblyModel{LengthUnit: a.Assembly.LengthUnit}
	for _, p := range a.Assembly.Products {
		ids[p.DefinitionID] = p.Identifier
		b := a.Products[p.DefinitionID]
		if len(b.Bodies) != len(b.Native) || len(b.Native) != len(b.SourceBodyIDs) {
			t.Fatal(b)
		}
		m.Products = append(m.Products, DiskHoleAssemblyPart{ID: p.Identifier, Name: p.Name, Bodies: b.Bodies})
		for _, id := range b.SourceBodyIDs {
			if _, e := simple(a.Assembly.Source, id, "MANIFOLD_SOLID_BREP", 2); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, o := range a.Assembly.Occurrences {
		m.Occurrences = append(m.Occurrences, AssemblyInstance{ID: o.Identifier, Name: o.Name, Parent: ids[o.Parent], Child: ids[o.Child], Transform: o.Transform})
	}
	var again bytes.Buffer
	if e = WriteDiskHoleAssembly(&again, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadDiskHoleExtrusions(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
		t.Fatal("standalone widened")
	}
	if _, e = ReadCylinderMaterialAssembly(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
		t.Fatal("cylinder widened")
	}
}
func TestDiskHoleAssemblyRejects(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if e := WriteDiskHoleAssembly(&out, diskAssemblyFixture()); e != nil {
		t.Fatal(e)
	}
	raw := out.Bytes()
	for _, l := range []Limits{{MaxBytes: 100}, {MaxSolids: 1}, {MaxFaces: 8}, {MaxVertices: 2}, {MaxEdgeUses: 4}} {
		a, e := ReadDiskHoleAssembly(bytes.NewReader(raw), l)
		if e == nil || a.Assembly.Source != nil {
			t.Fatal(l)
		}
	}
	for _, kind := range []string{"CIRCLE", "CYLINDRICAL_SURFACE", "ORIENTED_EDGE"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) == 1 && en.Records[0].Type == kind {
				r := en.Records[0]
				if kind == "ORIENTED_EDGE" {
					old := r.Args[4].Text
					r.Args[4] = Value{Kind: Enumeration, Text: "F"}
					if old == "F" {
						r.Args[4].Text = "T"
					}
				} else {
					r.Args[2] = Value{Kind: Number, Text: "9."}
				}
				en.Records[0] = r
				f.Entities[id] = en
				break
			}
		}
		a, e := ReadDiskHoleAssembly(bytes.NewReader(serializeWrappedProofFile(f)), Limits{})
		if e == nil || a.Assembly.Source != nil {
			t.Fatal(kind)
		}
	}
	m := diskAssemblyFixture()
	m.Occurrences[1].Transform[0][2] = 2
	out.Reset()
	if e := WriteDiskHoleAssembly(&out, m); e == nil || out.Len() != 0 {
		t.Fatal("nonrigid")
	}
}
func TestOCCTDiskHoleAssembly(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_ASSEMBLY")
	if bin == "" {
		t.Skip("optional pinned XCAF reader")
	}
	var out bytes.Buffer
	if e := WriteDiskHoleAssembly(&out, diskAssemblyFixture()); e != nil {
		t.Fatal(e)
	}
	vendor, e := os.ReadFile("testdata/occt-disk-hole-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{out.Bytes(), vendor} {
		path := filepath.Join(t.TempDir(), "bores.step")
		if e := os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		b, e := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(b))
		}
		at := bytes.LastIndex(b, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(b))
		}
		var p struct {
			Faces, Solids, Definitions, Occurrences int
			Volume, Area                            float64
			Center                                  [3]float64
		}
		if e = json.Unmarshal(b[at:], &p); e != nil {
			t.Fatal(e)
		}
		if p.Faces != 14 || p.Solids != 3 || p.Definitions != 2 || p.Occurrences != 3 || math.Abs(p.Volume-100*math.Pi) > 1e-7 || math.Abs(p.Area-140*math.Pi) > 1e-7 {
			t.Fatal(p)
		}
		for i, x := range []float64{.9, 6.8, 1.34} {
			if math.Abs(p.Center[i]-x) > 1e-8 {
				t.Fatal(p)
			}
		}
	}
}
func FuzzDiskHoleAssembly(f *testing.F) {
	var out bytes.Buffer
	if e := WriteDiskHoleAssembly(&out, diskAssemblyFixture()); e != nil {
		f.Fatal(e)
	}
	f.Add(out.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			t.Skip()
		}
		_, _ = ReadDiskHoleAssembly(bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 2048, MaxSolids: 4, MaxFaces: 32, MaxVertices: 64, MaxEdgeUses: 128})
	})
}

func TestDiskHoleAssemblyOwnership(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if e := WriteDiskHoleAssembly(&out, diskAssemblyFixture()); e != nil {
		t.Fatal(e)
	}
	a, e := ReadDiskHoleAssembly(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var supports []int64
	var faces []int64
	for _, p := range a.Assembly.Products {
		for _, id := range a.Products[p.DefinitionID].SourceBodyIDs {
			body, _ := simple(a.Assembly.Source, id, "MANIFOLD_SOLID_BREP", 2)
			shell, _ := simple(a.Assembly.Source, body.Args[1].Ref, "CLOSED_SHELL", 2)
			fs, _ := listRefs(shell.Args[1], 1)
			for _, face := range fs {
				r, _ := simple(a.Assembly.Source, face, "ADVANCED_FACE", 4)
				sid := r.Args[2].Ref
				s := a.Assembly.Source.Entities[sid]
				if s.Records[0].Type == "CYLINDRICAL_SURFACE" && s.Records[0].Args[2].Text == "3." {
					supports = append(supports, sid)
					faces = append(faces, face)
					break
				}
			}
		}
	}
	if len(supports) != 2 {
		t.Fatal(supports)
	}
	f := a.Assembly.Source
	en := f.Entities[faces[1]]
	en.Records[0].Args[2].Ref = supports[0]
	f.Entities[faces[1]] = en
	if got, e := ReadDiskHoleAssembly(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil || got.Assembly.Source != nil {
		t.Fatal("cross-product support")
	}
	vendor, e := os.ReadFile("testdata/occt-disk-hole-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadDiskHoleAssembly(bytes.NewReader(vendor), Limits{}); e == nil {
		t.Fatal("noncanonical vendor admitted")
	}
}
