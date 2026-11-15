package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"plainsolid/collision"
)

func TestPlanarSolidIGESAffineChart(t *testing.T) {
	raw, err := os.ReadFile("testdata/hexahedron-solid.igs")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m, err := ReadPlanarSolid(ctx, bytes.NewReader(raw), PlanarSolidOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if m.Solid.Topology().Faces != 6 || len(m.SourceEdges) != 12 ||
		m.SourceFaces[0].PlaneID != 29 || len(m.SourceFaces[0].ParameterCurves) != 1 {
		t.Fatalf("original affine face and edge incidence missing: %+v", m.SourceFaces)
	}
	owner := m.SourceFaces[0].ParameterCurves[0]
	if len(owner) != 4 || owner[0] != 31 || owner[1] != 33 {
		t.Fatalf("original owner pcurve records changed: %v", owner)
	}
	p, err := collision.MeasureExactPlanarMass(ctx, m.Solid, collision.LocationOptions{}, collision.ExactMassOptions{})
	if err != nil || p.Volume.RatString() != "13" || p.Centroid[0].RatString() != "5/2" ||
		p.Centroid[1].RatString() != "5" || p.Centroid[2].RatString() != "13/2" {
		t.Fatalf("affine chart material differs from independent fixture: %+v %v", p, err)
	}
	var canonical bytes.Buffer
	if err := WritePlanarSolid(ctx, &canonical, m.Solid, m.LengthUnit, PlanarSolidOptions{}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPlanarSolid(ctx, bytes.NewReader(canonical.Bytes()), PlanarSolidOptions{})
	if err != nil || !samePlanarIGESBody(m.Solid, got.Solid, m.Solid.Faces()) {
		t.Fatalf("canonical plane representation changed the material: %v", err)
	}
	for _, c := range []struct {
		id, index int
		token     string
	}{
		{31, 13, ".25"},                    // original pcurve starts at the wrong place
		{31, 13, ".000000000000000000001"}, // tiny but exact departure from original lift
		{29, 30, "14"},                     // declared chart loses its affine identity
		{39, 8, "33"},                      // replaces the distinct first owner by a neighbouring edge owner
		{3, 11, "1.00000000000000001"},     // same binary64 weight, unequal source decimals
	} {
		mutant := makePlanarIGESMutation(t, raw, c.id, c.index, c.token)
		if m, err := ReadPlanarSolid(ctx, bytes.NewReader(mutant), PlanarSolidOptions{}); err == nil || m.SolidID != 0 || m.SourceFaces != nil {
			t.Fatalf("changed original coedge/support yielded a solid: %+v %v", c, err)
		}
	}
	if m, err := ReadPlanarSolid(ctx, bytes.NewReader(raw), PlanarSolidOptions{Limits: Limits{MaxControlPoints: 95}}); err == nil || m.SolidID != 0 {
		t.Fatalf("affine chart proof ignored the source control budget: %v", err)
	}
	if bin := os.Getenv("PLAINSOLID_OCCT_PLANAR_SHELLS_IGES"); bin != "" {
		path := filepath.Join(t.TempDir(), "canonical-affine-polyhedron.igs")
		if err := os.WriteFile(path, canonical.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		for _, file := range []string{"testdata/hexahedron-solid.igs", path} {
			data, err := exec.Command(bin, "read", file).CombinedOutput()
			if err != nil {
				t.Fatalf("pinned OCCT affine chart/plane transfer: %v %s", err, data)
			}
			var m struct {
				Solids, Faces int
				Volume        float64
				Centroid      [3]float64
			}
			i := bytes.LastIndexByte(data, '{')
			if i < 0 || json.Unmarshal(data[i:], &m) != nil || m.Solids != 1 || m.Faces != 6 ||
				math.Abs(m.Volume-13) > 1e-9 || math.Abs(m.Centroid[0]-2.5) > 1e-9 ||
				math.Abs(m.Centroid[1]-5) > 1e-9 || math.Abs(m.Centroid[2]-6.5) > 1e-9 {
				t.Fatalf("pinned OCCT transfer measures: %s", data)
			}
		}
	}
}
