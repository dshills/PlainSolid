package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestOCCTExactDirectedCircularLoopSourceReceipt(t *testing.T) {
	ctx := context.Background()
	src, err := os.ReadFile("testdata/occt-directed-two-holes.step")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ReadOCCTExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(src), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var b, again, formatted bytes.Buffer
	if err = WriteOCCTExactDirectedCircularLoopExtrusionJSON(ctx, &b, p); err != nil {
		t.Fatal(err)
	}
	if err = json.Indent(&formatted, b.Bytes(), "", "  "); err != nil {
		t.Fatal(err)
	}
	q, err := ReadOCCTExactDirectedCircularLoopExtrusionJSON(ctx, &formatted, IndependentMaterialJSONLimits{})
	if err != nil || !reflect.DeepEqual(q.CapBounds, p.CapBounds) || !reflect.DeepEqual(q.Rims, p.Rims) ||
		!reflect.DeepEqual(q.Product, p.Product) || !reflect.DeepEqual(q.Descriptor, p.Descriptor) {
		t.Fatalf("source receipt did not preserve ring ownership or product: %v", err)
	}
	if err = WriteOCCTExactDirectedCircularLoopExtrusionJSON(ctx, &again, q); err != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatalf("receipt did not re-emit canonical bytes: %v", err)
	}
	for _, mutation := range []struct{ from, to string }{
		{`"CapBounds":[[406,412,418],[430,436,442]]`, `"CapBounds":[[406,418,412],[430,436,442]]`},
		{`"RingOffsets":[0,4,8,12]`, `"RingOffsets":[0,4,9,12]`},
		{`"Version":1,"Source":`, `"Version":2,"Version":1,"Source":`},
		{`"DefinitionID":5`, `"DefinitionID":7`},
		{`"LengthUnit":0.001`, `"LengthUnit":1`},
	} {
		if bytes.Count(b.Bytes(), []byte(mutation.from)) != 1 {
			t.Fatalf("missing unique claim: %s", mutation.from)
		}
		bad := bytes.Replace(b.Bytes(), []byte(mutation.from), []byte(mutation.to), 1)
		if x, e := ReadOCCTExactDirectedCircularLoopExtrusionJSON(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{}); e == nil || x.Source != nil {
			t.Fatalf("published modified claim %s", mutation.to)
		}
	}
	bad := bytes.Replace(b.Bytes(), []byte(`#172=EDGE_CURVE('',#146,#165,#173,.T.);`),
		[]byte(`#172=EDGE_CURVE('',#146,#165,#173,.F.);`), 1)
	if bytes.Equal(b.Bytes(), bad) {
		t.Fatal("missing receipt's embedded original edge")
	}
	if x, e := ReadOCCTExactDirectedCircularLoopExtrusionJSON(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{}); e == nil || x.Source != nil {
		t.Fatal("published a modified original")
	}
	q.CapBounds[0][1] = 436
	again.Reset()
	if err = WriteOCCTExactDirectedCircularLoopExtrusionJSON(ctx, &again, q); err == nil || again.Len() != 0 {
		t.Fatal("writer published an unproved cap bound")
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: int64(b.Len() - 1)}, {Source: Limits{MaxEntities: 457}}} {
		if x, e := ReadOCCTExactDirectedCircularLoopExtrusionJSON(ctx, bytes.NewReader(b.Bytes()), l); e == nil || x.Source != nil {
			t.Fatalf("exceeded receipt or source quota: %+v", l)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadOCCTExactDirectedCircularLoopExtrusionJSON(ctx, bytes.NewReader(b.Bytes()), IndependentMaterialJSONLimits{}); !errors.Is(e, context.Canceled) {
		t.Fatalf("receipt cancellation: %v", e)
	}
	if e := WriteOCCTExactDirectedCircularLoopExtrusionJSON(ctx, &again, p); !errors.Is(e, context.Canceled) || again.Len() != 0 {
		t.Fatalf("receipt writer cancellation: %v", e)
	}
	if e := WriteOCCTExactDirectedCircularLoopExtrusionJSON(context.Background(), nil, p); !errors.Is(e, geom.ErrInvalid) {
		t.Fatalf("nil writer: %v", e)
	}
}
