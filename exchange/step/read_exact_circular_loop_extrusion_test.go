package step

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"plainsolid/brep/trimmed"
	"reflect"
	"testing"
)

func exactLoopSTEPBytes(t testing.TB, negative bool) []byte {
	t.Helper()
	var b bytes.Buffer
	if e := WriteExactCircularLoopExtrusion(context.Background(), &b, exactLoopSTEPFixture(t, negative)); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestReadExactCircularLoopExtrusion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, negative := range []bool{false, true} {
		raw := exactLoopSTEPBytes(t, negative)
		got, e := ReadExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(got.Descriptor, exactLoopSTEPFixture(t, negative)) {
			t.Fatal("descriptor", got.Descriptor)
		}
		if len(got.Walls) != 6 || len(got.VerticalEdges) != 6 || len(got.Rims[0]) != 6 || len(got.Vertices[1]) != 6 {
			t.Fatal("source roles")
		}
		var native bytes.Buffer
		if e = trimmed.WriteJSON(&native, got.Native); e != nil {
			t.Fatal(e)
		}
		s, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
		if e != nil || !reflect.DeepEqual(s.Topology(), got.Native.Topology()) {
			t.Fatal("native", e)
		}
		var again bytes.Buffer
		if e = WriteExactCircularLoopExtrusion(ctx, &again, got.Descriptor); e != nil || !bytes.Equal(raw, again.Bytes()) {
			t.Fatal("roundtrip", e)
		}
		f := got.Source
		mapping := map[int64]int64{}
		for _, id := range f.Order {
			mapping[id] = id*3 + 17
		}
		var reordered bytes.Buffer
		reordered.Write(raw[:bytes.Index(raw, []byte("DATA;"))+5])
		for i := len(f.Order) - 1; i >= 0; i-- {
			id := f.Order[i]
			fmt.Fprintf(&reordered, "\n#%d=%s;", mapping[id], assemblyEntity(f.Entities[id], mapping))
		}
		reordered.WriteString("\nENDSEC;END-ISO-10303-21;\n")
		other, e := ReadExactCircularLoopExtrusion(ctx, &reordered, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		for j := range got.Walls {
			if other.Walls[j] != mapping[got.Walls[j]] || other.VerticalEdges[j] != mapping[got.VerticalEdges[j]] {
				t.Fatal("lost original role")
			}
			for cap := 0; cap < 2; cap++ {
				if other.Rims[cap][j] != mapping[got.Rims[cap][j]] || other.Vertices[cap][j] != mapping[got.Vertices[cap][j]] {
					t.Fatal("lost original rim/vertex")
				}
			}
		}
	}
}
func TestReadExactCircularLoopExtrusionReject(t *testing.T) {
	t.Parallel()
	raw := exactLoopSTEPBytes(t, false)
	ctx := context.Background()
	for _, test := range []struct {
		kind string
		edit func(*Record)
	}{
		{"CIRCLE", func(r *Record) { r.Args[2].Text = "0.626" }},
		{"CYLINDRICAL_SURFACE", func(r *Record) { r.Args[2].Text = "0.626" }},
		{"EDGE_CURVE", func(r *Record) { r.Args[4].Text = "F" }},
		{"ORIENTED_EDGE", func(r *Record) { r.Args[3] = r.Args[0] }},
		{"FACE_OUTER_BOUND", func(r *Record) { r.Args[2].Text = "T" }},
		{"DIRECTION", func(r *Record) { r.Args[1].Items[0].Text = "0.00000000000000001" }},
		{"PRODUCT", func(r *Record) { r.Args[0].Text = "foreign" }},
		{"LINE", func(r *Record) { r.Args[2] = r.Args[1] }},
	} {
		bad := sideDiskMutateSource(t, raw, func(f *File) {
			if !sideDiskEditRecord(f, test.kind, test.edit) {
				t.Fatal(test.kind)
			}
		})
		got, e := ReadExactCircularLoopExtrusion(ctx, bytes.NewReader(bad), Limits{})
		if e == nil || got.Source != nil {
			t.Fatal("accepted", test.kind)
		}
	}
	for _, l := range []Limits{{MaxBytes: 50}, {MaxEntities: 10}, {MaxValues: 20}, {MaxDepth: 1}} {
		if _, e := ReadExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limits")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadExactCircularLoopExtrusion(c, bytes.NewReader(raw), Limits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	vendor, e := os.ReadFile("testdata/occt-model-only-rounded-triangle-extrusion.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(vendor), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, en := range f.Entities {
		for _, r := range en.Records {
			if r.Type == "PCURVE" || r.Type == "SURFACE_CURVE" {
				t.Fatal("official mode did not remove pcurves")
			}
		}
	}
	if _, e = ReadExactCircularLoopExtrusion(ctx, bytes.NewReader(vendor), Limits{}); e == nil {
		t.Fatal("unproved vendor grammar accepted")
	}
}
func FuzzReadExactCircularLoopExtrusion(f *testing.F) {
	f.Add(exactLoopSTEPBytes(f, false))
	f.Add(exactLoopSTEPBytes(f, true))
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadExactCircularLoopExtrusion(context.Background(), bytes.NewReader(b), Limits{})
		if e != nil && m.Source != nil {
			t.Fatal("partial proof")
		}
	})
}
