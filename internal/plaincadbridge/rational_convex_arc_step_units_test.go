package plaincadbridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"

	rigidplacement "plainsolid/brep/placement"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
)

type sourceUnitReceipt interface {
	OriginalSTEP() string
	LocalDescriptor() step.ExactCircularLoopExtrusion
	Pose() rigidplacement.RationalRigid
	Solid() trimmed.Solid
}

func TestRationalWorldSTEPBridgeSourceUnits(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{1, 2, 3})
	var handles []string
	for _, tc := range []struct {
		name      string
		file      string
		unit      stepFileUnit
		metres    float64
		mmPerUnit float64
		grouped   bool
	}{
		{"metre", "occt-default-tilted-translated-oblique-prism-fillet-metre", stepMetre, 1, 1000, false},
		{"inch", "occt-default-tilted-translated-oblique-prism-fillet-inch", stepInch, .0254, 25.4, false},
		{"grouped-metre", "occt-both-default-tilted-oblique-prism-fillet-metre", stepMetre, 1, 1000, true},
		{"grouped-inch", "occt-both-default-tilted-oblique-prism-fillet-inch", stepInch, .0254, 25.4, true},
		{"converted-metre", "occt-default-tilted-translated-oblique-prism-fillet-converted-metre", stepMetre, 1, 1000, false},
		{"converted-inch", "occt-default-tilted-translated-oblique-prism-fillet-converted-inch", stepInch, .0254, 25.4, false},
		{"converted-grouped-metre", "occt-both-default-tilted-oblique-prism-fillet-converted-metre", stepMetre, 1, 1000, true},
		{"converted-grouped-inch", "occt-both-default-tilted-oblique-prism-fillet-converted-inch", stepInch, .0254, 25.4, true},
		{"converted-three-metre", "occt-three-default-tilted-oblique-prism-fillet-converted-metre", stepMetre, 1, 1000, true},
		{"converted-three-inch", "occt-three-default-tilted-oblique-prism-fillet-converted-inch", stepInch, .0254, 25.4, true},
		{"converted-four-metre", "occt-four-default-tilted-trapezoidal-prism-fillet-converted-metre", stepMetre, 1, 1000, true},
		{"converted-four-inch", "occt-four-default-tilted-trapezoidal-prism-fillet-converted-inch", stepInch, .0254, 25.4, true},
		{"converted-five-metre", "occt-five-default-tilted-pentagonal-prism-fillet-converted-metre", stepMetre, 1, 1000, true},
		{"converted-five-inch", "occt-five-default-tilted-pentagonal-prism-fillet-converted-inch", stepInch, .0254, 25.4, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := stepRepairFixture(t, tc.file)
			converted := strings.HasPrefix(tc.name, "converted-")
			three := strings.HasPrefix(tc.name, "converted-three-")
			four := strings.HasPrefix(tc.name, "converted-four-")
			five := strings.HasPrefix(tc.name, "converted-five-")
			convertedInch := converted && tc.unit == stepInch
			maxDistance, millimetresPerBase := 1e-12, tc.mmPerUnit
			if convertedInch {
				maxDistance, millimetresPerBase = 1e-11, 5*tc.mmPerUnit
			}
			other := stepMetre
			if tc.unit == stepMetre {
				other = stepInch
			}
			request := map[string]any{"op": "importStep", "source": source}
			rejectGroup(t, b, request)
			request["stepUnits"] = tc.unit
			rejectGroup(t, b, request)
			if convertedInch {
				request["stepRepair"] = map[string]any{"maxDistance": 1e-12, "nativeText": true}
				rejectGroup(t, b, request)
			}
			if converted && tc.grouped && !convertedInch {
				request["stepRepair"] = map[string]any{"maxDistance": 1e-13, "nativeText": true}
				rejectGroup(t, b, request)
			}
			request["stepRepair"] = map[string]any{"maxDistance": maxDistance, "nativeText": true}
			delete(request, "stepUnits")
			rejectGroup(t, b, request)
			request["stepUnits"] = other
			rejectGroup(t, b, request)
			request["stepUnits"] = tc.unit
			if five && convertedInch {
				rejectGroup(t, b, map[string]any{"op": "importStep", "source": stepRepairFixture(t, tc.file+"-unscaled"), "stepUnits": tc.unit,
					"stepRepair": map[string]any{"maxDistance": maxDistance, "nativeText": true}})
			}
			admitted := shape(t, b, request)
			handles = append(handles, admitted.Handle)
			inspect := shape(t, b, map[string]any{"op": "inspect", "handle": admitted.Handle})
			if inspect.StepUnits != tc.unit || !reflect.DeepEqual(inspect, admitted) || admitted.RepairEvidence == nil ||
				admitted.RepairEvidence.LengthUnit != tc.metres || admitted.RepairEvidence.MaxDistance != maxDistance || admitted.NativeProofJSON == "" {
				t.Fatal("retained native handle lost explicit source-unit evidence")
			}
			arc := b.shapes[admitted.Handle].rationalStepArc
			if arc == nil || arc.descriptor.LengthUnit != tc.metres {
				t.Fatal("bridge flattened the source-unit model")
			}
			ctx := context.Background()
			var reloaded sourceUnitReceipt
			var err error
			bounds := Bounds{Min: vector{1, 0, -.7}, Max: vector{7.025, 3, 5}}
			volume, area := 29.145041881053842, 66.186284791166074
			x, z := big.NewRat(8, 5), big.NewRat(6, 5)
			if convertedInch {
				x, z = big.NewRat(8, 1), big.NewRat(6, 1)
			}
			proof := strings.NewReader(admitted.NativeProofJSON)
			if tc.grouped {
				m, e := step.ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, proof, step.IndependentMaterialJSONLimits{})
				evidence := admitted.RepairEvidence
				finiteOwners, owners, repairs, axialTrims := 8, 28, 8, 1
				if converted {
					finiteOwners = 18
				}
				if three {
					owners, repairs, axialTrims = 34, 10, 2
				}
				if four {
					finiteOwners, owners, repairs, axialTrims = 24, 46, 10, 2
				}
				if five {
					finiteOwners, owners, repairs, axialTrims = 30, 58, 10, 2
				}
				if e != nil || evidence.Family != "occt-rational-posed-grouped-pcurve-convex-arc" || len(evidence.Repairs) != repairs ||
					evidence.PCurveOwners != owners || evidence.FiniteLineOwners != finiteOwners || evidence.RationalCaps != 2 || evidence.AxialTrims != axialTrims ||
					len(m.PCurves()) != owners || len(m.FiniteLineOwners()) != finiteOwners || len(m.FiniteTrims()) != 2 || len(m.AxialTrims()) != axialTrims {
					t.Fatal("grouped bridge receipt lost original finite owner evidence", e)
				}
				if five {
					checkFiveConvertedBridgeReceipt(t, tc.unit, m)
				} else if four {
					checkFourConvertedBridgeReceipt(t, tc.unit, m)
				} else if three {
					checkThreeConvertedBridgeReceipt(t, tc.unit, m)
				} else if converted {
					checkGroupedConvertedBridgeReceipt(t, tc.unit, m)
				}
				reloaded = m
				bounds = Bounds{Min: vector{0, 0, -2.7}, Max: vector{6.025, 3, 3}}
				volume, area = 29.077978807115539, 65.622954970084379
				if three {
					bounds.Max[1] = 2.84375
					volume, area = 28.968988172080639, 64.184278587623695
				}
				if four {
					bounds = Bounds{Min: vector{0, 0, -1.6}, Max: vector{5.2, 2.84375, 3}}
					volume, area = 22.155304183748054, 51.012178844414059
				}
				if five {
					bounds = Bounds{Min: vector{0, 0, -2.4}, Max: vector{5.8, 2.75, 3}}
					volume, area = 39.084768347549009, 68.682455982680239
				}
				x, z = new(big.Rat), new(big.Rat)
			} else {
				m, e := step.ReadRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, proof, step.IndependentMaterialJSONLimits{})
				if e == nil && converted {
					checkConvertedBridgeReceipt(t, tc.unit, admitted.RepairEvidence, m)
				}
				reloaded, err = m, e
			}
			if err != nil || reloaded.OriginalSTEP() != source || reloaded.LocalDescriptor().LengthUnit != tc.metres {
				t.Fatal("written source proof lost the original STEP unit", err)
			}
			matrix, err := reloaded.Pose().Matrix()
			if err != nil || !reflect.DeepEqual(&matrix, admitted.RepairEvidence.WorldPose) ||
				matrix[0][3].Cmp(x) != 0 || matrix[2][3].Cmp(z) != 0 {
				t.Fatal("source proof lost its exact source-unit world pose", err)
			}
			arcMatrix, err := arc.pose.Matrix()
			if err != nil || !reflect.DeepEqual(arcMatrix, matrix) {
				t.Fatal("retained occurrence pose changed during display unit conversion", err)
			}
			checkLocalSTEPArcJSON(t, arc.local, reloaded.Solid())

			for side, want := range [2]vector{bounds.Min, bounds.Max} {
				got := [2]vector{admitted.Bounds.Min, admitted.Bounds.Max}[side]
				for i := range got {
					near(t, got[i], want[i]*millimetresPerBase)
				}
			}
			volume *= math.Pow(millimetresPerBase, 3)
			area *= math.Pow(millimetresPerBase, 2)
			if math.Abs(admitted.GeometryAssertions.Volume/volume-1) > 1e-11 ||
				math.Abs(admitted.GeometryAssertions.SurfaceArea/area-1) > 1e-11 {
				t.Fatal("incorrect analytic millimetre estimates", admitted.GeometryAssertions)
			}
			var display Mesh
			deflection := .025 * tc.mmPerUnit
			if converted {
				deflection = .5 // PlainCAD's ordinary display deflection in millimetres.
			}
			if err = json.Unmarshal(invoke(t, b, map[string]any{"op": "mesh", "handle": admitted.Handle,
				"linearDeflection": deflection, "angularDeflection": .2}), &display); err != nil {
				t.Fatal(err)
			}
			world := rationalStepArcDisplayMesh(t, display, admitted.Bounds)
			p, err := world.MassProperties()
			if err != nil || math.Abs(p.Volume/volume-1) > .001 {
				t.Fatal("incorrect oriented world mesh in millimetres", p.Volume, err)
			}
			var edges displayEdgeResult
			if err = json.Unmarshal(invoke(t, b, map[string]any{"op": "displayEdges", "handle": admitted.Handle,
				"maxEdgeUses": 100, "maxEdgeSegments": 1000}), &edges); err != nil || len(edges.Segments) == 0 {
				t.Fatal("missing world display edges", err)
			}
			checkRationalStepArcDisplayCoordinates(t, edges.Segments, admitted.Bounds)
			shift := 1000.
			if tc.unit == stepInch {
				shift = 127
			}
			moved := shape(t, b, map[string]any{"op": "transform", "handle": admitted.Handle,
				"matrix": [3][4]float64{{1, 0, 0, shift}, {0, 1, 0, 0}, {0, 0, 1, 0}}})
			movedMatrix, err := b.shapes[moved.Handle].rationalStepArc.pose.Matrix()
			wantSourceShift := new(big.Rat).Quo(new(big.Rat).SetFloat64(shift), arc.mmPerUnit)
			wantSourceX := new(big.Rat).Add(matrix[0][3], wantSourceShift)
			if err != nil || movedMatrix[0][3].Cmp(wantSourceX) != 0 || moved.StepUnits != tc.unit {
				t.Fatal("millimetre placement lost exact source coordinates", err)
			}
			near(t, moved.Bounds.Min[0], admitted.Bounds.Min[0]+shift)

			q := map[string]any{"op": "exportStep", "handles": []string{admitted.Handle}}
			rejectGroup(t, b, q)
			q["stepUnits"] = other
			rejectGroup(t, b, q)
			q["stepUnits"] = tc.unit
			q["handles"] = []string{admitted.Handle, stock.Handle}
			rejectGroup(t, b, q)
			q["handles"] = []string{admitted.Handle}
			var exported stepExport
			if err = json.Unmarshal(invoke(t, b, q), &exported); err != nil || exported.Units != tc.unit || exported.ProofUnits != stepMillimetre ||
				len(exported.Before) != 1 || exported.Before[0] != (stepProof{Assertions: admitted.GeometryAssertions, Bounds: admitted.Bounds}) ||
				!reflect.DeepEqual(exported.Before, exported.After) {
				t.Fatal("source-unit export lost file units or millimetre proof", err)
			}
			canonical, err := base64.StdEncoding.DecodeString(exported.Base64)
			if err != nil {
				t.Fatal(err)
			}
			if tc.unit == stepInch && !bytes.Contains(canonical, []byte("LENGTH_MEASURE(0.0254)")) {
				t.Fatal("inch STEP lost exact conversion factor")
			}
			if tc.unit == stepMetre && !bytes.Contains(canonical, []byte("SI_UNIT($,.METRE.)")) {
				t.Fatal("metre STEP lost its original SI context")
			}
			canonicalRequest := map[string]any{"op": "importStep", "source": string(canonical)}
			rejectGroup(t, b, canonicalRequest)
			canonicalRequest["stepUnits"] = other
			rejectGroup(t, b, canonicalRequest)
			canonicalRequest["stepUnits"] = tc.unit
			strict := shape(t, b, canonicalRequest)
			if strict.StepUnits != tc.unit || strict.RepairEvidence != nil || strict.NativeProofJSON != "" || strict.Bounds != admitted.Bounds || strict.GeometryAssertions != admitted.GeometryAssertions {
				t.Fatal("strict canonical reopen changed units or physical properties")
			}
			strictArc := b.shapes[strict.Handle].rationalStepArc
			strictMatrix, err := strictArc.pose.Matrix()
			if err != nil || !reflect.DeepEqual(strictMatrix, matrix) {
				t.Fatal("strict canonical reopen changed the exact source pose", err)
			}
			checkLocalSTEPArcJSON(t, strictArc.local, arc.local)
			again, err := b.exportSTEPInUnits([]string{strict.Handle}, tc.unit)
			if err != nil || again.Base64 != exported.Base64 {
				t.Fatal("source-unit STEP changed on strict canonical re-export", err)
			}
		})
	}
	if len(handles) > 1 {
		rejectGroup(t, b, map[string]any{"op": "exportStep", "stepUnits": stepMetre, "handles": handles})
	}
}

func checkFiveConvertedBridgeReceipt(t *testing.T, unit stepFileUnit, m step.RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) {
	t.Helper()
	scale, knots, height := "1/1000", "5000", "5"
	if unit == stepInch {
		scale, knots, height = "5/127", "635", "25"
	}
	for _, owner := range m.FiniteLineOwners() {
		if owner.PCurveParameterScale == nil || owner.PCurveParameterScale.RatString() != scale {
			t.Fatal("lost the original five-fillet planar conversion", owner.PCurveID)
		}
	}
	spans, trims, caps := m.CylindricalSpanOwners(), m.AxialTrims(), m.FiniteTrims()
	if len(spans) != 20 || trims[0].EdgeID != 388 || trims[1].EdgeID != 653 {
		t.Fatal("lost original axial owners or finite charts")
	}
	var shifts []int64
	for _, span := range spans {
		if span.ModelAngleBounds != nil {
			shifts = append(shifts, span.PCurveID)
		}
		if span.PCurveID == 319 || span.PCurveID == 620 {
			v := "0"
			if span.PCurveID == 620 {
				v = height
			}
			if span.ModelAngleBounds == nil || span.ParameterInterval[0].RatString() != "314159265359/100000000000" ||
				span.ParameterInterval[1].RatString() != "942477796077/200000000000" || span.UVStart[0].Sign() != 0 || span.UVStart[1].RatString() != v ||
				span.UVDerivative[0].RatString() != "1" || span.UVDerivative[1].Sign() != 0 {
				t.Fatal("bridge receipt changed the original half-turn chart", span.PCurveID)
			}
		}
		if span.PCurveID == 666 {
			clip := span.ClippedOwnerParameters
			if span.RepairedToOwner == nil || span.ParameterInterval[1].RatString() != knots || span.UVDerivative[1].RatString() != scale ||
				(unit == stepInch && clip != nil) || (unit == stepMetre && (clip == nil || clip[0].Sign() != 0 || clip[1].RatString() != knots ||
				trims[1].OriginalParameters[1].RatString() != "1250000000000000000000000000001/250000000000000000000000000")) {
				t.Fatal("bridge receipt changed the original projected or authored axial stations", span.PCurveID)
			}
		}
	}
	if !reflect.DeepEqual(shifts, []int64{207, 263, 319, 528, 574, 620}) {
		t.Fatal("bridge receipt lost original shifted owners", shifts)
	}
	for i, cap := range caps {
		if cap.CapOwner.PCurveID != [2]int64{366, 423}[i] || cap.CylinderOwner.PCurveID != [2]int64{377, 417}[i] || cap.EndStation == nil {
			t.Fatal("bridge receipt changed the original cap root", cap.CapOwner.PCurveID)
		}
	}
}

func checkFourConvertedBridgeReceipt(t *testing.T, unit stepFileUnit, m step.RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) {
	t.Helper()
	scale := "1/1000"
	if unit == stepInch {
		scale = "5/127"
	}
	for _, owner := range m.FiniteLineOwners() {
		if owner.PCurveParameterScale == nil || owner.PCurveParameterScale.RatString() != scale {
			t.Fatal("lost original planar parameter scale", owner.PCurveID)
		}
	}
	spans, trims, caps := m.CylindricalSpanOwners(), m.AxialTrims(), m.FiniteTrims()
	if len(spans) != 16 || trims[0].EdgeID != 332 || trims[1].EdgeID != 551 {
		t.Fatal("lost converted cylinder span or axial trim evidence")
	}
	var shifted []int64
	axialSpans := make(map[int64]step.ConvexArcCylinderSpanOwner, 2)
	for _, span := range spans {
		if span.ModelAngleBounds != nil {
			shifted = append(shifted, span.PCurveID)
		}
		if span.RepairedToOwner != nil {
			axialSpans[span.PCurveID] = span
		}
		if span.ClippedOwnerParameters != nil && (unit != stepMetre || span.PCurveID != 564) {
			t.Fatal("unexpected clipped spline owner", span.PCurveID)
		}
	}
	if !reflect.DeepEqual(shifted, []int64{207, 263, 472, 518}) || len(axialSpans) != 2 {
		t.Fatal("lost original shifted charts or the two axial maps", shifted)
	}
	if unit == stepMetre {
		span := axialSpans[564]
		if span.ClippedOwnerParameters == nil || span.ParameterInterval[0].Sign() != 0 || span.ParameterInterval[1].RatString() != "5000" ||
			span.ClippedOwnerParameters[0].Sign() != 0 || span.ClippedOwnerParameters[1].RatString() != "5000" ||
			span.UVDerivative[1].RatString() != scale {
			t.Fatal("lost the bounded metre spline interval")
		}
		trim := trims[1]
		const excess = "1/25000000000000000000000000"
		const projectedEnd = "125000000000000000000000000001/25000000000000000000000000"
		if trim.OriginalParameters[0].Sign() != 0 || trim.OriginalParameters[1].RatString() != projectedEnd ||
			trim.RepairedParameters[0].RatString() != excess || trim.RepairedParameters[1].RatString() != projectedEnd ||
			span.RepairedToOwner[0].RatString() != "-"+excess || span.RepairedToOwner[1].RatString() != "1" {
			t.Fatal("bridge receipt changed projected, repaired or clipped axial stations", trim.EdgeID)
		}
	} else {
		const denominator = "/984251968504000000000000000"
		for i, id := range [2]int64{340, 564} {
			span := axialSpans[id]
			trim := trims[i]
			wantEnd := [2]string{"625000000000000000000000000001", "625000000000000000000000000003"}[i] + denominator
			wantStart := [2]string{"1", "3"}[i] + denominator
			if span.RepairedToOwner == nil || span.RepairedToOwner[1].Sign() <= 0 ||
				span.ParameterInterval[0].Sign() != 0 || span.ParameterInterval[1].RatString() != "635" ||
				span.UVDerivative[1].RatString() != scale ||
				trim.OriginalParameters[0].Sign() != 0 || trim.OriginalParameters[1].RatString() != wantEnd ||
				trim.RepairedParameters[0].RatString() != wantStart || trim.RepairedParameters[1].RatString() != wantEnd {
				t.Fatal("lost the exact inch chart or line stations", id)
			}
			for j, station := range trim.RepairedParameters {
				ownerParameter := new(big.Rat).Add(span.RepairedToOwner[0], new(big.Rat).Mul(span.RepairedToOwner[1], station))
				if ownerParameter.Cmp(trim.OriginalParameters[j]) != 0 || ownerParameter.Cmp(span.ParameterInterval[0]) < 0 ||
					ownerParameter.Cmp(span.ParameterInterval[1]) > 0 {
					t.Fatal("inch axial map changed the source stations or left the original knots", id)
				}
			}
		}
	}
	for i, cap := range caps {
		if cap.CapOwner.PCurveID != [2]int64{310, 367}[i] || cap.CylinderOwner.PCurveID != [2]int64{321, 361}[i] || cap.EndStation == nil {
			t.Fatal("lost the original rational cap pairing", cap.CapOwner.PCurveID)
		}
	}
}

func checkThreeConvertedBridgeReceipt(t *testing.T, unit stepFileUnit, m step.RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) {
	t.Helper()
	scale, end, height := "1/1000", "5000", "5"
	if unit == stepInch {
		scale, end, height = "5/127", "635", "25"
	}
	for _, owner := range m.FiniteLineOwners() {
		if owner.PCurveParameterScale == nil || owner.PCurveParameterScale.RatString() != scale {
			t.Fatal("bridge receipt lost original planar parameter map", owner.PCurveID)
		}
	}
	spans, trims := m.CylindricalSpanOwners(), m.FiniteTrims()
	if len(spans) != 12 || len(trims) != 2 {
		t.Fatal("bridge receipt lost converted cylinder owners or finite cap trims")
	}
	byOwner := make(map[int64]step.ConvexArcCylinderSpanOwner, len(spans))
	for _, span := range spans {
		byOwner[span.PCurveID] = span
	}
	for i, id := range [2]int64{207, 416} {
		span := byOwner[id]
		if span.ModelAngleBounds == nil || span.ParameterInterval[0].RatString() != "2498091544797/1000000000000" ||
			span.ParameterInterval[1].RatString() != "942477796077/200000000000" ||
			span.UVStart[0].Sign() != 0 || span.UVStart[1].RatString() != [2]string{"0", height}[i] ||
			span.UVDerivative[0].RatString() != "1" || span.UVDerivative[1].Sign() != 0 {
			t.Fatal("bridge receipt lost original phase and finite angle bounds", id)
		}
	}
	for i, id := range [2]int64{284, 462} {
		span := byOwner[id]
		trim := m.AxialTrims()[i]
		mapping := span.RepairedToOwner
		if mapping == nil || mapping[1].Sign() <= 0 || trim.EdgeID != [2]int64{276, 449}[i] ||
			span.ParameterInterval[0].Sign() != 0 || span.ParameterInterval[1].RatString() != end ||
			span.UVDerivative[1].RatString() != scale {
			t.Fatal("bridge receipt lost finite axial map", id)
		}
		for j, station := range trim.RepairedParameters {
			original := new(big.Rat).Add(mapping[0], new(big.Rat).Mul(mapping[1], station))
			if original.Cmp(trim.OriginalParameters[j]) != 0 || original.Cmp(span.ParameterInterval[0]) < 0 ||
				original.Cmp(span.ParameterInterval[1]) > 0 {
				t.Fatal("bridge receipt map exceeds authored axial knots", id)
			}
		}
		if trims[i].CapOwner.PCurveID != [2]int64{254, 311}[i] ||
			trims[i].CylinderOwner.PCurveID != [2]int64{265, 305}[i] || trims[i].EndStation == nil {
			t.Fatal("bridge receipt lost original rational cap pairing", trims[i].CapOwner.PCurveID)
		}
	}
}

func checkGroupedConvertedBridgeReceipt(t *testing.T, unit stepFileUnit, m step.RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) {
	t.Helper()
	ownerScale, axialEnd, repairedStart := "1/1000", "5000", "-3/500000000000000000000000000"
	if unit == stepInch {
		ownerScale, axialEnd, repairedStart = "5/127", "635", "1/984251968504000000000000000"
	}
	for _, owner := range m.FiniteLineOwners() {
		if owner.PCurveParameterScale == nil || owner.PCurveParameterScale.RatString() != ownerScale {
			t.Fatal("grouped conversion receipt changed a planar parameter map", owner)
		}
	}
	spans, trims := m.CylindricalSpanOwners(), m.FiniteTrims()
	if len(spans) != 8 {
		t.Fatal("grouped conversion receipt lost cylindrical spans")
	}
	want := map[int64]string{71: "0", 124: "0", 156: "1", 237: "-1", 256: "0", 277: "-1", 308: "0", 329: "1"}
	axial := m.AxialTrims()[0]
	for _, span := range spans {
		if span.UVDerivative[0].RatString() != want[span.PCurveID] {
			t.Fatal("grouped conversion receipt lost a cylindrical traversal", span)
		}
		delete(want, span.PCurveID)
		if span.PCurveID == 256 {
			mapping := span.RepairedToOwner
			if mapping == nil || mapping[1].Sign() <= 0 || axial.EdgeID != 248 || axial.RepairedParameters[0].RatString() != repairedStart ||
				span.ParameterInterval[0].Sign() != 0 || span.ParameterInterval[1].RatString() != axialEnd || span.UVDerivative[1].RatString() != ownerScale {
				t.Fatal("grouped conversion receipt lost the finite axial map", span)
			}
			for i, endpoint := range axial.RepairedParameters {
				authored := new(big.Rat).Add(mapping[0], new(big.Rat).Mul(mapping[1], endpoint))
				if authored.Cmp(axial.OriginalParameters[i]) != 0 || authored.Cmp(span.ParameterInterval[0]) < 0 ||
					authored.Cmp(span.ParameterInterval[1]) > 0 {
					t.Fatal("grouped conversion receipt map exceeds the original authored span", span)
				}
			}
		}
	}
	for i, trim := range trims {
		if trim.CapOwner.PCurveID != [2]int64{226, 283}[i] || trim.CylinderOwner.PCurveID != [2]int64{237, 277}[i] ||
			trim.CylinderOwner.Kind != "finite-reversed-cylinder-circle" || trim.EndStation == nil {
			t.Fatal("grouped conversion receipt lost the original cap pairing", trim)
		}
	}
}

func checkConvertedBridgeReceipt(t *testing.T, unit stepFileUnit, evidence *stepRepairEvidence, m step.RationalPosedPCurveRepairedOCCTConvexArcExtrusion) {
	t.Helper()
	lines, spans := m.FiniteLineOwners(), m.CylindricalSpanOwners()
	if evidence.Family != "occt-rational-posed-single-pcurve-convex-arc" || len(evidence.Repairs) != 3 ||
		evidence.FiniteLineOwners != 18 || evidence.PCurveOwners != 24 || len(lines) != 18 || len(spans) != 4 {
		t.Fatal("converted source bridge receipt lost its chart ledgers")
	}
	ownerScale, axialEnd, height := "1/1000", "5000", "5"
	if unit == stepInch {
		ownerScale, axialEnd, height = "5/127", "635", "25"
	}
	for _, owner := range lines {
		if owner.PCurveParameterScale == nil || owner.PCurveParameterScale.RatString() != ownerScale {
			t.Fatal("converted source receipt changed a planar parameter map", owner)
		}
	}
	const sweep = "2498091544797/1000000000000"
	want := map[int64][5]string{
		125: {axialEnd, "0", "0", "0", ownerScale},
		225: {axialEnd, sweep, "0", "0", ownerScale},
		157: {sweep, "0", "0", "1", "0"},
		246: {sweep, "0", height, "1", "0"},
	}
	for _, span := range spans {
		coefficients := [5]string{span.ParameterInterval[1].RatString(), span.UVStart[0].RatString(), span.UVStart[1].RatString(),
			span.UVDerivative[0].RatString(), span.UVDerivative[1].RatString()}
		if span.ParameterInterval[0].Sign() != 0 || coefficients != want[span.PCurveID] {
			t.Fatal("converted source receipt changed a cylindrical span", span)
		}
		delete(want, span.PCurveID)
	}
	if unit == stepInch {
		for _, owner := range m.PCurves() {
			if owner.PCurveID == 125 &&
				(owner.Kind != "finite-cylinder-line" || owner.DistanceSquaredUpper.RatString() != "625/244140625000031250000000001") {
				t.Fatal("converted inch receipt lost the exact axial drift bound", owner)
			}
		}
	}
}

func checkLocalSTEPArcJSON(t *testing.T, a, b trimmed.Solid) {
	t.Helper()
	var left, right bytes.Buffer
	if err := trimmed.WriteJSON(&left, a); err != nil {
		t.Fatal(err)
	}
	if err := trimmed.WriteJSON(&right, b); err != nil || !bytes.Equal(left.Bytes(), right.Bytes()) {
		t.Fatal("product-local native solid changed", err)
	}
}
