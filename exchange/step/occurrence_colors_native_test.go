package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep"
	"reflect"
	"strings"
	"testing"
	"time"
)

func occurrenceColorsNativeFixture(t testing.TB) (OccurrenceColoredAssembly, []byte) {
	t.Helper()
	raw, e := os.ReadFile("testdata/occt-ap242-occurrence-colors.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadOccurrenceColoredAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteOccurrenceColoredAssemblyJSON(context.Background(), &out, m); e != nil {
		t.Fatal(e)
	}
	return m, out.Bytes()
}
func TestOccurrenceColorsNative(t *testing.T) {
	m, raw := occurrenceColorsNativeFixture(t)
	n, e := ReadOccurrenceColoredAssemblyJSON(context.Background(), bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(m.Overrides, n.Overrides) || !reflect.DeepEqual(m.Assembly.Assembly.Occurrences, n.Assembly.Assembly.Occurrences) {
		t.Fatal("ancestry loss")
	}
	var again bytes.Buffer
	if e = WriteOccurrenceColoredAssemblyJSON(context.Background(), &again, n); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal("unstable native bytes", e)
	}
	for id, bs := range m.Assembly.ProductBodies {
		for i, b := range bs {
			got := n.Assembly.ProductBodies[id][i]
			if !reflect.DeepEqual(b.Body.Solid.Vertices(), got.Body.Solid.Vertices()) || !reflect.DeepEqual(b.Body.Solid.Edges(), got.Body.Solid.Edges()) || !reflect.DeepEqual(b.Body.Solid.Faces(), got.Body.Solid.Faces()) {
				t.Fatal("native topology loss")
			}
		}
	}
}
func TestOccurrenceColorsNativeTamper(t *testing.T) {
	_, raw := occurrenceColorsNativeFixture(t)
	mutations := []func(*occurrenceColorsEnvelope){
		func(q *occurrenceColorsEnvelope) {
			q.Source = strings.Replace(q.Source, "DRAUGHTING_PRE_DEFINED_COLOUR('blue')", "DRAUGHTING_PRE_DEFINED_COLOUR('green')", 1)
		},
		func(q *occurrenceColorsEnvelope) { q.Format = "plainsolid-step-independent-frustum-hierarchy" },
		func(q *occurrenceColorsEnvelope) { q.Version = 2 },
		func(q *occurrenceColorsEnvelope) { q.LengthUnit *= 1000 },
		func(q *occurrenceColorsEnvelope) { q.Definitions[0].Name += "forged" },
		func(q *occurrenceColorsEnvelope) { q.Occurrences[0].Transform[0][3]++ },
		func(q *occurrenceColorsEnvelope) { q.Overrides[0].Color = RGB{1, 0, 0} },
		func(q *occurrenceColorsEnvelope) { q.Overrides[0].OccurrenceID = q.Occurrences[0].ID },
		func(q *occurrenceColorsEnvelope) { q.Overrides[0].PlacementID++ },
		func(q *occurrenceColorsEnvelope) { q.Products[1].Bodies[0].Vertices[0].X++ },
		func(q *occurrenceColorsEnvelope) {
			q.Products[1].Bodies[0].Faces[0].Boundary[0].Reversed = !q.Products[1].Bodies[0].Faces[0].Boundary[0].Reversed
		},
		func(q *occurrenceColorsEnvelope) { q.Products[1].Bodies[0].SourceFaceIDs[0]++ },
		func(q *occurrenceColorsEnvelope) { q.Products[1].Bodies[0].SurfaceColor = &RGB{0, 1, 0} },
	}
	for i, mutate := range mutations {
		var q occurrenceColorsEnvelope
		if e := json.Unmarshal(raw, &q); e != nil {
			t.Fatal(e)
		}
		mutate(&q)
		bad, e := json.Marshal(q)
		if e != nil {
			t.Fatal(e)
		}
		if got, e := ReadOccurrenceColoredAssemblyJSON(context.Background(), bytes.NewReader(bad), IndependentMaterialJSONLimits{}); e == nil || len(got.Overrides) != 0 || len(got.Assembly.ProductBodies) != 0 {
			t.Fatal("accepted forged native claim", i)
		}
	}
}

func TestOccurrenceColorsNativeLimitsAndAtomicity(t *testing.T) {
	m, raw := occurrenceColorsNativeFixture(t)
	ctx := context.Background()
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: int64(len(raw) - 1)}, {MaxBytes: -1}, {MaxBytes: 9 << 20}, {Source: Limits{MaxEntities: 700}}} {
		if q, e := ReadOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(raw), l); e == nil || len(q.Overrides) != 0 {
			t.Fatal("limit accepted", l)
		}
	}
	for _, b := range [][]byte{append(append([]byte(nil), raw...), []byte("{}")...), bytes.Replace(raw, []byte(`"Version":1`), []byte(`"Version":1,"Unknown":true`), 1)} {
		if _, e := ReadOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("invalid outer JSON accepted")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	var out bytes.Buffer
	if _, e := ReadOccurrenceColoredAssemblyJSON(canceled, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if e := WriteOccurrenceColoredAssemblyJSON(canceled, &out, m); !errors.Is(e, context.Canceled) || out.Len() != 0 {
		t.Fatal(e)
	}
	during, c := context.WithCancel(ctx)
	if _, e := ReadOccurrenceColoredAssemblyJSON(during, &occurrenceCancelReader{Reader: bytes.NewReader(raw), cancel: c}, IndependentMaterialJSONLimits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	for _, mutate := range []func(*OccurrenceColoredAssembly){
		func(q *OccurrenceColoredAssembly) { q.Overrides[0].Color = RGB{1, 0, 0} },
		func(q *OccurrenceColoredAssembly) { q.Assembly.Assembly.Occurrences[0].Transform[0][3]++ },
		func(q *OccurrenceColoredAssembly) {
			for id, bs := range q.Assembly.ProductBodies {
				if len(bs) == 0 {
					continue
				}
				vertices := bs[0].Body.Solid.Vertices()
				for i := range vertices {
					vertices[i].X++
				}
				s, e := brep.NewSolid(vertices, bs[0].Body.Solid.Edges(), bs[0].Body.Solid.Faces())
				if e != nil {
					t.Fatal(e)
				}
				bs[0].Body.Solid = s
				q.Assembly.ProductBodies[id] = bs
				return
			}
		},
	} {
		q, e := ReadOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		mutate(&q)
		out.Reset()
		if e = WriteOccurrenceColoredAssemblyJSON(ctx, &out, q); e == nil || out.Len() != 0 {
			t.Fatal("forged writer input escaped")
		}
	}
	if e := WriteOccurrenceColoredAssemblyJSON(ctx, occurrenceShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	if _, e := ReadIndependentAnalyticHierarchyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("older native tag accepted")
	}
}

type occurrenceCancelReader struct {
	io.Reader
	cancel context.CancelFunc
}

func (r *occurrenceCancelReader) Read(p []byte) (int, error) { r.cancel(); return r.Reader.Read(p) }

type occurrenceShortWriter struct{}

func (occurrenceShortWriter) Write(b []byte) (int, error) { return len(b) - 1, nil }

func TestOccurrenceColorsNativeLive(t *testing.T) {
	_, raw := occurrenceColorsNativeFixture(t)
	m, e := ReadOccurrenceColoredAssemblyJSON(context.Background(), bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteOccurrenceColoredAssemblySource(&out, m.Assembly.Assembly.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	bin := os.Getenv("PLAINSOLID_OCCT_AP242_OCCURRENCE")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_AP242_OCCURRENCE")
	}
	path := filepath.Join(t.TempDir(), "native.step")
	if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, e := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
	if e != nil {
		t.Fatalf("%v: %s", e, result)
	}
	var q struct {
		Volume, Area float64
		Occurrences  []struct {
			Translation [3]float64
			Color       [3]float64
		}
	}
	at := bytes.IndexByte(result, '{')
	if at < 0 || json.Unmarshal(result[at:], &q) != nil {
		t.Fatal(string(result))
	}
	if math.Abs(q.Volume-54) > 1e-9 || math.Abs(q.Area-126) > 1e-9 || len(q.Occurrences) != 3 {
		t.Fatal(q)
	}
	expected := map[[3]float64][3]float64{{10, 0, 0}: {1, 0, 0}, {0, 20, 0}: {0, 0, 1}, {-10, -5, 7}: {0, 1, 0}}
	for _, o := range q.Occurrences {
		want, ok := expected[o.Translation]
		if !ok {
			t.Fatal(q)
		}
		for i, v := range o.Color {
			if math.Abs(v-want[i]) > 1e-12 {
				t.Fatal(q)
			}
		}
	}
}
func FuzzOccurrenceColorsNative(f *testing.F) {
	_, raw := occurrenceColorsNativeFixture(f)
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<17 {
			return
		}
		_, _ = ReadOccurrenceColoredAssemblyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 17, Source: Limits{MaxBytes: 1 << 16, MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16}})
	})
}
