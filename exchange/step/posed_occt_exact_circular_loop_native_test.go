package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

func TestPosedOCCTExactCircularLoopNative(t *testing.T) {
	ctx := context.Background()
	m, e := ReadPosedOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(posedOCCTLoopFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WritePosedOCCTExactCircularLoopExtrusionJSON(ctx, &b, m); e != nil {
		t.Fatal(e)
	}
	q, e := ReadPosedOCCTExactCircularLoopExtrusionJSON(ctx, bytes.NewReader(b.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = WritePosedOCCTExactCircularLoopExtrusionJSON(ctx, &again, q); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("unstable", e)
	}
	for _, read := range []func(context.Context, io.Reader, IndependentMaterialJSONLimits) (OCCTExactCircularLoopExtrusion, error){ReadOCCTExactCircularLoopExtrusionJSON} {
		if _, e = read(ctx, bytes.NewReader(b.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("old tag accepted")
		}
	}
	for _, edit := range []func(*posedOCCTLoopNativeEnvelope){
		func(q *posedOCCTLoopNativeEnvelope) { q.Format = occtLoopNativeFormat },
		func(q *posedOCCTLoopNativeEnvelope) { q.Version++ },
		func(q *posedOCCTLoopNativeEnvelope) { q.ReferenceCircleID++ },
		func(q *posedOCCTLoopNativeEnvelope) { q.NormalizationRotation[0][0] = 1 },
		func(q *posedOCCTLoopNativeEnvelope) { q.Recipe.Frame[0][3]++ },
		func(q *posedOCCTLoopNativeEnvelope) { q.Recipe.Height++ },
		func(q *posedOCCTLoopNativeEnvelope) { q.Walls[0]++ },
		func(q *posedOCCTLoopNativeEnvelope) { q.Native = json.RawMessage(`{}`) },
		func(q *posedOCCTLoopNativeEnvelope) { q.Source = "ISO-10303-21;" },
	} {
		var bad posedOCCTLoopNativeEnvelope
		if e = json.Unmarshal(b.Bytes(), &bad); e != nil {
			t.Fatal(e)
		}
		edit(&bad)
		raw, _ := json.Marshal(bad)
		got, e := ReadPosedOCCTExactCircularLoopExtrusionJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
		if e == nil || got.Source != nil {
			t.Fatal("forged claim accepted")
		}
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 20}, {Source: Limits{MaxEntities: 10}}} {
		if _, e = ReadPosedOCCTExactCircularLoopExtrusionJSON(ctx, bytes.NewReader(b.Bytes()), l); e == nil {
			t.Fatal("limit")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadPosedOCCTExactCircularLoopExtrusionJSON(c, bytes.NewReader(b.Bytes()), IndependentMaterialJSONLimits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if e = WritePosedOCCTExactCircularLoopExtrusionJSON(ctx, sideDiskShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	m.ReferenceCircleID++
	var out bytes.Buffer
	if e = WritePosedOCCTExactCircularLoopExtrusionJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("stale writer")
	}
}
