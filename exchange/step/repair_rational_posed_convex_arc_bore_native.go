package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"

	"plainsolid/geom"
)

const rationalPosedConvexArcThroughBoreFormat = "plainsolid-step-rational-posed-convex-arc-through-bore-source"

func rationalPosedThroughBoreClaims(ctx context.Context, m RationalPosedConvexArcThroughBore) ([]byte, error) {
	pose, err := m.Pose().Matrix()
	if err != nil {
		return nil, err
	}
	exterior, err := rationalPosedPCurveClaims(ctx, m.outer, m.outer.base.tolerance)
	if err != nil {
		return nil, err
	}
	material, err := m.Native(ctx)
	if err != nil {
		return nil, err
	}
	native, err := prismNativeBytes(material)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		SourceSHA256         string
		CorrectedModelSHA256 string
		NativeSHA256         string
		MaxDistance          float64
		LengthUnit           float64
		Pose                 [3][4]*big.Rat
		Exterior             json.RawMessage
		Bore                 ConvexArcThroughBoreEvidence
	}{
		SourceSHA256: rationalPosedRepairDigest([]byte(m.OriginalSTEP())), CorrectedModelSHA256: rationalPosedRepairDigest([]byte(m.CorrectedModelSTEP())),
		NativeSHA256: rationalPosedRepairDigest(native), MaxDistance: m.outer.base.tolerance, LengthUnit: m.LocalDescriptor().LengthUnit,
		Pose: pose, Exterior: exterior, Bore: m.Bore,
	})
}

// WriteRationalPosedConvexArcThroughBoreJSON stores the complete original
// source with its exact interior and exterior owner claims and native digest.
func WriteRationalPosedConvexArcThroughBoreJSON(ctx context.Context, w io.Writer, m RationalPosedConvexArcThroughBore) error {
	if ctx == nil || w == nil || m.OriginalSTEP() == "" {
		return geom.ErrInvalid
	}
	distance := m.outer.base.tolerance
	source := []byte(m.OriginalSTEP())
	verified, err := InspectRationalPosedConvexArcThroughBore(ctx, bytes.NewReader(source), Limits{}, ConvexArcRepairOptions{MaxDistance: distance})
	if err != nil {
		return err
	}
	given, err := json.Marshal(m.Bore)
	if err != nil {
		return err
	}
	want, err := json.Marshal(verified.Bore)
	if err != nil || !bytes.Equal(given, want) {
		return geom.ErrInvalid
	}
	claims, err := rationalPosedThroughBoreClaims(ctx, verified)
	if err != nil {
		return err
	}
	return writeRationalPosedRepairEnvelope(ctx, w, rationalPosedPCurveRepairEnvelope{
		Format: rationalPosedConvexArcThroughBoreFormat, Version: 1, Source: source, MaxDistance: distance, Claims: claims,
	})
}

// ReadRationalPosedConvexArcThroughBoreJSON reconstructs the original geometry
// and checks the complete source, pose, and exact cap and bore owner claims.
func ReadRationalPosedConvexArcThroughBoreJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (RationalPosedConvexArcThroughBore, error) {
	var zero RationalPosedConvexArcThroughBore
	q, err := readRationalPosedRepairEnvelope(ctx, r, l, rationalPosedConvexArcThroughBoreFormat)
	if err != nil {
		return zero, err
	}
	m, err := InspectRationalPosedConvexArcThroughBore(ctx, bytes.NewReader(q.Source), l.Source, ConvexArcRepairOptions{MaxDistance: q.MaxDistance})
	if err != nil {
		return zero, err
	}
	want, err := rationalPosedThroughBoreClaims(ctx, m)
	if err != nil {
		return zero, err
	}
	claim, err := independentCompact(q.Claims)
	if err != nil || !bytes.Equal(claim, want) {
		return zero, geom.ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return m, nil
}
