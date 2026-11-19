package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestReadMixedConicalAP242Assembly(t *testing.T) {
	slowtest.Skip(t)
	ctx, options := context.Background(), geom.SurfacePatchOptions{}
	raw := mixedConicalAP242Fixture(t, "exact")
	a, err := ReadMixedConicalAP242Assembly(ctx, bytes.NewReader(raw), Limits{}, options)
	if err != nil {
		t.Fatal(err)
	}
	if a.Assembly.LengthUnit != .001 || len(a.Parts) != 2 || len(a.Occurrences) != 3 || a.Assembly.Roots[0] != 5 {
		t.Fatalf("lost source unit or mixed product structure: %v", a.Assembly.Roots)
	}
	rounded, chamfered := a.Parts[35], a.Parts[200]
	if rounded.Rounded == nil || rounded.Chamfered != nil || chamfered.Rounded != nil || chamfered.Chamfered == nil ||
		rounded.Product.Name != "Conical rounded through-bore" || chamfered.Product.Name != "Chamfered eccentric annular stock" {
		t.Fatal("changed original material definitions or selected the wrong local proof")
	}
	owner := map[int64][]int64{35: rounded.Rounded.NativeFaceSourceIDs, 200: chamfered.Chamfered.NativeFaceSourceIDs}
	for _, faces := range owner {
		for _, id := range faces {
			if _, err := simple(a.Assembly.Source, id, "ADVANCED_FACE", 4); err != nil {
				t.Fatalf("native face owner #%d is missing from the original assembly", id)
			}
		}
	}
	native := map[int64]trimmed.Solid{35: rounded.Rounded.Solid, 200: chamfered.Chamfered.Solid}
	for product, counts := range map[int64][3]int{35: {6, 28, 24}, 200: {8, 38, 32}} {
		top := native[product].Topology()
		if [3]int{top.Faces, top.Edges, top.Vertices} != counts || top.InnerWires != 2 || top.EulerCharacteristic != 0 {
			t.Fatalf("changed local body topology: %v", top)
		}
	}
	assertNative := func(variant string, expected trimmed.Solid, read func([]byte) (trimmed.Solid, error)) {
		t.Helper()
		source, err := os.ReadFile(filepath.Join("testdata", variant))
		if err != nil {
			t.Fatal(err)
		}
		b, err := read(source)
		if err != nil {
			t.Fatal(err)
		}
		var want, got bytes.Buffer
		if err := trimmed.WriteJSON(&want, expected); err != nil {
			t.Fatal(err)
		}
		if err := trimmed.WriteJSON(&got, b); err != nil || !bytes.Equal(got.Bytes(), want.Bytes()) {
			t.Fatalf("placement changed the material bytes for %s: %v", variant, err)
		}
	}
	assertNative("occt-conical-double-rim-eccentric-bore-exact.step", native[35], func(data []byte) (trimmed.Solid, error) {
		p, err := ReadConicalRimFilletBore(ctx, bytes.NewReader(data), Limits{}, options)
		return p.Solid, err
	})
	assertNative("occt-eccentric-annular-chamfer-exact.step", native[200], func(data []byte) (trimmed.Solid, error) {
		p, err := ReadEccentricAnnularChamfer(ctx, bytes.NewReader(data), Limits{}, options)
		return p.Solid, err
	})

	want := [3][3][4]string{
		{{"1", "0", "0", "30"}, {"0", "1", "0", "2"}, {"0", "0", "1", "4"}},
		{{"0", "0", "1", "-30"}, {"1", "0", "0", "24"}, {"0", "1", "0", "3"}},
		{{"3/5", "-4/5", "0", "0"}, {"4/5", "3/5", "0", "-30"}, {"0", "0", "1", "8"}},
	}
	stats := make(map[int64]parametric.Properties, len(native))
	for id, body := range native {
		p, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil || p.Volume <= 0 {
			t.Fatalf("invalid local signed material: %v %v", p, err)
		}
		stats[id] = p
	}
	var measurements []mixedConicalOccurrenceMeasure
	for i, u := range a.Occurrences {
		if u.NAUOID != [3]int64{191, 197, 389}[i] || u.ChildID != [3]int64{35, 35, 200}[i] || u.ParentID != 5 {
			t.Fatalf("lost original NAUO ownership: %v", u)
		}
		var measurement mixedConicalOccurrenceMeasure
		matrix := u.Matrix()
		for row := range matrix {
			for col := range matrix[row] {
				q, _ := new(big.Rat).SetString(want[i][row][col])
				if matrix[row][col].Cmp(q) != 0 {
					t.Fatalf("original pose %d changed: %v", u.NAUOID, matrix)
				}
				measurement.Pose[row*4+col], _ = q.Float64()
			}
		}
		local := stats[u.ChildID]
		world, err := u.ApplyExact(local.Centroid)
		if err != nil {
			t.Fatal(err)
		}
		for j := range world {
			measurement.Centroid[j], _ = world[j].Float64()
		}
		measurement.SharedWithFirst = i < 2
		measurement.Volume, measurement.Area = local.Volume, local.Area
		measurement.Topology = [4]int{1, 6, 10, 6}
		if i == 2 {
			measurement.Topology = [4]int{1, 8, 14, 8}
		}
		measurements = append(measurements, measurement)
	}
	if cmd := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242"); cmd != "" {
		checkMixedConicalAP242Live(t, cmd, raw, measurements)
	}
}

func TestReadMixedConicalAP242AssemblyReject(t *testing.T) {
	slowtest.Skip(t)
	ctx, o := context.Background(), geom.SurfacePatchOptions{}
	exact := mixedConicalAP242Fixture(t, "exact")
	rounded := mixedConicalAP242Fixture(t, "rounded")
	if a, err := ReadAssembly(bytes.NewReader(rounded), Limits{}); err != nil || len(a.Occurrences) != 3 {
		t.Fatalf("default independent assembly stopped exercising material authentication: %v", err)
	}
	if _, err := ReadIndependentAnalyticHierarchy(ctx, bytes.NewReader(exact), Limits{}, 1e-9); err == nil {
		t.Fatal("legacy independent hierarchy unexpectedly accepted the mixed source")
	}
	for _, c := range []struct {
		name, before, after string
		source              []byte
		l                   Limits
		checkRoundedPose    bool
	}{
		{name: "default rounded body supports", source: rounded},
		{name: "whole assembly face budget", source: exact, l: Limits{MaxFaces: 13}},
		{name: "lower source torus no longer tangent", before: "TOROIDAL_SURFACE('',#95,7.6875,0.9375)", after: "TOROIDAL_SURFACE('',#95,7.75,0.9375)"},
		{name: "inexact rational rotation hidden by binary rounding", before: "#26 = DIRECTION('',(0.6,0.8,0.));", after: "#26 = DIRECTION('',(0.6,0.800000000000000001,0.));", checkRoundedPose: true},
		{name: "wrong repeated child owner", before: "#197 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('2','=>[0:1:1:2]','',#5,#35,$);", after: "#197 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('2','=>[0:1:1:2]','',#5,#200,$);"},
		{name: "duplicated transform owner", before: "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION(#195)", after: "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION(#189)"},
		{name: "shared leaf direction with unchanged physical coordinates", before: "#56 = AXIS2_PLACEMENT_3D('',#57,#58,#59);", after: "#56 = AXIS2_PLACEMENT_3D('',#57,#223,#59);"},
		{name: "mismatched assembly length unit", before: "#28 = ( LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.) );", after: "#28 = ( LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.CENTI.,.METRE.) );"},
		{name: "detached geometry", before: "ENDSEC;\nEND-ISO-10303-21;", after: "#801 = CARTESIAN_POINT('',(0.,0.,0.));\nENDSEC;\nEND-ISO-10303-21;"},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := c.source
			if p == nil {
				if bytes.Count(exact, []byte(c.before)) != 1 {
					t.Fatalf("mutation does not target a unique original source value: %s", c.before)
				}
				p = bytes.Replace(exact, []byte(c.before), []byte(c.after), 1)
			}
			if c.checkRoundedPose {
				a, err := ReadAssembly(bytes.NewReader(p), Limits{})
				want, e := ReadAssembly(bytes.NewReader(exact), Limits{})
				if err != nil || e != nil || len(a.Occurrences) != 3 || a.Occurrences[2].Transform != want.Occurrences[2].Transform {
					t.Fatalf("control did not preserve the legacy rounded placement: %v, %v", err, e)
				}
			}
			got, err := ReadMixedConicalAP242Assembly(ctx, bytes.NewReader(p), c.l, o)
			if err == nil || len(got.Parts) != 0 || got.Assembly.Source != nil {
				t.Fatalf("accepted unproved source or exposed a partial assembly: %v", err)
			}
		})
	}
}

type mixedConicalOccurrenceMeasure struct {
	SharedWithFirst, Valid bool
	Volume, Area           float64
	Centroid               [3]float64
	Topology               [4]int
	Pose                   [12]float64
}

func TestMixedConicalAP242PinnedAuthor(t *testing.T) {
	t.Parallel()
	cmd := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242")
	if cmd == "" {
		t.Skip("pinned mixed conical AP242 author not configured")
	}
	for _, c := range []struct{ mode, fixture string }{{"write", "rounded"}, {"write-exact", "exact"}} {
		path := filepath.Join(t.TempDir(), "mixed.step")
		if out, err := exec.Command(cmd, c.mode, path).CombinedOutput(); err != nil {
			t.Fatalf("pinned author %s: %v\n%s", c.mode, err, out)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := mixedConicalAP242Fixture(t, c.fixture)
		i, j := bytes.Index(got, []byte("\nDATA;\n")), bytes.Index(want, []byte("\nDATA;\n"))
		if i < 0 || j < 0 || !bytes.Equal(got[i:], want[j:]) {
			t.Fatalf("pinned writer changed the %s assembly DATA graph", c.fixture)
		}
	}
}

func mixedConicalAP242Fixture(t *testing.T, kind string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/occt-mixed-conical-curved-ap242-" + kind + ".step")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func checkMixedConicalAP242Live(t *testing.T, cmd string, raw []byte, expected []mixedConicalOccurrenceMeasure) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "assembly.step")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	pinned, err := exec.Command(cmd, "read", path).CombinedOutput()
	start := bytes.Index(pinned, []byte(`{"occurrences":`))
	var result struct {
		Occurrences []mixedConicalOccurrenceMeasure
	}
	if err != nil || start < 0 || json.Unmarshal(pinned[start:], &result) != nil || len(result.Occurrences) != len(expected) {
		t.Fatalf("pinned AP242 transfer: %v\n%s", err, pinned)
	}
	for i, got := range result.Occurrences {
		want := expected[i]
		if !got.Valid || got.SharedWithFirst != want.SharedWithFirst || got.Topology != want.Topology ||
			got.Pose != want.Pose || got.Volume <= 0 || math.Abs(got.Volume-want.Volume) > 1e-8*want.Volume ||
			math.Abs(got.Area-want.Area) > 1e-8*want.Area {
			t.Fatalf("pinned AP242 occurrence %d changed identity, pose or signed material: %v", i, got)
		}
		for d, q := range got.Centroid {
			if math.Abs(q-want.Centroid[d]) > .0002 {
				t.Fatalf("pinned world centroid outside comparison bound: %v versus %v", got.Centroid, want.Centroid)
			}
		}
	}
}
