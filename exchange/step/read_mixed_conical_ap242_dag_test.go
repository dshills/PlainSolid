package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestReadMixedConicalAP242DAG(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	data := mixedConicalAP242DAGFixture(t, "exact")
	if _, err := ReadMixedConicalAP242Assembly(ctx, bytes.NewReader(data), Limits{}, geom.SurfacePatchOptions{}); err == nil {
		t.Fatal("flat reader accepted the depth-three fixture")
	}
	if _, err := ReadNestedMixedConicalAP242Assembly(ctx, bytes.NewReader(data), Limits{}, geom.SurfacePatchOptions{}); err == nil {
		t.Fatal("two-level reader accepted the depth-three fixture")
	}
	l := MixedConicalAP242DAGLimits{
		Source:             Limits{MaxFaces: 14, MaxVertices: 56, MaxEdgeUses: 132, MaxSolids: 6},
		MaxOccurrenceDepth: 3, MaxLeafInstances: 7,
	}
	got, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(data), l, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Assembly.LengthUnit != .001 || !slices.Equal(got.Assembly.Roots, []int64{5}) || len(got.Assembly.Products) != 6 ||
		len(got.Parts) != 2 || len(got.Occurrences) != 8 || len(got.Instances) != 7 {
		t.Fatalf("lost the bounded product DAG: %v", got.Assembly.Products)
	}
	incomingCore := []int64{}
	var relationships []int64
	for _, u := range got.Occurrences {
		relationships = append(relationships, u.NAUOID)
		if u.ChildID == 58 {
			incomingCore = append(incomingCore, u.ParentID)
		}
	}
	if !slices.Equal(incomingCore, []int64{35, 447}) ||
		!slices.Equal(relationships, []int64{54, 85, 245, 251, 443, 466, 472, 477}) {
		t.Fatalf("lost shared core or original occurrence identities: %v %v", incomingCore, relationships)
	}
	if got.Parts[89].Rounded == nil || got.Parts[254].Chamfered == nil {
		t.Fatal("lost the two independent local definitions")
	}
	local := map[int64]trimmed.Solid{89: got.Parts[89].Rounded.Solid, 254: got.Parts[254].Chamfered.Solid}
	for id, want := range map[int64]string{
		89:  "df411bbb977e59552daf4bb7620ea2ff5a7263ef87be0c5248daa075ef058dec",
		254: "868c5fbc34da853155b930450bbac9c5b8b9a9c769b82e8875819e100c921ed5",
	} {
		native, err := prismNativeBytes(local[id])
		if err != nil || rationalPosedRepairDigest(native) != want {
			t.Fatalf("local definition #%d changed from independent native source: %v", id, err)
		}
	}
	paths := [][]int64{{54, 85, 245}, {54, 85, 251}, {54, 85, 443}, {466, 472, 245}, {466, 472, 251}, {466, 472, 443}, {477}}
	want := [7][3][4]string{
		{{"-4/5", "0", "3/5", "412/5"}, {"3/5", "0", "4/5", "266/5"}, {"0", "1", "0", "8"}},
		{{"0", "3/5", "-4/5", "649/5"}, {"0", "4/5", "3/5", "82/5"}, {"1", "0", "0", "30"}},
		{{"-12/25", "16/25", "3/5", "544/5"}, {"9/25", "-12/25", "4/5", "192/5"}, {"4/5", "3/5", "0", "-24"}},
		{{"3/5", "0", "4/5", "-414/5"}, {"16/25", "3/5", "-12/25", "-113/25"}, {"-12/25", "4/5", "9/25", "741/25"}},
		{{"0", "4/5", "3/5", "-598/5"}, {"3/5", "-12/25", "16/25", "-731/25"}, {"4/5", "9/25", "-12/25", "1892/25"}},
		{{"9/25", "-12/25", "4/5", "-488/5"}, {"108/125", "-19/125", "-12/25", "-1121/25"}, {"44/125", "108/125", "9/25", "497/25"}},
		{{"1", "0", "0", "0"}, {"0", "1", "0", "150"}, {"0", "0", "1", "40"}},
	}
	for i, u := range got.Instances {
		if u.ProductID != [7]int64{89, 89, 254, 89, 89, 254, 254}[i] || !slices.Equal(u.Path, paths[i]) {
			t.Fatalf("lost source path identity %v", u.Path)
		}
		for row, values := range u.Matrix() {
			for col, v := range values {
				if v.RatString() != want[i][row][col] {
					t.Fatalf("changed composed exact world on %v: %v", u.Path, u.Matrix())
				}
			}
		}
	}
	point, err := got.Instances[5].ApplyExact(geom.Vec3{X: 1.5, Y: 2, Z: 4})
	if err != nil || point[0].RatString() != "-4741/50" || point[1].RatString() != "-5721/125" || point[2].RatString() != "2947/125" {
		t.Fatalf("changed exact world point through the shared core: %v %v", point, err)
	}
	got.Instances[0].Matrix()[0][0].SetInt64(0)
	got.Instances[0].Path[0] = 0
	if got.Instances[0].Matrix()[0][0].RatString() != "-4/5" || got.Instances[1].Path[0] != 54 {
		t.Fatal("expanded poses or paths alias shared storage")
	}
	got.Instances[0].Path[0] = 54
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_DAG"); oracle != "" {
		checkMixedConicalAP242DAGOracle(t, oracle, data, got.Instances, local, 254)
	}
}

func TestReadMixedConicalAP242DAGReject(t *testing.T) {
	ctx := context.Background()
	exact := mixedConicalAP242DAGFixture(t, "exact")
	reject := func(t *testing.T, file []byte, l MixedConicalAP242DAGLimits, want error) {
		t.Helper()
		got, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(file), l, geom.SurfacePatchOptions{})
		if err == nil || got.Assembly.Source != nil || len(got.Parts) != 0 || len(got.Instances) != 0 || want != nil && !errors.Is(err, want) {
			t.Fatalf("invalid or over-budget source returned material: %v", err)
		}
	}
	reject(t, exact, MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 2}, geom.ErrConvergence)
	reject(t, exact, MixedConicalAP242DAGLimits{MaxLeafInstances: 6}, geom.ErrConvergence)
	reject(t, exact, MixedConicalAP242DAGLimits{MaxLeafInstances: 257}, geom.ErrInvalid)
	reject(t, exact, MixedConicalAP242DAGLimits{Source: Limits{MaxSolids: 5}}, nil)
	rounded := mixedConicalAP242DAGFixture(t, "rounded")
	reject(t, rounded, MixedConicalAP242DAGLimits{MaxLeafInstances: 6}, geom.ErrConvergence)
	reject(t, rounded, MixedConicalAP242DAGLimits{}, nil)
	for _, tc := range []struct {
		name   string
		common bool
		edits  [][2]string
	}{
		{"directed cycle", false, [][2]string{
			{"#82 = ( REPRESENTATION_RELATIONSHIP('','',#63,#40)", "#82 = ( REPRESENTATION_RELATIONSHIP('','',#10,#40)"},
			{"#85 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('4','=>[0:1:1:4]','',#35,#58,$);", "#85 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('4','=>[0:1:1:4]','',#35,#5,$);"},
		}},
		{"solid as parent", true, [][2]string{
			{"#474 = ( REPRESENTATION_RELATIONSHIP('','',#259,#10)", "#474 = ( REPRESENTATION_RELATIONSHIP('','',#259,#94)"},
			{"#475 = ITEM_DEFINED_TRANSFORMATION('','',#11,#23);", "#475 = ITEM_DEFINED_TRANSFORMATION('','',#11,#11);"},
			{"#477 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('8','=>[0:1:1:3]','',#5,#254,$);", "#477 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('8','=>[0:1:1:3]','',#89,#254,$);"},
		}},
		{"empty assembly leaf", true, [][2]string{
			{"#469 = ( REPRESENTATION_RELATIONSHIP('','',#63,#452)", "#469 = ( REPRESENTATION_RELATIONSHIP('','',#63,#40)"},
			{"#470 = ITEM_DEFINED_TRANSFORMATION('','',#11,#453);", "#470 = ITEM_DEFINED_TRANSFORMATION('','',#11,#11);"},
			{"#472 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('6','=>[0:1:1:4]','',#447,#58,$);", "#472 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('6','=>[0:1:1:4]','',#35,#58,$);"},
		}},
		{"nonrigid middle pose below representable precision", true, [][2]string{
			{"#456 = DIRECTION('',(0.6,0.,-0.8));", "#456 = DIRECTION('',(0.6,0.,-0.800000000000000001));"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := exact
			for _, edit := range tc.edits {
				if bytes.Count(file, []byte(edit[0])) != 1 {
					t.Fatalf("invalid edited control: %s", edit[0])
				}
				file = bytes.Replace(file, []byte(edit[0]), []byte(edit[1]), 1)
			}
			_, err := ReadAssembly(bytes.NewReader(file), Limits{})
			if (err == nil) != tc.common {
				t.Fatalf("common graph validation result for control changed: %v", err)
			}
			reject(t, file, MixedConicalAP242DAGLimits{}, nil)
		})
	}
}

func TestMixedConicalAP242DAGPinnedAuthor(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_DAG")
	if oracle == "" {
		t.Skip("pinned mixed conical DAG author not configured")
	}
	for _, c := range []struct{ mode, fixture string }{{"write-dag", "rounded"}, {"write-dag-exact", "exact"}} {
		path := filepath.Join(t.TempDir(), c.fixture+".step")
		if b, err := exec.Command(oracle, c.mode, path).CombinedOutput(); err != nil {
			t.Fatalf("pinned DAG author: %v\n%s", err, b)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := mixedConicalAP242DAGFixture(t, c.fixture)
		i, j := bytes.Index(got, []byte("\nDATA;\n")), bytes.Index(want, []byte("\nDATA;\n"))
		if i < 0 || j < 0 || !bytes.Equal(got[i:], want[j:]) {
			t.Fatalf("pinned author changed its %s DATA section", c.fixture)
		}
	}
}

func mixedConicalAP242DAGFixture(t *testing.T, kind string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/occt-dag-mixed-conical-curved-ap242-" + kind + ".step")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func checkMixedConicalAP242DAGOracle(t *testing.T, oracle string, raw []byte, worlds []RationalHierarchyInstance, local map[int64]trimmed.Solid, chamferedID int64) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dag.step")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	data, err := exec.Command(oracle, "read-dag", path).CombinedOutput()
	var hit struct {
		SharedAssemblies, AssemblyDefinitions, MaterialDefinitions int
		Occurrences                                                []struct {
			mixedConicalOccurrenceMeasure
			Path       []int
			Definition int
		}
	}
	i := bytes.Index(data, []byte(`{"sharedAssemblies":`))
	if err != nil || i < 0 || json.Unmarshal(data[i:], &hit) != nil || hit.SharedAssemblies != 1 ||
		hit.AssemblyDefinitions != 4 || hit.MaterialDefinitions != 2 || len(hit.Occurrences) != len(worlds) {
		t.Fatalf("pinned XCAF changed original shared hierarchy: %v\n%s", err, data)
	}
	props := map[int64]parametric.Properties{}
	for id, s := range local {
		p, err := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil {
			t.Fatal(err)
		}
		props[id] = p
	}
	paths := [][]int{{1, 1, 1}, {1, 1, 2}, {1, 1, 3}, {2, 1, 1}, {2, 1, 2}, {2, 1, 3}, {3}}
	for i, u := range hit.Occurrences {
		world := worlds[i]
		mass := props[world.ProductID]
		topology, definition := [4]int{1, 6, 10, 6}, 1
		if world.ProductID == chamferedID {
			topology, definition = [4]int{1, 8, 14, 8}, 2
		}
		if !u.Valid || !slices.Equal(u.Path, paths[i]) || u.Definition != definition || u.SharedWithFirst != (definition == 1) ||
			u.Topology != topology || u.Volume <= 0 || math.Abs(u.Volume-mass.Volume) > 1e-8*mass.Volume || math.Abs(u.Area-mass.Area) > 1e-8*mass.Area {
			t.Fatalf("pinned occurrence changed original owner or signed material: %v", u)
		}
		for row, cells := range world.Matrix() {
			for col, q := range cells {
				x, _ := q.Float64()
				if math.Abs(u.Pose[4*row+col]-x) > 1e-12 {
					t.Fatalf("pinned occurrence changed world placement %v", u.Path)
				}
			}
		}
		x, err := world.ApplyExact(mass.Centroid)
		if err != nil {
			t.Fatal(err)
		}
		for k, q := range x {
			v, _ := q.Float64()
			if math.Abs(u.Centroid[k]-v) > .0002 {
				t.Fatalf("unexpected independent world center of mass %v", u.Path)
			}
		}
	}
}
