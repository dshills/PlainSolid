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
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
	"testing"
	"time"
)

func posedOCCTLoopFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-model-only-posed-rounded-triangle.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestPosedOCCTExactCircularLoop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw := posedOCCTLoopFixture(t)
	m, e := ReadPosedOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Descriptor.Height != 2 || m.Descriptor.Frame.XAxis() != (geom.Vec3{Y: 1}) || m.Descriptor.Frame.ZAxis() != (geom.Vec3{X: 1}) || m.Descriptor.Frame.Origin() != (geom.Vec3{X: -1}) {
		t.Fatal("normalized pose", m.Descriptor)
	}
	if _, e = ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old unposed mode widened")
	}
	if _, e = ReadExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("canonical mode widened")
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(f, m.Source) {
		t.Fatal("original source changed")
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Native); e != nil {
		t.Fatal(e)
	}
	if _, e = trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{}); e != nil {
		t.Fatal(e)
	}
	var source, canonical bytes.Buffer
	if e = WritePosedOCCTExactCircularLoopSource(ctx, &source, m.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	again, e := ReadPosedOCCTExactCircularLoopExtrusion(ctx, &source, Limits{})
	if e != nil || !reflect.DeepEqual(m.Walls, again.Walls) || m.ReferenceCircleID != again.ReferenceCircleID {
		t.Fatal("source role", e)
	}
	if e = WriteExactCircularLoopExtrusion(ctx, &canonical, m.Descriptor); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadExactCircularLoopExtrusion(ctx, &canonical, Limits{}); e != nil {
		t.Fatal(e)
	}
	if e = WritePosedOCCTExactCircularLoopSource(ctx, sideDiskShortWriter{}, m.Source, Limits{}); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
}
func TestPosedOCCTExactCircularLoopDirectionRatios(t *testing.T) {
	t.Parallel()
	raw := sideDiskMutateSource(t, posedOCCTLoopFixture(t), func(f *File) {
		sideDiskEditRecord(f, "CIRCLE", func(r *Record) {
			p := f.Entities[r.Args[1].Ref].Records[0]
			d := f.Entities[p.Args[2].Ref].Records[0]
			d.Args = append([]Value(nil), d.Args...)
			d.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "2."}, {Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}}}
			ovalSet(f, p.Args[2].Ref, d)
		})
	})
	m, e := ReadPosedOCCTExactCircularLoopExtrusion(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil || m.Descriptor.Frame.ZAxis() != (geom.Vec3{X: 1}) {
		t.Fatal("exact direction ratio", e)
	}
}

func TestPosedOCCTExactCircularLoopReject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw := posedOCCTLoopFixture(t)
	for _, edit := range []func(*File){func(f *File) {
		sideDiskEditRecord(f, "CYLINDRICAL_SURFACE", func(r *Record) {
			p := f.Entities[r.Args[1].Ref].Records[0]
			d := f.Entities[p.Args[2].Ref].Records[0]
			d.Args = append([]Value(nil), d.Args...)
			d.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0."}, {Kind: Number, Text: "1."}, {Kind: Number, Text: "0."}}}
			ovalSet(f, p.Args[2].Ref, d)
		})
	}, func(f *File) {
		sideDiskEditRecord(f, "CIRCLE", func(r *Record) {
			p := f.Entities[r.Args[1].Ref].Records[0]
			d := f.Entities[p.Args[2].Ref].Records[0]
			d.Args = append([]Value(nil), d.Args...)
			d.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0.6"}, {Kind: Number, Text: "0.8"}, {Kind: Number, Text: "0."}}}
			ovalSet(f, p.Args[2].Ref, d)
		})
	}, func(f *File) { sideDiskEditRecord(f, "DIRECTION", func(r *Record) { r.Args[1].Items[0].Text = "1." }) }} {
		bad := sideDiskMutateSource(t, raw, edit)
		m, e := ReadPosedOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(bad), Limits{})
		if e == nil || m.Source != nil {
			t.Fatal("unsupported/mismatched pose accepted")
		}
	}
	for _, l := range []Limits{{MaxBytes: 20}, {MaxEntities: 10}, {MaxFaces: 2}} {
		if _, e := ReadPosedOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadPosedOCCTExactCircularLoopExtrusion(c, bytes.NewReader(raw), Limits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestPosedOCCTExactCircularLoopLive(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_EXACT_CIRCULAR_LOOP")
	if bin == "" {
		t.Skip("optional independent posed author")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	original := filepath.Join(dir, "original.step")
	if b, e := exec.CommandContext(ctx, bin, "write-model-only-posed", original).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	raw, e := os.ReadFile(original)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPosedOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var persisted bytes.Buffer
	if e = WritePosedOCCTExactCircularLoopExtrusionJSON(ctx, &persisted, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadPosedOCCTExactCircularLoopExtrusionJSON(ctx, &persisted, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var source, generated bytes.Buffer
	if e = WritePosedOCCTExactCircularLoopSource(ctx, &source, m.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	if e = WriteExactCircularLoopExtrusion(ctx, &generated, m.Descriptor); e != nil {
		t.Fatal(e)
	}
	type mass struct {
		Volume, Area                   float64
		Centroid                       [3]float64
		Solids, Faces, Edges, Vertices int
	}
	var first mass
	for i, b := range [][]byte{raw, source.Bytes(), generated.Bytes()} {
		p := filepath.Join(dir, string(rune('a'+i))+".step")
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
		var q mass
		if e = json.Unmarshal(reply[at:], &q); e != nil {
			t.Fatal(e)
		}
		if q.Solids != 1 || q.Faces != 8 || q.Edges != 18 || q.Vertices != 12 || math.Abs(q.Volume-(27+25*math.Pi/32)) > 1e-8 || math.Abs(q.Area-(51+105*math.Pi/32)) > 1e-8 {
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

func FuzzPosedOCCTExactCircularLoop(f *testing.F) {
	f.Add(posedOCCTLoopFixture(f))
	f.Add(occtLoopFixture(f))
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadPosedOCCTExactCircularLoopExtrusion(context.Background(), bytes.NewReader(b), Limits{})
		if e != nil && m.Source != nil {
			t.Fatal("partial pose proof")
		}
	})
}
