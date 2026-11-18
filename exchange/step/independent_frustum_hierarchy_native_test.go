package step

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestIndependentFrustumHierarchyNative(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	var source bytes.Buffer
	if e := WriteIndependentFrustumHierarchy(ctx, &source, IndependentFrustumHierarchyModel(frustumHierarchyModel(t)), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewReader(source.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteIndependentFrustumHierarchyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	n, e := ReadIndependentFrustumHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = WriteIndependentFrustumHierarchyJSON(ctx, &again, n); e != nil || !bytes.Equal(native.Bytes(), again.Bytes()) {
		t.Fatal("unstable native", e)
	}
	var emitted bytes.Buffer
	if e = WriteIndependentFrustumHierarchySource(ctx, &emitted, n.Assembly.Source, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	var original bytes.Buffer
	if e = WriteIndependentFrustumHierarchySource(ctx, &original, m.Assembly.Source, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(original.Bytes(), emitted.Bytes()) {
		t.Fatal("source IDs changed")
	}
	if _, e = ReadIndependentSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old tag accepted")
	}
	for _, mutation := range []string{"tag", "version", "native", "role", "pose", "support", "definition", "source"} {
		t.Run(mutation, func(t *testing.T) {
			var q independentAnalyticDAGEnvelope
			if e = json.Unmarshal(native.Bytes(), &q); e != nil {
				t.Fatal(e)
			}
			switch mutation {
			case "tag":
				q.Format = independentSphericalHierarchyFormat
			case "version":
				q.Version++
			case "native":
				q.Proof.Members[len(q.Proof.Members)-1].Native = json.RawMessage(`{}`)
			case "role":
				q.Instances[len(q.Instances)-1].Path[0]++
			case "pose":
				q.Instances[len(q.Instances)-1].Transform[0][3]++
			case "support":
				q.Proof.Members[len(q.Proof.Members)-1].Claims = json.RawMessage(`{}`)
			case "definition":
				q.Proof.Members[len(q.Proof.Members)-1].DefinitionID++
			case "source":
				q.Proof.Source = "ISO-10303-21;END-ISO-10303-21;"
			}
			b, _ := json.Marshal(q)
			r, e := ReadIndependentFrustumHierarchyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
			if e == nil || len(r.Products) != 0 {
				t.Fatal("forged envelope accepted")
			}
		})
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 1}, {Source: Limits{MaxSolids: 4}}} {
		if r, e := ReadIndependentFrustumHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), l); e == nil || len(r.Products) != 0 {
			t.Fatal("limit partial result")
		}
	}
	cancel, c := context.WithCancel(ctx)
	c()
	if r, e := ReadIndependentFrustumHierarchyJSON(cancel, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{}); e == nil || len(r.Products) != 0 {
		t.Fatal("cancel")
	}
	// Forged caller-owned instance and support roles fail before any write.
	n.Instances[0].Transform = geom.Affine{}
	var rejected bytes.Buffer
	if e = WriteIndependentFrustumHierarchyJSON(ctx, &rejected, n); e == nil || rejected.Len() != 0 {
		t.Fatal("forged writer")
	}
	if bin := os.Getenv("PLAINSOLID_OCCT_FRUSTUM_HIERARCHY"); bin != "" {
		file := filepath.Join(t.TempDir(), "reloaded.step")
		if e = os.WriteFile(file, emitted.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		raw, e := exec.Command(bin, "read", file).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(raw))
		}
		at := bytes.LastIndex(raw, []byte("{"))
		var q struct{ Solids int }
		if at < 0 || json.Unmarshal(raw[at:], &q) != nil || q.Solids != 9 {
			t.Fatal(string(raw))
		}
	}
}

func FuzzIndependentFrustumHierarchyNative(f *testing.F) {
	ctx := context.Background()
	var source bytes.Buffer
	if e := WriteIndependentFrustumHierarchy(ctx, &source, IndependentFrustumHierarchyModel(frustumHierarchyModel(f)), Limits{}, 1e-9); e != nil {
		f.Fatal(e)
	}
	m, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewReader(source.Bytes()), Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteIndependentFrustumHierarchyJSON(ctx, &native, m); e != nil {
		f.Fatal(e)
	}
	f.Add(native.Bytes())
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"Format":"plainsolid-step-independent-frustum-hierarchy","Version":1}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadIndependentFrustumHierarchyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 20, Source: Limits{MaxBytes: 1 << 20, MaxEntities: 4096}})
		if e != nil {
			if len(m.Products) != 0 || len(m.Instances) != 0 {
				t.Fatal("partial rejection")
			}
			return
		}
		if len(m.Products) < 4 || len(m.Products) > 5 {
			t.Fatal("family count")
		}
	})
}
