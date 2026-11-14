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
	"strings"
	"testing"
)

func TestToroidalMaterial(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/toroidal-material.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadToroidalMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Solid.Faces()) != 16 || m.SourceFaceID != 21 || m.SourceSeamCurveIDs != [2]int{11, 13} {
		t.Fatal("source ancestry")
	}
	proof, e := m.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal(e)
	}
	for _, u := range proof.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			t.Fatal("nonexact seam")
		}
	}
	mass, e := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(mass.Volume-8*math.Pi*math.Pi) > 1e-7 || math.Abs(mass.Area-16*math.Pi*math.Pi) > 1e-7 || mass.Centroid.Length() > 1e-8 {
		t.Fatal(mass, e)
	}
	var plain bytes.Buffer
	if e = trimmed.WriteJSON(&plain, m.Solid); e != nil {
		t.Fatal(e)
	}
	if _, e = trimmed.ReadJSON(&plain, trimmed.JSONLimits{}); e != nil {
		t.Fatal(e)
	}
	mesh, e := m.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .1, MaxSubdivisions: 128, MaxTriangles: 1000000})
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	var native, retained, output bytes.Buffer
	if e = WriteToroidalMaterialJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	p, e := ReadToroidalMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteToroidalMaterialSource(ctx, &retained, p); e != nil || !bytes.Equal(raw, retained.Bytes()) {
		t.Fatal("source changed", e)
	}
	if e = WriteToroidalMaterial(ctx, &output, p.Descriptor, p.LengthUnit, 1e-9); e != nil {
		t.Fatal(e)
	}
	for _, d := range []ToroidalMaterialDescriptor{{1, 1}, {1, 2}, {4, 0}, {math.Inf(1), 1}} {
		var b bytes.Buffer
		if e = WriteToroidalMaterial(ctx, &b, d, .001, 1e-9); e == nil || b.Len() != 0 {
			t.Fatal("invalid ring material")
		}
	}
	for _, mutation := range []struct {
		prefix string
		field  int
		value  string
	}{{"198,", 3, "1"}, {"124,", 6, "1"}, {"508,", 5, "0"}, {"504,", 4, "9"}, {"514,", 3, "0"}, {"100,", 4, "9"}} {
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
		if _, e = ReadToroidalMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("wrong source seam/support", mutation)
		}
	}
	var q toroidalMaterialEnvelope
	if e = json.Unmarshal(native.Bytes(), &q); e != nil {
		t.Fatal(e)
	}
	q.Native = json.RawMessage(`{}`)
	bad, _ := json.Marshal(q)
	if _, e = ReadToroidalMaterialJSON(ctx, bytes.NewReader(bad), Limits{}); e == nil {
		t.Fatal("forged native")
	}
	for _, l := range []Limits{{MaxEntities: 12}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 8}} {
		if _, e = ReadToroidalMaterial(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
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
func FuzzToroidalMaterial(f *testing.F) {
	b, e := os.ReadFile("testdata/toroidal-material.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadToroidalMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
