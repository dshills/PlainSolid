package plaincadbridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/trimmed"
)

func TestOrderedMappedSTEPRepairBridge(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := stepRepairFixture(t, "occt_mapped_chain_ordered_joined_independent_roofs")
	bad := stepRepairFixture(t, "occt_mapped_chain_ordered_joined_moved_joint")
	b := New()
	q := func(source string) map[string]any {
		return map[string]any{
			"op": "importStep", "source": source,
			"stepRepair": map[string]any{"family": orderedMappedSTEPFamily, "maxDistance": 1e-12, "native": true, "nativeText": true},
		}
	}
	rejectGroup(t, b, map[string]any{"op": "importStep", "source": raw})
	rejectGroup(t, b, map[string]any{"op": "importStep", "source": raw, "stepRepair": map[string]any{"maxDistance": 1e-12}})
	rejectGroup(t, b, q(bad))
	wrongUnits := q(raw)
	wrongUnits["stepUnits"] = "m"
	rejectGroup(t, b, wrongUnits)
	rejectGroup(t, b, q(stepRepairFixture(t, "occt_mapped_chain_kinked_joined_independent_roofs")))
	if len(b.shapes) != 0 {
		t.Fatal("unproved input published a shape handle")
	}

	admitted := shape(t, b, q(raw))
	if !admitted.GeometryAssertions.Valid || admitted.GeometryAssertions.SolidCount != 1 {
		t.Fatal("expected one verified source solid", admitted.GeometryAssertions)
	}
	near(t, admitted.GeometryAssertions.Volume, .6448671346810797)
	near(t, admitted.GeometryAssertions.SurfaceArea, 4.6513347531185012)
	for i, want := range []float64{0, 0, -1} {
		near(t, admitted.Bounds.Min[i], want)
	}
	near(t, admitted.Bounds.Max[0], .75)
	near(t, admitted.Bounds.Max[1], .75)
	evidence := admitted.RepairEvidence
	if evidence == nil || evidence.Family != orderedMappedSTEPFamily ||
		evidence.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(raw))) ||
		evidence.LengthUnit != .001 || evidence.MaxDistance != 1e-12 || evidence.PCurveOwners != 52 ||
		len(evidence.Repairs) != 0 || evidence.MappedChain == nil {
		t.Fatal("the bridge must expose the opt-in source proof and actual owner count", evidence)
	}
	graph := evidence.MappedChain
	if graph.BoundaryVersion != 13 || len(graph.OriginalFaces) != 7 || len(graph.OriginalEdges) != 4 || len(graph.SupportEvents) != 7 {
		t.Fatal("the bridge must retain every original face, model cell and support event", graph)
	}
	stations := []string{"1/12", "1/4", "3/10", "1/2", "19/36", "3/4", "13/17"}
	masks := [][2]bool{{false, true}, {true, true}, {true, false}, {false, true}, {false, true}, {true, false}, {true, false}}
	for i, event := range graph.SupportEvents {
		if event.ModelStation != stations[i] || event.OriginalJoint != (i%2 == 1) || event.OwnersChangingCells != masks[i] {
			t.Fatal("the bridge must retain the exact original author order", i, event)
		}
	}
	if admitted.NativeProof == nil || string(*admitted.NativeProof) != admitted.NativeProofJSON ||
		fmt.Sprintf("%x", sha256.Sum256([]byte(admitted.NativeProofJSON))) != "c705f9dbb44f21d4e14ae38dea64fa6c00fe207e4c96dddb95ad28b9b663eaa0" {
		t.Fatal("both native receipt transports must retain the audited exact codec bytes")
	}
	shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(*admitted.NativeProof), trimmed.MappedChainShellNativeOptions{})
	if err != nil || len(shell.JoinedSupportEvents()) != 7 || shell.Topology().Faces != 7 {
		t.Fatal("native receipt must independently reprove the ordered original source", err)
	}

	clone := shape(t, b, map[string]any{"op": "clone", "handle": admitted.Handle})
	inspected := shape(t, b, map[string]any{"op": "inspect", "handle": admitted.Handle})
	if !reflect.DeepEqual(admitted, inspected) || clone.Handle == admitted.Handle || clone.NativeProofJSON != admitted.NativeProofJSON {
		t.Fatal("inspect and clone must retain the original exact source receipt")
	}
	rejectGroup(t, b, map[string]any{"op": "mesh", "handle": admitted.Handle, "linearDeflection": .05})
	rejectGroup(t, b, map[string]any{"op": "mesh", "handle": admitted.Handle, "angularDeflection": .2})
	rejectGroup(t, b, map[string]any{"op": "fuse", "a": admitted.Handle, "b": clone.Handle})
	rejectGroup(t, b, map[string]any{"op": "exportStep", "handles": []string{admitted.Handle, clone.Handle}})

	var display Mesh
	if err := json.Unmarshal(invoke(t, b, map[string]any{
		"op": "mesh", "handle": admitted.Handle, "linearDeflection": .36, "angularDeflection": math.Pi,
	}), &display); err != nil {
		t.Fatal(err)
	}
	body, _ := b.get(admitted.Handle)
	if len(display.Indices) != 3*2822 || len(display.Positions) != len(display.Normals) ||
		display.Bounds != admitted.Bounds || display.GeometryAssertions != admitted.GeometryAssertions ||
		body.mappedChainSTEP.display == nil || body.mappedChainSTEP.display.ValidateSolid() != nil {
		t.Fatal("the application must receive the certified closed display and original analytic assertions")
	}

	var exported stepExport
	if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "exportStep", "handles": []string{admitted.Handle}}), &exported); err != nil {
		t.Fatal(err)
	}
	canonical, err := base64.StdEncoding.DecodeString(exported.Base64)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(canonical)) != "db0329b3ff7c4b9b04770aff59ce97f8a19d26acf91a7facb60dca3deef356e8" {
		t.Fatal("application export must retain the original canonical STEP bytes", err)
	}
	want := []stepProof{{admitted.GeometryAssertions, admitted.Bounds}}
	if exported.Units != stepMillimetre || !reflect.DeepEqual(exported.Before, want) || !reflect.DeepEqual(exported.After, want) {
		t.Fatal("export must certify unchanged analytic measures and bounds on strict private reopen", exported.Before, exported.After)
	}
	rejectGroup(t, b, map[string]any{"op": "importStep", "source": string(canonical)})
	reopened := shape(t, b, q(string(canonical)))
	if reopened.NativeProofJSON != admitted.NativeProofJSON || reopened.GeometryAssertions != admitted.GeometryAssertions ||
		reopened.RepairEvidence.MappedChain.BoundaryVersion != 13 {
		t.Fatal("explicit application reopen must retain the audited material and native recipe")
	}
}
