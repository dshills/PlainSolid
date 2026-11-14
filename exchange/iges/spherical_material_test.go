package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestSphericalMaterial(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/spherical-material.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSphericalMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Solid.Faces()) != 1 || m.SourceFaceID != 5 || m.SourceMeridianCurveID != 23 {
		t.Fatal("source ancestry")
	}
	proof, e := m.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal(e)
	}
	for _, u := range proof.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Cmp(new(big.Rat).SetFloat64(1e-18)) > 0 {
			t.Fatal("native continuous agreement exceeds bound")
		}
	}
	mass, e := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(mass.Volume-32*math.Pi/3) > 1e-7 || math.Abs(mass.Area-16*math.Pi) > 1e-7 || mass.Centroid.Length() > 1e-8 {
		t.Fatal(mass, e)
	}
	// Analytic native evaluation is numerical; source support and endpoints above
	// are authenticated exactly, while these checks qualify the native convention.
	surface := m.Solid.Faces()[0].Surface
	for _, v := range []float64{-math.Pi / 2, -.7, 0, .9, math.Pi / 2} {
		a, e := surface.Eval(0, v)
		if e != nil {
			t.Fatal(e)
		}
		b, e := surface.Eval(2*math.Pi, v)
		if e != nil || a.Distance(b) > 1e-12 || math.Abs(a.Length()-2) > 1e-12 {
			t.Fatal("native seam/radius", a, b, e)
		}
	}
	verts := m.Solid.Vertices()
	if len(verts) != 2 || verts[0] != (geom.Vec3{Z: -2}) || verts[1] != (geom.Vec3{Z: 2}) {
		t.Fatal("native poles", verts)
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-spherical-material.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadSphericalMaterial(ctx, bytes.NewReader(vendor), Limits{}, 1e-9); e == nil {
		t.Fatal("rounded source accepted")
	}
	var plain bytes.Buffer
	if e = trimmed.WriteJSON(&plain, m.Solid); e != nil {
		t.Fatal(e)
	}
	if _, e = trimmed.ReadJSON(&plain, trimmed.JSONLimits{}); e != nil {
		t.Fatal(e)
	}
	mesh, e := m.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .1, AnalyticBound: true, MaxSubdivisions: 128, MaxTriangles: 1000000})
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	var native, retained, output bytes.Buffer
	if e = WriteSphericalMaterialJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	p, e := ReadSphericalMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteSphericalMaterialSource(ctx, &retained, p); e != nil || !bytes.Equal(raw, retained.Bytes()) {
		t.Fatal("source changed", e)
	}
	if e = WriteSphericalMaterial(ctx, &output, p.Descriptor, p.LengthUnit, 1e-9); e != nil {
		t.Fatal(e)
	}
	for _, d := range []SphericalMaterialDescriptor{{0}, {-1}, {math.Inf(1)}} {
		var b bytes.Buffer
		if e = WriteSphericalMaterial(ctx, &b, d, .001, 1e-9); e == nil || b.Len() != 0 {
			t.Fatal("invalid ring material")
		}
	}
	for _, mutation := range []struct {
		prefix string
		field  int
		value  string
	}{{"196,", 2, "1"}, {"124,", 6, "1"}, {"508,", 4, "2"}, {"504,", 4, "9"}, {"514,", 3, "0"}, {"100,", 4, "9"}} {
		bad := append([]byte(nil), raw...)
		changed := false
		for _, line := range bytes.Split(bad, []byte("\n")) {
			if len(line) != 80 || line[72] != 'P' || !bytes.HasPrefix(line, []byte(mutation.prefix)) {
				continue
			}
			v := strings.Split(strings.TrimSpace(string(line[:64])), ",")
			suffix := ""
			if strings.HasSuffix(v[mutation.field], ";") {
				suffix = ";"
			}
			v[mutation.field] = mutation.value + suffix
			text := strings.Join(v, ",")
			if len(text) > 64 {
				t.Fatal("overflow")
			}
			copy(line[:64], text+strings.Repeat(" ", 64-len(text)))
			changed = true
			break
		}
		if !changed {
			t.Fatal("missing mutation")
		}
		if _, e = ReadSphericalMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("wrong source seam/support", mutation)
		}
	}
	var q sphericalMaterialEnvelope
	if e = json.Unmarshal(native.Bytes(), &q); e != nil {
		t.Fatal(e)
	}
	q.Native = json.RawMessage(`{}`)
	bad, _ := json.Marshal(q)
	if _, e = ReadSphericalMaterialJSON(ctx, bytes.NewReader(bad), Limits{}); e == nil {
		t.Fatal("forged native")
	}
	for _, l := range []Limits{{MaxEntities: 15}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 8}} {
		if _, e = ReadSphericalMaterial(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
			t.Fatal("limit")
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID"); oracle != "" {
		for _, b := range [][]byte{raw, output.Bytes()} {
			path := filepath.Join(t.TempDir(), "torus.igs")
			if e = os.WriteFile(path, b, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := exec.Command(oracle, path).CombinedOutput()
			if e != nil {
				t.Fatalf("%s %v", data, e)
			}
			var q struct {
				Faces, Solids int
				Volume, Area  float64
			}
			at := bytes.LastIndex(data, []byte("{"))
			if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Faces != 1 || q.Solids != 1 || math.Abs(q.Volume-mass.Volume) > 1e-7 || math.Abs(q.Area-mass.Area) > 1e-7 {
				t.Fatalf("%s", data)
			}
		}
	}
}
func FuzzSphericalMaterial(f *testing.F) {
	b, e := os.ReadFile("testdata/spherical-material.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadSphericalMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
