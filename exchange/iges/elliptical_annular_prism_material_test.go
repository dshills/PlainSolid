package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestEllipticalAnnularPrismMaterial(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/elliptical-annular-prism-topology.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadEllipticalAnnularPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Solid.Faces()) != 10 || m.NativeFaceSourceIDs != ([10]int{51, 49, 45, 45, 45, 45, 47, 47, 47, 47}) || m.SourceRimCurveIDs != ([2][2]int{{23, 19}, {29, 25}}) {
		t.Fatal(m.NativeFaceSourceIDs)
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
		t.Fatal(e)
	}
	loaded, e := trimmed.ReadJSON(&native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []trimmed.Solid{m.Solid, loaded} {
		proof, e := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !proof.Complete {
			t.Fatal(e)
		}
		for _, u := range proof.Uses {
			if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
				t.Fatal("nonzero exact discrepancy")
			}
		}
		mass, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil || math.Abs(mass.Volume-32*math.Pi) > 1e-6 || math.Abs(mass.Area-181.07172572228598) > 1e-6 || mass.Centroid.Distance(geom.Vec3{Z: 5}) > 1e-7 {
			t.Fatal(mass, e)
		}
	}
	mesh, e := loaded.Tessellate(ctx, parametric.TessellationOptions{Deflection: .1, MaxSubdivisions: 256, MaxTriangles: 1000000})
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	var envelope bytes.Buffer
	if e = WriteEllipticalAnnularPrismMaterialJSON(ctx, &envelope, m); e != nil {
		t.Fatal(e)
	}
	q, e := ReadEllipticalAnnularPrismMaterialJSON(ctx, bytes.NewReader(envelope.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = WriteEllipticalAnnularPrismMaterialJSON(ctx, &again, q); e != nil || !bytes.Equal(envelope.Bytes(), again.Bytes()) {
		t.Fatal("unstable native", e)
	}

	if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_ELLIPTICAL_PRISM_TOPOLOGY"); oracle != "" {
		var out bytes.Buffer
		if e = WriteEllipticalAnnularPrismTopology(ctx, &out, q.Topology.Descriptor, q.Topology.LengthUnit); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "reloaded.igs")
		if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		b, e := exec.Command(oracle, "read", path).CombinedOutput()
		if e != nil {
			t.Fatalf("%s %v", b, e)
		}
		var measure struct {
			Faces, Solids int
			Volume, Area  float64
		}
		at := bytes.LastIndex(b, []byte("{"))
		if at < 0 || json.Unmarshal(bytes.TrimSpace(b[at:]), &measure) != nil || measure.Faces != 4 || measure.Solids != 1 || math.Abs(measure.Volume-32*math.Pi) > 1e-8 || math.Abs(measure.Area-181.07172572228598) > 1e-7 {
			t.Fatalf("%s", b)
		}
	}
	for _, field := range []string{"Version", "Native", "Faces", "Rims", "SourceRims", "Tolerance", "Topology"} {
		var v map[string]any
		if json.Unmarshal(envelope.Bytes(), &v) != nil {
			t.Fatal("json")
		}
		v[field] = 0
		b, _ := json.Marshal(v)
		if _, e = ReadEllipticalAnnularPrismMaterialJSON(ctx, bytes.NewReader(b), Limits{}); e == nil {
			t.Fatal("tamper", field)
		}
	}
	stale := m
	stale.NativeFaceSourceIDs[0] = 31
	var empty bytes.Buffer
	if WriteEllipticalAnnularPrismMaterialJSON(ctx, &empty, stale) == nil || empty.Len() != 0 {
		t.Fatal("stale writer")
	}
	for _, tol := range []float64{0, -1, math.NaN()} {
		if _, e = ReadEllipticalAnnularPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, tol); e == nil {
			t.Fatal("tolerance")
		}
	}
	vendor, e := os.ReadFile("testdata/occt-vendor-elliptical-prism.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEllipticalAnnularPrismMaterial(ctx, bytes.NewReader(vendor), Limits{}, 1); e == nil {
		t.Fatal("tolerance repaired source")
	}
}
func FuzzEllipticalAnnularPrismMaterialNative(f *testing.F) {
	raw, e := os.ReadFile("testdata/elliptical-annular-prism-topology.igs")
	if e != nil {
		f.Fatal(e)
	}
	m, e := ReadEllipticalAnnularPrismMaterial(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteEllipticalAnnularPrismMaterialJSON(context.Background(), &b, m); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		_, _ = ReadEllipticalAnnularPrismMaterialJSON(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536})
	})
}
