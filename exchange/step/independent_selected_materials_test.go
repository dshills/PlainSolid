package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"testing"
)

func TestSelectedIndependentMaterials(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-selected-materials.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	roots := []int64{}
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "SHAPE_DEFINITION_REPRESENTATION" {
			roots = append(roots, id)
		}
	}
	if len(roots) != 8 {
		t.Fatal("fixture")
	}
	selected := []int64{roots[6], roots[0]}
	m, e := ReadSelectedIndependentMaterials(ctx, bytes.NewReader(raw), selected, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Members) != 2 || len(m.OmittedRootIDs) != 6 || m.Members[0].SourceRootID != roots[0] || m.Members[1].SourceRootID != roots[6] {
		t.Fatal("source order")
	}
	var sources []*File
	for _, p := range m.Members {
		sources = append(sources, p.Source)
		var native bytes.Buffer
		if e = trimmed.WriteJSON(&native, p.Solid); e != nil {
			t.Fatal(e)
		}
		if _, e = trimmed.ReadJSON(&native, trimmed.JSONLimits{}); e != nil {
			t.Fatal(e)
		}
	}
	var out bytes.Buffer
	if e = WriteIndependentSphereMaterials(ctx, &out, sources, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadIndependentSphereMaterials(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("whole accepted")
	}
	for _, ids := range [][]int64{nil, {roots[0], roots[0]}, {999999}, {roots[7]}} {
		if v, e := ReadSelectedIndependentMaterials(ctx, bytes.NewReader(raw), ids, Limits{}, 1e-9); e == nil || len(v.Members) != 0 {
			t.Fatal("selection", ids)
		}
	}
	for _, kind := range []string{"borrow", "cycle", "orphan"} {
		q, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		sdr := q.Entities[roots[7]].Records[0]
		repID := sdr.Args[1].Ref
		rep := q.Entities[repID]
		bodyID := rep.Records[0].Args[1].Items[1].Ref
		switch kind {
		case "borrow":
			rep.Records[0].Args[1].Items[1].Ref = m.Members[0].SourceBodyID
			q.Entities[repID] = rep
		case "cycle":
			en := q.Entities[bodyID]
			en.Records[0].Args[1].Ref = bodyID
			q.Entities[bodyID] = en
		case "orphan":
			id := int64(99999)
			q.Order = append(q.Order, id)
			q.Entities[id] = Entity{ID: id, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0"}, {Kind: Number, Text: "0"}, {Kind: Number, Text: "0"}}}}}}}
		}
		b, e := mappedPlanarSourceBytes(q)
		if e != nil {
			t.Fatal(e)
		}
		if v, e := ReadSelectedIndependentMaterials(ctx, bytes.NewReader(b), selected, Limits{}, 1e-9); e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}
	if _, e = ReadSelectedIndependentMaterials(ctx, bytes.NewReader(raw), selected, Limits{MaxSolids: 7}, 1e-9); e == nil {
		t.Fatal("omitted root budget")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadSelectedIndependentMaterials(cancelled, bytes.NewReader(raw), selected, Limits{}, 1e-9); e == nil {
		t.Fatal("cancel")
	}
	for _, l := range []Limits{{MaxBytes: 1}, {MaxEntities: 100}, {MaxFaces: 1}, {MaxVertices: 1}, {MaxEdgeUses: 1}} {
		if v, e := ReadSelectedIndependentMaterials(ctx, bytes.NewReader(raw), selected, l, 1e-9); e == nil || len(v.Members) != 0 {
			t.Fatal("bounded")
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_SELECTED_MATERIALS"); oracle != "" {
		for i, b := range [][]byte{raw, out.Bytes()} {
			p := filepath.Join(t.TempDir(), "model.step")
			if e = os.WriteFile(p, b, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := exec.Command(oracle, "read", p).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			at := bytes.LastIndex(data, []byte("{"))
			var q struct {
				Solids int
				Volume float64
			}
			if at < 0 || json.Unmarshal(data[at:], &q) != nil {
				t.Fatal(string(data))
			}
			if i == 0 && (q.Solids != 8 || math.Abs(q.Volume-(24+(135.25+32.0/3)*math.Pi+6*math.Pi*math.Pi)) > 1e-6) {
				t.Fatal(string(data))
			}
			if i == 1 && (q.Solids != 2 || math.Abs(q.Volume-(24+32*math.Pi/3)) > 1e-6) {
				t.Fatal(string(data))
			}
		}
	}
}
func FuzzSelectedIndependentMaterials(f *testing.F) {
	b, e := os.ReadFile("testdata/independent-selected-materials.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		v, e := ReadSelectedIndependentMaterials(context.Background(), bytes.NewReader(b), []int64{3}, Limits{MaxEntities: 1024, MaxValues: 16384, MaxSolids: 8}, 1e-9)
		if e != nil && len(v.Members) != 0 {
			t.Fatal("partial")
		}
	})
}
