package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"plainsolid/geom"
)

func TestMixedConicalAP242DAGAppearance(t *testing.T) {
	ctx := context.Background()
	raw := mixedDAGAppearanceFixture(t)
	m, err := ReadMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(raw), MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 3, MaxLeafInstances: 7}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if m.OriginalSTEP() != string(raw) || len(m.DAG.Instances) != 7 || len(m.DAG.Parts) != 2 {
		t.Fatal("lost the original appearance bytes or shared hierarchy")
	}
	products := []MixedConicalAP242SurfaceStyle{
		{DefinitionID: 89, BodyID: 95, StyledItemID: 484, PresentationID: 483, AssignmentID: 485, Color: RGB{1, 0, 0}},
		{DefinitionID: 254, BodyID: 260, StyledItemID: 501, PresentationID: 500, AssignmentID: 502, Color: RGB{0, 1, 0}},
	}
	face := MixedConicalAP242SurfaceStyle{DefinitionID: 254, BodyID: 260, FaceID: 336, StyledItemID: 509,
		PresentationID: 500, AssignmentID: 510, NativeFaceIndex: 3, Color: RGB{1, 1, 0}}
	if !reflect.DeepEqual(m.ProductStyles, products) || !reflect.DeepEqual(m.FaceStyles, []MixedConicalAP242SurfaceStyle{face}) ||
		m.DAG.Parts[254].Chamfered.NativeFaceSourceIDs[face.NativeFaceIndex] != face.FaceID {
		t.Fatalf("wrong original body or face style owner: %v %v", m.ProductStyles, m.FaceStyles)
	}
	c := DeepSHUOColor{PrefixSHUOID: 479, TerminalSHUOID: 478, Path: [3]int64{54, 85, 251}, Context: OccurrenceSurfaceColor{
		OccurrenceID: 251, BodyID: 95, StyledItemID: 492, BaseStyledItemID: 484, AssignmentID: 493, ContextRepresentationID: 482,
		DefinitionRepresentationID: 480, ProductShapeID: 481, CDSRID: 247, TransformationID: 249, PlacementID: 68, Color: RGB{0, 0, 1},
	}}
	if !reflect.DeepEqual(m.PathOverride, c) || !slices.Equal(m.DAG.Instances[1].Path, c.Path[:]) ||
		m.DAG.Instances[4].Path[2] != c.Context.OccurrenceID || slices.Equal(m.DAG.Instances[4].Path, c.Path[:]) {
		t.Fatalf("wrong contextual path through the shared core: %v", m.PathOverride)
	}
	if _, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(raw), MixedConicalAP242DAGLimits{}, geom.SurfacePatchOptions{}); err == nil {
		t.Fatal("unstyled reader accepted the separate styled dialect")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_APPEARANCE"); oracle != "" {
		checkMixedDAGAppearanceOracle(t, oracle, []byte(m.OriginalSTEP()))
	}
}

func TestMixedConicalAP242DAGAppearanceReject(t *testing.T) {
	ctx := context.Background()
	original := mixedDAGAppearanceFixture(t)
	for _, edits := range [][2]string{
		{"#336,#501);", "#97,#501);"},
		{"#336,#501);", "#336,#484);"},
		{"#482 = SHAPE_REPRESENTATION('',(#68),#236);", "#482 = SHAPE_REPRESENTATION('',(#64),#236);"},
		{"#482 = SHAPE_REPRESENTATION('',(#68),#236);", "#482 = SHAPE_REPRESENTATION('',(#68),#434);"},
		{"#5,#58,$,#54,#85);", "#5,#58,$,#466,#85);"},
		{"#5,#89,$,#479,#251);", "#5,#89,$,#479,#245);"},
		{"ENDSEC;\nEND-ISO-10303-21;", "#999 = PRODUCT_DEFINITION_SHAPE('other',$,#479);\nENDSEC;\nEND-ISO-10303-21;"},
	} {
		file := replaceMixedDAGAppearance(t, original, edits)
		m, err := ReadMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(file), MixedConicalAP242DAGLimits{}, geom.SurfacePatchOptions{})
		if err == nil || m.OriginalSTEP() != "" || len(m.DAG.Instances) != 0 {
			t.Fatalf("accepted malformed style owner %q: %v", edits[1], err)
		}
	}
	for _, l := range []MixedConicalAP242DAGLimits{{MaxLeafInstances: 6}, {MaxOccurrenceDepth: 2}} {
		m, err := ReadMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(original), l, geom.SurfacePatchOptions{})
		if err == nil || m.OriginalSTEP() != "" {
			t.Fatalf("style view bypassed path budget: %v", err)
		}
	}
}

func TestMixedConicalAP242DAGAppearanceRetarget(t *testing.T) {
	// Both parents reach the original same inner NAUO. Only the complete
	// prefix selects the override, even after moving it to the other branch.
	raw := replaceMixedDAGAppearance(t, mixedDAGAppearanceFixture(t), [2]string{"#5,#58,$,#54,#85);", "#5,#58,$,#466,#472);"})
	m, err := ReadMixedConicalAP242DAGAppearance(context.Background(), bytes.NewReader(raw), MixedConicalAP242DAGLimits{}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if m.PathOverride.Path != [3]int64{466, 472, 251} || !slices.Equal(m.PathOverride.Path[:], m.DAG.Instances[4].Path) {
		t.Fatalf("lost exact retargeted prefix: %v", m.PathOverride)
	}
}

func TestMixedConicalAP242DAGAppearancePinnedAuthor(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_APPEARANCE")
	if oracle == "" {
		t.Skip("pinned XCAF appearance author not configured")
	}
	path := filepath.Join(t.TempDir(), "appearance.step")
	if out, err := exec.Command(oracle, "write-dag-appearance", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned author: %v\n%s", err, out)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := mixedDAGAppearanceFixture(t)
	i, j := bytes.Index(got, []byte("\nDATA;\n")), bytes.Index(want, []byte("\nDATA;\n"))
	if i < 0 || j < 0 || !bytes.Equal(got[i:], want[j:]) {
		t.Fatal("pinned XCAF changed its original styled DATA section")
	}
}

func mixedDAGAppearanceFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/occt-dag-mixed-conical-ap242-appearance.step")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func replaceMixedDAGAppearance(t *testing.T, raw []byte, edit [2]string) []byte {
	t.Helper()
	if bytes.Count(raw, []byte(edit[0])) != 1 {
		t.Fatalf("test edit does not refer to one original record: %s", edit[0])
	}
	return bytes.Replace(raw, []byte(edit[0]), []byte(edit[1]), 1)
}

func checkMixedDAGAppearanceOracle(t *testing.T, oracle string, raw []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "appearance.step")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(oracle, "read-dag-appearance", path).CombinedOutput()
	var hit struct {
		StyledPaths []struct {
			Path     []int
			Product  [3]float64
			Override *[3]float64
			Faces    []struct {
				PlaneZ float64
				Color  [3]float64
			}
		}
	}
	paths := [][]int{{1, 1, 1}, {1, 1, 2}, {1, 1, 3}, {2, 1, 1}, {2, 1, 2}, {2, 1, 3}, {3}}
	start := bytes.Index(out, []byte(`{"styledPaths":`))
	if err != nil || start < 0 || json.Unmarshal(out[start:], &hit) != nil || len(hit.StyledPaths) != len(paths) {
		t.Fatalf("pinned XCAF appearance transfer failed: %v\n%s", err, out)
	}
	equal := func(got, want [3]float64) bool {
		for i, v := range got {
			if math.Abs(v-want[i]) > 1e-12 {
				return false
			}
		}
		return true
	}
	for i, p := range hit.StyledPaths {
		if !slices.Equal(p.Path, paths[i]) || (p.Override != nil) != (i == 1) ||
			p.Override != nil && !equal(*p.Override, [3]float64{0, 0, 1}) {
			t.Fatalf("pinned XCAF changed path-specific blue ownership on %v", p.Path)
		}
		if i == 2 || i == 5 || i == 6 {
			if !equal(p.Product, [3]float64{0, 1, 0}) || len(p.Faces) != 1 || p.Faces[0].PlaneZ != 2 ||
				!equal(p.Faces[0].Color, [3]float64{1, 1, 0}) {
				t.Fatalf("pinned XCAF changed chamfer product or face color on %v", p.Path)
			}
		} else if !equal(p.Product, [3]float64{1, 0, 0}) || len(p.Faces) != 0 {
			t.Fatalf("pinned XCAF changed rolling product color on %v", p.Path)
		}
	}
}
