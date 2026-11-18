package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func foreignBoreRimSTEP(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/occt-model-only-eccentric-bore-rim-" + name + ".step")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReadWrappedCylinderBoreRimFilletOCCT(t *testing.T) {
	slowtest.Skip(t)
	ctx, opt := context.Background(), geom.SurfacePatchOptions{}
	raw := foreignBoreRimSTEP(t, "upper")
	result, err := ReadWrappedCylinderBoreRimFillet(ctx, bytes.NewReader(raw), Limits{}, opt)
	if err != nil {
		t.Fatal(err)
	}
	p, err := result.Solid.ValidateWrappedCylinderBoreRimFillet(ctx, opt)
	if err != nil || !p.Complete || p.Stock.CenterY.Cmp(big.NewRat(64, 1)) != 0 || p.Hole.CenterX.Cmp(big.NewRat(3, 2)) != 0 ||
		p.Hole.CenterY.Cmp(big.NewRat(66, 1)) != 0 || p.Fillets[0].Sign() != 0 || p.Fillets[1].Cmp(big.NewRat(3, 4)) != 0 ||
		result.LengthUnit != .001 || result.SourceBodyID != 15 || result.SourceShellID != 16 || result.Source == nil {
		t.Fatalf("lost original axes, fillet, units or source identity: %+v, %v", p, err)
	}
	if result.NativeFaceSourceIDs[p.Blends[1]] != 45 || result.NativeFaceSourceIDs[p.Outer] != 71 ||
		result.NativeFaceSourceIDs[p.Bore] != 96 || result.NativeFaceSourceIDs[p.Caps[0]] != 121 ||
		result.NativeFaceSourceIDs[p.Caps[1]] != 17 {
		t.Fatalf("lost original face roles: %v", result.NativeFaceSourceIDs)
	}
	split := map[int64]int{}
	for _, id := range result.NativeEdgeSourceIDs {
		split[id]++
	}
	for id, count := range map[int64]int{21: 4, 32: 4, 49: 1, 60: 4, 76: 1, 84: 4, 100: 1, 110: 4} {
		if split[id] != count {
			t.Fatalf("source edge %d has %d native pieces, expected %d", id, split[id], count)
		}
	}
	if len(split) != 8 || len(result.NativeEdgeSourceIDs) != 23 {
		t.Fatalf("foreign edge mapping changed: %v", result.NativeEdgeSourceIDs)
	}
	vertices := map[int64]int{}
	for _, id := range result.NativeVertexSourceIDs {
		vertices[id]++
	}
	if vertices[0] != 15 || len(vertices) != 6 {
		t.Fatalf("foreign seam vertices or exact quadrant split changed: %v", result.NativeVertexSourceIDs)
	}
	props, err := result.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(props.Volume-879.57274568077219) > 1e-7 || math.Abs(props.Area-631.19948065961216) > 1e-7 {
		t.Fatalf("native positive signed mass differs from OCCT: %+v, %v", props, err)
	}
	var native bytes.Buffer
	if err := trimmed.WriteJSON(&native, result.Solid); err != nil {
		t.Fatal(err)
	}
	reloaded, err := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	var canonical bytes.Buffer
	if err := WriteWrappedCylinderBoreRimFillet(ctx, &canonical, reloaded, "foreign eccentric bore", result.LengthUnit, opt); err != nil {
		t.Fatal(err)
	}
	reopened, err := ReadWrappedCylinderBoreRimFillet(ctx, bytes.NewReader(canonical.Bytes()), Limits{}, opt)
	if err != nil || reopened.Solid.Topology().EulerCharacteristic != 0 || reopened.LengthUnit != result.LengthUnit {
		t.Fatalf("strict reopen of normal quadrant writer: %+v, %v", reopened, err)
	}
	var inInches bytes.Buffer
	if err := WriteWrappedCylinderBoreRimFillet(ctx, &inInches, reloaded, "physical inch unit", .0254, opt); err != nil {
		t.Fatal(err)
	}
	inch, err := ReadWrappedCylinderBoreRimFillet(ctx, bytes.NewReader(inInches.Bytes()), Limits{}, opt)
	if err != nil || inch.LengthUnit != .0254 || len(inch.Solid.Faces()) != 5 {
		t.Fatalf("original inch unit on the same file coordinates: %v", err)
	}
	badUnit := bytes.Replace(inInches.Bytes(), []byte("LENGTH_MEASURE(0.0254)"), []byte("LENGTH_MEASURE(0.025400000000000001)"), 1)
	if bytes.Equal(badUnit, inInches.Bytes()) {
		t.Fatal("missing exact inch source claim")
	}
	if got, err := ReadWrappedCylinderBoreRimFillet(ctx, bytes.NewReader(badUnit), Limits{}, opt); err == nil || got.Source != nil {
		t.Fatalf("rounded physical unit admitted: %v", err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_BORE_RIM_TRANSFER"); oracle != "" {
		path := filepath.Join(t.TempDir(), "reconstructed.step")
		if err := os.WriteFile(path, canonical.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		source, err := exec.CommandContext(ctx, oracle, path).CombinedOutput()
		var read struct {
			Solids, Faces int
			Volume, Area  float64
			Toroids       [][9]float64
		}
		if err != nil || json.Unmarshal(source, &read) != nil || read.Solids != 1 || read.Faces != 5 ||
			math.Abs(read.Volume-props.Volume) > 1e-7 || math.Abs(read.Area-props.Area) > 1e-7 || len(read.Toroids) != 1 ||
			read.Toroids[0] != [9]float64{1.5, 66, 9.25, 2.75, .75, 2, 2.75, 9.25, 10} {
			t.Fatalf("independent retransfer of strict native result: %s, %v", source, err)
		}
		t.Logf("pinned OCCT retransfer preserves translated axis and positive signed volume %.16g", read.Volume)
	}
}

func TestReadWrappedCylinderBoreRimFilletStrict(t *testing.T) {
	slowtest.Skip(t)
	ctx, opt := context.Background(), geom.SurfacePatchOptions{}
	raw := foreignBoreRimSTEP(t, "upper")
	reject := func(t *testing.T, raw []byte, l Limits) {
		t.Helper()
		out, err := ReadWrappedCylinderBoreRimFillet(ctx, bytes.NewReader(raw), l, opt)
		if err == nil || len(out.Solid.Faces()) != 0 || out.Source != nil || len(out.NativeEdgeSourceIDs) != 0 {
			t.Fatalf("source failed to reject atomically: %+v, %v", out, err)
		}
	}
	for _, name := range []string{"off-support", "conflicting-bound"} {
		t.Run(name, func(t *testing.T) {
			reject(t, foreignBoreRimSTEP(t, name), Limits{})
		})
	}
	for _, c := range []struct{ name, original, changed string }{
		{"rounded major", "TOROIDAL_SURFACE('',#67,2.75,0.75)", "TOROIDAL_SURFACE('',#67,2.7500000000000001,0.75)"},
		{"exact different major", "TOROIDAL_SURFACE('',#67,2.75,0.75)", "TOROIDAL_SURFACE('',#67,2.875,0.75)"},
		{"complementary meridian", "#49 = EDGE_CURVE('',#22,#50,#52,.T.);", "#49 = EDGE_CURVE('',#22,#50,#52,.F.);"},
		{"opposite meridian normal", "#55 = DIRECTION('',(0.,-1.,0.));", "#55 = DIRECTION('',(0.,1.,0.));"},
		{"altered trim vertex", "#23 = CARTESIAN_POINT('',(4.25,66.,10.));", "#23 = CARTESIAN_POINT('',(4.25,66.00000000000001,10.));"},
		{"double use of one face bound", "(#18,#29)", "(#18,#18)"},
		{"substituted bore shoulder", "#59 = ORIENTED_EDGE('',*,*,#60,.F.);", "#59 = ORIENTED_EDGE('',*,*,#32,.F.);"},
		{"ignored detached geometry", "ENDSEC;\nEND-ISO-10303-21;", "#900 = CIRCLE('',#25,2.75);\nENDSEC;\nEND-ISO-10303-21;"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if !bytes.Contains(raw, []byte(c.original)) {
				t.Fatal("missing original mutation control")
			}
			reject(t, bytes.Replace(raw, []byte(c.original), []byte(c.changed), 1), Limits{})
		})
	}
	reject(t, raw, Limits{MaxVertices: 4})
	reject(t, raw, Limits{MaxFaces: 4})
	reject(t, raw, Limits{MaxEdgeUses: 6})
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := ReadWrappedCylinderBoreRimFillet(canceled, bytes.NewReader(raw), Limits{}, opt); !errors.Is(err, context.Canceled) || got.Source != nil {
		t.Fatalf("canceled proof exposed a result: %v", err)
	}
}
