package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestSideOpeningPairSTEP(t *testing.T) {
	normals := []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	for ni, n := range normals {
		for di, depths := range [][2]float64{{4, 4}, {4, 7}, {20, 7}, {7, 20}, {20, 20}} {
			frame, e := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
			if e != nil {
				t.Fatal(e)
			}
			p := SideOpeningPairBody{Name: "overlapping side openings", Frame: frame, Width: 40, Back: 30, Length: 20, Cuts: [2]trimmed.SideOpeningCut{
				{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 14, Y: 1}, Radius: 3}, Depth: depths[0]},
				{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 18, Y: 1}, Radius: 3}, Depth: depths[1]},
			}}
			m := SideOpeningPairModel{Body: p, LengthUnit: .001}
			var out bytes.Buffer
			if e := WriteSideOpeningPair(&out, m); e != nil {
				t.Fatal(ni, di, e)
			}
			got, e := ReadSideOpeningPair(bytes.NewReader(out.Bytes()), Limits{})
			if e != nil || !reflect.DeepEqual(got, m) {
				t.Fatal(ni, di, got, e)
			}
			reverse := m
			reverse.Body.Cuts[0], reverse.Body.Cuts[1] = reverse.Body.Cuts[1], reverse.Body.Cuts[0]
			var other bytes.Buffer
			if e := WriteSideOpeningPair(&other, reverse); e != nil || !bytes.Equal(out.Bytes(), other.Bytes()) {
				t.Fatal("noncanonical input order", ni, di, e)
			}
			if ni == 0 && di == 1 {
				// Changing a cylindrical support without changing its finite loops is not
				// accepted as an alternative recipe merely because the header is valid.
				re := regexp.MustCompile(`CYLINDRICAL_SURFACE\(([^\n]+),3\.?\)`)
				bad := re.ReplaceAll(out.Bytes(), []byte(`CYLINDRICAL_SURFACE($1,3.25)`))
				if bytes.Equal(bad, out.Bytes()) {
					t.Fatal("mutation did not modify cylindrical support")
				}
				if _, e := ReadSideOpeningPair(bytes.NewReader(bad), Limits{}); e == nil {
					t.Fatal("accepted inconsistent support")
				}

				edgePattern := regexp.MustCompile(`ORIENTED_EDGE\([^;\n]+,\.[TF]\.\)`)
				loc := edgePattern.FindIndex(out.Bytes())
				if loc == nil {
					t.Fatal("no oriented edge")
				}
				bad = append([]byte(nil), out.Bytes()...)
				sense := loc[1] - 3
				if bad[sense] == 'T' {
					bad[sense] = 'F'
				} else if bad[sense] == 'F' {
					bad[sense] = 'T'
				} else {
					t.Fatal("unexpected edge sense encoding")
				}
				if _, e := ReadSideOpeningPair(bytes.NewReader(bad), Limits{}); e == nil {
					t.Fatal("accepted changed edge sense")
				}
			}
			if os.Getenv("PLAINSOLID_OCCT_STEP") != "" {
				path, report := filepath.Join(t.TempDir(), "pair.step"), filepath.Join(t.TempDir(), "mass.json")
				if e := os.WriteFile(path, out.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				stepOracle(t, report, "step-mass-precise", path)
				raw, e := os.ReadFile(report)
				if e != nil {
					t.Fatal(e)
				}
				var oracle struct {
					Volume, Area float64
					Centroid     [3]float64
				}
				if e = json.Unmarshal(raw, &oracle); e != nil {
					t.Fatal(e)
				}
				native, e := p.Solid(context.Background())
				if e != nil {
					t.Fatal(e)
				}
				mass, e := native.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if e != nil {
					t.Fatal(e)
				}
				if math.Abs(mass.Volume-oracle.Volume) > 1e-7 || math.Abs(mass.Area-oracle.Area) > 1e-7 || mass.Centroid.Distance(geom.Vec3{X: oracle.Centroid[0], Y: oracle.Centroid[1], Z: oracle.Centroid[2]}) > 1e-7 {
					t.Fatal(ni, di, mass, oracle)
				}
			}
		}
	}
}
