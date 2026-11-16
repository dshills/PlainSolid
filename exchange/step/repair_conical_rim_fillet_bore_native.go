package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"plainsolid/geom"
)

const conicalRimFilletBoreRepairFormat = "plainsolid-step-conical-rim-fillet-bore-repair-source"

type conicalRimFilletBoreRepairClaim struct {
	SourceSHA256, CorrectedSHA256, NativeSHA256 string
	LengthUnit, MaxDistance                     float64
	SourceBodyID, SourceShellID                 int64
	FaceIDs, EdgeIDs, VertexIDs                 []int64
	Evidence                                    ConicalRimFilletBoreRepairEvidence
}

func conicalRimFilletBoreRepairClaims(m RepairedConicalRimFilletBoreSTEP) ([]byte, error) {
	native, err := prismNativeBytes(m.material.Solid)
	if err != nil {
		return nil, err
	}
	return json.Marshal(conicalRimFilletBoreRepairClaim{
		SourceSHA256: rationalPosedRepairDigest([]byte(m.original)), CorrectedSHA256: rationalPosedRepairDigest([]byte(m.corrected)),
		NativeSHA256: rationalPosedRepairDigest(native), LengthUnit: m.material.LengthUnit, MaxDistance: m.maxDistance,
		SourceBodyID: m.material.SourceBodyID, SourceShellID: m.material.SourceShellID, FaceIDs: m.material.NativeFaceSourceIDs,
		EdgeIDs: m.material.NativeEdgeSourceIDs, VertexIDs: m.material.NativeVertexSourceIDs, Evidence: m.evidence,
	})
}

// WriteRepairedConicalRimFilletBoreJSON writes the original STEP bytes and
// the reproducible repair claims. It verifies the source again before writing.
func WriteRepairedConicalRimFilletBoreJSON(ctx context.Context, w io.Writer, m RepairedConicalRimFilletBoreSTEP) error {
	if ctx == nil || w == nil || m.original == "" {
		return geom.ErrInvalid
	}
	replayed, err := RepairConicalRimFilletBore(ctx, strings.NewReader(m.original), Limits{}, ConicalRimFilletBoreRepairOptions{m.maxDistance})
	if err != nil {
		return err
	}
	claims, err := conicalRimFilletBoreRepairClaims(m)
	if err != nil {
		return err
	}
	reconstructed, err := conicalRimFilletBoreRepairClaims(replayed)
	if err != nil {
		return err
	}
	if !bytes.Equal(claims, reconstructed) {
		return geom.ErrInvalid
	}
	return writeRationalPosedRepairEnvelope(ctx, w, rationalPosedPCurveRepairEnvelope{
		Format: conicalRimFilletBoreRepairFormat, Version: 1, Source: []byte(m.original), MaxDistance: m.maxDistance, Claims: claims,
	})
}

// ReadRepairedConicalRimFilletBoreJSON replays the native repair and requires
// all stored claims to match. The receipt contains no trusted native shape.
func ReadRepairedConicalRimFilletBoreJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (RepairedConicalRimFilletBoreSTEP, error) {
	var zero RepairedConicalRimFilletBoreSTEP
	q, err := readRationalPosedRepairEnvelope(ctx, r, l, conicalRimFilletBoreRepairFormat)
	if err != nil {
		return zero, err
	}
	m, err := RepairConicalRimFilletBore(ctx, bytes.NewReader(q.Source), l.Source, ConicalRimFilletBoreRepairOptions{q.MaxDistance})
	if err != nil {
		return zero, err
	}
	claims, err := independentCompact(q.Claims)
	if err != nil {
		return zero, err
	}
	want, err := conicalRimFilletBoreRepairClaims(m)
	if err != nil {
		return zero, err
	}
	if !bytes.Equal(claims, want) {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return m, nil
}
