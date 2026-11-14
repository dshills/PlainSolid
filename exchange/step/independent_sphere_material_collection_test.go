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

func TestIndependentSphereMaterials(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-sphere-materials.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentSphereMaterials(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Members) != 7 || m.Members[6].Sphere == nil {
		t.Fatal("kind")
	}
	original, e := os.ReadFile("testdata/occt-independent-sphere-product.step")
	if e != nil {
		t.Fatal(e)
	}
	if solo, e := ReadIndependentSphereMaterials(ctx, bytes.NewReader(original), Limits{}, 1e-9); e != nil || len(solo.Members) != 1 {
		t.Fatal("original", e)
	}
	var rejected bytes.Buffer
	if e = WriteIndependentSphereMaterials(ctx, &rejected, []*File{nil}, Limits{}, 1e-9); e == nil || rejected.Len() != 0 {
		t.Fatal("atomic")
	}
	q := m.Members[6]
	if q.Sphere.Radius != 2 || q.Sphere.Frame.Origin().X != 1 || q.SourceProductID == 0 {
		t.Fatal("geometry")
	}
	if len(q.Sphere.NativeFaceSourceIDs) != len(q.Solid.Faces()) {
		t.Fatal("native ancestry")
	}
	for _, id := range q.Sphere.NativeFaceSourceIDs {
		if id != q.Sphere.SourceFaceID {
			t.Fatal("face owner")
		}
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, q.Solid); e != nil {
		t.Fatal(e)
	}
	if _, e = trimmed.ReadJSON(&native, trimmed.JSONLimits{}); e != nil {
		t.Fatal(e)
	}
	var sources []*File
	for _, p := range m.Members {
		sources = append(sources, p.Source)
	}
	var out bytes.Buffer
	if e = WriteIndependentSphereMaterials(ctx, &out, sources, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if n, e := ReadIndependentSphereMaterials(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9); e != nil || len(n.Members) != 7 {
		t.Fatal(e)
	}
	if _, e = ReadIndependentCylinderMaterials(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old widened")
	}
	for _, kind := range []string{"pole", "radius", "sense", "loop", "owner"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		id := q.Sphere.SourcePointID
		switch kind {
		case "radius":
			id = q.Sphere.SourceSurfaceID
		case "sense":
			id = q.Sphere.SourceFaceID
		case "loop":
			id = q.Sphere.SourceLoopID
		case "owner":
			id = q.Sphere.SourceVertexID
		}
		en := f.Entities[id]
		r := &en.Records[0]
		switch kind {
		case "pole":
			r.Args[1].Items[2].Text = "1.0000000000000002"
		case "radius":
			r.Args[2].Text = "2.0000000000000004"
		case "sense":
			r.Args[3].Text = "F"
		case "loop":
			r.Type = "EDGE_LOOP"
		case "owner":
			r.Args[1].Ref = q.Sphere.SourceSurfaceID
		}
		f.Entities[id] = en
		b, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		if n, e := ReadIndependentSphereMaterials(ctx, bytes.NewReader(b), Limits{}, 1e-9); e == nil || len(n.Members) != 0 {
			t.Fatal(kind)
		}
	}
	if n, e := ReadIndependentSphereMaterials(ctx, bytes.NewReader(raw), Limits{MaxSolids: 6}, 1e-9); e == nil || len(n.Members) != 0 {
		t.Fatal("limit")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadIndependentSphereMaterials(cancelled, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("cancel")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_SPHERE_MATERIALS"); oracle != "" {
		for i, b := range [][]byte{raw, out.Bytes()} {
			p := filepath.Join(t.TempDir(), "model.step")
			if e = os.WriteFile(p, b, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := exec.Command(oracle, "read", p).CombinedOutput()
			if e != nil {
				t.Fatal(i, e, string(data))
			}
			at := bytes.LastIndex(data, []byte("{"))
			var v struct {
				Solids, Faces int
				Volume, Area  float64
			}
			if at < 0 || json.Unmarshal(data[at:], &v) != nil || v.Solids != 7 || v.Faces != 27 || math.Abs(v.Volume-(24+(135.25+32.0/3)*math.Pi)) > 1e-6 || math.Abs(v.Area-(522.168698195557+56*math.Pi)) > 1e-6 {
				t.Fatal(string(data))
			}
		}
	}
}
func FuzzIndependentSphereMaterials(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-independent-sphere-product.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		m, e := ReadIndependentSphereMaterials(context.Background(), bytes.NewReader(b), Limits{MaxEntities: 1024, MaxValues: 16384, MaxSolids: 7}, 1e-9)
		if e != nil && len(m.Members) != 0 {
			t.Fatal("partial")
		}
	})
}
