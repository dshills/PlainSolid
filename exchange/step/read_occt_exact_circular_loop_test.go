package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"testing"
	"time"
)

func occtLoopFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-model-only-rounded-triangle-extrusion.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestReadOCCTExactCircularLoop(t *testing.T) {
	ctx := context.Background()
	m, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(occtLoopFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Walls) != 6 || len(m.VerticalEdges) != 6 || m.Descriptor.Height != 2 || m.Product.Name != "Product 1" {
		t.Fatal(m.Descriptor)
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Native); e != nil {
		t.Fatal(e)
	}
	if _, e = trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{}); e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteExactCircularLoopExtrusion(ctx, &out, m.Descriptor); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadExactCircularLoopExtrusion(ctx, &out, Limits{}); e != nil {
		t.Fatal(e)
	}
	if e = WriteOCCTExactCircularLoopSource(ctx, sideDiskShortWriter{}, m.Source, Limits{}); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
}

func TestReadOCCTExactCircularLoopReject(t *testing.T) {
	raw := occtLoopFixture(t)
	ctx := context.Background()
	for _, test := range []struct {
		kind string
		edit func(*Record)
	}{
		{"EDGE_LOOP", func(r *Record) { r.Args[1].Items[0] = r.Args[1].Items[1] }},
		{"CIRCLE", func(r *Record) { r.Args = r.Args[:2] }},
		{"CYLINDRICAL_SURFACE", func(r *Record) { r.Args[2].Text = "0.62500000000000001" }},
		{"EDGE_CURVE", func(r *Record) { r.Args[4].Text = "F" }},
		{"FACE_BOUND", func(r *Record) { r.Args[2].Text = "T" }},
		{"ADVANCED_FACE", func(r *Record) { r.Args[3].Text = "T" }},
		{"APPLICATION_PROTOCOL_DEFINITION", func(r *Record) { r.Args[3].Ref = 7 }},
		{"PRODUCT_RELATED_PRODUCT_CATEGORY", func(r *Record) { r.Args[2].Items[0].Ref = 8 }},
		{"GLOBAL_UNIT_ASSIGNED_CONTEXT", func(r *Record) { r.Args[0].Items = append(r.Args[0].Items, Value{Kind: Reference, Ref: 13}) }},
	} {
		bad := sideDiskMutateSource(t, raw, func(f *File) {
			if !sideDiskEditRecord(f, test.kind, test.edit) {
				t.Fatal(test.kind)
			}
		})
		got, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(bad), Limits{})
		if e == nil || got.Source != nil {
			t.Fatal("accepted", test.kind)
		}
	}
	for _, pair := range [][2]string{{"(-0.8,0.6,0.)", "(-0.8,0.60000000000000001,0.)"}, {"(-0.6,-0.8,0.)", "(-0.60000000000000001,-0.8,0.)"}, {"(4.375,0.5,0.)", "(4.375,0.50000000000000001,0.)"}, {"SI_UNIT($,.RADIAN.)", "SI_UNIT($,.DEGREE.)"}} {
		bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(raw, bad) {
			t.Fatal("missing mutation")
		}
		got, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(bad), Limits{})
		if e == nil || got.Source != nil {
			t.Fatal("accepted", pair)
		}
	}
	for _, l := range []Limits{{MaxBytes: 20}, {MaxEntities: 10}, {MaxFaces: 2}, {MaxVertices: 2}, {MaxEdgeUses: 2}} {
		if _, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("ignored limit")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadOCCTExactCircularLoopExtrusion(c, bytes.NewReader(raw), Limits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	bad := sideDiskMutateSource(t, raw, func(f *File) {
		f.Entities[999] = Entity{ID: 999, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}}}}}}}
		f.Order = append(f.Order, 999)
	})
	if _, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(bad), Limits{}); e == nil {
		t.Fatal("detached geometry")
	}
}
func TestReadOCCTExactCircularLoopReindex(t *testing.T) {
	ctx := context.Background()
	raw := occtLoopFixture(t)
	m, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	mapping := map[int64]int64{}
	for _, id := range m.Source.Order {
		mapping[id] = id*5 + 31
	}
	var out bytes.Buffer
	out.Write(raw[:bytes.Index(raw, []byte("DATA;"))+5])
	for i := len(m.Source.Order) - 1; i >= 0; i-- {
		id := m.Source.Order[i]
		fmt.Fprintf(&out, "\n#%d=%s;", mapping[id], assemblyEntity(m.Source.Entities[id], mapping))
	}
	out.WriteString("\nENDSEC;END-ISO-10303-21;\n")
	q, e := ReadOCCTExactCircularLoopExtrusion(ctx, &out, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for j := range m.Walls {
		if q.Walls[j] != mapping[m.Walls[j]] || q.VerticalEdges[j] != mapping[m.VerticalEdges[j]] {
			t.Fatal("role mapping")
		}
	}
	if q.Product.ProductID != mapping[m.Product.ProductID] {
		t.Fatal("product")
	}
}
func TestReadOCCTExactCircularLoopLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_EXACT_CIRCULAR_LOOP")
	if bin == "" {
		t.Skip("optional pinned author/TransferRoots")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	fresh := filepath.Join(dir, "original.step")
	if b, e := exec.CommandContext(ctx, bin, "write-model-only", fresh).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	raw, e := os.ReadFile(fresh)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var envelope bytes.Buffer
	if e = WriteOCCTExactCircularLoopExtrusionJSON(ctx, &envelope, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadOCCTExactCircularLoopExtrusionJSON(ctx, &envelope, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Native); e != nil {
		t.Fatal(e)
	}
	if _, e = trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{}); e != nil {
		t.Fatal(e)
	}
	var generated, source bytes.Buffer
	if e = WriteExactCircularLoopExtrusion(ctx, &generated, m.Descriptor); e != nil {
		t.Fatal(e)
	}
	if e = WriteOCCTExactCircularLoopSource(ctx, &source, m.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	type report struct {
		Volume, Area                   float64
		Centroid                       [3]float64
		Solids, Faces, Edges, Vertices int
	}
	var original report
	for i, b := range [][]byte{raw, source.Bytes(), generated.Bytes()} {
		p := filepath.Join(dir, fmt.Sprintf("%d.step", i))
		if e = os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		reply, e := exec.CommandContext(ctx, bin, "read", p).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(reply))
		}
		at := bytes.LastIndex(reply, []byte("{\"volume\""))
		if at < 0 {
			t.Fatal(string(reply))
		}
		var got report
		if e = json.Unmarshal(reply[at:], &got); e != nil {
			t.Fatal(e)
		}
		if got.Solids != 1 || got.Faces != 8 || got.Edges != 18 || got.Vertices != 12 || math.Abs(got.Volume-(27+25*math.Pi/32)) > 1e-8 || math.Abs(got.Area-(51+105*math.Pi/32)) > 1e-8 {
			t.Fatal(got)
		}
		if i == 0 {
			original = got
		} else {
			for j := range got.Centroid {
				if math.Abs(got.Centroid[j]-original.Centroid[j]) > 1e-9 {
					t.Fatal("centroid")
				}
			}
		}
	}
}
func FuzzReadOCCTExactCircularLoop(f *testing.F) {
	f.Add(occtLoopFixture(f))
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadOCCTExactCircularLoopExtrusion(context.Background(), bytes.NewReader(b), Limits{})
		if e != nil && m.Source != nil {
			t.Fatal("partial material")
		}
	})
}

func TestReadOCCTExactCircularLoopEquivalentSource(t *testing.T) {
	ctx := context.Background()
	raw := occtLoopFixture(t)
	shuffled := sideDiskMutateSource(t, raw, func(f *File) {
		for _, id := range f.Order {
			en := f.Entities[id]
			for i := range en.Records {
				r := &en.Records[i]
				if r.Type == "CLOSED_SHELL" {
					v := r.Args[1].Items
					for a, b := 0, len(v)-1; a < b; a, b = a+1, b-1 {
						v[a], v[b] = v[b], v[a]
					}
				}
				if r.Type == "EDGE_LOOP" {
					v := r.Args[1].Items
					r.Args[1].Items = append(append([]Value(nil), v[1:]...), v[0])
				}
			}
		}
	})
	if _, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(shuffled), Limits{}); e != nil {
		t.Fatal("source incidence reorder", e)
	}
	// Exact decimal point lies on the same infinite LINE although its binary64
	// approximation would not certify the 3:4:5 ratio. It is not a vertex.
	shifted := bytes.Replace(raw, []byte("#113 = CARTESIAN_POINT('',(4.375,0.5,0.));"), []byte("#113 = CARTESIAN_POINT('',(3.575,1.1,0.));"), 1)
	if bytes.Equal(raw, shifted) {
		t.Fatal("missing source")
	}
	if _, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(shifted), Limits{}); e != nil {
		t.Fatal("exact same line", e)
	}
}
