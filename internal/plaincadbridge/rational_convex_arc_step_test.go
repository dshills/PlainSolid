package plaincadbridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"plainsolid/mesh"
)

func TestRationalWorldSTEPRepairBridge(t *testing.T) {
	const fiveSource = "occt-five-default-tilted-pentagonal-prism-fillet"
	for _, tc := range []struct {
		file, family string
		bounds       Bounds
		volume, area float64
		caps, sites  int
		axial        int
		owners       int
		phaseOwners  []int64
	}{
		{"occt-default-tilted-oblique-prism-fillet", "occt-rational-posed-single-pcurve-convex-arc",
			Bounds{vector{0, 0, -2.7}, vector{6.025, 3, 3}}, 29.145041881053842, 66.186284791166074, 0, 3, 0, 24, nil},
		{"occt-default-tilted-translated-oblique-prism-fillet", "occt-rational-posed-single-pcurve-convex-arc",
			Bounds{vector{1, 0, -.7}, vector{7.025, 3, 5}}, 29.145041881053842, 66.186284791166074, 0, 3, 0, 24, nil},
		{"occt-both-default-tilted-oblique-prism-fillet", "occt-rational-posed-grouped-pcurve-convex-arc",
			Bounds{vector{0, 0, -2.7}, vector{6.025, 3, 3}}, 29.077978807115539, 65.622954970084379, 2, 8, 1, 28, nil},
		{"occt-three-default-tilted-oblique-prism-fillet", "occt-rational-posed-grouped-pcurve-convex-arc",
			Bounds{vector{0, 0, -2.7}, vector{6.025, 2.84375, 3}}, 28.968988172080639, 64.184278587623695, 2, 10, 2, 34, []int64{210, 425}},
		{"occt-four-default-tilted-trapezoidal-prism-fillet", "occt-rational-posed-grouped-pcurve-convex-arc",
			Bounds{vector{0, 0, -1.6}, vector{5.2, 2.84375, 3}}, 22.155304183748054, 51.012178844414059, 2, 10, 2, 46, []int64{210, 267, 482, 529}},
		{fiveSource, "occt-rational-posed-grouped-pcurve-convex-arc",
			Bounds{vector{0, 0, -2.4}, vector{5.8, 2.75, 3}}, 39.084768347549009, 68.682455982680239, 2, 10, 2, 58, []int64{210, 267, 324, 539, 586, 633}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			b := New()
			source := stepRepairFixture(t, tc.file)
			rejectGroup(t, b, map[string]any{"op": "importStep", "source": source})
			finiteLineOwners := 8
			if tc.file == fiveSource {
				finiteLineOwners = 12
				rejectGroup(t, b, map[string]any{"op": "importStep", "source": stepRepairFixture(t, fiveSource+"-nondyadic"),
					"stepRepair": map[string]any{"maxDistance": 1e-12, "native": true}})
			}
			linearDeflection := .025
			if len(tc.phaseOwners) > 0 {
				linearDeflection = .5
				rejectGroup(t, b, map[string]any{"op": "importStep", "source": source,
					"stepRepair": map[string]any{"maxDistance": 1e-13, "native": true}})
			}
			result := shape(t, b, map[string]any{
				"op": "importStep", "source": source, "stepRepair": map[string]any{"maxDistance": 1e-12, "native": true},
			})
			arc := b.shapes[result.Handle].rationalStepArc
			if arc == nil || b.shapes[result.Handle].analytic != nil || frameMap(arc.descriptor.Frame) != geom.Identity() {
				t.Fatal("import did not retain an independently placed product-local solid")
			}
			evidence := result.RepairEvidence
			if evidence == nil || evidence.Family != tc.family || evidence.WorldPose == nil || result.NativeProof == nil ||
				evidence.PCurveOwners != tc.owners || evidence.RationalCaps != tc.caps || evidence.AxialTrims != tc.axial ||
				len(evidence.Repairs) != tc.sites || evidence.FiniteLineOwners != finiteLineOwners || evidence.LengthUnit != .001 {
				t.Fatal("missing rational source evidence")
			}
			pose, err := arc.pose.Matrix()
			if err != nil || !reflect.DeepEqual(pose, *evidence.WorldPose) {
				t.Fatal("response and retained pose disagree", err)
			}
			ctx := context.Background()
			proof := bytes.NewReader(*result.NativeProof)
			var reloadedSource string
			var proofPose [3][4]*big.Rat
			if tc.caps == 0 {
				m, e := step.ReadRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, proof, step.IndependentMaterialJSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				reloadedSource = m.OriginalSTEP()
				proofPose, err = m.Pose().Matrix()
			} else {
				m, e := step.ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, proof, step.IndependentMaterialJSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				if len(tc.phaseOwners) > 0 {
					var ids []int64
					for _, owner := range m.PCurves() {
						if owner.Kind == "phase-cylinder-circle" && owner.DistanceSquaredUpper.Sign() > 0 {
							ids = append(ids, owner.PCurveID)
						}
					}
					if !reflect.DeepEqual(ids, tc.phaseOwners) || len(m.AxialTrims()) != tc.axial || len(m.FiniteTrims()) != tc.caps {
						t.Fatal("native receipt lost an original phased owner proof", ids)
					}
				}
				if tc.owners == 46 {
					caps, trims := m.FiniteTrims(), m.AxialTrims()
					for i, cap := range caps {
						if cap.CapOwner.PCurveID != [2]int64{315, 375}[i] || cap.CylinderOwner.PCurveID != [2]int64{326, 368}[i] ||
							cap.EndStation == nil || trims[i].EdgeID != [2]int64{338, 563}[i] {
							t.Fatal("four-fillet receipt lost an original trim", cap.CapOwner.PCurveID)
						}
					}
				}
				if tc.file == fiveSource {
					checkFiveRoundBridgeReceipt(t, m)
				}
				reloadedSource = m.OriginalSTEP()
				proofPose, err = m.Pose().Matrix()
			}
			if err != nil || reloadedSource != source || !reflect.DeepEqual(proofPose, pose) {
				t.Fatal("native proof lost exact source placement", err)
			}
			checkRationalStepArcShape(t, result, tc.bounds, tc.volume, tc.area)
			clone := shape(t, b, map[string]any{"op": "clone", "handle": result.Handle})
			if clone.Handle == result.Handle || !reflect.DeepEqual(clone.RepairEvidence, result.RepairEvidence) ||
				!reflect.DeepEqual(clone.NativeProof, result.NativeProof) {
				t.Fatal("clone lost opted-in source evidence")
			}
			inspected := shape(t, b, map[string]any{"op": "inspect", "handle": result.Handle})
			if !reflect.DeepEqual(inspected, result) {
				t.Fatal("inspect changed source evidence")
			}

			var display Mesh
			if err = json.Unmarshal(invoke(t, b, map[string]any{
				"op": "mesh", "handle": result.Handle, "linearDeflection": linearDeflection, "angularDeflection": .2, "analyticNormals": true,
			}), &display); err != nil {
				t.Fatal(err)
			}
			world := rationalStepArcDisplayMesh(t, display, tc.bounds)
			p, err := world.MassProperties()
			if err != nil || p.Volume < .999*tc.volume || p.Volume > 1.001*tc.volume {
				t.Fatal("rounded world mesh lost material orientation or volume", p, err)
			}
			for _, q := range []map[string]any{
				{"op": "displayEdges", "handle": result.Handle, "maxSegments": 5000},
				{"op": "displayEdges", "handle": result.Handle, "maxEdgeUses": 100, "maxEdgeSegments": 5000},
			} {
				var lines struct {
					Positions, Segments []float64
					EdgeUses            int
				}
				if err = json.Unmarshal(invoke(t, b, q), &lines); err != nil ||
					lines.EdgeUses == 0 || len(lines.Positions)+len(lines.Segments) == 0 {
					t.Fatal("display edges lost original analytic boundary", err)
				}
				checkRationalStepArcDisplayCoordinates(t, append(lines.Positions, lines.Segments...), tc.bounds)
			}

			var exported stepExport
			if err = json.Unmarshal(invoke(t, b, map[string]any{"op": "exportStep", "handles": []string{result.Handle}}), &exported); err != nil {
				t.Fatal(err)
			}
			raw, err := base64.StdEncoding.DecodeString(exported.Base64)
			if err != nil || exported.Units != "mm" || len(exported.Before) != 1 || len(exported.After) != 1 {
				t.Fatal("bad canonical export", err)
			}
			strict := shape(t, b, map[string]any{"op": "importStep", "source": string(raw)})
			strictArc := b.shapes[strict.Handle].rationalStepArc
			if strict.RepairEvidence != nil || strict.NativeProof != nil || strictArc == nil ||
				strict.Bounds != result.Bounds || strict.GeometryAssertions != result.GeometryAssertions {
				t.Fatal("ordinary strict reimport differs from repaired handle")
			}
			strictPose, err := strictArc.pose.Matrix()
			if err != nil || !reflect.DeepEqual(strictPose, pose) {
				t.Fatal("strict reimport changed exact pose", err)
			}
			var originalNative, strictNative bytes.Buffer
			if err = trimmed.WriteJSON(&originalNative, arc.local); err != nil {
				t.Fatal(err)
			}
			if err = trimmed.WriteJSON(&strictNative, strictArc.local); err != nil || !bytes.Equal(originalNative.Bytes(), strictNative.Bytes()) {
				t.Fatal("strict reimport changed native product geometry", err)
			}
			again, err := b.exportSTEP([]string{strict.Handle})
			if err != nil || again.Base64 != exported.Base64 {
				t.Fatal("canonical world STEP was not byte-stable", err)
			}

			if tc.axial != 0 {
				verifyRationalStepArcTransform(t, b, result, tc.bounds, tc.volume, tc.area)
			}
		})
	}
}

func checkFiveRoundBridgeReceipt(t *testing.T, m step.RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) {
	t.Helper()
	halfTurns := map[int64]int64{324: 331, 633: 635}
	for _, owner := range m.PCurves() {
		geometry, ok := halfTurns[owner.PCurveID]
		if !ok {
			continue
		}
		if owner.Kind != "phase-cylinder-circle" || owner.GeometryID != geometry || owner.SupportID != 325 ||
			owner.DistanceSquaredUpper.Sign() <= 0 {
			t.Fatal("half-turn receipt changed its original curve or support", owner.PCurveID)
		}
		delete(halfTurns, owner.PCurveID)
	}
	if len(halfTurns) != 0 {
		t.Fatal("half-turn receipt lost an original owner", halfTurns)
	}
	caps, axial := m.FiniteTrims(), m.AxialTrims()
	for i, cap := range caps {
		if cap.CapOwner.PCurveID != [2]int64{372, 432}[i] || cap.CylinderOwner.PCurveID != [2]int64{383, 425}[i] ||
			cap.EndStation == nil || axial[i].EdgeID != [2]int64{395, 667}[i] {
			t.Fatal("five-fillet receipt lost an original trim", cap.CapOwner.PCurveID)
		}
	}
}

func checkRationalStepArcShape(t *testing.T, s Shape, bounds Bounds, volume, area float64) {
	t.Helper()
	if !s.GeometryAssertions.Valid || s.GeometryAssertions.SolidCount != 1 {
		t.Fatal("expected one valid analytic solid", s.GeometryAssertions)
	}
	near(t, s.GeometryAssertions.Volume, volume)
	near(t, s.GeometryAssertions.SurfaceArea, area)
	for axis := range 3 {
		near(t, s.Bounds.Min[axis], bounds.Min[axis])
		near(t, s.Bounds.Max[axis], bounds.Max[axis])
	}
}

func checkRationalStepArcDisplayCoordinates(t *testing.T, positions []float64, bounds Bounds) {
	t.Helper()
	for i, v := range positions {
		axis := i % 3
		if v < bounds.Min[axis]-1e-12 || v > bounds.Max[axis]+1e-12 {
			t.Fatal("display vertex outside world bounds", i, v, bounds)
		}
	}
}

func rationalStepArcDisplayMesh(t *testing.T, display Mesh, bounds Bounds) mesh.Mesh {
	t.Helper()
	if len(display.Indices) == 0 || len(display.Indices)%3 != 0 ||
		len(display.Positions)%3 != 0 || len(display.Positions) != len(display.Normals) {
		t.Fatal("invalid bridge display stream")
	}
	checkRationalStepArcDisplayCoordinates(t, display.Positions, bounds)
	var out mesh.Mesh
	points := map[geom.Vec3]int{}
	ids := make([]int, len(display.Positions)/3)
	for i := range ids {
		p := vector(display.Positions[3*i : 3*i+3]).vec()
		index, ok := points[p]
		if !ok {
			index = len(out.Vertices)
			points[p] = index
			out.Vertices = append(out.Vertices, p)
		}
		ids[i] = index
	}
	for i := 0; i < len(display.Indices); i += 3 {
		out.Triangles = append(out.Triangles, [3]int{
			ids[display.Indices[i]], ids[display.Indices[i+1]], ids[display.Indices[i+2]],
		})
	}
	return out
}

func verifyRationalStepArcTransform(t *testing.T, b *Bridge, result Shape, bounds Bounds, volume, area float64) {
	t.Helper()
	base := box(t, b, vector{}, vector{1, 2, 3})
	rejectGroup(t, b, map[string]any{"op": "common", "a": result.Handle, "b": base.Handle})
	rejectGroup(t, b, map[string]any{"op": "cutMany", "a": base.Handle, "tools": []string{result.Handle}})
	rejectGroup(t, b, map[string]any{
		"op": "transform", "handle": result.Handle,
		"matrix": [3][4]float64{{1, 1e-13, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}},
	})
	offset := vector{1, -2, 3}
	moved := shape(t, b, map[string]any{
		"op": "transform", "handle": result.Handle,
		"matrix": [3][4]float64{{1, 0, 0, offset[0]}, {0, 1, 0, offset[1]}, {0, 0, 1, offset[2]}},
	})
	for i, q := range offset {
		bounds.Min[i] += q
		bounds.Max[i] += q
	}
	checkRationalStepArcShape(t, moved, bounds, volume, area)
	if moved.RepairEvidence != nil || moved.NativeProof != nil {
		t.Fatal("transformed handle described the original world occurrence")
	}
	movedArc := b.shapes[moved.Handle].rationalStepArc
	matrix, err := movedArc.pose.Matrix()
	for i, q := range offset {
		if err != nil || matrix[i][3].Cmp(new(big.Rat).SetFloat64(q)) != 0 {
			t.Fatal("transformed pose was not composed exactly", matrix, err)
		}
	}
	if _, err = b.exportSTEP([]string{moved.Handle}); err != nil {
		t.Fatal("transformed exact occurrence failed strict export", err)
	}
}
