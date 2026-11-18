package plaincadbridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"plainsolid/internal/slowtest"
	"reflect"
	"strings"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestRationalSTEPBoreBridge(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	const fixture = "occt-both-default-tilted-oblique-prism-fillet-"
	raw := stepRepairFixture(t, fixture+"through-bore")
	blind := stepRepairFixture(t, fixture+"blind-bore")
	b := New()
	options := map[string]any{"maxDistance": 1e-12, "native": true, "nativeText": true}
	rejectGroup(t, b, map[string]any{"op": "importStep", "source": raw})
	rejectGroup(t, b, map[string]any{"op": "importStep", "source": raw, "stepRepair": map[string]any{"maxDistance": 1e-13}})
	rejectGroup(t, b, map[string]any{"op": "importStep", "source": raw, "stepUnits": "m", "stepRepair": options})
	rejectGroup(t, b, map[string]any{"op": "importStep", "source": blind, "stepRepair": options})
	admitted := shape(t, b, map[string]any{"op": "importStep", "source": raw, "stepRepair": options})
	const boundsVolume = 28.096231102869041
	bounds := Bounds{Min: vector{0, 0, -2.7}, Max: vector{6.025, 3, 3}}
	checkRationalStepArcShape(t, admitted, bounds, boundsVolume, 73.08423752236105)
	e := admitted.RepairEvidence
	if e == nil || e.Family != "occt-rational-posed-grouped-through-bore-pcurve-convex-arc" || e.LengthUnit != .001 || e.MaxDistance != 1e-12 ||
		e.PCurveOwners != 34 || e.RationalCaps != 2 || e.ThroughBore == nil || *e.ThroughBore != (stepRepairBoreEvidence{CapFaces: [2]int64{137, 382},
		CapBounds: [2]int64{247, 432}, RimEdges: [2]int64{250, 435}, WallFace: 492, SeamEdge: 497, SeamOwners: [2]int64{503, 510}}) {
		t.Fatal("bridge lost original cap and bore ownership")
	}
	checkNativeBoreRecipe := func(handle string) {
		t.Helper()
		p := b.shapes[handle].rationalStepArc
		if p == nil || p.hole == nil || p.hole.Center != (geom.Vec2{X: -1, Y: 1}) || p.hole.Radius != .25 {
			t.Fatal("shape handle lost exact native bore recipe")
		}
		faces := p.local.Faces()
		if len(faces) != 11 || len(faces[0].Holes) != 1 || len(faces[1].Holes) != 1 {
			t.Fatal("shape handle lacks both native cap holes")
		}
	}
	checkNativeBoreRecipe(admitted.Handle)
	if admitted.NativeProof == nil || admitted.NativeProofJSON == "" || !bytes.Equal(bytes.TrimSpace([]byte(admitted.NativeProofJSON)), *admitted.NativeProof) {
		t.Fatal("bridge text and native proof disagree")
	}
	proof, err := step.ReadRationalPosedConvexArcThroughBoreJSON(ctx, strings.NewReader(admitted.NativeProofJSON), step.IndependentMaterialJSONLimits{})
	if err != nil || proof.OriginalSTEP() != raw || proof.Bore.SeamAngles[0].RatString() != "314159265359/50000000000" ||
		proof.Bore.SeamParameters[0].RatString() != "1" || proof.Bore.SeamParameters[1].RatString() != "6" {
		t.Fatal("bridge receipt lost the original source and seam chart", err)
	}
	pose, err := proof.Pose().Matrix()
	if err != nil || !reflect.DeepEqual(e.WorldPose, &pose) {
		t.Fatal("native proof lost the original world pose", err)
	}
	var display Mesh
	if err = json.Unmarshal(invoke(t, b, map[string]any{"op": "mesh", "handle": admitted.Handle,
		"linearDeflection": .5, "angularDeflection": .2}), &display); err != nil {
		t.Fatal(err)
	}
	world := rationalStepArcDisplayMesh(t, display, admitted.Bounds)
	if len(world.Triangles) == 0 || len(world.Vertices)-len(world.Triangles)/2 != 0 {
		t.Fatal("bridge world display filled the bore")
	}
	var edges displayEdgeResult
	if err = json.Unmarshal(invoke(t, b, map[string]any{"op": "displayEdges", "handle": admitted.Handle,
		"maxEdgeUses": 100, "maxEdgeSegments": 2000}), &edges); err != nil || len(edges.Segments) == 0 {
		t.Fatal("bridge through-hole display edges are unavailable", err)
	}
	checkRationalStepArcDisplayCoordinates(t, edges.Segments, admitted.Bounds)
	exported, err := b.exportSTEPInUnits([]string{admitted.Handle}, "")
	if err != nil || exported.Units != stepMillimetre {
		t.Fatal("bridge canonical world output lost millimetres", err)
	}
	encoded, err := base64.StdEncoding.DecodeString(exported.Base64)
	if err != nil || !bytes.Contains(encoded, []byte("SI_UNIT(.MILLI.,.METRE.)")) {
		t.Fatal("canonical bytes lost their physical unit", err)
	}
	rejectGroup(t, b, map[string]any{"op": "importStep", "source": string(encoded), "stepUnits": "in"})
	strict := shape(t, b, map[string]any{"op": "importStep", "source": string(encoded)})
	checkNativeBoreRecipe(strict.Handle)
	if strict.RepairEvidence != nil || strict.NativeProofJSON != "" || strict.Bounds != admitted.Bounds || strict.GeometryAssertions != admitted.GeometryAssertions {
		t.Fatal("ordinary strict canonical reopened a different material")
	}
	again, err := b.exportSTEPInUnits([]string{strict.Handle}, "")
	if err != nil || again.Base64 != exported.Base64 {
		t.Fatal("strict holed canonical reopened to different bytes", err)
	}
}
