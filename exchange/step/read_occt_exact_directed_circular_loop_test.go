package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
)

func TestOCCTExactDirectedCircularLoopOriginalWires(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw, err := os.ReadFile("testdata/occt-directed-two-holes.step")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ReadOCCTExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Product.ProductID != 7 || p.Body != 15 || p.Shell != 16 || p.Caps != [2]int64{405, 429} ||
		!reflect.DeepEqual(p.RingOffsets, []int{0, 4, 8, 12}) ||
		!reflect.DeepEqual(p.CapBounds, [2][]int64{{406, 412, 418}, {430, 436, 442}}) ||
		len(p.Walls) != 12 || len(p.Vertices[0]) != 12 || len(p.Rims[1]) != 12 ||
		p.Descriptor.Frame.Origin().Z != -1 || p.Descriptor.Height != 2 || p.Descriptor.LengthUnit != .001 {
		t.Fatalf("lost original topology, ring offsets, or position: %+v", p.ExactDirectedCircularLoopExtrusionProduct)
	}
	m, err := p.Native.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(m.Volume-(48-math.Pi)) > 1e-8 || math.Abs(m.Area-(88+3*math.Pi)) > 1e-8 {
		t.Fatalf("native material differs: %+v %v", m, err)
	}
	var source, canonical bytes.Buffer
	if err = WriteOCCTExactDirectedCircularLoopSource(ctx, &source, p.Source, Limits{}); err != nil {
		t.Fatal(err)
	}
	again, err := ReadOCCTExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(source.Bytes()), Limits{})
	if err != nil || !reflect.DeepEqual(again.Rims, p.Rims) || !reflect.DeepEqual(again.CapBounds, p.CapBounds) {
		t.Fatalf("original re-export changed source identities: %v", err)
	}
	if err = WriteExactDirectedCircularLoopExtrusion(ctx, &canonical, p.Descriptor); err != nil {
		t.Fatal(err)
	}
	q, err := ReadExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(canonical.Bytes()), Limits{})
	if err != nil || !reflect.DeepEqual(q.RingOffsets, p.RingOffsets) {
		t.Fatalf("canonical re-export changed original rings: %v", err)
	}
	m2, err := q.Native.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || m2 != m {
		t.Fatalf("canonical material differs: %+v / %+v %v", m, m2, err)
	}

	for _, x := range []struct{ name, from, to string }{
		{"major arc", "#172 = EDGE_CURVE('',#146,#165,#173,.T.);", "#172 = EDGE_CURVE('',#146,#165,#173,.F.);"},
		{"outside cylinder normal", "#141 = ADVANCED_FACE('',(#142),#178,.F.);", "#141 = ADVANCED_FACE('',(#142),#178,.T.);"},
		{"reversed source hole", "#436 = FACE_BOUND('',#437,.F.);", "#436 = FACE_BOUND('',#437,.T.);"},
		{"hole labelled exterior", "#436 = FACE_BOUND('',#437,.F.);", "#436 = FACE_OUTER_BOUND('',#437,.F.);"},
		{"off-axis cap circle", "#175 = CARTESIAN_POINT('',(2.,2.,-1.));", "#175 = CARTESIAN_POINT('',(2.,2.,-0.9999999999999999));"},
		{"rounded shared rim", "#147 = CARTESIAN_POINT('',(2.5,2.,-1.));", "#147 = CARTESIAN_POINT('',(2.5000000000000001,2.,-1.));"},
	} {
		t.Run(x.name, func(t *testing.T) {
			if bytes.Count(raw, []byte(x.from)) != 1 {
				t.Fatal("expected unique original source record")
			}
			bad := bytes.Replace(raw, []byte(x.from), []byte(x.to), 1)
			if b, e := ReadOCCTExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(bad), Limits{}); e == nil || b.Source != nil {
				t.Fatalf("published a modified original: %v", e)
			}
		})
	}
	if q, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); e == nil || q.Source != nil {
		t.Fatal("legacy single-wire reader accepted hole topology")
	}
}

func TestOCCTExactDirectedCircularLoopLiveOriginal(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_DIRECTED_LOOP_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_DIRECTED_CIRCULAR_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned independent STEP author and reader")
	}
	ctx := context.Background()
	dir := t.TempDir()
	input := filepath.Join(dir, "source.step")
	if out, err := exec.Command(author, input).CombinedOutput(); err != nil {
		t.Fatalf("author: %v %s", err, out)
	}
	raw, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile("testdata/occt-directed-two-holes.step")
	if err != nil {
		t.Fatal(err)
	}
	dataPart := func(b []byte) []byte {
		i := bytes.Index(b, []byte("\nDATA;\n"))
		if i < 0 {
			t.Fatal("missing DATA section")
		}
		return b[i:]
	}
	if !bytes.Equal(dataPart(raw), dataPart(committed)) {
		t.Fatal("stored fixture differs from the independent OCCT author")
	}
	p, err := ReadOCCTExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var original, canonical bytes.Buffer
	if err = WriteOCCTExactDirectedCircularLoopSource(ctx, &original, p.Source, Limits{}); err != nil {
		t.Fatal(err)
	}
	if err = WriteExactDirectedCircularLoopExtrusion(ctx, &canonical, p.Descriptor); err != nil {
		t.Fatal(err)
	}
	for name, src := range map[string][]byte{"source": raw, "original replay": original.Bytes(), "native canonical": canonical.Bytes()} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "read.step")
			if err := os.WriteFile(path, src, 0600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(reader, "read", path).CombinedOutput()
			i := bytes.LastIndexByte(out, '{')
			var got struct {
				Solids, Faces int
				Volume, Area  float64
			}
			if err != nil || i < 0 || json.Unmarshal(out[i:], &got) != nil ||
				got.Solids != 1 || got.Faces != 14 || math.Abs(got.Volume-(48-math.Pi)) > 1e-8 || math.Abs(got.Area-(88+3*math.Pi)) > 1e-8 {
				t.Fatalf("independent STEP transfer: %+v %v %s", got, err, out)
			}
		})
	}
}
