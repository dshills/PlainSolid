package trimmed

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
)

func TestProjectiveImprintedShellNativeExactSplineRoundTrip(t *testing.T) {
	ctx := context.Background()
	a, b := projectiveImprintShellFixture(t, true, false), projectiveImprintShellFixture(t, false, true)
	r, err := a.ImprintProjectiveSplineShellFaces(ctx, 5, b, 5, [2]int{0, 1}, ProjectiveShellImprintOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, x := range r.Owners {
		var data, replay bytes.Buffer
		if err := WriteJSON(&data, x.Solid); err != nil {
			t.Fatal("imprinted original closed shell lacks an exact native codec", i, err)
		}
		if !bytes.Contains(data.Bytes(), []byte("exact-bezier-curve")) {
			t.Fatal("native record discarded the standalone exact spline", i)
		}
		reopened, err := ReadJSONContext(ctx, bytes.NewReader(data.Bytes()), JSONLimits{})
		if err != nil {
			t.Fatal("cannot reopen an exact imprinted shell", i, err)
		}
		if len(reopened.faces) != 7 || len(reopened.edges) != 15 || len(reopened.vertices) != 10 {
			t.Fatal("native replay changed closed topology", i, len(reopened.faces), len(reopened.edges), len(reopened.vertices))
		}
		if err := WriteJSON(&replay, reopened); err != nil || !bytes.Equal(data.Bytes(), replay.Bytes()) {
			t.Fatal("native replay bytes changed", i, err)
		}
		m, err := reopened.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil || math.Abs(m.Volume-1) > 1e-10 {
			t.Fatal("native replay changed the independently integrated volume", i, m, err)
		}
	}
}
