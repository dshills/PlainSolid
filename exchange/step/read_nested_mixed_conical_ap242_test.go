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

func TestReadNestedMixedConicalAP242Assembly(t *testing.T) {
	slowtest.Skip(t)
	ctx, o := context.Background(), geom.SurfacePatchOptions{}
	raw := nestedMixedConicalAP242Fixture(t, "exact")
	// These limits can hold exactly the two original material definitions.
	a, err := ReadNestedMixedConicalAP242Assembly(ctx, bytes.NewReader(raw), Limits{MaxFaces: 14, MaxVertices: 56, MaxEdgeUses: 132}, o)
	if err != nil {
		t.Fatal(err)
	}
	if a.Assembly.LengthUnit != .001 || a.Assembly.Roots[0] != 5 || a.Group.DefinitionID != 31 ||
		a.Group.Name != "Original mixed conical group" || len(a.Parts) != 2 || len(a.Occurrences) != 5 || len(a.Instances) != 6 {
		t.Fatalf("changed original AP242 group or product multiplicities: %v", a.Group)
	}
	if a.Parts[62].Rounded == nil || a.Parts[227].Chamfered == nil {
		t.Fatal("lost an original local product")
	}
	flat, err := ReadMixedConicalAP242Assembly(ctx, bytes.NewReader(mixedConicalAP242Fixture(t, "exact")), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	local := map[int64]trimmed.Solid{62: a.Parts[62].Rounded.Solid, 227: a.Parts[227].Chamfered.Solid}
	props := map[int64]parametric.Properties{}
	for id, want := range map[int64]trimmed.Solid{62: flat.Parts[35].Rounded.Solid, 227: flat.Parts[200].Chamfered.Solid} {
		var actual, independent bytes.Buffer
		if err := trimmed.WriteJSON(&actual, local[id]); err != nil {
			t.Fatal(err)
		}
		if err := trimmed.WriteJSON(&independent, want); err != nil || !bytes.Equal(actual.Bytes(), independent.Bytes()) {
			t.Fatalf("nested source changed local product %d: %v", id, err)
		}
		p, err := local[id].MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil || p.Volume <= 0 {
			t.Fatalf("invalid local signed material: %v %v", p, err)
		}
		props[id] = p
	}
	want := [6][3][4]string{
		{{"3/5", "-4/5", "0", "582/5"}, {"4/5", "3/5", "0", "226/5"}, {"0", "0", "1", "16"}},
		{{"-4/5", "0", "3/5", "314/5"}, {"3/5", "0", "4/5", "52/5"}, {"0", "1", "0", "15"}},
		{{"-7/25", "-24/25", "0", "124"}, {"24/25", "-7/25", "0", "2"}, {"0", "0", "1", "20"}},
		{{"1", "0", "0", "-70"}, {"0", "3/5", "-4/5", "-22"}, {"0", "4/5", "3/5", "34"}},
		{{"0", "0", "1", "-130"}, {"3/5", "-4/5", "0", "-8"}, {"4/5", "3/5", "0", "51"}},
		{{"3/5", "-4/5", "0", "-100"}, {"12/25", "9/25", "-4/5", "-222/5"}, {"16/25", "12/25", "3/5", "54/5"}},
	}
	var expected []nestedMixedOccurrenceMeasure
	for i, instance := range a.Instances {
		outer, inner := i/3, i%3
		path := [2]int64{[2]int64{58, 422}[outer], [3]int64{218, 224, 416}[inner]}
		if instance.Path != path || instance.ProductID != [3]int64{62, 62, 227}[inner] {
			t.Fatalf("lost original outer/inner occurrence identity: %v", instance.Path)
		}
		m := nestedMixedOccurrenceMeasure{Path: [2]int{outer + 1, inner + 1}}
		actual := instance.Matrix()
		for row := range actual {
			for col := range actual[row] {
				q, _ := new(big.Rat).SetString(want[i][row][col])
				if actual[row][col].Cmp(q) != 0 {
					t.Fatalf("world placement %v changed: %v", path, actual)
				}
				m.Pose[row*4+col], _ = q.Float64()
			}
		}
		p := props[instance.ProductID]
		world, err := instance.ApplyExact(p.Centroid)
		if err != nil {
			t.Fatal(err)
		}
		for j := range world {
			m.Centroid[j], _ = world[j].Float64()
		}
		m.Volume, m.Area, m.SharedWithFirst = p.Volume, p.Area, inner < 2
		m.Topology = [4]int{1, 6, 10, 6}
		if inner == 2 {
			m.Topology = [4]int{1, 8, 14, 8}
		}
		expected = append(expected, m)
	}
	x, err := a.Instances[5].ApplyExact(geom.Vec3{X: 1.5, Y: 2, Z: 4})
	if err != nil || x[0].RatString() != "-1007/10" || x[1].RatString() != "-1154/25" || x[2].RatString() != "378/25" {
		t.Fatalf("lost exact world point through two levels: %v %v", x, err)
	}
	matrix := a.Instances[5].Matrix()
	matrix[0][0].SetInt64(0)
	if got := a.Instances[5].Matrix()[0][0].RatString(); got != "3/5" {
		t.Fatalf("world matrix aliases its returned copy: %s", got)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242"); oracle != "" {
		checkNestedMixedConicalAP242Live(t, oracle, raw, expected)
	}
}

func TestReadNestedMixedConicalAP242AssemblyReject(t *testing.T) {
	t.Parallel()
	ctx, o := context.Background(), geom.SurfacePatchOptions{}
	exact := nestedMixedConicalAP242Fixture(t, "exact")
	if _, err := ReadMixedConicalAP242Assembly(ctx, bytes.NewReader(exact), Limits{}, o); err == nil {
		t.Fatal("flat reader unexpectedly accepted the actual nested fixture")
	}
	for _, c := range []struct {
		name        string
		input       []byte
		changes     [][2]string
		validCommon bool
	}{
		{name: "actual default carrier", input: nestedMixedConicalAP242Fixture(t, "rounded")},
		{name: "flat source", input: mixedConicalAP242Fixture(t, "exact")},
		{name: "misattached direct leaf with a valid common product graph", validCommon: true, changes: [][2]string{
			{"#422 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('5','=>[0:1:1:4]','',#5,#31,$);", "#422 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('5','=>[0:1:1:4]','',#5,#62,$);"},
			{"#419 = ( REPRESENTATION_RELATIONSHIP('','',#36,#10)", "#419 = ( REPRESENTATION_RELATIONSHIP('','',#67,#10)"},
		}},
		{name: "inexact outer rotation below binary64 resolution", changes: [][2]string{
			{"#18 = DIRECTION('',(0.6,0.8,0.));", "#18 = DIRECTION('',(0.6,0.800000000000000001,0.));"},
		}},
		{name: "shared outer transformation record", changes: [][2]string{
			{"REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION(#420)", "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION(#56)"},
		}},
		{name: "inexact original inner rotation", changes: [][2]string{
			{"#48 = DIRECTION('',(0.6,0.8,0.));", "#48 = DIRECTION('',(0.6,0.800000000000000001,0.));"},
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			source := c.input
			if source == nil {
				source = bytes.Clone(exact)
			}
			for _, change := range c.changes {
				if bytes.Count(source, []byte(change[0])) != 1 {
					t.Fatalf("mutation does not identify one original record: %s", change[0])
				}
				source = bytes.Replace(source, []byte(change[0]), []byte(change[1]), 1)
			}
			if c.validCommon {
				if _, err := ReadAssembly(bytes.NewReader(source), Limits{}); err != nil {
					t.Fatalf("control did not preserve a valid common graph: %v", err)
				}
			}
			got, err := ReadNestedMixedConicalAP242Assembly(ctx, bytes.NewReader(source), Limits{}, o)
			if err == nil || len(got.Instances) != 0 || len(got.Parts) != 0 || got.Assembly.Source != nil {
				t.Fatalf("unproved source returned material: %v", err)
			}
		})
	}
}

type nestedMixedOccurrenceMeasure struct {
	mixedConicalOccurrenceMeasure
	Path [2]int
}

func TestNestedMixedConicalAP242PinnedAuthor(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242")
	if oracle == "" {
		t.Skip("pinned mixed conical AP242 author not configured")
	}
	for _, c := range []struct{ mode, fixture string }{{"write-nested", "rounded"}, {"write-nested-exact", "exact"}} {
		path := filepath.Join(t.TempDir(), "nested.step")
		if out, err := exec.Command(oracle, c.mode, path).CombinedOutput(); err != nil {
			t.Fatalf("pinned author %s: %v\n%s", c.mode, err, out)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := nestedMixedConicalAP242Fixture(t, c.fixture)
		i, j := bytes.Index(got, []byte("\nDATA;\n")), bytes.Index(want, []byte("\nDATA;\n"))
		if i < 0 || j < 0 || !bytes.Equal(got[i:], want[j:]) {
			t.Fatalf("pinned writer changed the %s nested assembly DATA graph", c.fixture)
		}
	}
}

func nestedMixedConicalAP242Fixture(t *testing.T, kind string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/occt-nested-mixed-conical-curved-ap242-" + kind + ".step")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func checkNestedMixedConicalAP242Live(t *testing.T, oracle string, raw []byte, expected []nestedMixedOccurrenceMeasure) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nested.step")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	pinned, err := exec.Command(oracle, "read-nested", path).CombinedOutput()
	start := bytes.Index(pinned, []byte(`{"groupsShared":`))
	var hit struct {
		GroupsShared bool
		Occurrences  []nestedMixedOccurrenceMeasure
	}
	if err != nil || start < 0 || json.Unmarshal(pinned[start:], &hit) != nil || !hit.GroupsShared || len(hit.Occurrences) != 6 {
		t.Fatalf("pinned XCAF hierarchy transfer: %v\n%s", err, pinned)
	}
	for i, got := range hit.Occurrences {
		want := expected[i]
		if !got.Valid || got.Path != want.Path || got.SharedWithFirst != want.SharedWithFirst || got.Topology != want.Topology ||
			got.Volume <= 0 || math.Abs(got.Volume-want.Volume) > 1e-8*want.Volume || math.Abs(got.Area-want.Area) > 1e-8*want.Area {
			t.Fatalf("pinned occurrence %v changed identity or signed material: %v", want.Path, got)
		}
		for j, v := range got.Pose {
			if math.Abs(v-want.Pose[j]) > 1e-12 {
				t.Fatalf("pinned occurrence %v changed world pose: %v", want.Path, got.Pose)
			}
		}
		for j, v := range got.Centroid {
			if math.Abs(v-want.Centroid[j]) > .0002 {
				t.Fatalf("pinned occurrence %v world centroid outside comparison bound: %v versus %v", want.Path, got.Centroid, want.Centroid)
			}
		}
	}
}
