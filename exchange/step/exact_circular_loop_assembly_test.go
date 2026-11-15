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

func exactCircularLoopAssemblyFixture(t testing.TB) []byte {
	t.Helper()
	ctx := context.Background()
	m, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(occtLoopFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	pose := geom.Affine{{0, 0, 1, 10}, {1, 0, 0, -2}, {0, 1, 0, 3}}
	model := ExactCircularLoopAssemblyModel{LengthUnit: m.Descriptor.LengthUnit, Products: []ExactCircularLoopAssemblyPart{{ID: "root", Name: "Root"}, {ID: "local", Name: "Rounded", Source: m.Source}}, Occurrences: []AssemblyInstance{{ID: "a", Name: "first", Parent: "root", Child: "local", Transform: geom.Identity()}, {ID: "b", Name: "second", Parent: "root", Child: "local", Transform: pose}}}
	var out bytes.Buffer
	if e = WriteExactCircularLoopAssembly(ctx, &out, model, Limits{}); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
func TestExactCircularLoopAssembly(t *testing.T) {
	raw := exactCircularLoopAssemblyFixture(t)
	ctx := context.Background()
	m, e := ReadExactCircularLoopAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Occurrences) != 2 || len(m.Assembly.Products) != 2 || len(m.Body.Walls) != 6 {
		t.Fatal("shared body")
	}
	native, e := prismNativeBytes(m.Body.Native)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = trimmed.ReadJSONContext(ctx, bytes.NewReader(native), trimmed.JSONLimits{}); e != nil {
		t.Fatal(e)
	}
	for _, u := range m.Occurrences {
		if u.ChildID != m.Body.Product.DefinitionID || u.NAUOID == 0 || u.CDSRID == 0 {
			t.Fatal("source ancestry")
		}
	}
	var out bytes.Buffer
	if e = WriteExactCircularLoopAssemblySource(ctx, &out, m.Assembly.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadExactCircularLoopAssembly(ctx, &out, Limits{}); e != nil {
		t.Fatal(e)
	}
}

func TestExactCircularLoopAssemblyOriginal(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-model-only-rounded-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadExactCircularLoopAssembly(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Occurrences) != 2 {
		t.Fatal("occurrences")
	}
}

func TestExactCircularLoopAssemblyReject(t *testing.T) {
	ctx := context.Background()
	raw := exactCircularLoopAssemblyFixture(t)
	edits := []func(*File){
		func(f *File) {
			sideDiskEditRecord(f, "CIRCLE", func(r *Record) { r.Args[2] = Value{Kind: Number, Text: "0.5"} })
		},
		func(f *File) {
			sideDiskEditRecord(f, "NEXT_ASSEMBLY_USAGE_OCCURRENCE", func(r *Record) { r.Args[3] = r.Args[4] })
		},
		func(f *File) {
			sideDiskEditRecord(f, "ITEM_DEFINED_TRANSFORMATION", func(r *Record) { r.Args[2] = r.Args[3] })
		},
		func(f *File) {
			id := int64(99999)
			f.Order = append(f.Order, id)
			f.Entities[id] = Entity{ID: id, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}}}}}}}
		},
	}
	// Change the nonidentity transform's source to its target, not the identity use.
	edits[2] = func(f *File) {
		n := 0
		for _, id := range f.Order {
			r, e := simple(f, id, "ITEM_DEFINED_TRANSFORMATION", 4)
			if e == nil {
				n++
				if n == 2 {
					r.Args = append([]Value(nil), r.Args...)
					r.Args[2] = r.Args[3]
					ovalSet(f, id, r)
					return
				}
			}
		}
	}
	for _, edit := range edits {
		bad := sideDiskMutateSource(t, raw, edit)
		m, e := ReadExactCircularLoopAssembly(ctx, bytes.NewReader(bad), Limits{})
		if e == nil || m.Assembly.Source != nil {
			t.Fatal("bad graph accepted")
		}
	}
	for _, l := range []Limits{{MaxBytes: 20}, {MaxEntities: 10}, {MaxFaces: 2}} {
		if _, e := ReadExactCircularLoopAssembly(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadExactCircularLoopAssembly(c, bytes.NewReader(raw), Limits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestExactCircularLoopAssemblyLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_EXACT_CIRCULAR_LOOP")
	if bin == "" {
		t.Skip("optional OCCT XCAF")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	p := filepath.Join(dir, "original.step")
	if b, e := exec.CommandContext(ctx, bin, "write-assembly", p).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	raw, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadExactCircularLoopAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var persisted bytes.Buffer
	if e = WriteExactCircularLoopAssemblyJSON(ctx, &persisted, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadExactCircularLoopAssemblyJSON(ctx, &persisted, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var source bytes.Buffer
	if e = WriteExactCircularLoopAssemblySource(ctx, &source, m.Assembly.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	type mass struct {
		Volume, Area                   float64
		Centroid                       [3]float64
		Solids, Faces, Edges, Vertices int
	}
	var first mass
	for i, b := range [][]byte{raw, source.Bytes(), regeneratedExactLoopAssembly(t, m)} {
		path := filepath.Join(dir, string(rune('a'+i))+".step")
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		reply, e := exec.CommandContext(ctx, bin, "read-assembly", path).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(reply))
		}
		at := bytes.LastIndex(reply, []byte("{\"volume\""))
		if at < 0 {
			t.Fatal(string(reply))
		}
		var q mass
		if e = json.Unmarshal(reply[at:], &q); e != nil {
			t.Fatal(e)
		}
		if q.Solids != 2 || q.Faces != 16 || q.Edges != 36 || q.Vertices != 24 || math.Abs(q.Volume-2*(27+25*math.Pi/32)) > 1e-8 || math.Abs(q.Area-2*(51+105*math.Pi/32)) > 1e-8 {
			t.Fatal(q)
		}
		if i == 0 {
			first = q
		} else {
			for j := range q.Centroid {
				if math.Abs(q.Centroid[j]-first.Centroid[j]) > 1e-9 {
					t.Fatal("world centroid", q, first)
				}
			}
		}
	}
}
func FuzzExactCircularLoopAssembly(f *testing.F) {
	f.Add(exactCircularLoopAssemblyFixture(f))
	raw, e := os.ReadFile("testdata/occt-model-only-rounded-assembly.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadExactCircularLoopAssembly(context.Background(), bytes.NewReader(b), Limits{})
		if e != nil && m.Assembly.Source != nil {
			t.Fatal("partial proof")
		}
	})
}

// Regenerate local analytic geometry from the re-proved recipe, retaining exact
// occurrence transforms in the established standard assembly emitter.
func regeneratedExactLoopAssembly(t testing.TB, m ExactCircularLoopAssembly) []byte {
	t.Helper()
	graph := AssemblyModel{LengthUnit: m.Assembly.LengthUnit}
	counts := make([]int, len(m.Assembly.Products))
	for i, p := range m.Assembly.Products {
		graph.Products = append(graph.Products, AssemblyPart{ID: fmt.Sprint(p.DefinitionID), Name: p.Name})
		if p.DefinitionID == m.Body.Product.DefinitionID {
			counts[i] = 1
		}
	}
	for _, u := range m.Assembly.Occurrences {
		graph.Occurrences = append(graph.Occurrences, AssemblyInstance{ID: fmt.Sprint(u.ID), Name: u.Name, Parent: fmt.Sprint(u.Parent), Child: fmt.Sprint(u.Child), Transform: u.Transform})
	}
	var out bytes.Buffer
	e := writeAssemblyGeometry(&out, graph, counts, func(i int) (*File, []int64, error) {
		var b bytes.Buffer
		if e := WriteExactCircularLoopExtrusion(context.Background(), &b, m.Body.Descriptor); e != nil {
			return nil, nil, e
		}
		return stagedAssemblyBodies(&b)
	})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Assembly.Products) == 3 {
		_, e = ReadNestedExactCircularLoopAssembly(context.Background(), bytes.NewReader(out.Bytes()), Limits{})
	} else {
		_, e = ReadExactCircularLoopAssembly(context.Background(), bytes.NewReader(out.Bytes()), Limits{})
	}
	if e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
