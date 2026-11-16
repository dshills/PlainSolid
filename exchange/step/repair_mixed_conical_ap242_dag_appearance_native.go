package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"plainsolid/geom"
)

const mixedConicalAP242DAGAppearanceRepairFormat = "plainsolid-step-mixed-conical-ap242-dag-appearance-repair-source"

func mixedConicalDAGAppearanceRepairClaims(m RepairedMixedConicalAP242DAGAppearance) ([]byte, error) {
	geometry, err := mixedConicalAP242DAGRepairClaims(m.repair)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Geometry                  json.RawMessage
		ProductStyles, FaceStyles []MixedConicalAP242SurfaceStyle
		PathOverride              DeepSHUOColor
	}{geometry, m.productStyles, m.faceStyles, m.pathOverride})
}

// WriteRepairedMixedConicalAP242DAGAppearanceJSON writes the exact original
// styled source and its deterministic repair and style claims. It re-proves
// both before writing the receipt.
func WriteRepairedMixedConicalAP242DAGAppearanceJSON(ctx context.Context, w io.Writer, m RepairedMixedConicalAP242DAGAppearance) error {
	if ctx == nil || w == nil || m.repair.original == "" {
		return geom.ErrInvalid
	}
	replayed, err := RepairMixedConicalAP242DAGAppearance(ctx, strings.NewReader(m.repair.original),
		MixedConicalAP242DAGLimits{}, MixedConicalAP242DAGRepairOptions{m.repair.maxDistance})
	if err != nil {
		return err
	}
	claims, err := mixedConicalDAGAppearanceRepairClaims(m)
	if err != nil {
		return err
	}
	want, err := mixedConicalDAGAppearanceRepairClaims(replayed)
	if err != nil {
		return err
	}
	if !bytes.Equal(claims, want) {
		return geom.ErrInvalid
	}
	return writeRationalPosedRepairEnvelope(ctx, w, rationalPosedPCurveRepairEnvelope{
		Format: mixedConicalAP242DAGAppearanceRepairFormat, Version: 1,
		Source: []byte(m.repair.original), MaxDistance: m.repair.maxDistance, Claims: claims,
	})
}

// ReadRepairedMixedConicalAP242DAGAppearanceJSON reconstructs the exact
// geometry, original scoped appearance and corrected carrier from the
// retained source before trusting any stored claim.
func ReadRepairedMixedConicalAP242DAGAppearanceJSON(ctx context.Context, r io.Reader, l MixedConicalAP242DAGRepairJSONLimits) (RepairedMixedConicalAP242DAGAppearance, error) {
	var zero RepairedMixedConicalAP242DAGAppearance
	q, err := readRationalPosedRepairEnvelope(ctx, r, IndependentMaterialJSONLimits{MaxBytes: l.MaxBytes, Source: l.Source.Source}, mixedConicalAP242DAGAppearanceRepairFormat)
	if err != nil {
		return zero, err
	}
	m, err := RepairMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(q.Source), l.Source, MixedConicalAP242DAGRepairOptions{q.MaxDistance})
	if err != nil {
		return zero, err
	}
	claims, err := independentCompact(q.Claims)
	if err != nil {
		return zero, err
	}
	want, err := mixedConicalDAGAppearanceRepairClaims(m)
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
