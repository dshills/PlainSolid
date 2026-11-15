package plaincadbridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
	"plainsolid/mesh"
)

func stepRepairFixture(t *testing.T, name string) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "exchange", "step", "testdata", name+".step"))
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}

func TestSTEPRepairImportWorldMeshAndCanonicalExport(t *testing.T) {
	const singleVolume, singleArea = 29.145041881053835, 66.186284791166074
	const groupVolume, groupArea = 29.077978807115535, 65.622954970084379
	const maxX = 54.0 / 16
	for _, tc := range []struct {
		name                  string
		family                string
		repairs, owners, caps int
		volume, area          float64
		bounds                Bounds
	}{
		{"occt-default-oblique-prism-fillet", "occt-single-pcurve-convex-arc", 3, 24, 0, singleVolume, singleArea, Bounds{vector{0, 0, 0}, vector{maxX, 3, 5}}},
		{"occt-both-default-oblique-prism-fillet", "occt-grouped-pcurve-convex-arc", 9, 28, 2, groupVolume, groupArea, Bounds{vector{0, 0, 0}, vector{maxX, 3, 5}}},
		{"occt-default-two-fillets-x", "occt-posed-grouped-pcurve-convex-arc", 6, 28, 2, groupVolume, groupArea, Bounds{vector{2, -3, 0}, vector{7, maxX - 3, 3}}},
		{"occt-default-two-fillets-minus-x", "occt-posed-grouped-pcurve-convex-arc", 6, 28, 2, groupVolume, groupArea, Bounds{vector{-3, -3 - maxX, 0}, vector{2, -3, 3}}},
		{"occt-default-two-fillets-translated", "occt-grouped-pcurve-convex-arc", 6, 28, 2, groupVolume, groupArea, Bounds{vector{2, 0, 3}, vector{2 + maxX, 3, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := New()
			source := stepRepairFixture(t, tc.name)
			rejectGroup(t, b, map[string]any{"op": "importStep", "source": source})
			result := shape(t, b, map[string]any{
				"op": "importStep", "source": source,
				"stepRepair": map[string]any{"maxDistance": 1e-12, "native": true},
			})
			if !result.GeometryAssertions.Valid || result.GeometryAssertions.SolidCount != 1 {
				t.Fatal(result)
			}
			near(t, result.GeometryAssertions.Volume, tc.volume)
			near(t, result.GeometryAssertions.SurfaceArea, tc.area)
			for i := range 3 {
				near(t, result.Bounds.Min[i], tc.bounds.Min[i])
				near(t, result.Bounds.Max[i], tc.bounds.Max[i])
			}

			evidence := result.RepairEvidence
			if evidence == nil || evidence.Family != tc.family ||
				evidence.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(source))) ||
				evidence.LengthUnit != .001 || evidence.MaxDistance != 1e-12 ||
				len(evidence.Repairs) != tc.repairs || evidence.PCurveOwners != tc.owners || evidence.RationalCaps != tc.caps {
				t.Fatalf("repair evidence %+v", evidence)
			}
			if result.NativeProof == nil {
				t.Fatal("missing requested source proof")
			}
			ctx := context.Background()
			reader, limits := bytes.NewReader(*result.NativeProof), step.IndependentMaterialJSONLimits{}
			var reloaded stepRepairMaterial
			var err error
			switch tc.family {
			case "occt-single-pcurve-convex-arc":
				reloaded, err = step.ReadPCurveRepairedOCCTConvexArcJSON(ctx, reader, limits)
			case "occt-grouped-pcurve-convex-arc":
				reloaded, err = step.ReadGroupedPCurveRepairedOCCTConvexArcJSON(ctx, reader, limits)
			case "occt-posed-grouped-pcurve-convex-arc":
				reloaded, err = step.ReadPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, reader, limits)
			}
			if err != nil || reloaded.OriginalSTEP() != source || !reflect.DeepEqual(reloaded.Provenance(), evidence.Provenance) ||
				!reflect.DeepEqual(reloaded.Repairs(), evidence.Repairs) || len(reloaded.PCurves()) != evidence.PCurveOwners {
				t.Fatal("native proof did not re-prove original source evidence", err)
			}
			inspected := shape(t, b, map[string]any{"op": "inspect", "handle": result.Handle})
			if !reflect.DeepEqual(inspected, result) {
				t.Fatal("inspect lost source evidence")
			}

			var display Mesh
			if err := json.Unmarshal(invoke(t, b, map[string]any{
				"op": "mesh", "handle": result.Handle, "analyticNormals": true,
				"linearDeflection": .025, "angularDeflection": .2,
			}), &display); err != nil {
				t.Fatal(err)
			}
			if len(display.Indices) == 0 || len(display.Positions) != len(display.Normals) ||
				display.Bounds != result.Bounds || display.GeometryAssertions != result.GeometryAssertions {
				t.Fatal("invalid display mesh")
			}
			var nativeMesh mesh.Mesh
			points := make(map[vector]int)
			indices := make([]int, len(display.Positions)/3)
			for j := range indices {
				v := vector(display.Positions[j*3 : j*3+3])
				for axis, x := range v {
					if x < tc.bounds.Min[axis]-1e-8 || x > tc.bounds.Max[axis]+1e-8 {
						t.Fatal("mesh vertex outside original world bounds", v, tc.bounds)
					}
				}
				id, ok := points[v]
				if !ok {
					id = len(nativeMesh.Vertices)
					points[v] = id
					nativeMesh.Vertices = append(nativeMesh.Vertices, geom.Vec3{X: v[0], Y: v[1], Z: v[2]})
				}
				indices[j] = id
			}
			for j := 0; j < len(display.Indices); j += 3 {
				nativeMesh.Triangles = append(nativeMesh.Triangles, [3]int{
					indices[display.Indices[j]], indices[display.Indices[j+1]], indices[display.Indices[j+2]],
				})
			}
			p, err := nativeMesh.MassProperties()
			if err != nil || p.Volume < .999*tc.volume || p.Volume > 1.001*tc.volume {
				t.Fatal("closed display mesh differs from original material", p, err)
			}

			var exported stepExport
			if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "exportStep", "handles": []string{result.Handle}}), &exported); err != nil {
				t.Fatal(err)
			}
			raw, err := base64.StdEncoding.DecodeString(exported.Base64)
			if err != nil || exported.Units != "mm" || len(exported.Before) != 1 || len(exported.After) != 1 {
				t.Fatal("invalid export result", err)
			}
			if bytes.Contains(raw, []byte("PCURVE")) {
				t.Fatal("model writer retained original pcurves")
			}
			strict := shape(t, b, map[string]any{"op": "importStep", "source": string(raw)})
			if strict.RepairEvidence != nil || strict.NativeProof != nil || strict.Bounds != result.Bounds ||
				strict.GeometryAssertions != result.GeometryAssertions {
				t.Fatal("ordinary STEP reload differs from exported material", strict, result.Bounds)
			}
			again, err := b.exportSTEP([]string{strict.Handle})
			if err != nil || again.Base64 != exported.Base64 {
				t.Fatal("canonical STEP changed on ordinary re-export", err)
			}
		})
	}
}

func TestSTEPRepairRejectsInvalidRequestsAndOriginalOwners(t *testing.T) {
	b := New()
	source := stepRepairFixture(t, "occt-default-oblique-prism-fillet")
	for _, options := range []any{
		nil,
		true,
		map[string]any{},
		map[string]any{"maxDistance": nil},
		map[string]any{"maxDistance": -1},
		map[string]any{"maxDistance": 1e-12, "unknown": true},
		map[string]any{"maxDistance": 1e-14},
	} {
		rejectGroup(t, b, map[string]any{"op": "importStep", "source": source, "stepRepair": options})
	}
	options := map[string]any{"maxDistance": 1e-12}
	rejectGroup(t, b, map[string]any{
		"op": "importStep", "source": strings.Replace(source, "2.498091544797", "2.4", 1), "stepRepair": options,
	})
	rejectGroup(t, b, map[string]any{
		"op": "box", "min": vector{}, "max": vector{1, 1, 1}, "stepRepair": options,
	})
	result := shape(t, b, map[string]any{"op": "importStep", "source": source, "stepRepair": options})
	if result.RepairEvidence == nil || result.NativeProof != nil {
		t.Fatal("repair without native proof has unexpected evidence", result)
	}
	stock := box(t, b, vector{}, vector{1, 2, 3})
	exported, err := b.exportSTEP([]string{stock.Handle})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(exported.Base64)
	if err != nil {
		t.Fatal(err)
	}
	strict := shape(t, b, map[string]any{"op": "importStep", "source": string(raw), "stepRepair": options})
	if strict.GeometryAssertions != stock.GeometryAssertions || strict.Bounds != stock.Bounds ||
		strict.RepairEvidence != nil || strict.NativeProof != nil {
		t.Fatal("repair opt-in altered ordinary valid STEP", strict)
	}
}
