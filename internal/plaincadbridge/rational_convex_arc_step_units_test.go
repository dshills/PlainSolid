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

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
)

func TestRationalWorldSTEPBridgeSourceUnits(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{1, 2, 3})
	var handles []string
	for _, tc := range []struct {
		fixture   string
		unit      stepFileUnit
		metres    float64
		mmPerUnit float64
	}{
		{"metre", stepMetre, 1, 1000},
		{"inch", stepInch, .0254, 25.4},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			source := stepRepairFixture(t, "occt-default-tilted-translated-oblique-prism-fillet-"+tc.fixture)
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
			reloaded, err := step.ReadRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, strings.NewReader(admitted.NativeProofJSON), step.IndependentMaterialJSONLimits{})
			if err != nil || reloaded.OriginalSTEP() != source || reloaded.LocalDescriptor().LengthUnit != tc.metres {
				t.Fatal("written source proof lost the original STEP unit", err)
			}
			matrix, err := reloaded.Pose().Matrix()
			if err != nil || !reflect.DeepEqual(&matrix, admitted.RepairEvidence.WorldPose) ||
				matrix[0][3].Cmp(big.NewRat(8, 5)) != 0 || matrix[2][3].Cmp(big.NewRat(6, 5)) != 0 {
				t.Fatal("source proof lost its exact source-unit world pose", err)
			}
			arcMatrix, err := arc.pose.Matrix()
			if err != nil || !reflect.DeepEqual(arcMatrix, matrix) {
				t.Fatal("retained occurrence pose changed during display unit conversion", err)
			}
			checkLocalSTEPArcJSON(t, arc.local, reloaded.Solid())

			for axis, want := range []vector{{1, 0, -.7}, {7.025, 3, 5}} {
				got := [2]vector{admitted.Bounds.Min, admitted.Bounds.Max}[axis]
				for i := range got {
					near(t, got[i], want[i]*tc.mmPerUnit)
				}
			}
			volume := 29.145041881053842 * math.Pow(tc.mmPerUnit, 3)
			area := 66.186284791166074 * math.Pow(tc.mmPerUnit, 2)
			if math.Abs(admitted.GeometryAssertions.Volume/volume-1) > 1e-11 ||
				math.Abs(admitted.GeometryAssertions.SurfaceArea/area-1) > 1e-11 {
				t.Fatal("incorrect analytic millimetre estimates", admitted.GeometryAssertions)
			}
			var display Mesh
			if err = json.Unmarshal(invoke(t, b, map[string]any{"op": "mesh", "handle": admitted.Handle,
				"linearDeflection": .025 * tc.mmPerUnit, "angularDeflection": .2}), &display); err != nil {
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
	rejectGroup(t, b, map[string]any{"op": "exportStep", "stepUnits": stepMetre, "handles": handles})
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
