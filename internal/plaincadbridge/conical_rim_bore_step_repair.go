package plaincadbridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"plainsolid/exchange/step"
)

const conicalRimBoreSTEPFamily = "occt-conical-rim-fillet-bore"

type conicalRimBoreSTEPEvidence struct {
	CorrectedSHA256 string                                  `json:"correctedSHA256"`
	Ledger          step.ConicalRimFilletBoreRepairEvidence `json:"ledger"`
}

func (b *Bridge) importConicalRimBoreSTEP(ctx context.Context, source string, q stepRepairRequest, units stepFileUnit) (Shape, error) {
	if len(source) == 0 || len(source) > maxSTEPBytes {
		return Shape{}, &failureError{"LIMIT", "STEP source must contain between 1 byte and 1 MiB"}
	}
	if err := checkRationalSTEPUnit(.001, units); err != nil {
		return Shape{}, err
	}
	maxDistance := float64(q.MaxDistance)
	repaired, err := step.RepairConicalRimFilletBore(ctx, strings.NewReader(source), stepLimits(), step.ConicalRimFilletBoreRepairOptions{MaxDistance: maxDistance})
	if err != nil {
		if ctx.Err() != nil {
			return Shape{}, ctx.Err()
		}
		return Shape{}, unsupportedEdge(fmt.Sprintf("conical rim and bore STEP repair could not be proved within maxDistance: %v", err))
	}
	m := repaired.Material()
	if err := checkRationalSTEPUnit(m.LengthUnit, units); err != nil {
		return Shape{}, err
	}
	var proof bytes.Buffer
	if q.Native || q.NativeText {
		if err := step.WriteRepairedConicalRimFilletBoreJSON(ctx, &proof, repaired); err != nil {
			return Shape{}, err
		}
		if proof.Len() > MaxRequestBytes {
			return Shape{}, &failureError{"LIMIT", "STEP repair native proof exceeds 1 MiB"}
		}
	}
	corrected := repaired.CorrectedModelSTEP()
	p := provedSTEPRevolution{source: corrected, conical: &m}
	shape, err := b.putProvedSTEPRevolution(ctx, m.Solid, p)
	if err != nil {
		return Shape{}, err
	}
	originalSHA := sha256.Sum256([]byte(repaired.OriginalSTEP()))
	correctedSHA := sha256.Sum256([]byte(corrected))
	return b.attachSTEPRepairEvidence(shape, stepRepairEvidence{
		Family:       conicalRimBoreSTEPFamily,
		SourceSHA256: hex.EncodeToString(originalSHA[:]),
		LengthUnit:   m.LengthUnit,
		MaxDistance:  maxDistance,
		Repairs:      []step.ConvexArcRepairSite{},
		ConicalRimBore: &conicalRimBoreSTEPEvidence{
			CorrectedSHA256: hex.EncodeToString(correctedSHA[:]),
			Ledger:          repaired.Evidence(),
		},
	}, q, proof), nil
}
