package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"strings"
	"testing"
)

func frustumHierarchyModel(t testing.TB) IndependentTorusAssemblyModel {
	t.Helper()
	m := IndependentTorusAssemblyModel(sphericalHierarchyModel(t))
	raw, e := os.ReadFile("testdata/wrapped-circular-frustum.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m.Products = append(m.Products, IndependentTorusAssemblyPart{ID: "frustum", Name: "original rational taper", Source: f})
	for i, x := range []float64{30, -30} {
		id := "draft-a"
		if i == 1 {
			id = "draft-b"
		}
		pose := geom.Affine{{1, 0, 0, x}, {0, 1, 0, 20}, {0, 0, 1, 0}}
		if i == 1 {
			pose = geom.Affine{{0, 0, 1, x}, {0, 1, 0, 20}, {-1, 0, 0, 0}}
		}
		m.Occurrences = append(m.Occurrences, AssemblyInstance{ID: id, Parent: "root", Child: "frustum", Transform: pose})
	}
	return m
}
func TestIndependentFrustumHierarchy(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	m := frustumHierarchyModel(t)
	raw, e := os.ReadFile("testdata/wrapped-circular-frustum.step")
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteIndependentFrustumHierarchy(ctx, &out, IndependentFrustumHierarchyModel(m), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	got, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Products) != 5 || len(got.Instances) != 9 {
		t.Fatal("counts", len(got.Products), len(got.Instances))
	}
	if _, e = ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9); e == nil {
		t.Fatal("legacy accepted frustum")
	}

	var fr *WrappedFrustumBody
	for _, p := range got.Products {
		if p.Frustum != nil {
			fr = p.Frustum
		}
	}
	if fr == nil {
		t.Fatal("lost frustum")
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, fr.Solid); e != nil {
		t.Fatal(e)
	}
	restored, e := trimmed.ReadJSON(bytes.NewReader(native.Bytes()), trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	mass, e := restored.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	if bin := os.Getenv("PLAINSOLID_OCCT_FRUSTUM_HIERARCHY"); bin != "" {
		for _, data := range [][]byte{raw, out.Bytes()} {
			file := filepath.Join(t.TempDir(), "frustum.step")
			if e = os.WriteFile(file, data, 0600); e != nil {
				t.Fatal(e)
			}
			result, e := exec.Command(bin, "read", file).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(result))
			}
			at := bytes.LastIndex(result, []byte("{"))
			var q struct {
				Area, Volume float64
				Solids       int
			}
			if at < 0 || json.Unmarshal(result[at:], &q) != nil {
				t.Fatal(string(result))
			}
			wantA, wantV, count := mass.Area, mass.Volume, 1
			if len(data) != len(raw) {
				wantA = 52 + 24*math.Pi*math.Pi + 72*math.Pi + 2*mass.Area
				wantV = 24 + 12*math.Pi*math.Pi + (24+64./3)*math.Pi + 2*mass.Volume
				count = 9
			}
			if q.Solids != count || math.Abs(q.Area-wantA) > 1e-5 || math.Abs(q.Volume-wantV) > 1e-5 {
				t.Fatalf("oracle %+v expected %g %g %d", q, wantA, wantV, count)
			}
		}
	}
	// Keep unsupported detached geometry and invalid source orientation atomic.
	detached := strings.Replace(out.String(), "ENDSEC;\nEND-ISO-10303-21;", "#99999=CARTESIAN_POINT('',(9.,9.,9.));\nENDSEC;\nEND-ISO-10303-21;", 1)
	if detached == out.String() {
		detached = strings.Replace(out.String(), "ENDSEC;END-ISO-10303-21;", "#99999=CARTESIAN_POINT('',(9.,9.,9.));\nENDSEC;END-ISO-10303-21;", 1)
	}
	if detached == out.String() {
		t.Fatal("detached mutation not applied")
	}
	if x, e := ReadIndependentFrustumHierarchy(ctx, strings.NewReader(detached), Limits{}, 1e-9); e == nil || len(x.Products) != 0 {
		t.Fatal("detached geometry accepted")
	}
	// The original source support and coedge graph remain authoritative.
	bad := strings.Replace(out.String(), "B_SPLINE_SURFACE_WITH_KNOTS", "BEZIER_SURFACE", 1)
	if x, e := ReadIndependentFrustumHierarchy(ctx, strings.NewReader(bad), Limits{}, 1e-9); e == nil || len(x.Products) != 0 {
		t.Fatal("malformed support admitted")
	}
	cancel, c := context.WithCancel(ctx)
	c()
	if _, e = ReadIndependentFrustumHierarchy(cancel, bytes.NewReader(out.Bytes()), Limits{}, 1e-9); e == nil {
		t.Fatal("cancellation")
	}
	if _, e = ReadIndependentFrustumHierarchy(ctx, bytes.NewReader(out.Bytes()), Limits{MaxSolids: 4}, 1e-9); e == nil {
		t.Fatal("solid budget")
	}
	var re bytes.Buffer
	if e = WriteIndependentFrustumHierarchySource(ctx, &re, got.Assembly.Source, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadIndependentFrustumHierarchy(ctx, bytes.NewReader(re.Bytes()), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
}
