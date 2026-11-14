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
	"testing"
)

func TestCylinderSphereCavityMaterial(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/cylinder-sphere-cavity.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadCylinderSphereCavityMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Solid.Faces()) != 7 || m.SourceShellIDs != [2]int{45, 49} {
		t.Fatal("ancestry")
	}
	if !m.Solid.Faces()[6].Reversed {
		t.Fatal("inner sphere orientation")
	}
	proof, e := m.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal(e)
	}
	for _, u := range proof.Uses {
		if u.Report.SquaredUpperBound == nil || (u.Face < 6 && u.Report.SquaredUpperBound.Sign() != 0) || (u.Report.SquaredUpperBound != nil && u.Report.SquaredUpperBound.Cmp(new(big.Rat).SetFloat64(1e-18)) > 0) {
			t.Fatal("nonexact native edge")
		}
	}
	mass, e := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	center := geom.Vec3{Z: 189. / 370}
	if e != nil || math.Abs(mass.Volume-185*math.Pi/3) > 1e-7 || math.Abs(mass.Area-64*math.Pi) > 1e-7 || mass.Centroid.Distance(center) > 1e-8 {
		t.Fatal(mass, e)
	}
	var native, source, output, plain bytes.Buffer
	if e = WriteCylinderSphereCavityMaterialJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	p, e := ReadCylinderSphereCavityMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteCylinderSphereCavityMaterialSource(ctx, &source, p); e != nil || !bytes.Equal(raw, source.Bytes()) {
		t.Fatal("source", e)
	}
	if e = trimmed.WriteJSON(&plain, m.Solid); e != nil {
		t.Fatal(e)
	}
	if _, e = trimmed.ReadJSON(&plain, trimmed.JSONLimits{}); e != nil {
		t.Fatal(e)
	}
	if e = WriteCylinderSphereCavityMaterial(ctx, &output, m.Descriptor, m.LengthUnit, 1e-9); e != nil {
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
			d.Stock.Origin.Z = -1
		case 1:
			d.Stock.Height = 4
		case 2:
			d.Stock.Radius = 1
		case 3:
			d.SphereRadius = 4
		case 4:
			d.Stock.Origin.X = 1
		}
		var dst bytes.Buffer
		if e = WriteCylinderSphereCavityMaterial(ctx, &dst, d, 1, 1e-9); e == nil || dst.Len() != 0 {
			t.Fatal("invalid nesting", which)
		}
	}
	for _, q := range []struct {
		id, index int
		text      string
	}{{47, 5, "1"}, {47, 4, "45"}, {53, 2, "3"}, {51, 1, "9"}, {67, 3, "15"}, {55, 1, "1"}} {
		l, _ := (Limits{}).defaults()
		doc, e := readPlacedTopologyRecords(bytes.NewReader(raw), l, true, false, false, false, false, false, true, true, true, true, true, true)
		if e != nil {
			t.Fatal(e)
		}
		doc.entities[(q.id-1)/2].values[q.index].text = q.text
		bad, e := mixedRoofFile(doc)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadCylinderSphereCavityMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("forged source", q)
		}
	}
	forged := m
	forged.SourceShellIDs[1] = 45
	var reject bytes.Buffer
	if e = WriteCylinderSphereCavityMaterialJSON(ctx, &reject, forged); e == nil || reject.Len() != 0 {
		t.Fatal("forged ancestry")
	}
	for _, l := range []Limits{{MaxEntities: 38}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 8}} {
		if _, e = ReadCylinderSphereCavityMaterial(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
			t.Fatal("limit")
		}
	}
	if _, e = ReadCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e == nil {
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
			if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Faces != 4 || q.Solids != 1 || math.Abs(q.Volume-mass.Volume) > 1e-7 || math.Abs(q.Area-mass.Area) > 1e-7 || (geom.Vec3{X: q.Center[0], Y: q.Center[1], Z: q.Center[2]}).Distance(center) > 1e-8 {
				t.Fatalf("%s", data)
			}
		}
	}
}
func FuzzCylinderSphereCavityMaterial(f *testing.F) {
	b, e := os.ReadFile("testdata/cylinder-sphere-cavity.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadCylinderSphereCavityMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
