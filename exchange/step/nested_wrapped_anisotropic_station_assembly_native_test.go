package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"plainsolid/internal/slowtest"
	"testing"
)

func nestedAnisotropicAssemblyNativeFixture(t testing.TB, bore bool) []byte {
	t.Helper()
	ctx := context.Background()
	var b bytes.Buffer
	if bore {
		m, e := ReadNestedWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(nestedAnisotropicFixture(t, true)), Limits{}, WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if e = WriteNestedWrappedAnisotropicStationBoreAssemblyJSON(ctx, &b, m); e != nil {
			t.Fatal(e)
		}
	} else {
		m, e := ReadNestedWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(nestedAnisotropicFixture(t, false)), Limits{}, WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if e = WriteNestedWrappedAnisotropicStationAssemblyJSON(ctx, &b, m); e != nil {
			t.Fatal(e)
		}
	}
	return b.Bytes()
}
func TestNestedWrappedAnisotropicAssemblyNative(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, bore := range []bool{false, true} {
		raw := nestedAnisotropicAssemblyNativeFixture(t, bore)
		var stable bytes.Buffer
		if bore {
			m, e := ReadNestedWrappedAnisotropicStationBoreAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			if e = WriteNestedWrappedAnisotropicStationBoreAssemblyJSON(ctx, &stable, m); e != nil {
				t.Fatal(e)
			}
			if e = WriteNestedWrappedAnisotropicStationBoreAssemblyJSON(ctx, sideDiskShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
				t.Fatal(e)
			}
			m.Body.Outer.Walls[0].Name += "forged"
			var out bytes.Buffer
			if e = WriteNestedWrappedAnisotropicStationBoreAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
				t.Fatal("stale bore output")
			}
		} else {
			m, e := ReadNestedWrappedAnisotropicStationAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			if e = WriteNestedWrappedAnisotropicStationAssemblyJSON(ctx, &stable, m); e != nil {
				t.Fatal(e)
			}
			if e = WriteNestedWrappedAnisotropicStationAssemblyJSON(ctx, sideDiskShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
				t.Fatal(e)
			}
			m.Assembly.Occurrences[0].Transform[0][3]++
			var out bytes.Buffer
			if e = WriteNestedWrappedAnisotropicStationAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
				t.Fatal("stale pose output")
			}
		}
		if !bytes.Equal(raw, stable.Bytes()) {
			t.Fatal("unstable native encoding")
		}
		if _, e := ReadWrappedRationalStationAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("old tag widened")
		}
		if _, _, e := readAnisotropicAssemblyNative(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}, !bore); e == nil {
			t.Fatal("cross-family native")
		}
	}
}
func TestNestedWrappedAnisotropicAssemblyNativeReject(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, bore := range []bool{false, true} {
		raw := nestedAnisotropicAssemblyNativeFixture(t, bore)
		for _, edit := range []func(*nestedAnisotropicNativeEnvelope){
			func(q *nestedAnisotropicNativeEnvelope) { q.Format = weightedAssemblyNativeFormat },
			func(q *nestedAnisotropicNativeEnvelope) { q.Version++ },
			func(q *nestedAnisotropicNativeEnvelope) { q.GroupID++ },
			func(q *nestedAnisotropicNativeEnvelope) { q.Instances[0].Path[1]++ },
			func(q *nestedAnisotropicNativeEnvelope) { q.Instances[0].Transform[0][3]++ },
			func(q *nestedAnisotropicNativeEnvelope) { q.Instances[0].ProductID++ },
			func(q *nestedAnisotropicNativeEnvelope) { q.Format = anisotropicAssemblyNativeFormat },
			func(q *nestedAnisotropicNativeEnvelope) { q.Source = "ISO-10303-21;" },
			func(q *nestedAnisotropicNativeEnvelope) { q.SourceBodyID++ },
			func(q *nestedAnisotropicNativeEnvelope) { q.SourceFaceIDs[0]++ },
			func(q *nestedAnisotropicNativeEnvelope) { q.Assembly.Products[0].DefinitionID++ },
			func(q *nestedAnisotropicNativeEnvelope) { q.Assembly.Occurrences[0].Transform[0][3]++ },
			func(q *nestedAnisotropicNativeEnvelope) { q.Occurrences[0].Source.RelationshipID++ },
			func(q *nestedAnisotropicNativeEnvelope) { q.Occurrences[0].Matrix[0][3] = "99" },
			func(q *nestedAnisotropicNativeEnvelope) { q.Sides[0].Walls[0].SideEdges[0]++ },
			func(q *nestedAnisotropicNativeEnvelope) { q.Sides[0].Walls[0].Surface.Weights[0] = .25 },
			func(q *nestedAnisotropicNativeEnvelope) { q.Native = json.RawMessage(`{}`) },
		} {
			var q nestedAnisotropicNativeEnvelope
			if e := json.Unmarshal(raw, &q); e != nil {
				t.Fatal(e)
			}
			edit(&q)
			bad, _ := json.Marshal(q)
			a, h, e := readAnisotropicAssemblyNative(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{}, bore)
			if e == nil || a.Assembly.Source != nil || h.Assembly.Source != nil {
				t.Fatal("forged native")
			}
		}
		for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 20}, {Source: Limits{MaxEntities: 10}}, {Source: Limits{MaxFaces: 2}}} {
			if _, _, e := readAnisotropicAssemblyNative(ctx, bytes.NewReader(raw), l, bore); e == nil {
				t.Fatal("limits")
			}
		}
		c, cancel := context.WithCancel(ctx)
		cancel()
		if _, _, e := readAnisotropicAssemblyNative(c, bytes.NewReader(raw), IndependentMaterialJSONLimits{}, bore); !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
		c, cancel = context.WithCancel(ctx)
		if _, _, e := readAnisotropicAssemblyNative(c, &occurrenceCancelReader{Reader: bytes.NewReader(raw), cancel: cancel}, IndependentMaterialJSONLimits{}, bore); !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
		cancel()
	}
}
func FuzzNestedWrappedAnisotropicAssemblyNative(f *testing.F) {
	f.Add(nestedAnisotropicAssemblyNativeFixture(f, false), false)
	f.Add(nestedAnisotropicAssemblyNativeFixture(f, true), true)
	f.Fuzz(func(t *testing.T, b []byte, bore bool) {
		if len(b) > 16<<20 {
			t.Skip()
		}
		a, h, e := readAnisotropicAssemblyNative(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{}, bore)
		if e != nil && (a.Assembly.Source != nil || h.Assembly.Source != nil) {
			t.Fatal("partial")
		}
	})
}
