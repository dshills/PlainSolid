package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

func anisotropicAssemblyNativeFixture(t testing.TB, bore bool) []byte {
	t.Helper()
	ctx := context.Background()
	var b bytes.Buffer
	if bore {
		m, e := ReadWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(anisotropicAssemblyFixture(t, true)), Limits{}, WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if e = WriteWrappedAnisotropicStationBoreAssemblyJSON(ctx, &b, m); e != nil {
			t.Fatal(e)
		}
	} else {
		m, e := ReadWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(anisotropicAssemblyFixture(t, false)), Limits{}, WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if e = WriteWrappedAnisotropicStationAssemblyJSON(ctx, &b, m); e != nil {
			t.Fatal(e)
		}
	}
	return b.Bytes()
}
func TestWrappedAnisotropicAssemblyNative(t *testing.T) {
	ctx := context.Background()
	for _, bore := range []bool{false, true} {
		raw := anisotropicAssemblyNativeFixture(t, bore)
		var stable bytes.Buffer
		if bore {
			m, e := ReadWrappedAnisotropicStationBoreAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			if e = WriteWrappedAnisotropicStationBoreAssemblyJSON(ctx, &stable, m); e != nil {
				t.Fatal(e)
			}
			if e = WriteWrappedAnisotropicStationBoreAssemblyJSON(ctx, sideDiskShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
				t.Fatal(e)
			}
			m.Body.Outer.Walls[0].Name += "forged"
			var out bytes.Buffer
			if e = WriteWrappedAnisotropicStationBoreAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
				t.Fatal("stale bore output")
			}
		} else {
			m, e := ReadWrappedAnisotropicStationAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			if e = WriteWrappedAnisotropicStationAssemblyJSON(ctx, &stable, m); e != nil {
				t.Fatal(e)
			}
			if e = WriteWrappedAnisotropicStationAssemblyJSON(ctx, sideDiskShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
				t.Fatal(e)
			}
			m.Assembly.Occurrences[0].Transform[0][3]++
			var out bytes.Buffer
			if e = WriteWrappedAnisotropicStationAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
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
func TestWrappedAnisotropicAssemblyNativeReject(t *testing.T) {
	ctx := context.Background()
	for _, bore := range []bool{false, true} {
		raw := anisotropicAssemblyNativeFixture(t, bore)
		for _, edit := range []func(*anisotropicAssemblyNativeEnvelope){
			func(q *anisotropicAssemblyNativeEnvelope) { q.Format = weightedAssemblyNativeFormat },
			func(q *anisotropicAssemblyNativeEnvelope) { q.Version++ },
			func(q *anisotropicAssemblyNativeEnvelope) { q.Source = "ISO-10303-21;" },
			func(q *anisotropicAssemblyNativeEnvelope) { q.SourceBodyID++ },
			func(q *anisotropicAssemblyNativeEnvelope) { q.SourceFaceIDs[0]++ },
			func(q *anisotropicAssemblyNativeEnvelope) { q.Assembly.Products[0].DefinitionID++ },
			func(q *anisotropicAssemblyNativeEnvelope) { q.Assembly.Occurrences[0].Transform[0][3]++ },
			func(q *anisotropicAssemblyNativeEnvelope) { q.Occurrences[0].Source.RelationshipID++ },
			func(q *anisotropicAssemblyNativeEnvelope) { q.Occurrences[0].Matrix[0][3] = "99" },
			func(q *anisotropicAssemblyNativeEnvelope) { q.Sides[0].Walls[0].SideEdges[0]++ },
			func(q *anisotropicAssemblyNativeEnvelope) { q.Sides[0].Walls[0].Surface.Weights[0] = .25 },
			func(q *anisotropicAssemblyNativeEnvelope) { q.Native = json.RawMessage(`{}`) },
		} {
			var q anisotropicAssemblyNativeEnvelope
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
func FuzzWrappedAnisotropicAssemblyNative(f *testing.F) {
	f.Add(anisotropicAssemblyNativeFixture(f, false), false)
	f.Add(anisotropicAssemblyNativeFixture(f, true), true)
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
