package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
	"time"
)

func nestedLoopFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-model-only-nested-rounded-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func nestedLoopModel(t testing.TB) ExactCircularLoopAssemblyModel {
	t.Helper()
	m, e := ReadOCCTExactCircularLoopExtrusion(context.Background(), bytes.NewReader(occtLoopFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return ExactCircularLoopAssemblyModel{LengthUnit: m.Descriptor.LengthUnit, Products: []ExactCircularLoopAssemblyPart{{ID: "root", Name: "Root"}, {ID: "group", Name: "Shared group"}, {ID: "leaf", Name: "Rounded", Source: m.Source}}, Occurrences: []AssemblyInstance{{ID: "a", Parent: "root", Child: "group", Transform: geom.Identity()}, {ID: "b", Parent: "root", Child: "group", Transform: geom.Affine{{0, 0, 1, 10}, {1, 0, 0, -2}, {0, 1, 0, 3}}}, {ID: "c", Parent: "group", Child: "leaf", Transform: geom.Affine{{1, 0, 0, 1}, {0, 1, 0, 2}, {0, 0, 1, 0}}}}}
}
func TestNestedExactCircularLoopAssembly(t *testing.T) {
	ctx := context.Background()
	var authored bytes.Buffer
	if e := WriteNestedExactCircularLoopAssembly(ctx, &authored, nestedLoopModel(t), Limits{}); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{nestedLoopFixture(t), authored.Bytes()} {
		m, e := ReadNestedExactCircularLoopAssembly(ctx, bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(m.Assembly.Products) != 3 || len(m.Occurrences) != 3 || len(m.Instances) != 2 || m.Instances[0].Path[1] != m.Instances[1].Path[1] || m.Instances[0].Path[0] == m.Instances[1].Path[0] {
			t.Fatal("source paths")
		}
		want := []geom.Affine{{{1, 0, 0, 1}, {0, 1, 0, 2}, {0, 0, 1, 0}}, {{0, 0, 1, 10}, {1, 0, 0, -1}, {0, 1, 0, 5}}}
		for _, instance := range m.Instances {
			if instance.Transform != want[0] && instance.Transform != want[1] {
				t.Fatal("composition", instance)
			}
		}
		if _, e = ReadExactCircularLoopAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
			t.Fatal("flat widened")
		}
		var b bytes.Buffer
		if e = WriteNestedExactCircularLoopAssemblyJSON(ctx, &b, m); e != nil {
			t.Fatal(e)
		}
		again, e := ReadNestedExactCircularLoopAssemblyJSON(ctx, bytes.NewReader(b.Bytes()), IndependentMaterialJSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		var stable bytes.Buffer
		if e = WriteNestedExactCircularLoopAssemblyJSON(ctx, &stable, again); e != nil || !bytes.Equal(b.Bytes(), stable.Bytes()) {
			t.Fatal("unstable", e)
		}
		if _, e = ReadExactCircularLoopAssemblyJSON(ctx, bytes.NewReader(b.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("flat native widened")
		}
		if e = WriteNestedExactCircularLoopAssemblyJSON(ctx, sideDiskShortWriter{}, again); !errors.Is(e, io.ErrShortWrite) {
			t.Fatal(e)
		}
	}
}
func TestNestedExactCircularLoopAssemblyReject(t *testing.T) {
	ctx := context.Background()
	raw := nestedLoopFixture(t)
	for _, edit := range []func(*File){
		func(f *File) {
			sideDiskEditRecord(f, "NEXT_ASSEMBLY_USAGE_OCCURRENCE", func(r *Record) { r.Args[4] = r.Args[3] })
		},
		func(f *File) {
			sideDiskEditRecord(f, "CIRCLE", func(r *Record) { r.Args[2] = Value{Kind: Number, Text: "1."} })
		},
		func(f *File) {
			sideDiskEditRecord(f, "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION", func(r *Record) { r.Args[1] = r.Args[0] })
		},
	} {
		bad := sideDiskMutateSource(t, raw, edit)
		m, e := ReadNestedExactCircularLoopAssembly(ctx, bytes.NewReader(bad), Limits{})
		if e == nil || m.Assembly.Source != nil {
			t.Fatal("bad graph")
		}
	}
	model := nestedLoopModel(t)
	model.Occurrences[2].Child = "root"
	var out bytes.Buffer
	if e := WriteNestedExactCircularLoopAssembly(ctx, &out, model, Limits{}); e == nil || out.Len() != 0 {
		t.Fatal("cycle output")
	}
	model = nestedLoopModel(t)
	model.Occurrences[2].Transform[0][3] = .1
	model.Occurrences[0].Transform[0][3] = .2
	if e := WriteNestedExactCircularLoopAssembly(ctx, &out, model, Limits{}); e == nil || out.Len() != 0 {
		t.Fatal("rounded composed translation")
	}
	m, e := ReadNestedExactCircularLoopAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteNestedExactCircularLoopAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*nestedExactLoopAssemblyNativeEnvelope){
		func(q *nestedExactLoopAssemblyNativeEnvelope) { q.Format = exactLoopAssemblyNativeFormat },
		func(q *nestedExactLoopAssemblyNativeEnvelope) { q.GroupID++ },
		func(q *nestedExactLoopAssemblyNativeEnvelope) { q.Instances[0].Path[1]++ },
		func(q *nestedExactLoopAssemblyNativeEnvelope) { q.Instances[0].Transform[0][3]++ },
		func(q *nestedExactLoopAssemblyNativeEnvelope) { q.Body.Walls[0]++ },
		func(q *nestedExactLoopAssemblyNativeEnvelope) { q.Body.Native = json.RawMessage(`{}`) },
	} {
		var q nestedExactLoopAssemblyNativeEnvelope
		if e = json.Unmarshal(native.Bytes(), &q); e != nil {
			t.Fatal(e)
		}
		edit(&q)
		b, _ := json.Marshal(q)
		got, e := ReadNestedExactCircularLoopAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
		if e == nil || got.Assembly.Source != nil {
			t.Fatal("native forgery")
		}
	}
	for _, l := range []Limits{{MaxBytes: 20}, {MaxEntities: 10}, {MaxFaces: 2}} {
		if _, e = ReadNestedExactCircularLoopAssembly(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limits")
		}
	}
	if _, e = ReadNestedExactCircularLoopAssemblyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{MaxBytes: 20}); e == nil {
		t.Fatal("native limit")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadNestedExactCircularLoopAssemblyJSON(c, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestNestedExactCircularLoopAssemblyLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_EXACT_CIRCULAR_LOOP")
	if bin == "" {
		t.Skip("optional XCAF")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	path := filepath.Join(dir, "original.step")
	if b, e := exec.CommandContext(ctx, bin, "write-nested-assembly", path).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadNestedExactCircularLoopAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteNestedExactCircularLoopAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadNestedExactCircularLoopAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var source, authored bytes.Buffer
	if e = WriteNestedExactCircularLoopAssemblySource(ctx, &source, m.Assembly.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	if e = WriteNestedExactCircularLoopAssembly(ctx, &authored, nestedLoopModel(t), Limits{}); e != nil {
		t.Fatal(e)
	}
	type mass struct {
		Volume, Area                   float64
		Centroid                       [3]float64
		Solids, Faces, Edges, Vertices int
	}
	var first mass
	for i, b := range [][]byte{raw, source.Bytes(), authored.Bytes(), regeneratedExactLoopAssembly(t, m.ExactCircularLoopAssembly)} {
		p := filepath.Join(dir, string(rune('a'+i))+".step")
		if e = os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		reply, e := exec.CommandContext(ctx, bin, "read-nested-assembly", p).CombinedOutput()
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
					t.Fatal(q, first)
				}
			}
		}
	}
}
func FuzzNestedExactCircularLoopAssembly(f *testing.F) {
	f.Add(nestedLoopFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadNestedExactCircularLoopAssembly(context.Background(), bytes.NewReader(b), Limits{})
		if e != nil && m.Assembly.Source != nil {
			t.Fatal("partial")
		}
	})
}
