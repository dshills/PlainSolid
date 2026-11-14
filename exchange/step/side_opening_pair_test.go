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
		for di, depths := range [][2]float64{{4, 4}, {4, 7}, {20, 7}, {7, 20}, {20, 20}, {14, 12}, {12, 14}} {
			frame, e := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
			if e != nil {
				t.Fatal(e)
			}
			p := SideOpeningPairBody{Name: "overlapping side openings", Frame: frame, Width: 40, Back: 30, Length: 20, Cuts: [2]trimmed.SideOpeningCut{
				{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 14, Y: 1}, Radius: 3}, Depth: depths[0]},
				{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 18, Y: 1}, Radius: 3}, Depth: depths[1]},
			}}
			if di == 5 {
				p.Cuts[1].OppositeCap = true
			}
			if di == 6 {
				p.Cuts[0].OppositeCap = true
			}
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
			checkSideOpeningPairOCCT(t, p, out.Bytes())
		}
	}
}

// Reimport the exact canonical file with the independent kernel, comparing
// integrated native measures rather than repeating recipe formulas.
func checkSideOpeningPairOCCT(t *testing.T, p SideOpeningPairBody, data []byte) {
	t.Helper()
	if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		return
	}
	path, report := filepath.Join(t.TempDir(), "pair.step"), filepath.Join(t.TempDir(), "mass.json")
	if e := os.WriteFile(path, data, 0600); e != nil {
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
		t.Fatal(mass, oracle)
	}
}

func TestSideOpeningPairDifferentSidesSTEP(t *testing.T) {
	normals := []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	for _, family := range []struct {
		name    string
		back    float64
		centers [2]geom.Vec2
	}{
		{"adjacent", 20, [2]geom.Vec2{{X: 0, Y: 3.5}, {X: 3.5, Y: 0}}},
		{"opposite", 8, [2]geom.Vec2{{X: 8, Y: 1.5}, {X: 9, Y: 6.5}}},
	} {
		for ni, n := range normals {
			for _, interval := range []struct {
				name    string
				depths  [2]float64
				opposed bool
			}{
				{"same-entry", [2]float64{4, 6}, false},
				{"opposed-web", [2]float64{3, 3}, true},
				{"opposed-overlap", [2]float64{6, 5}, true},
				{"through-blind", [2]float64{8, 5}, true},
			} {
				t.Run(family.name+"/"+interval.name+"/"+string(rune('0'+ni)), func(t *testing.T) {
					frame, e := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
					if e != nil {
						t.Fatal(e)
					}
					p := SideOpeningPairBody{Name: "different side openings", Frame: frame, Width: 20, Back: family.back, Length: 8}
					for i := range p.Cuts {
						p.Cuts[i] = trimmed.SideOpeningCut{ExtrusionCircle: trimmed.ExtrusionCircle{Center: family.centers[i], Radius: 3}, Depth: interval.depths[i], OppositeCap: i == 1 && interval.opposed}
					}
					model := SideOpeningPairModel{Body: p, LengthUnit: .001}
					var out bytes.Buffer
					if e := WriteSideOpeningPair(&out, model); e != nil {
						t.Fatal(e)
					}
					got, e := ReadSideOpeningPair(bytes.NewReader(out.Bytes()), Limits{})
					if e != nil || !reflect.DeepEqual(got, model) {
						t.Fatal(got, e)
					}
					reversed := model
					reversed.Body.Cuts[0], reversed.Body.Cuts[1] = reversed.Body.Cuts[1], reversed.Body.Cuts[0]
					var reverse bytes.Buffer
					if e := WriteSideOpeningPair(&reverse, reversed); e != nil || !bytes.Equal(out.Bytes(), reverse.Bytes()) {
						t.Fatal("noncanonical order", e)
					}
					checkSideOpeningPairOCCT(t, p, out.Bytes())
				})
			}
		}
	}
}

func TestSideOpeningPairSTEPRejectsOppositeThroughSplit(t *testing.T) {
	frame, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	p := SideOpeningPairBody{Name: "split stock", Frame: frame, Width: 20, Back: 8, Length: 8, Cuts: [2]trimmed.SideOpeningCut{
		{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 8, Y: 1.5}, Radius: 3}, Depth: 8},
		{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 9, Y: 6.5}, Radius: 3}, Depth: 8},
	}}
	var out bytes.Buffer
	if e := WriteSideOpeningPair(&out, SideOpeningPairModel{Body: p, LengthUnit: .001}); e == nil || out.Len() != 0 {
		t.Fatal("published unsupported split stock", e)
	}
}
