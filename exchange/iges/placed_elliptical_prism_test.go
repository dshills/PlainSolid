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
	"plainsolid/geom"
	"testing"
)

func TestPlacedEllipticalPrismMaterial(t *testing.T) {
	ctx := context.Background()
	for _, hollow := range []bool{false, true} {
		name := "placed-elliptical-prism-topology.igs"
		volume, area, faces := 40*math.Pi, 154.88430345445522, 3
		if hollow {
			name = "placed-elliptical-annular-prism-topology.igs"
			volume, area, faces = 32*math.Pi, 181.07172572228598, 4
		}
		raw, e := os.ReadFile("testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadPlacedEllipticalPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		if (m.Annular != nil) != hollow || (m.Filled != nil) == hollow {
			t.Fatal("family")
		}
		mass, e := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil || math.Abs(mass.Volume-volume) > 1e-6 || math.Abs(mass.Area-area) > 1e-6 || mass.Centroid.Distance(geom.Vec3{X: 12.5, Y: -4.25, Z: 2.125}) > 1e-7 {
			t.Fatal(mass, e)
		}
		proof, e := m.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !proof.Complete {
			t.Fatal(e)
		}
		for _, u := range proof.Uses {
			if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
				t.Fatal("nonexact join")
			}
		}
		var native bytes.Buffer
		if e = WritePlacedEllipticalPrismMaterialJSON(ctx, &native, m); e != nil {
			t.Fatal(e)
		}
		q, e := ReadPlacedEllipticalPrismMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var again bytes.Buffer
		if e = WritePlacedEllipticalPrismMaterialJSON(ctx, &again, q); e != nil || !bytes.Equal(native.Bytes(), again.Bytes()) {
			t.Fatal("reload", e)
		}
		var out bytes.Buffer
		if hollow {
			e = WritePlacedEllipticalAnnularPrismTopology(ctx, &out, q.Annular.Descriptor, q.Placement, q.Annular.LengthUnit, 1e-9)
		} else {
			e = WritePlacedEllipticalPrismTopology(ctx, &out, q.Filled.Descriptor, q.Placement, q.Filled.LengthUnit, 1e-9)
		}
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadPlacedEllipticalPrismMaterial(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9); e != nil {
			t.Fatal(e)
		}
		bad := append([]byte(nil), raw...)
		for _, line := range bytes.Split(bad, []byte("\n")) {
			if len(line) == 80 && line[72] == 'D' {
				kind, _ := field(string(line[:8]))
				id, _ := field(string(line[73:]))
				if id%2 == 1 && kind == 104 {
					copy(line[48:56], []byte("       0"))
					break
				}
			}
		}
		if _, e = ReadPlacedEllipticalPrismMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("unplaced model leaf")
		}

		terminalBad := bytes.Replace(raw, []byte("14.5,0.75,2.125"), []byte("13.5,0.75,2.125"), 1)
		if bytes.Equal(terminalBad, raw) {
			t.Fatal("terminal fixture changed")
		}
		if _, e = ReadPlacedEllipticalPrismMaterial(ctx, bytes.NewReader(terminalBad), Limits{}, 1e-9); e == nil {
			t.Fatal("wrong world terminal")
		}
		if hollow {
			if _, e = ReadEllipticalAnnularPrismTopology(ctx, bytes.NewReader(raw), Limits{}); e == nil {
				t.Fatal("unplaced annular widened")
			}
		} else {
			if _, e = ReadEllipticalPrismTopology(ctx, bytes.NewReader(raw), Limits{}); e == nil {
				t.Fatal("unplaced filled widened")
			}
		}
		for _, pose := range []geom.Affine{{{1, 0, 0, .1}, {0, 1, 0, 0}, {0, 0, 1, 0}}, {{.6, 0, .8, 0}, {0, 1, 0, 0}, {-.8, 0, .6, 0}}, {{-1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}} {
			var b bytes.Buffer
			if hollow {
				e = WritePlacedEllipticalAnnularPrismTopology(ctx, &b, q.Annular.Descriptor, pose, .001, 1e-9)
			} else {
				e = WritePlacedEllipticalPrismTopology(ctx, &b, q.Filled.Descriptor, pose, .001, 1e-9)
			}
			if e == nil || b.Len() != 0 {
				t.Fatal("nonexact/reflected placement")
			}
		}

		pose := geom.Affine{{1, 0, 0, 8}, {0, -1, 0, 2}, {0, 0, -1, -1}}
		var flipped bytes.Buffer
		if hollow {
			e = WritePlacedEllipticalAnnularPrismTopology(ctx, &flipped, q.Annular.Descriptor, pose, .001, 1e-9)
		} else {
			e = WritePlacedEllipticalPrismTopology(ctx, &flipped, q.Filled.Descriptor, pose, .001, 1e-9)
		}
		if e != nil {
			t.Fatal(e)
		}
		flippedModel, e := ReadPlacedEllipticalPrismMaterial(ctx, &flipped, Limits{}, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		fm, e := flippedModel.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil || fm.Centroid.Distance(geom.Vec3{X: 8, Y: 2, Z: -6}) > 1e-7 {
			t.Fatal(fm, e)
		}
		var env placedEllipseEnvelope
		if json.Unmarshal(native.Bytes(), &env) != nil {
			t.Fatal("json")
		}
		env.Data.Placement[0][3]++
		b, _ := json.Marshal(env)
		if _, e = ReadPlacedEllipticalPrismMaterialJSON(ctx, bytes.NewReader(b), Limits{}); e == nil {
			t.Fatal("stale pose")
		}
		if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_ELLIPTICAL_PRISM_TOPOLOGY"); oracle != "" {
			p := filepath.Join(t.TempDir(), name)
			if e = os.WriteFile(p, out.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			for _, path := range []string{"testdata/" + name, p} {
				b, e := exec.Command(oracle, "read", path).CombinedOutput()
				if e != nil {
					t.Fatalf("%s %v", b, e)
				}
				var v struct {
					Faces, Solids int
					Area, Volume  float64
					Center        [3]float64
				}
				at := bytes.LastIndex(b, []byte("{"))
				if at < 0 || json.Unmarshal(bytes.TrimSpace(b[at:]), &v) != nil || v.Faces != faces || v.Solids != 1 || math.Abs(v.Volume-volume) > 1e-8 || math.Abs(v.Area-area) > 1e-7 || math.Abs(v.Center[0]-12.5) > 1e-10 || math.Abs(v.Center[1]+4.25) > 1e-10 || math.Abs(v.Center[2]-2.125) > 1e-10 {
					t.Fatalf("%s", b)
				}
			}
		}
	}
}
func FuzzPlacedEllipticalPrism(f *testing.F) {
	for _, p := range []string{"placed-elliptical-prism-topology.igs", "placed-elliptical-annular-prism-topology.igs"} {
		b, e := os.ReadFile("testdata/" + p)
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadPlacedEllipticalPrismMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
