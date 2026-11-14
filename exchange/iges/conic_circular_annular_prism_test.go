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

func TestConicCircularAnnularPrismMaterial(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/conic-circular-annular-prism-topology.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if m.Topology.Descriptor.OuterCoefficients != [6]float64{4, 0, 4, 0, 0, -100} || m.Topology.Parameters[19][3] != 4 {
		t.Fatal("lost original circle")
	}
	if _, e = ReadEllipticalAnnularPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old ellipse accepts circle")
	}
	ellipse, e := os.ReadFile("testdata/elliptical-annular-prism-topology.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(ellipse), Limits{}, 1e-9); e == nil {
		t.Fatal("circle accepts ellipse")
	}
	mass, e := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(mass.Volume-84*math.Pi) > 1e-7 || math.Abs(mass.Area-98*math.Pi) > 1e-7 || math.Abs(mass.Centroid.Z-5) > 1e-9 {
		t.Fatal(mass, e)
	}
	var native bytes.Buffer
	if e = WriteConicCircularAnnularPrismMaterialJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadConicCircularAnnularPrismMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(loaded.Topology.source, raw) {
		t.Fatal("lost original source")
	}
	var plain bytes.Buffer
	if e = trimmed.WriteJSON(&plain, loaded.Solid); e != nil {
		t.Fatal(e)
	}
	if _, e = trimmed.ReadJSON(&plain, trimmed.JSONLimits{}); e != nil {
		t.Fatal(e)
	}
	proof, e := loaded.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal("edge proof", e)
	}
	for _, u := range proof.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			t.Fatal("nonzero exact edge discrepancy")
		}
	}
	mesh, e := loaded.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .1, MaxSubdivisions: 256, MaxTriangles: 1000000})
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	var forged conicCircularAnnularEnvelope
	if e = json.Unmarshal(native.Bytes(), &forged); e != nil {
		t.Fatal(e)
	}
	forged.Topology.Descriptor.OuterCoefficients[2] = 16
	badEnvelope, e := json.Marshal(forged)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadConicCircularAnnularPrismMaterialJSON(ctx, bytes.NewReader(badEnvelope), Limits{}); e == nil {
		t.Fatal("forged circle proof")
	}
	invalid := m.Topology.Descriptor
	invalid.OuterCoefficients[0] = math.Inf(1)
	invalid.OuterCoefficients[2] = math.Inf(1)
	var atomic bytes.Buffer
	if e = WriteConicCircularAnnularPrism(ctx, &atomic, invalid, .001, 1e-9); e == nil || atomic.Len() != 0 {
		t.Fatal("invalid nonfinite writer")
	}
	if m.Topology.Descriptor.InnerCoefficients != [6]float64{1, 0, 1, 0, 0, -4} || m.SourceRimCurveIDs != [2][2]int{{23, 19}, {29, 25}} || len(m.Solid.Faces()) != 10 {
		t.Fatal("inner source lineage")
	}
	for _, radiusSquared := range []float64{-25, -36, 0} {
		bad := m.Topology.Descriptor
		bad.InnerCoefficients = [6]float64{1, 0, 1, 0, 0, radiusSquared}
		var out bytes.Buffer
		if e = WriteConicCircularAnnularPrism(ctx, &out, bad, .001, 1e-9); e == nil || out.Len() != 0 {
			t.Fatal("uncontained or degenerate inner circle")
		}
	}
	wrongHole := append([]byte(nil), raw...)
	for _, line := range bytes.Split(wrongHole, []byte("\n")) {
		if len(line) != 80 || line[72] != 'P' {
			continue
		}
		owner, _ := field(string(line[64:72]))
		if owner != 39 {
			continue
		}
		v := strings.Split(strings.TrimSuffix(strings.TrimSpace(string(line[:64])), ";"), ",")
		v[5] = "1"
		text := strings.Join(v, ",") + ";"
		copy(line[:64], text+strings.Repeat(" ", 64-len(text)))
	}
	if _, e = ReadConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(wrongHole), Limits{}, 1e-9); e == nil {
		t.Fatal("incorrect inner cap use")
	}
	var output bytes.Buffer
	if e = WriteConicCircularAnnularPrism(ctx, &output, m.Topology.Descriptor, m.Topology.LengthUnit, 1e-9); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(output.Bytes()), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	for _, mutation := range []struct {
		prefix string
		field  int
		value  string
	}{{"104,", 3, "5"}, {"110,", 2, "1"}, {"116,", 2, "1"}, {"123,", 2, "1"}, {"122,", 3, "1"}, {"502,", 3, "1"}} {
		bad := append([]byte(nil), raw...)
		changed := false
		for _, line := range bytes.Split(bad, []byte("\n")) {
			if len(line) != 80 || line[72] != 'P' || !bytes.HasPrefix(line, []byte(mutation.prefix)) {
				continue
			}
			v := strings.Split(strings.TrimSpace(string(line[:64])), ",")
			v[mutation.field] = mutation.value
			text := strings.Join(v, ",")
			if len(text) > 64 {
				t.Fatal("mutation overflow")
			}
			copy(line[:64], text+strings.Repeat(" ", 64-len(text)))
			changed = true
			break
		}
		if !changed {
			t.Fatal("missing mutation")
		}
		if _, e = ReadConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("accepted noninvariant record", mutation)
		}
	}
	for _, l := range []Limits{{MaxEntities: 27}, {MaxBytes: 80}, {MaxValues: 10}, {MaxRecords: 2}} {
		if _, e = ReadConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
			t.Fatal("limit")
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_ELLIPTICAL_PRISM_TOPOLOGY"); oracle != "" {
		for _, b := range [][]byte{raw, output.Bytes()} {
			p := filepath.Join(t.TempDir(), "circle.igs")
			if e = os.WriteFile(p, b, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := exec.Command(oracle, "read", p).CombinedOutput()
			if e != nil {
				t.Fatalf("%s %v", data, e)
			}
			var q struct {
				Faces, Solids int
				Area, Volume  float64
				Center        [3]float64
			}
			at := bytes.LastIndex(data, []byte("{"))
			if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Faces != 4 || q.Solids != 1 || math.Abs(q.Volume-84*math.Pi) > 1e-7 || math.Abs(q.Area-98*math.Pi) > 1e-7 || math.Abs(q.Center[2]-5) > 1e-9 {
				t.Fatalf("%s", data)
			}
		}
	}
}
func FuzzConicCircularAnnularPrism(f *testing.F) {
	b, e := os.ReadFile("testdata/conic-circular-annular-prism-topology.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadConicCircularAnnularPrismMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
