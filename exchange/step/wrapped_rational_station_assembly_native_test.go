package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func weightedAssemblyNativeFixture(t testing.TB) (WrappedRationalStationAssembly, []byte) {
	t.Helper()
	ctx := context.Background()
	m, e := ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(weightedAssemblyFixture(t)), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteWrappedRationalStationAssemblyJSON(ctx, &b, m); e != nil {
		t.Fatal(e)
	}
	return m, b.Bytes()
}
func TestWrappedRationalStationAssemblyNative(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	original, b := weightedAssemblyNativeFixture(t)
	m, e := ReadWrappedRationalStationAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(m.SourceWallFaceIDs, original.SourceWallFaceIDs) || !reflect.DeepEqual(m.Occurrences, original.Occurrences) {
		t.Fatal("source roles or poses changed")
	}
	var again bytes.Buffer
	if e = WriteWrappedRationalStationAssemblyJSON(ctx, &again, m); e != nil || !bytes.Equal(b, again.Bytes()) {
		t.Fatalf("unstable native: %v", e)
	}
	var source bytes.Buffer
	if e = WriteWrappedRationalStationAssemblySource(ctx, &source, m.Assembly.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedRationalStationAssembly(ctx, &source, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	if e = WriteWrappedRationalStationAssemblyJSON(ctx, occurrenceShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
}
func TestWrappedRationalStationAssemblyNativeTamper(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	m, b := weightedAssemblyNativeFixture(t)
	mutations := []func(*weightedAssemblyNativeEnvelope){
		func(q *weightedAssemblyNativeEnvelope) { q.Version++ }, func(q *weightedAssemblyNativeEnvelope) { q.Format = "plainsolid-step-independent-frustum-hierarchy" },
		func(q *weightedAssemblyNativeEnvelope) { q.SourceBodyID++ }, func(q *weightedAssemblyNativeEnvelope) { q.SourceWallFaceIDs[0]++ },
		func(q *weightedAssemblyNativeEnvelope) { q.Walls[0].Surface.Weights[1] = 2 }, func(q *weightedAssemblyNativeEnvelope) { q.Walls[0].SideEdges[0]++ },
		func(q *weightedAssemblyNativeEnvelope) { q.Occurrences[0].Matrix[0][3] = "11" }, func(q *weightedAssemblyNativeEnvelope) { q.Occurrences[0].Source.ChildID++ },
		func(q *weightedAssemblyNativeEnvelope) { q.Native = json.RawMessage(`{}`) }, func(q *weightedAssemblyNativeEnvelope) { q.Assembly.LengthUnit = 1 },
	}
	for i, mutate := range mutations {
		var q weightedAssemblyNativeEnvelope
		if e := json.Unmarshal(b, &q); e != nil {
			t.Fatal(e)
		}
		mutate(&q)
		bad, _ := json.Marshal(q)
		got, e := ReadWrappedRationalStationAssemblyJSON(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{})
		if e == nil || got.Assembly.Source != nil {
			t.Fatalf("accepted tamper %d", i)
		}
	}
	for _, bad := range [][]byte{append(append([]byte(nil), b...), []byte(`{}`)...), bytes.Replace(b, []byte(`"Version":1`), []byte(`"Unknown":0,"Version":1`), 1)} {
		if _, e := ReadWrappedRationalStationAssemblyJSON(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("accepted malformed JSON")
		}
	}
	if _, e := ReadWrappedRationalStationAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 10}); e == nil {
		t.Fatal("ignored byte limit")
	}
	if _, e := ReadWrappedRationalStationAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{Source: Limits{MaxEntities: 10}}); e == nil {
		t.Fatal("ignored source limit")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadWrappedRationalStationAssemblyJSON(canceled, bytes.NewReader(b), IndependentMaterialJSONLimits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	m.Body.Walls[0].SideEdges[0]++
	var out bytes.Buffer
	if e := WriteWrappedRationalStationAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("non-atomic stale write")
	}
}
func FuzzWrappedRationalStationAssemblyNative(f *testing.F) {
	_, b := weightedAssemblyNativeFixture(f)
	f.Add(b)
	f.Add([]byte(`{"Format":"wrong","Version":1}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadWrappedRationalStationAssemblyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 20})
		if e != nil && m.Assembly.Source != nil {
			t.Fatal("partial result")
		}
	})
}
