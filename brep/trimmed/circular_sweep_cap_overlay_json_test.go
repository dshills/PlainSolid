package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"plainsolid/geom"
)

func TestExactCircularSweepCapOverlaySourceReceipt(t *testing.T) {
	ctx := context.Background()
	a, b := circularSweepOverlayFixture()
	var first, pretty, replay bytes.Buffer
	if err := WriteExactCircularSweepCapOverlayJSON(ctx, &first, a, b); err != nil {
		t.Fatal(err)
	}
	if err := json.Indent(&pretty, first.Bytes(), "", "  "); err != nil {
		t.Fatal(err)
	}
	out, err := ReadExactCircularSweepCapOverlayJSON(ctx, &pretty, CircularSweepOverlayJSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	edge := out.Join.Edges[0][9][1]
	if edge.Minimum.RatString() != "1/2" || edge.Map[1][1].RatString() != "-1/6" || len(out.Join.SharedRimEdges) != 10 {
		t.Fatalf("lost exact original arc or ownership in native replay: %+v", edge)
	}
	if err = WriteExactCircularSweepCapOverlayJSON(ctx, &replay, out.Sources[0], out.Sources[1]); err != nil || !bytes.Equal(first.Bytes(), replay.Bytes()) {
		t.Fatalf("source receipt changed after rebuild: %v", err)
	}
	for _, edit := range []struct{ before, after string }{
		{`"-1/6"`, `"-1/5"`},
		{`"Version":1,"Sources"`, `"Version":2,"Version":1,"Sources"`},
		{`"SharedRimEdges":[`, `"SharedRimEdges":[99,`},
	} {
		changed := bytes.Replace(first.Bytes(), []byte(edit.before), []byte(edit.after), 1)
		if bytes.Equal(changed, first.Bytes()) {
			t.Fatalf("missing test claim %s", edit.before)
		}
		p, e := ReadExactCircularSweepCapOverlayJSON(ctx, bytes.NewReader(changed), CircularSweepOverlayJSONLimits{})
		if e == nil || len(p.Join.Solid.Faces()) != 0 {
			t.Fatalf("published an edited or ambiguous receipt: %v", e)
		}
	}
	for _, limits := range []CircularSweepOverlayJSONLimits{{MaxSegments: 8}, {MaxBytes: int64(first.Len() - 1)}} {
		out, e := ReadExactCircularSweepCapOverlayJSON(ctx, bytes.NewReader(first.Bytes()), limits)
		if !errors.Is(e, geom.ErrConvergence) || len(out.Join.Solid.Faces()) != 0 {
			t.Fatalf("ignored exact receipt limit: %#v %v", limits, e)
		}
	}
}
