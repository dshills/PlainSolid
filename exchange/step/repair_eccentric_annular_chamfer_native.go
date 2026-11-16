package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"plainsolid/geom"
)

const eccentricAnnularChamferRepairFormat = "plainsolid-step-eccentric-annular-chamfer-repair-source"

type eccentricAnnularChamferRepairClaim struct {
	SourceSHA256, CorrectedSHA256, NativeSHA256 string
	LengthUnit, MaxDistance                     float64
	SourceBodyID, SourceShellID                 int64
	FaceIDs, EdgeIDs, VertexIDs                 []int64
	Walls                                       [6]EccentricAnnularChamferWallRepair
}

func eccentricAnnularChamferRepairClaims(m RepairedEccentricAnnularChamferSTEP) ([]byte, error) {
	p := m.material
	native, err := prismNativeBytes(p.Solid)
	if err != nil {
		return nil, err
	}
	return json.Marshal(eccentricAnnularChamferRepairClaim{
		SourceSHA256: rationalPosedRepairDigest([]byte(m.original)), CorrectedSHA256: rationalPosedRepairDigest([]byte(m.corrected)),
		NativeSHA256: rationalPosedRepairDigest(native), LengthUnit: p.LengthUnit, MaxDistance: m.maxDistance,
		SourceBodyID: p.SourceBodyID, SourceShellID: p.SourceShellID, FaceIDs: p.NativeFaceSourceIDs,
		EdgeIDs: p.NativeEdgeSourceIDs, VertexIDs: p.NativeVertexSourceIDs, Walls: m.walls,
	})
}

// WriteRepairedEccentricAnnularChamferJSON writes the original STEP bytes
// and source-bound proof claims after verifying them by replay.
func WriteRepairedEccentricAnnularChamferJSON(ctx context.Context, w io.Writer, m RepairedEccentricAnnularChamferSTEP) error {
	if ctx == nil || w == nil || m.original == "" {
		return geom.ErrInvalid
	}
	replayed, err := RepairEccentricAnnularChamfer(ctx, strings.NewReader(m.original), Limits{}, EccentricAnnularChamferRepairOptions{m.maxDistance})
	if err != nil {
		return err
	}
	claim, err := eccentricAnnularChamferRepairClaims(m)
	if err != nil {
		return err
	}
	want, err := eccentricAnnularChamferRepairClaims(replayed)
	if err != nil {
		return err
	}
	if !bytes.Equal(claim, want) {
		return geom.ErrInvalid
	}
	return writeRationalPosedRepairEnvelope(ctx, w, rationalPosedPCurveRepairEnvelope{
		Format: eccentricAnnularChamferRepairFormat, Version: 1, Source: []byte(m.original), MaxDistance: m.maxDistance, Claims: claim,
	})
}

// ReadRepairedEccentricAnnularChamferJSON repairs the original again and
// verifies every source-bound claim. No stored native solid is trusted.
func ReadRepairedEccentricAnnularChamferJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (RepairedEccentricAnnularChamferSTEP, error) {
	var zero RepairedEccentricAnnularChamferSTEP
	q, err := readRationalPosedRepairEnvelope(ctx, r, l, eccentricAnnularChamferRepairFormat)
	if err != nil {
		return zero, err
	}
	m, err := RepairEccentricAnnularChamfer(ctx, bytes.NewReader(q.Source), l.Source, EccentricAnnularChamferRepairOptions{q.MaxDistance})
	if err != nil {
		return zero, err
	}
	claim, err := independentCompact(q.Claims)
	if err != nil {
		return zero, err
	}
	want, err := eccentricAnnularChamferRepairClaims(m)
	if err != nil {
		return zero, err
	}
	if !bytes.Equal(claim, want) {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return m, nil
}
