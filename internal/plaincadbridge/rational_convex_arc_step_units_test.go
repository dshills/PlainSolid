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
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := stepRepairFixture(t, tc.file)
			converted := tc.name == "converted-metre"
			other := stepMetre
			if tc.unit == stepMetre {
				other = stepInch
			}
			request := map[string]any{"op": "importStep", "source": source}
			rejectGroup(t, b, request)
			request["stepUnits"] = tc.unit
			rejectGroup(t, b, request)
			request["stepRepair"] = map[string]any{"maxDistance": 1e-12, "nativeText": true}
			delete(request, "stepUnits")
			rejectGroup(t, b, request)
			request["stepUnits"] = other
			rejectGroup(t, b, request)
			request["stepUnits"] = tc.unit
			admitted := shape(t, b, request)
			handles = append(handles, admitted.Handle)
			inspect := shape(t, b, map[string]any{"op": "inspect", "handle": admitted.Handle})
			if inspect.StepUnits != tc.unit || !reflect.DeepEqual(inspect, admitted) || admitted.RepairEvidence == nil ||
				admitted.RepairEvidence.LengthUnit != tc.metres || admitted.NativeProofJSON == "" {
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
			proof := strings.NewReader(admitted.NativeProofJSON)
			if tc.grouped {
				m, e := step.ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, proof, step.IndependentMaterialJSONLimits{})
				evidence := admitted.RepairEvidence
				if e != nil || evidence.Family != "occt-rational-posed-grouped-pcurve-convex-arc" || len(evidence.Repairs) != 8 ||
					evidence.PCurveOwners != 28 || evidence.FiniteLineOwners != 8 || evidence.RationalCaps != 2 || evidence.AxialTrims != 1 ||
					len(m.PCurves()) != 28 || len(m.FiniteLineOwners()) != 8 || len(m.FiniteTrims()) != 2 || len(m.AxialTrims()) != 1 {
					t.Fatal("grouped bridge receipt lost original finite owner evidence", e)
				}
				reloaded = m
				bounds = Bounds{Min: vector{0, 0, -2.7}, Max: vector{6.025, 3, 3}}
				volume, area = 29.077978807115539, 65.622954970084379
				x, z = new(big.Rat), new(big.Rat)
			} else {
				m, e := step.ReadRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, proof, step.IndependentMaterialJSONLimits{})
				if e == nil && converted {
					checkConvertedMetreBridgeReceipt(t, admitted.RepairEvidence, m)
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
					near(t, got[i], want[i]*tc.mmPerUnit)
				}
			}
			volume *= math.Pow(tc.mmPerUnit, 3)
			area *= math.Pow(tc.mmPerUnit, 2)
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
				len(exported.Before) != 1 || !reflect.DeepEqual(exported.Before, exported.After) {
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

func checkConvertedMetreBridgeReceipt(t *testing.T, evidence *stepRepairEvidence, m step.RationalPosedPCurveRepairedOCCTConvexArcExtrusion) {
	t.Helper()
	lines, spans := m.FiniteLineOwners(), m.CylindricalSpanOwners()
	if evidence.Family != "occt-rational-posed-single-pcurve-convex-arc" || len(evidence.Repairs) != 3 ||
		evidence.FiniteLineOwners != 18 || evidence.PCurveOwners != 24 || len(lines) != 18 || len(spans) != 4 {
		t.Fatal("converted-metre bridge receipt lost its chart ledgers")
	}
	for _, owner := range lines {
		if owner.PCurveParameterScale == nil || owner.PCurveParameterScale.RatString() != "1/1000" {
			t.Fatal("converted-metre receipt changed a planar parameter map", owner)
		}
	}
	want := map[int64][5]string{
		125: {"5000", "0", "0", "0", "1/1000"},
		225: {"5000", "2498091544797/1000000000000", "0", "0", "1/1000"},
		157: {"2498091544797/1000000000000", "0", "0", "1", "0"},
		246: {"2498091544797/1000000000000", "0", "5", "1", "0"},
	}
	for _, span := range spans {
		coefficients := [5]string{span.ParameterInterval[1].RatString(), span.UVStart[0].RatString(), span.UVStart[1].RatString(),
			span.UVDerivative[0].RatString(), span.UVDerivative[1].RatString()}
		if span.ParameterInterval[0].Sign() != 0 || coefficients != want[span.PCurveID] {
			t.Fatal("converted-metre receipt changed a cylindrical span", span)
		}
		delete(want, span.PCurveID)
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
