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

func TestConical45CavityMaterial(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/conical45-cavity-material.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadConical45CavityMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Solid.Faces()) != 12 || m.SourceShellIDs != [2]int{37, 77} {
		t.Fatal("ancestry")
	}
	for i := 0; i < 6; i++ {
		if m.Solid.Faces()[i].Reversed == m.Solid.Faces()[i+6].Reversed {
			t.Fatal("inner orientation")
		}
	}
	proof, e := m.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal(e)
	}
	for _, u := range proof.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			t.Fatal("nonexact native edge")
		}
	}
	mass, e := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	center := geom.Vec3{Z: 366. / 145}
	if e != nil || math.Abs(mass.Volume-290*math.Pi/3) > 1e-7 || math.Abs(mass.Area-math.Pi*(68+48*math.Sqrt2)) > 1e-7 || mass.Centroid.Distance(center) > 1e-8 {
		t.Fatal(mass, e)
	}
	var native, source, output, plain bytes.Buffer
	if e = WriteConical45CavityMaterialJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	p, e := ReadConical45CavityMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteConical45CavityMaterialSource(ctx, &source, p); e != nil || !bytes.Equal(raw, source.Bytes()) {
		t.Fatal("source", e)
	}
	if e = trimmed.WriteJSON(&plain, m.Solid); e != nil {
		t.Fatal(e)
	}
	if _, e = trimmed.ReadJSON(&plain, trimmed.JSONLimits{}); e != nil {
		t.Fatal(e)
	}
	if e = WriteConical45CavityMaterial(ctx, &output, m.Descriptor, m.LengthUnit, 1e-9); e != nil {
		t.Fatal(e)
	}
	mesh, e := m.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .1, MaxSubdivisions: 128, MaxTriangles: 1000000})
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	for _, which := range []int{0, 1, 2, 3, 4} {
		d := m.Descriptor
		switch which {
		case 0:
			d.Inner.Origin.Z = 0
		case 1:
			d.Inner.Height = 3
		case 2:
			d.Inner.Radius = 4
		case 3:
			d.Inner.Radius = 5
		case 4:
			d.Inner.Origin.X = 1
		}
		var dst bytes.Buffer
		if e = WriteConical45CavityMaterial(ctx, &dst, d, 1, 1e-9); e == nil || dst.Len() != 0 {
			t.Fatal("invalid cavity", which)
		}
	}
	for _, q := range []struct {
		id, index int
		text      string
	}{{39, 5, "1"}, {39, 4, "37"}, {49, 3, "4"}, {71, 1, "9"}, {63, 2, "17"}, {49, 4, "44"}} {
		l, _ := (Limits{}).defaults()
		doc, e := readPlacedTopologyRecords(bytes.NewReader(raw), l, true, false, false, false, false, false, false, true, true)
		if e != nil {
			t.Fatal(e)
		}
		doc.entities[(q.id-1)/2].values[q.index].text = q.text
		bad, e := mixedRoofFile(doc)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadConical45CavityMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("forged source", q)
		}
	}
	forged := m
	forged.SourceShellIDs[1] = 37
	var reject bytes.Buffer
	if e = WriteConical45CavityMaterialJSON(ctx, &reject, forged); e == nil || reject.Len() != 0 {
		t.Fatal("forged ancestry")
	}
	for _, l := range []Limits{{MaxEntities: 38}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 8}} {
		if _, e = ReadConical45CavityMaterial(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
			t.Fatal("limit")
		}
	}
	if _, e = ReadConical45FrustumMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old reader widened")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID"); oracle != "" {
		for _, b := range [][]byte{raw, output.Bytes()} {
			path := filepath.Join(t.TempDir(), "cavity.igs")
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
				Center        [3]float64
			}
			at := bytes.LastIndex(data, []byte("{"))
			if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Faces != 6 || q.Solids != 1 || math.Abs(q.Volume-mass.Volume) > 1e-7 || math.Abs(q.Area-mass.Area) > 1e-7 || (geom.Vec3{X: q.Center[0], Y: q.Center[1], Z: q.Center[2]}).Distance(center) > 1e-8 {
				t.Fatalf("%s", data)
			}
		}
	}
}
func FuzzConical45CavityMaterial(f *testing.F) {
	b, e := os.ReadFile("testdata/conical45-cavity-material.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadConical45CavityMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
