package step

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"reflect"
	"regexp"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestSideOpeningPairSTEP(t *testing.T) {
	slowtest.Skip(t)
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
	checkSideOpeningCutsOCCT(t, p.cutsBody(), data)
}
func checkSideOpeningCutsOCCT(t *testing.T, p SideOpeningCutsBody, data []byte) {
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
	slowtest.Skip(t)
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
	t.Parallel()
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

func TestSideOpeningCutsSTEP(t *testing.T) {
	slowtest.Skip(t)
	normals := []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	for _, family := range []struct {
		name        string
		width, back float64
		centers     []geom.Vec2
		depths      []float64
	}{
		{"three-chain", 26, 20, []geom.Vec2{{X: 6}, {X: 10}, {X: 14}}, []float64{3, 6, 4}},
		{"four-chain", 26, 20, []geom.Vec2{{X: 6}, {X: 10}, {X: 14}, {X: 18}}, []float64{3, 6, 4, 8}},
		{"three-adjacent", 20, 20, []geom.Vec2{{Y: 3.5}, {X: 3.5}, {X: 7.5}}, []float64{6, 3, 4}},
	} {
		for ni, n := range normals {
			for _, opposed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/%t", family.name, ni, opposed), func(t *testing.T) {
					frame, e := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
					if e != nil {
						t.Fatal(e)
					}
					p := SideOpeningCutsBody{Name: "multiple side openings", Frame: frame, Width: family.width, Back: family.back, Length: 8}
					for i, c := range family.centers {
						p.Cuts = append(p.Cuts, trimmed.SideOpeningCut{ExtrusionCircle: trimmed.ExtrusionCircle{Center: c, Radius: 3}, Depth: family.depths[i], OppositeCap: opposed && i == 1})
					}
					model := SideOpeningCutsModel{Body: p, LengthUnit: .001}
					var out bytes.Buffer
					if e := WriteSideOpeningCuts(&out, model); e != nil {
						t.Fatal(e)
					}
					got, e := ReadSideOpeningCuts(bytes.NewReader(out.Bytes()), Limits{})
					if e != nil || !reflect.DeepEqual(got, model) {
						t.Fatal(got, e)
					}
					reordered := model
					reordered.Body.Cuts = append([]trimmed.SideOpeningCut(nil), p.Cuts...)
					for i, j := 0, len(p.Cuts)-1; i < j; i, j = i+1, j-1 {
						reordered.Body.Cuts[i], reordered.Body.Cuts[j] = reordered.Body.Cuts[j], reordered.Body.Cuts[i]
					}
					var reverse bytes.Buffer
					if e := WriteSideOpeningCuts(&reverse, reordered); e != nil || !bytes.Equal(out.Bytes(), reverse.Bytes()) {
						t.Fatal("reordered source changed canonical STEP", e)
					}
					if _, e := ReadSideOpeningPair(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
						t.Fatal("pair reader accepted multiple sites")
					}
					if _, e := ReadSideOpeningCuts(bytes.NewReader(out.Bytes()), Limits{MaxFaces: 2}); e == nil {
						t.Fatal("ignored caller face budget")
					}
					checkSideOpeningCutsOCCT(t, p, out.Bytes())
				})
			}
		}
	}
}

func TestSideOpeningPairLegacySTEPBytes(t *testing.T) {
	t.Parallel()
	f, e := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	p := SideOpeningPairBody{Name: "overlapping side openings", Frame: f, Width: 40, Back: 30, Length: 20, Cuts: [2]trimmed.SideOpeningCut{
		{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 14, Y: 1}, Radius: 3}, Depth: 4},
		{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 18, Y: 1}, Radius: 3}, Depth: 4},
	}}
	var out bytes.Buffer
	if e := WriteSideOpeningPair(&out, SideOpeningPairModel{Body: p, LengthUnit: .001}); e != nil {
		t.Fatal(e)
	}
	// Captured before generalizing the two-cutter constructor and codec.
	if got := fmt.Sprintf("%x", sha256.Sum256(out.Bytes())); got != "b820225c711afbc6ac313b51619d954d120b5a62dadf75992116024ff368dd44" {
		t.Fatal("legacy STEP bytes changed", got)
	}
	if _, e := ReadSideOpeningCuts(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		t.Fatal(e)
	}
}

func TestSideOpeningCutsSTEPRejectsOverBudget(t *testing.T) {
	t.Parallel()
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := SideOpeningCutsBody{Name: "too many sites", Frame: frame, Width: 100, Back: 20, Length: 8}
	for i := 0; i <= trimmed.MaxSideOpeningCuts; i++ {
		p.Cuts = append(p.Cuts, trimmed.SideOpeningCut{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 6 + 4*float64(i)}, Radius: 3}, Depth: 4})
	}
	var out bytes.Buffer
	if e := WriteSideOpeningCuts(&out, SideOpeningCutsModel{Body: p, LengthUnit: .001}); e == nil || out.Len() != 0 {
		t.Fatal("published over-budget site set", e)
	}
}

func TestSideOpeningCutsSTEPInteriorOnlySite(t *testing.T) {
	t.Parallel()
	for _, opposite := range []bool{false, true} {
		frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
		p := SideOpeningCutsBody{Name: "interior-only cylindrical support", Frame: frame, Width: 20, Back: 10, Length: 8, Cuts: []trimmed.SideOpeningCut{
			{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5.5}, Radius: 1}, Depth: 6, OppositeCap: opposite},
			{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 6}, Radius: 3}, Depth: 3, OppositeCap: opposite},
			{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 10}, Radius: 3}, Depth: 3, OppositeCap: opposite},
		}}
		model := SideOpeningCutsModel{Body: p, LengthUnit: .001}
		var out bytes.Buffer
		if e := WriteSideOpeningCuts(&out, model); e != nil {
			t.Fatal(opposite, e)
		}
		got, e := ReadSideOpeningCuts(bytes.NewReader(out.Bytes()), Limits{})
		if e != nil || !reflect.DeepEqual(got, model) {
			t.Fatal(opposite, got, e)
		}
		checkSideOpeningCutsOCCT(t, p, out.Bytes())
	}
}

func TestSideOpeningCutsSTEPAtLimit(t *testing.T) {
	t.Parallel()
	frame, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, geom.Vec3{Z: 1})
	p := SideOpeningCutsBody{Name: "sixteen side opening cutters", Frame: frame, Width: 72, Back: 20, Length: 8}
	for i := 0; i < trimmed.MaxSideOpeningCuts; i++ {
		p.Cuts = append(p.Cuts, trimmed.SideOpeningCut{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 6 + 4*float64(i)}, Radius: 3}, Depth: 4})
	}
	native, e := p.Solid(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if len(native.Faces()) <= 64 {
		t.Fatal("fixture no longer exercises extended graph budget", len(native.Faces()))
	}
	model := SideOpeningCutsModel{Body: p, LengthUnit: .001}
	var out bytes.Buffer
	if e := WriteSideOpeningCuts(&out, model); e != nil {
		t.Fatal(e)
	}
	got, e := ReadSideOpeningCuts(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(got, model) {
		t.Fatal(got, e)
	}
	if _, e := ReadSideOpeningCuts(bytes.NewReader(out.Bytes()), Limits{MaxFaces: 64}); e == nil {
		t.Fatal("ignored explicit smaller face budget")
	}
	checkSideOpeningCutsOCCT(t, p, out.Bytes())
}

func TestSideOpeningCutsSTEPThroughSiteBuriedAtBothCaps(t *testing.T) {
	t.Parallel()
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := SideOpeningCutsBody{Name: "buried through site", Frame: frame, Width: 20, Back: 10, Length: 8, Cuts: []trimmed.SideOpeningCut{
		{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5.5}, Radius: 1}, Depth: 8},
		{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 6}, Radius: 3}, Depth: 3},
		{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 7}, Radius: 3}, Depth: 3, OppositeCap: true},
	}}
	model := SideOpeningCutsModel{Body: p, LengthUnit: .001}
	var out bytes.Buffer
	if e := WriteSideOpeningCuts(&out, model); e != nil {
		t.Fatal(e)
	}
	got, e := ReadSideOpeningCuts(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(got, model) {
		t.Fatal(got, e)
	}
	checkSideOpeningCutsOCCT(t, p, out.Bytes())
}

func TestSideOpeningCutsSTEPNestedBlindFloor(t *testing.T) {
	t.Parallel()
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := SideOpeningCutsBody{Name: "connected material with nested blind floor", Frame: frame, Width: 12, Back: 10, Length: 8}
	for _, center := range []geom.Vec2{{X: 4.75, Y: 1}, {X: 4.75, Y: 9}, {X: 10.5, Y: 5}} {
		p.Cuts = append(p.Cuts, trimmed.SideOpeningCut{ExtrusionCircle: trimmed.ExtrusionCircle{Center: center, Radius: 4.2}, Depth: 4})
	}
	native, e := p.Solid(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	innerWire := false
	for _, f := range native.Faces() {
		innerWire = innerWire || len(f.Holes) > 0
	}
	if !innerWire {
		t.Fatal("fixture did not retain nested floor wire")
	}
	model := SideOpeningCutsModel{Body: p, LengthUnit: .001}
	var out bytes.Buffer
	if e := WriteSideOpeningCuts(&out, model); e != nil {
		t.Fatal(e)
	}
	got, e := ReadSideOpeningCuts(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(got, model) {
		t.Fatal(got, e)
	}
	checkSideOpeningCutsOCCT(t, p, out.Bytes())
}

func TestSideOpeningCutsSTEPRedundantSiteCanonicalBytes(t *testing.T) {
	t.Parallel()
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := SideOpeningCutsBody{Name: "canonical redundant site", Frame: frame, Width: 20, Back: 10, Length: 8, Cuts: []trimmed.SideOpeningCut{
		{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 6}, Radius: 3}, Depth: 3},
		{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 10}, Radius: 3}, Depth: 3},
	}}
	model := SideOpeningCutsModel{Body: p, LengthUnit: .001}
	var expected bytes.Buffer
	if e := WriteSideOpeningCuts(&expected, model); e != nil {
		t.Fatal(e)
	}
	redundant := model
	redundant.Body.Cuts = append([]trimmed.SideOpeningCut{{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5.5}, Radius: 1}, Depth: 2}}, p.Cuts...)
	var actual bytes.Buffer
	if e := WriteSideOpeningCuts(&actual, redundant); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(actual.Bytes(), expected.Bytes()) {
		t.Fatal("redundant site changed canonical STEP graph")
	}
	got, e := ReadSideOpeningCuts(bytes.NewReader(actual.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(got, model) {
		t.Fatal(got, e)
	}
	checkSideOpeningCutsOCCT(t, p, actual.Bytes())
}

func checkCornerSideOpeningSTEP(t *testing.T, p SideOpeningCutsBody) {
	t.Helper()
	model := SideOpeningCutsModel{Body: p, LengthUnit: .001}
	var out bytes.Buffer
	if e := WriteSideOpeningCuts(&out, model); e != nil {
		t.Fatal(e)
	}
	got, e := ReadSideOpeningCuts(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(got, model) {
		t.Fatal(got, e)
	}
	reordered := model
	reordered.Body.Cuts = append([]trimmed.SideOpeningCut(nil), p.Cuts...)
	for i, j := 0, len(p.Cuts)-1; i < j; i, j = i+1, j-1 {
		reordered.Body.Cuts[i], reordered.Body.Cuts[j] = reordered.Body.Cuts[j], reordered.Body.Cuts[i]
	}
	var reverse bytes.Buffer
	if e := WriteSideOpeningCuts(&reverse, reordered); e != nil || !bytes.Equal(out.Bytes(), reverse.Bytes()) {
		t.Fatal("noncanonical corner cutter order", e)
	}
	if len(p.Cuts) == 1 {
		if _, e := ReadSideOpeningPair(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
			t.Fatal("legacy pair reader accepted single support")
		}
	}
	checkSideOpeningCutsOCCT(t, p, out.Bytes())
}

func TestCornerSideOpeningCutsSTEP(t *testing.T) {
	slowtest.Skip(t)
	normals := []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	for ni, n := range normals {
		frame, e := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		if e != nil {
			t.Fatal(e)
		}
		for _, family := range []struct {
			name   string
			second geom.Vec2
			radius float64
		}{
			{"corner-side", geom.Vec2{X: 5}, 3}, {"two-offset-corners", geom.Vec2{X: 1, Y: 1}, 3},
		} {
			for _, span := range []struct {
				name    string
				depths  [2]float64
				opposed bool
			}{
				{"equal-blind", [2]float64{4, 4}, false}, {"unequal-blind", [2]float64{3, 6}, false},
				{"through", [2]float64{8, 8}, false}, {"through-blind", [2]float64{8, 5}, false}, {"opposed", [2]float64{6, 5}, true},
			} {
				t.Run(fmt.Sprintf("%s/%s/%d", family.name, span.name, ni), func(t *testing.T) {
					p := SideOpeningCutsBody{Name: "corner and side openings", Frame: frame, Width: 20, Back: 20, Length: 8, Cuts: []trimmed.SideOpeningCut{
						{ExtrusionCircle: trimmed.ExtrusionCircle{Radius: 4}, Depth: span.depths[0]},
						{ExtrusionCircle: trimmed.ExtrusionCircle{Center: family.second, Radius: family.radius}, Depth: span.depths[1], OppositeCap: span.opposed},
					}}
					checkCornerSideOpeningSTEP(t, p)
				})
			}
		}
		t.Run(fmt.Sprintf("equal-radius-offset-corners/%d", ni), func(t *testing.T) {
			p := SideOpeningCutsBody{Name: "buried corner opening", Frame: frame, Width: 20, Back: 20, Length: 8, Cuts: []trimmed.SideOpeningCut{
				{ExtrusionCircle: trimmed.ExtrusionCircle{Radius: 4}, Depth: 6},
				{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 1, Y: 1}, Radius: 4}, Depth: 3},
			}}
			checkCornerSideOpeningSTEP(t, p)
		})
		for _, single := range []struct {
			name          string
			center        geom.Vec2
			radius, depth float64
			opposite      bool
		}{
			{"blind", geom.Vec2{X: 1, Y: 1}, 4, 3, false}, {"opposite-blind", geom.Vec2{X: 1, Y: 1}, 4, 3, true},
			{"through", geom.Vec2{X: 1, Y: 1}, 4, 8, false}, {"small-external-clip", geom.Vec2{X: -2, Y: -2}, 3, 8, false},
		} {
			t.Run(fmt.Sprintf("single/%s/%d", single.name, ni), func(t *testing.T) {
				p := SideOpeningCutsBody{Name: "single offset corner", Frame: frame, Width: 20, Back: 20, Length: 8, Cuts: []trimmed.SideOpeningCut{{ExtrusionCircle: trimmed.ExtrusionCircle{Center: single.center, Radius: single.radius}, Depth: single.depth, OppositeCap: single.opposite}}}
				checkCornerSideOpeningSTEP(t, p)
			})
		}
		t.Run(fmt.Sprintf("no-original-corner-vertices/%d", ni), func(t *testing.T) {
			p := SideOpeningCutsBody{Name: "all original corners removed", Frame: frame, Width: 20, Back: 20, Length: 8}
			for i, c := range []geom.Vec2{{}, {Y: 20}, {X: 5}, {X: 20}, {X: 20, Y: 20}} {
				r := 3.
				if i == 0 {
					r = 4
				}
				p.Cuts = append(p.Cuts, trimmed.SideOpeningCut{ExtrusionCircle: trimmed.ExtrusionCircle{Center: c, Radius: r}, Depth: 8})
			}
			native, e := p.Solid(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			for _, v := range native.Vertices() {
				q := frame.ToLocal(v)
				if (q.X == 0 || q.X == 20) && (q.Y == 0 || q.Y == 20) {
					t.Fatal("original corner survived", q)
				}
			}
			checkCornerSideOpeningSTEP(t, p)
		})
	}
}

func TestSideOpeningPairWriterRejectsSingleRecoveredSupport(t *testing.T) {
	t.Parallel()
	frame, err := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	cut := trimmed.SideOpeningCut{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 1, Y: 1}, Radius: 4}, Depth: 3}
	model := SideOpeningPairModel{Body: SideOpeningPairBody{Name: "duplicate corner cutters", Frame: frame, Width: 20, Back: 20, Length: 8, Cuts: [2]trimmed.SideOpeningCut{cut, cut}}, LengthUnit: .001}
	var out bytes.Buffer
	if err := WriteSideOpeningPair(&out, model); err == nil || out.Len() != 0 {
		t.Fatal("pair writer published a single recovered support", err, out.Len())
	}
}

func TestVertexSideOpeningCutsSTEP(t *testing.T) {
	slowtest.Skip(t)
	for ni, normal := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		frame, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, normal)
		if err != nil {
			t.Fatal(err)
		}
		for _, site := range []struct {
			name   string
			center geom.Vec2
		}{
			{"lower-left", geom.Vec2{X: 3, Y: 4}},
			{"upper-right", geom.Vec2{X: 17, Y: 16}},
			{"outside-x", geom.Vec2{X: -3, Y: 4}},
			{"outside-y", geom.Vec2{X: 3, Y: -4}},
			{"endpoint-tangent-x", geom.Vec2{X: 5}},
			{"endpoint-tangent-y", geom.Vec2{Y: 5}},
		} {
			for _, depth := range []float64{3, 8} {
				t.Run(fmt.Sprintf("single/%s/%g/%d", site.name, depth, ni), func(t *testing.T) {
					p := SideOpeningCutsBody{Name: "exact stock vertex opening", Frame: frame, Width: 20, Back: 20, Length: 8, Cuts: []trimmed.SideOpeningCut{{ExtrusionCircle: trimmed.ExtrusionCircle{Center: site.center, Radius: 5}, Depth: depth}}}
					checkCornerSideOpeningSTEP(t, p)
				})
			}
		}
		for _, span := range []struct {
			name    string
			depths  [2]float64
			opposed bool
		}{
			{"blind", [2]float64{3, 6}, false}, {"through", [2]float64{8, 8}, false}, {"opposed", [2]float64{6, 5}, true},
		} {
			t.Run(fmt.Sprintf("group/%s/%d", span.name, ni), func(t *testing.T) {
				p := SideOpeningCutsBody{Name: "vertex and side openings", Frame: frame, Width: 20, Back: 20, Length: 8, Cuts: []trimmed.SideOpeningCut{
					{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 3, Y: 4}, Radius: 5}, Depth: span.depths[0]},
					{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 8}, Radius: 3}, Depth: span.depths[1], OppositeCap: span.opposed},
				}}
				checkCornerSideOpeningSTEP(t, p)
			})
		}
		t.Run(fmt.Sprintf("external-point-only/%d", ni), func(t *testing.T) {
			p := SideOpeningCutsBody{Name: "external point contact", Frame: frame, Width: 20, Back: 20, Length: 8, Cuts: []trimmed.SideOpeningCut{{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: -3, Y: -4}, Radius: 5}, Depth: 8}}}
			var out bytes.Buffer
			if err := WriteSideOpeningCuts(&out, SideOpeningCutsModel{Body: p, LengthUnit: .001}); err == nil || out.Len() != 0 {
				t.Fatal("point-only external contact published a cut", err, out.Len())
			}
		})
	}
}

func TestVertexSideOpeningSharedIntersectionSTEP(t *testing.T) {
	t.Parallel()
	for ni, normal := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		t.Run(fmt.Sprint(ni), func(t *testing.T) {
			frame, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, normal)
			if err != nil {
				t.Fatal(err)
			}
			p := SideOpeningCutsBody{Name: "shared exact vertex", Frame: frame, Width: 20, Back: 20, Length: 8, Cuts: []trimmed.SideOpeningCut{
				{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: -4, Y: 3}, Radius: 5}, Depth: 5},
				{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 3, Y: 4}, Radius: 5}, Depth: 2},
			}}
			checkCornerSideOpeningSTEP(t, p)
		})
	}
}

func TestSideOpeningCutsSTEPConcealedCanonicalBytes(t *testing.T) {
	slowtest.Skip(t)
	for ni, normal := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		frame, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, normal)
		if err != nil {
			t.Fatal(err)
		}
		for _, fixture := range []struct {
			name    string
			visible []trimmed.SideOpeningCut
			hidden  trimmed.SideOpeningCut
		}{
			{"disjoint-visible", []trimmed.SideOpeningCut{{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 3, Y: 4}, Radius: 5}, Depth: 5}, {ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 14}, Radius: 2}, Depth: 4}}, trimmed.SideOpeningCut{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: -4, Y: 3}, Radius: 5}, Depth: 2}},
			{"vertex", []trimmed.SideOpeningCut{{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 3, Y: 4}, Radius: 5}, Depth: 5}}, trimmed.SideOpeningCut{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: -4, Y: 3}, Radius: 5}, Depth: 2}},
			{"union-same-depth", []trimmed.SideOpeningCut{{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 8}, Radius: 3}, Depth: 4}, {ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 12}, Radius: 3}, Depth: 4}}, trimmed.SideOpeningCut{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 10}, Radius: 2}, Depth: 2}},
			{"union-varied-depth", []trimmed.SideOpeningCut{{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 8}, Radius: 3}, Depth: 4}, {ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 12}, Radius: 3}, Depth: 6}}, trimmed.SideOpeningCut{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 10}, Radius: 2}, Depth: 2}},
		} {
			t.Run(fmt.Sprintf("%s/%d", fixture.name, ni), func(t *testing.T) {
				p := SideOpeningCutsBody{Name: "concealed opening", Frame: frame, Width: 20, Back: 20, Length: 8, Cuts: fixture.visible}
				model := SideOpeningCutsModel{Body: p, LengthUnit: .001}
				var expected bytes.Buffer
				if err := WriteSideOpeningCuts(&expected, model); err != nil {
					t.Fatal(err)
				}
				for _, first := range []bool{false, true} {
					augmented := model
					augmented.Body.Cuts = append([]trimmed.SideOpeningCut(nil), p.Cuts...)
					if first {
						augmented.Body.Cuts = append([]trimmed.SideOpeningCut{fixture.hidden}, augmented.Body.Cuts...)
					} else {
						augmented.Body.Cuts = append(augmented.Body.Cuts, fixture.hidden)
					}
					var actual bytes.Buffer
					if err := WriteSideOpeningCuts(&actual, augmented); err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(expected.Bytes(), actual.Bytes()) {
						t.Fatal("concealed cutter changed canonical STEP bytes")
					}
				}
				got, err := ReadSideOpeningCuts(bytes.NewReader(expected.Bytes()), Limits{})
				if err != nil || !reflect.DeepEqual(got, model) {
					t.Fatal(got, err)
				}
				oracleBody := p
				oracleBody.Cuts = append(append([]trimmed.SideOpeningCut(nil), p.Cuts...), fixture.hidden)
				checkSideOpeningCutsOCCT(t, oracleBody, expected.Bytes())
			})
		}
	}
}

func TestSideOpeningCutsSTEPOneCapBuriedThrough(t *testing.T) {
	t.Parallel()
	for ni, normal := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		t.Run(fmt.Sprint(ni), func(t *testing.T) {
			frame, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, normal)
			if err != nil {
				t.Fatal(err)
			}
			p := SideOpeningCutsBody{Name: "one cap buried through cutter", Frame: frame, Width: 20, Back: 20, Length: 8, Cuts: []trimmed.SideOpeningCut{
				{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 8}, Radius: 3}, Depth: 6},
				{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 10}, Radius: 1.75}, Depth: 8},
				{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 12}, Radius: 3}, Depth: 6},
			}}
			checkCornerSideOpeningSTEP(t, p)
		})
	}
}

func TestSideOpeningCutsSTEPMeetingOppositeFloors(t *testing.T) {
	slowtest.Skip(t)
	for ni, normal := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		frame, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, normal)
		if err != nil {
			t.Fatal(err)
		}
		for _, fixture := range []struct {
			name    string
			back    float64
			centers [2]geom.Vec2
			radii   [2]float64
		}{
			{"same-side", 20, [2]geom.Vec2{{X: 6}, {X: 10}}, [2]float64{3, 3}},
			{"adjacent-sides", 20, [2]geom.Vec2{{Y: 3.5}, {X: 3.5}}, [2]float64{3, 3}},
			{"opposite-sides", 8, [2]geom.Vec2{{X: 8, Y: 1.5}, {X: 9, Y: 6.5}}, [2]float64{3, 3}},
			{"corner-side", 20, [2]geom.Vec2{{}, {X: 5}}, [2]float64{4, 3}},
		} {
			for _, depth := range []float64{3, 4} {
				t.Run(fmt.Sprintf("%s/%g/%d", fixture.name, depth, ni), func(t *testing.T) {
					p := SideOpeningCutsBody{Name: "meeting opposite blind floors", Frame: frame, Width: 20, Back: fixture.back, Length: 8, Cuts: []trimmed.SideOpeningCut{
						{ExtrusionCircle: trimmed.ExtrusionCircle{Center: fixture.centers[0], Radius: fixture.radii[0]}, Depth: depth},
						{ExtrusionCircle: trimmed.ExtrusionCircle{Center: fixture.centers[1], Radius: fixture.radii[1]}, Depth: 8 - depth, OppositeCap: true},
					}}
					checkCornerSideOpeningSTEP(t, p)
				})
			}
		}
	}
}
