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

func TestConical45CylinderThrough(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/conical45-cylinder-through.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadConical45CylinderThroughSolid(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out, native, source bytes.Buffer
	if e = WriteConical45CylinderThroughSolid(ctx, &out, m.Descriptor, m.LengthUnit); e != nil {
		t.Fatal(e)
	}
	for _, b := range [][]byte{raw, out.Bytes()} {
		p, e := ReadConical45CylinderThroughSolid(ctx, bytes.NewReader(b), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		mass, e := p.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil || math.Abs(mass.Volume-304*math.Pi/3) > 1e-7 || math.Abs(mass.Area-math.Pi*(64+40*math.Sqrt2)) > 1e-7 || mass.Centroid.Distance(geom.Vec3{Z: 48. / 19}) > 1e-8 {
			t.Fatal(mass, e)
		}
		agreement, e := p.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		for _, u := range agreement.Uses {
			if u.Report.SquaredUpperBound.Sign() != 0 {
				t.Fatal("nonexact rim")
			}
		}
		if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID"); oracle != "" {
			path := filepath.Join(t.TempDir(), "shape.igs")
			os.WriteFile(path, b, 0600)
			result, e := exec.Command(oracle, path).CombinedOutput()
			if e != nil {
				t.Fatal(string(result), e)
			}
			var q struct {
				Faces, Solids int
				Volume, Area  float64
			}
			at := bytes.LastIndex(result, []byte("{"))
			if at < 0 || json.Unmarshal(result[at:], &q) != nil || q.Faces != 4 || q.Solids != 1 || math.Abs(q.Volume-mass.Volume) > 1e-7 || math.Abs(q.Area-mass.Area) > 1e-7 {
				t.Fatal(string(result))
			}
		}
	}
	if e = WriteConical45CylinderThroughJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	p, e := ReadConical45CylinderThroughJSON(ctx, &native, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteConical45CylinderThroughSource(ctx, &source, p); e != nil || !bytes.Equal(raw, source.Bytes()) {
		t.Fatal(e)
	}
	var plain bytes.Buffer
	if e := trimmed.WriteJSON(&plain, m.Solid); e != nil {
		t.Fatal(e)
	}
	restored, e := trimmed.ReadJSON(&plain, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	report, e := restored.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !report.Complete {
		t.Fatal(e)
	}
	for _, u := range report.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			t.Fatal("reload rim")
		}
	}
	mesh, e := restored.Tessellate(ctx, parametric.TessellationOptions{Deflection: .5, MaxSubdivisions: 256, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	var forged conicalThroughEnvelope
	var encoded bytes.Buffer
	if e = WriteConical45CylinderThroughJSON(ctx, &encoded, m); e != nil {
		t.Fatal(e)
	}
	if json.Unmarshal(encoded.Bytes(), &forged) != nil {
		t.Fatal("envelope")
	}
	forged.Native = json.RawMessage(`{}`)
	badNative, _ := json.Marshal(forged)
	if _, e = ReadConical45CylinderThroughJSON(ctx, bytes.NewReader(badNative), Limits{}); e == nil {
		t.Fatal("forged native")
	}
	for _, l := range []Limits{{MaxBytes: 80}, {MaxEntities: 27}, {MaxValues: 8}, {MaxRecords: 2}} {
		if _, e := ReadConical45CylinderThroughSolid(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	stale := m
	stale.SourceFaceIDs[0] = stale.SourceFaceIDs[1]
	var rejected bytes.Buffer
	if e := WriteConical45CylinderThroughSource(ctx, &rejected, stale); e == nil || rejected.Len() != 0 {
		t.Fatal("stale source")
	}
	for _, d := range []Conical45CylinderThroughDescriptor{{Radius: 3, Height: 4, BoreRadius: 3}, {Radius: 3, Height: 0, BoreRadius: 1}} {
		var dst bytes.Buffer
		if e := WriteConical45CylinderThroughSolid(ctx, &dst, d, 1); e == nil || dst.Len() != 0 {
			t.Fatal("invalid clearance")
		}
	}
	for _, change := range []struct {
		id, index int
		value     string
	}{{9, 4, "44"}, {11, 3, "3"}, {31, 3, "1"}, {53, 5, "1"}, {49, 5, "41"}, {55, 1, "53.5"}} {
		limits, _ := (Limits{}).defaults()
		doc, e := readPlacedTopologyRecords(bytes.NewReader(raw), limits, true, false, false, false, false, false, false, true, true)
		if e != nil {
			t.Fatal(e)
		}
		for i, r := range doc.entities {
			if r.id == change.id {
				doc.entities[i].values[change.index].text = change.value
			}
		}
		bad, e := mixedRoofFile(doc)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadConical45CylinderThroughSolid(ctx, bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal("mutation", change)
		}
	}
}

func FuzzConical45CylinderThrough(f *testing.F) {
	b, e := os.ReadFile("testdata/conical45-cylinder-through.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadConical45CylinderThroughSolid(context.Background(), bytes.NewReader(b), Limits{})
	})
}
