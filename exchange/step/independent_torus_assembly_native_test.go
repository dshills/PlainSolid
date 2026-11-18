package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func torusAssemblyNativeFixture(t testing.TB) (IndependentTorusAssembly, []byte) {
	t.Helper()
	raw, e := os.ReadFile("testdata/independent-torus-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentTorusAssembly(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteIndependentTorusAssemblyJSON(context.Background(), &b, m); e != nil {
		t.Fatal(e)
	}
	return m, b.Bytes()
}
func TestIndependentTorusAssemblyNative(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m, raw := torusAssemblyNativeFixture(t)
	n, e := ReadIndependentTorusAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteIndependentTorusAssemblyJSON(ctx, &out, n); e != nil || !bytes.Equal(raw, out.Bytes()) {
		t.Fatal("canonical reload", e)
	}
	if len(n.Products) != 2 || len(n.Assembly.Occurrences) != 3 {
		t.Fatal("shared local count")
	}
	for id, p := range m.Products {
		q := n.Products[id]
		if p.SourceRootID != q.SourceRootID || p.SourceBodyID != q.SourceBodyID {
			t.Fatal("source ownership")
		}
	}
	for _, read := range []func(context.Context, io.Reader, IndependentMaterialJSONLimits) (IndependentExactMaterialCollection, error){ReadIndependentExactMaterialsJSON, ReadIndependentCylinderMaterialsJSON, ReadIndependentSphereMaterialsJSON, ReadIndependentToroidalMaterialsJSON} {
		if _, e = read(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("old tag widened")
		}
	}
	for _, kind := range []string{"tag", "version", "source", "units", "root", "occurrence-child", "occurrence-pose", "product", "member-id", "frame", "seam", "native", "member-source", "omit", "trailing", "unknown"} {
		var q independentTorusAssemblyEnvelope
		if e = json.Unmarshal(raw, &q); e != nil {
			t.Fatal(e)
		}
		switch kind {
		case "tag":
			q.Format = independentTorusNativeFormat
		case "version":
			q.Version++
		case "source":
			q.Source = q.Source[:len(q.Source)/2]
		case "units":
			q.Assembly.LengthUnit = 1
		case "root":
			q.Assembly.Roots[0]++
		case "occurrence-child":
			q.Assembly.Occurrences[1].Child = q.Assembly.Roots[0]
		case "occurrence-pose":
			q.Assembly.Occurrences[1].Transform[0][3]++
		case "product":
			q.Assembly.Products[1].Name = "forged"
		case "member-id":
			q.Members[0].DefinitionID++
		case "frame":
			for i := range q.Members {
				if q.Members[i].TorusFrame != nil {
					q.Members[i].TorusFrame[0][3]++
				}
			}
		case "native":
			q.Members[0].Native = json.RawMessage(`{}`)
		case "omit":
			q.Members = q.Members[:1]
		case "member-source", "seam":
			changed := false
			for i := range q.Members {
				var claim map[string]any
				if json.Unmarshal(q.Members[i].Claims, &claim) != nil {
					t.Fatal("claim")
				}
				if kind == "member-source" {
					claim["SourceBodyID"] = float64(1)
					changed = true
				} else if torus, ok := claim["Torus"].(map[string]any); ok {
					torus["SourceEdgeIDs"] = []int{1, 1}
					changed = true
				}
				if changed {
					q.Members[i].Claims, _ = json.Marshal(claim)
					break
				}
			}
			if !changed {
				t.Fatal("mutation", kind)
			}
		}
		b, _ := json.Marshal(q)
		if kind == "trailing" {
			b = append(b, []byte(` {}`)...)
		}
		if kind == "unknown" {
			b = append([]byte(`{"Unknown":1,`), b[1:]...)
		}
		if got, e := ReadIndependentTorusAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil || len(got.Products) != 0 || got.Assembly.Source != nil {
			t.Fatal("accepted/partial", kind)
		}
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 1}, {MaxBytes: -1}, {MaxBytes: 17 << 20}, {Source: Limits{MaxSolids: 2}}, {Source: Limits{MaxEntities: 10}}, {Source: Limits{MaxFaces: 6}}, {Source: Limits{MaxDepth: 2}}} {
		if _, e := ReadIndependentTorusAssemblyJSON(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit", l)
		}
	}
	cancel, c := context.WithCancel(ctx)
	c()
	if _, e := ReadIndependentTorusAssemblyJSON(cancel, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	for _, kind := range []string{"pose", "native", "frame", "source-role", "source", "cancel"} {
		n, e := ReadIndependentTorusAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		switch kind {
		case "pose":
			n.Assembly.Occurrences[1].Transform[0][3]++
		case "source":
			for _, p := range n.Products {
				if p.Torus != nil {
					id := p.Torus.SourceSurfaceID
					en := n.Assembly.Source.Entities[id]
					en.Records[0].Args[2].Text = "1."
					n.Assembly.Source.Entities[id] = en
				}
			}
		case "frame":
			for id, p := range n.Products {
				if p.Torus != nil {
					q := *p.Torus
					q.Frame, _ = geom.NewFrame(geom.Vec3{X: 99}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
					p.Torus = &q
					n.Products[id] = p
				}
			}
		case "source-role":
			for id, p := range n.Products {
				p.SourceRootID++
				n.Products[id] = p
				break
			}
		case "native":
			ids := []int64{}
			for id := range n.Products {
				ids = append(ids, id)
			}
			p := n.Products[ids[0]]
			p.Solid = n.Products[ids[1]].Solid
			n.Products[ids[0]] = p
		}
		var rejected bytes.Buffer
		cctx := ctx
		if kind == "cancel" {
			cctx = cancel
		}
		if e = WriteIndependentTorusAssemblyJSON(cctx, &rejected, n); e == nil || rejected.Len() != 0 {
			t.Fatal("atomic", kind)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY"); oracle != "" {
		var source bytes.Buffer
		if e = WriteIndependentTorusAssemblySource(ctx, &source, n.Assembly.Source, Limits{}, 1e-9); e != nil {
			t.Fatal(e)
		}
		for _, raw := range [][]byte{[]byte(mustTorusAssemblyEnvelope(t, raw).Source), source.Bytes()} {
			p := filepath.Join(t.TempDir(), "native-assembly.step")
			if e = os.WriteFile(p, raw, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := exec.Command(oracle, "read", p).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			at := bytes.LastIndex(data, []byte("{"))
			var q struct {
				Solids, Faces int
				Area, Volume  float64
				Bounds        [6]float64
			}
			if at < 0 || json.Unmarshal(data[at:], &q) != nil {
				t.Fatal(string(data))
			}
			if q.Solids != 3 || q.Faces != 8 || math.Abs(q.Volume-(24+12*math.Pi*math.Pi)) > 1e-8 || math.Abs(q.Area-(52+24*math.Pi*math.Pi)) > 1e-8 {
				t.Fatal(q)
			}
			for i, w := range [6]float64{-10, -4, -2, 25, 25, 6} {
				if math.Abs(q.Bounds[i]-w) > 2e-7 {
					t.Fatal(q.Bounds)
				}
			}
		}
	}
}
func mustTorusAssemblyEnvelope(t testing.TB, b []byte) independentTorusAssemblyEnvelope {
	t.Helper()
	var q independentTorusAssemblyEnvelope
	if e := json.Unmarshal(b, &q); e != nil {
		t.Fatal(e)
	}
	return q
}
func FuzzIndependentTorusAssemblyNative(f *testing.F) {
	_, b := torusAssemblyNativeFixture(f)
	f.Add(b)
	f.Add([]byte(`{"Format":"plainsolid-step-independent-torus-assembly","Version":1}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		m, e := ReadIndependentTorusAssemblyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 20, Source: Limits{MaxEntities: 1024, MaxValues: 16384, MaxSolids: 3, MaxFaces: 20, MaxVertices: 100, MaxEdgeUses: 100, MaxDepth: 16}})
		if e != nil && (len(m.Products) != 0 || m.Assembly.Source != nil) {
			t.Fatal("partial")
		}
	})
}
