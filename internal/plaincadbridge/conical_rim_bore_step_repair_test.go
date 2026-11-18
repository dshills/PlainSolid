package plaincadbridge

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math"
	"plainsolid/internal/slowtest"
	"reflect"
	"strings"
	"testing"

	"plainsolid/exchange/step"
)

func TestConicalRimBoreSTEPRepairBridge(t *testing.T) {
	slowtest.Skip(t)
	source := stepRepairFixture(t, "occt-conical-double-rim-eccentric-bore-rounded")
	b := New()
	request := map[string]any{"op": "importStep", "source": source}
	rejectGroup(t, b, request)
	request["stepRepair"] = map[string]any{"maxDistance": 1e-11}
	rejectGroup(t, b, request)
	options := map[string]any{"family": conicalRimBoreSTEPFamily, "maxDistance": 1e-12, "nativeText": true}
	request["stepRepair"] = options
	rejectGroup(t, b, request)
	options["maxDistance"] = 1e-11
	request["stepUnits"] = "in"
	rejectGroup(t, b, request)
	delete(request, "stepUnits")
	result := shape(t, b, request)
	if !result.GeometryAssertions.Valid || result.GeometryAssertions.SolidCount != 1 || math.Abs(result.GeometryAssertions.Volume-2282.401400976394) > 1e-7 {
		t.Fatal("repair did not preserve conical material", result.GeometryAssertions)
	}
	proof := result.RepairEvidence
	originalSHA := sha256.Sum256([]byte(source))
	if proof == nil || proof.Family != conicalRimBoreSTEPFamily || proof.SourceSHA256 != hex.EncodeToString(originalSHA[:]) ||
		proof.LengthUnit != .001 || proof.MaxDistance != 1e-11 || proof.ConicalRimBore == nil || proof.ConicalRimBore.Ledger.ExactSlope != "4/3" ||
		proof.ConicalRimBore.Ledger.ReversedBoundID != 51 || result.NativeProofJSON == "" || result.NativeProof != nil {
		t.Fatal("repair evidence was lost in dispatch", proof)
	}
	reopened, err := step.ReadRepairedConicalRimFilletBoreJSON(context.Background(), strings.NewReader(result.NativeProofJSON), step.IndependentMaterialJSONLimits{})
	if err != nil || reopened.OriginalSTEP() != source || !reflect.DeepEqual(reopened.Evidence(), proof.ConicalRimBore.Ledger) {
		t.Fatal("repair receipt did not reprove the original source", err)
	}
	exported, err := b.exportSTEP([]string{result.Handle})
	if err != nil || exported.Units != stepMillimetre || !reflect.DeepEqual(exported.Before, exported.After) {
		t.Fatal("corrected STEP export did not reproduce material", err)
	}
	raw, err := base64.StdEncoding.DecodeString(exported.Base64)
	correctedSHA := sha256.Sum256(raw)
	if err != nil || string(raw) == source || string(raw) != reopened.CorrectedModelSTEP() || hex.EncodeToString(correctedSHA[:]) != proof.ConicalRimBore.CorrectedSHA256 {
		t.Fatal("export did not preserve the corrected STEP bytes", err)
	}
	strict := shape(t, b, map[string]any{"op": "importStep", "source": string(raw)})
	if strict.RepairEvidence != nil || strict.Bounds != result.Bounds || strict.GeometryAssertions != result.GeometryAssertions {
		t.Fatal("strict reopen did not recover the repaired material", strict)
	}
	again, err := b.exportSTEP([]string{strict.Handle})
	if err != nil || again.Base64 != exported.Base64 {
		t.Fatal("strict re-export changed the corrected source", err)
	}
}
