package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"plainsolid/internal/slowtest"
	"testing"
)

func exactLoopAssemblyNativeFixture(t testing.TB) []byte {
	t.Helper()
	raw, e := os.ReadFile("testdata/occt-model-only-rounded-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadExactCircularLoopAssembly(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteExactCircularLoopAssemblyJSON(context.Background(), &b, m); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestExactCircularLoopAssemblyNative(t *testing.T) {
	ctx := context.Background()
	raw := exactLoopAssemblyNativeFixture(t)
	m, e := ReadExactCircularLoopAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = WriteExactCircularLoopAssemblyJSON(ctx, &again, m); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal("unstable", e)
	}
	if len(m.Occurrences) != 2 || m.Occurrences[0].ChildID != m.Occurrences[1].ChildID {
		t.Fatal("shared identity")
	}
	var source bytes.Buffer
	if e = WriteExactCircularLoopAssemblySource(ctx, &source, m.Assembly.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadExactCircularLoopAssembly(ctx, &source, Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadOCCTExactCircularLoopExtrusionJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old standalone tag")
	}
	if e = WriteExactCircularLoopAssemblyJSON(ctx, sideDiskShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	m.Assembly.Occurrences[0].Transform[0][3]++
	var out bytes.Buffer
	if e = WriteExactCircularLoopAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("stale claim output")
	}
}
func TestExactCircularLoopAssemblyNativeReject(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := exactLoopAssemblyNativeFixture(t)
	for _, edit := range []func(*exactLoopAssemblyNativeEnvelope){
		func(q *exactLoopAssemblyNativeEnvelope) { q.Format = occtLoopNativeFormat },
		func(q *exactLoopAssemblyNativeEnvelope) { q.Version++ },
		func(q *exactLoopAssemblyNativeEnvelope) { q.Assembly.Products[1].DefinitionID++ },
		func(q *exactLoopAssemblyNativeEnvelope) { q.Assembly.Occurrences[0].Transform[0][3]++ },
		func(q *exactLoopAssemblyNativeEnvelope) { q.Occurrences[0].Source.NAUOID++ },
		func(q *exactLoopAssemblyNativeEnvelope) { q.Occurrences[0].Source.FromPlacementID++ },
		func(q *exactLoopAssemblyNativeEnvelope) { q.Occurrences[0].Matrix[0][3] = "1" },
		func(q *exactLoopAssemblyNativeEnvelope) { q.Body.Recipe.Height++ },
		func(q *exactLoopAssemblyNativeEnvelope) { q.Body.Vertices[0][0]++ },
		func(q *exactLoopAssemblyNativeEnvelope) { q.Body.Native = json.RawMessage(`{}`) },
		func(q *exactLoopAssemblyNativeEnvelope) { q.Source = "ISO-10303-21;" },
	} {
		var q exactLoopAssemblyNativeEnvelope
		if e := json.Unmarshal(raw, &q); e != nil {
			t.Fatal(e)
		}
		edit(&q)
		b, _ := json.Marshal(q)
		m, e := ReadExactCircularLoopAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
		if e == nil || m.Assembly.Source != nil {
			t.Fatal("forged claims")
		}
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 20}, {Source: Limits{MaxEntities: 10}}, {Source: Limits{MaxFaces: 2}}} {
		if _, e := ReadExactCircularLoopAssemblyJSON(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limits")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadExactCircularLoopAssemblyJSON(c, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	c, cancel = context.WithCancel(ctx)
	defer cancel()
	if _, e := ReadExactCircularLoopAssemblyJSON(c, &occurrenceCancelReader{Reader: bytes.NewReader(raw), cancel: cancel}, IndependentMaterialJSONLimits{}); !errors.Is(e, context.Canceled) {
		t.Fatal("during read", e)
	}
	for _, b := range [][]byte{append(append([]byte(nil), raw...), []byte(`{}`)...), []byte(`{"Format":"plainsolid-step-exact-circular-loop-assembly","Version":1,"Unknown":0}`)} {
		if _, e := ReadExactCircularLoopAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("payload grammar")
		}
	}
}
func FuzzExactCircularLoopAssemblyNative(f *testing.F) {
	f.Add(exactLoopAssemblyNativeFixture(f))
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 8<<20 {
			t.Skip()
		}
		m, e := ReadExactCircularLoopAssemblyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{})
		if e != nil && m.Assembly.Source != nil {
			t.Fatal("partial")
		}
	})
}
