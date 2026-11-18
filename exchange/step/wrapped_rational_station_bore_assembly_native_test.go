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

func weightedBoreAssemblyNativeFixture(t testing.TB) (WrappedRationalStationBoreAssembly, []byte) {
	t.Helper()
	ctx := context.Background()
	m, e := ReadWrappedRationalStationBoreAssembly(ctx, bytes.NewReader(weightedBoreAssemblyFixture(t)), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteWrappedRationalStationBoreAssemblyJSON(ctx, &b, m); e != nil {
		t.Fatal(e)
	}
	return m, b.Bytes()
}
func TestWrappedRationalStationBoreAssemblyNative(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	original, b := weightedBoreAssemblyNativeFixture(t)
	m, e := ReadWrappedRationalStationBoreAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(m.SourceOuterWallFaceIDs, original.SourceOuterWallFaceIDs) || !reflect.DeepEqual(m.Occurrences, original.Occurrences) {
		t.Fatal("source roles or poses changed")
	}
	var again bytes.Buffer
	if e = WriteWrappedRationalStationBoreAssemblyJSON(ctx, &again, m); e != nil || !bytes.Equal(b, again.Bytes()) {
		t.Fatalf("unstable native: %v", e)
	}
	var source bytes.Buffer
	if e = WriteWrappedRationalStationBoreAssemblySource(ctx, &source, m.Assembly.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedRationalStationBoreAssembly(ctx, &source, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	if e = WriteWrappedRationalStationBoreAssemblyJSON(ctx, occurrenceShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
}
func TestWrappedRationalStationBoreAssemblyNativeTamper(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	m, b := weightedBoreAssemblyNativeFixture(t)
	_, filled := weightedAssemblyNativeFixture(t)
	if _, e := ReadWrappedRationalStationBoreAssemblyJSON(ctx, bytes.NewReader(filled), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("accepted filled envelope")
	}
	if _, e := ReadWrappedRationalStationAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("filled accepted bore envelope")
	}
	mutations := []func(*weightedBoreAssemblyNativeEnvelope){
		func(q *weightedBoreAssemblyNativeEnvelope) { q.Inner.Native = json.RawMessage(`{}`) },
		func(q *weightedBoreAssemblyNativeEnvelope) { q.OuterFaces, q.InnerFaces = q.InnerFaces, q.OuterFaces },
		func(q *weightedBoreAssemblyNativeEnvelope) { q.Proof.Version++ }, func(q *weightedBoreAssemblyNativeEnvelope) {
			q.Proof.Format = "plainsolid-step-independent-frustum-hierarchy"
		},
		func(q *weightedBoreAssemblyNativeEnvelope) { q.Proof.SourceBodyID++ }, func(q *weightedBoreAssemblyNativeEnvelope) { q.InnerFaces[0]++ },
		func(q *weightedBoreAssemblyNativeEnvelope) { q.Inner.Walls[0].Surface.Weights[1] = 2 }, func(q *weightedBoreAssemblyNativeEnvelope) { q.Proof.Walls[0].SideEdges[0]++ },
		func(q *weightedBoreAssemblyNativeEnvelope) { q.Proof.Occurrences[0].Matrix[0][3] = "11" }, func(q *weightedBoreAssemblyNativeEnvelope) { q.Proof.Occurrences[0].Source.ChildID++ },
		func(q *weightedBoreAssemblyNativeEnvelope) { q.Proof.Native = json.RawMessage(`{}`) }, func(q *weightedBoreAssemblyNativeEnvelope) { q.Proof.Assembly.LengthUnit = 1 },
	}
	for i, mutate := range mutations {
		var q weightedBoreAssemblyNativeEnvelope
		if e := json.Unmarshal(b, &q); e != nil {
			t.Fatal(e)
		}
		mutate(&q)
		bad, _ := json.Marshal(q)
		got, e := ReadWrappedRationalStationBoreAssemblyJSON(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{})
		if e == nil || got.Assembly.Source != nil {
			t.Fatalf("accepted tamper %d", i)
		}
	}
	for _, bad := range [][]byte{append(append([]byte(nil), b...), []byte(`{}`)...), bytes.Replace(b, []byte(`"Version":1`), []byte(`"Unknown":0,"Version":1`), 1)} {
		if _, e := ReadWrappedRationalStationBoreAssemblyJSON(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("accepted malformed JSON")
		}
	}
	if _, e := ReadWrappedRationalStationBoreAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 10}); e == nil {
		t.Fatal("ignored byte limit")
	}
	if _, e := ReadWrappedRationalStationBoreAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{Source: Limits{MaxEntities: 10}}); e == nil {
		t.Fatal("ignored source limit")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadWrappedRationalStationBoreAssemblyJSON(canceled, bytes.NewReader(b), IndependentMaterialJSONLimits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	m.Body.Outer.Walls[0].SideEdges[0]++
	var out bytes.Buffer
	if e := WriteWrappedRationalStationBoreAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("non-atomic stale write")
	}
}
func FuzzWrappedRationalStationBoreAssemblyNative(f *testing.F) {
	_, b := weightedBoreAssemblyNativeFixture(f)
	f.Add(b)
	f.Add([]byte(`{"Format":"wrong","Version":1}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadWrappedRationalStationBoreAssemblyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 20})
		if e != nil && m.Assembly.Source != nil {
			t.Fatal("partial result")
		}
	})
}
