package plaincadbridge

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math"
	"reflect"
	"strings"
	"testing"

	"plainsolid/exchange/step"
)

func TestAnnularChamferSTEPRepairBridge(t *testing.T) {
	source := stepRepairFixture(t, "occt-eccentric-annular-chamfer-rounded")
	b := New()
	q := map[string]any{"op": "importStep", "source": source}
	rejectGroup(t, b, q)
	repair := map[string]any{"family": annularChamferSTEPFamily, "maxDistance": 1.6e-12, "nativeText": true}
	q["stepRepair"] = repair
	rejectGroup(t, b, q)
	repair["maxDistance"] = 2e-12
	q["stepUnits"] = "m"
	rejectGroup(t, b, q)
	delete(q, "stepUnits")
	s := shape(t, b, q)
	sourceSHA := sha256.Sum256([]byte(source))
	ep := s.RepairEvidence
	if !s.GeometryAssertions.Valid || s.GeometryAssertions.SolidCount != 1 || math.Abs(s.GeometryAssertions.Volume-726.98826535488854) > 1e-5 ||
		ep == nil || ep.Family != annularChamferSTEPFamily || ep.AnnularChamfer == nil || ep.LengthUnit != .001 || ep.SourceSHA256 != hex.EncodeToString(sourceSHA[:]) || ep.MaxDistance != 2e-12 {
		t.Fatal("rounded annular material or repair proof changed in the bridge", s.GeometryAssertions, ep)
	}
	r, err := step.ReadRepairedEccentricAnnularChamferJSON(context.Background(), strings.NewReader(s.NativeProofJSON), step.IndependentMaterialJSONLimits{})
	if err != nil || r.OriginalSTEP() != source || r.Walls() != ep.AnnularChamfer.Walls {
		t.Fatal("the six original wall proofs did not rederive from the receipt", err)
	}
	out, err := b.exportSTEP([]string{s.Handle})
	if err != nil || out.Units != stepMillimetre || !reflect.DeepEqual(out.Before, out.After) {
		t.Fatal("repair export changed analytic material or world bounds", err)
	}
	raw, err := base64.StdEncoding.DecodeString(out.Base64)
	corrected := sha256.Sum256(raw)
	if err != nil || string(raw) != r.CorrectedModelSTEP() || string(raw) == source || hex.EncodeToString(corrected[:]) != ep.AnnularChamfer.CorrectedSHA256 {
		t.Fatal("repair export did not retain the strict carrier", err)
	}
	strict := shape(t, New(), map[string]any{"op": "importStep", "source": string(raw)})
	if strict.RepairEvidence != nil || strict.Bounds != s.Bounds || strict.GeometryAssertions != s.GeometryAssertions {
		t.Fatal("ordinary reopen did not recover the corrected solid", strict.GeometryAssertions)
	}
}
