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
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestIndependentExactMaterialsNative(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/occt-independent-exact-materials.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentExactMaterials(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteIndependentExactMaterialsJSON(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	p, e := ReadIndependentExactMaterialsJSON(ctx, bytes.NewReader(out.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = WriteIndependentExactMaterialsJSON(ctx, &again, p); e != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
		t.Fatal("stable", e)
	}
	source, e := mappedPlanarSourceBytes(m.Source)
	if e != nil {
		t.Fatal(e)
	}
	restored, e := mappedPlanarSourceBytes(p.Source)
	if e != nil || !bytes.Equal(source, restored) {
		t.Fatal("source", e)
	}
	for i, b := range p.Members {
		a := m.Members[i]
		if b.SourceRootID != a.SourceRootID || b.SourceBodyID != a.SourceBodyID || b.SourceProductID != a.SourceProductID || b.SourceDefinitionID != a.SourceDefinitionID || b.SourceRepresentationID != a.SourceRepresentationID || !reflect.DeepEqual(b.Source.Order, a.Source.Order) {
			t.Fatal("ancestry")
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_MATERIALS"); oracle != "" {
		sources := []*File{}
		for _, member := range p.Members {
			sources = append(sources, member.Source)
		}
		var exported bytes.Buffer
		if e := WriteIndependentExactMaterials(ctx, &exported, sources, Limits{}, 1e-9); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "native-reload.step")
		if e := os.WriteFile(path, exported.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		b, e := exec.Command(oracle, "read", path).CombinedOutput()
		if e != nil {
			t.Fatal(string(b), e)
		}
		var q struct {
			Solids int
			Volume float64
		}
		at := bytes.LastIndex(b, []byte("{"))
		if at < 0 || json.Unmarshal(b[at:], &q) != nil || q.Solids != 4 || math.Abs(q.Volume-(24+111.25*math.Pi)) > 1e-7 {
			t.Fatal(string(b))
		}
	}

	for _, name := range []string{"kind", "native", "member", "unit", "source", "tolerance", "version", "claims"} {
		var q independentMaterialEnvelope
		if e = json.Unmarshal(out.Bytes(), &q); e != nil {
			t.Fatal(e)
		}
		switch name {
		case "kind":
			q.Format = "wrong"
		case "native":
			q.Members[1].Native = json.RawMessage(`{}`)
		case "member":
			q.Members[0], q.Members[1] = q.Members[1], q.Members[0]
		case "unit":
			q.LengthUnit = 1
		case "source":
			q.Source = ""
		case "tolerance":
			q.Tolerance = -1
		case "version":
			q.Version++
		case "claims":
			var c map[string]json.RawMessage
			json.Unmarshal(q.Members[0].Claims, &c)
			c["SourceBodyID"] = json.RawMessage(`999`)
			q.Members[0].Claims, _ = json.Marshal(c)
		}
		bad, _ := json.Marshal(q)
		v, e := ReadIndependentExactMaterialsJSON(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{})
		if e == nil || len(v.Members) != 0 {
			t.Fatal("forged", name)
		}
	}
	for _, limits := range []IndependentMaterialJSONLimits{{MaxBytes: int64(out.Len() - 1)}, {Source: Limits{MaxEntities: 607}}, {Source: Limits{MaxSolids: 3}}, {Source: Limits{MaxFaces: 1}}, {Source: Limits{MaxBytes: 80}}} {
		if v, e := ReadIndependentExactMaterialsJSON(ctx, bytes.NewReader(out.Bytes()), limits); e == nil || len(v.Members) != 0 {
			t.Fatal("limit")
		}
	}
	for _, tail := range []string{` {}`, ` garbage`} {
		if _, e := ReadIndependentExactMaterialsJSON(ctx, bytes.NewReader(append(append([]byte(nil), out.Bytes()...), tail...)), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("trailing")
		}
	}
	for _, which := range []string{"body", "native", "ellipse-ancestry", "source-coverage"} {
		q := m
		q.Members = append([]IndependentExactMaterial(nil), m.Members...)
		switch which {
		case "body":
			q.Members[0].SourceBodyID++
		case "native":
			q.Members[0].Solid = q.Members[1].Solid
		case "source-coverage":
			q.Members[0].Source = q.Members[1].Source
		case "ellipse-ancestry":
			f := *q.Members[1].Elliptical
			p := *f.Tilted
			p.NativeFaceSourceIDs[0]++
			f.Tilted = &p
			q.Members[1].Elliptical = &f
		}
		var reject bytes.Buffer
		if e := WriteIndependentExactMaterialsJSON(ctx, &reject, q); e == nil || reject.Len() != 0 {
			t.Fatal("stale", which)
		}
	}
	for _, which := range []string{"nil", "cycle"} {
		q := m
		q.Source = nil
		if which == "cycle" {
			items := make([]Value, 1)
			v := Value{Kind: Aggregate, Items: items}
			items[0] = v
			q.Source = &File{Order: []int64{1}, Entities: map[int64]Entity{1: {ID: 1, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{v}}}}}}
		}
		var reject bytes.Buffer
		if e := WriteIndependentExactMaterialsJSON(ctx, &reject, q); e == nil || reject.Len() != 0 {
			t.Fatal("unsafe source", which)
		}
	}
	during, cancelDuring := context.WithCancel(ctx)
	if _, e := ReadIndependentExactMaterialsJSON(during, &independentCancelReader{Reader: bytes.NewReader(out.Bytes()), cancel: cancelDuring}, IndependentMaterialJSONLimits{}); e != context.Canceled {
		t.Fatal("during read", e)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadIndependentExactMaterialsJSON(canceled, bytes.NewReader(out.Bytes()), IndependentMaterialJSONLimits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	var reject bytes.Buffer
	if e = WriteIndependentExactMaterialsJSON(canceled, &reject, m); !errors.Is(e, context.Canceled) || reject.Len() != 0 {
		t.Fatal(e)
	}
	if e = WriteIndependentExactMaterialsJSON(ctx, independentShortWriter{}, m); e != io.ErrShortWrite {
		t.Fatal(e)
	}
}

type independentShortWriter struct{}

func (independentShortWriter) Write(b []byte) (int, error) { return len(b) - 1, nil }
func FuzzIndependentExactMaterialsNative(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-independent-exact-materials.step")
	if e != nil {
		f.Fatal(e)
	}
	m, e := ReadIndependentExactMaterials(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteIndependentExactMaterialsJSON(context.Background(), &b, m); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		v, e := ReadIndependentExactMaterialsJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 20, Source: Limits{MaxEntities: 1024, MaxSolids: 4}})
		if e != nil && len(v.Members) != 0 {
			t.Fatal("partial")
		}
	})
}

type independentCancelReader struct {
	io.Reader
	cancel context.CancelFunc
}

func (r *independentCancelReader) Read(b []byte) (int, error) {
	n, e := r.Reader.Read(b)
	r.cancel()
	return n, e
}
