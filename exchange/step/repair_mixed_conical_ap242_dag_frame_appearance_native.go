package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"plainsolid/geom"
)

const mixedConicalAP242DAGFrameAppearanceRepairFormat = "plainsolid-step-mixed-conical-ap242-dag-frame-appearance-repair-source"

func mixedConicalDAGFrameAppearanceRepairClaims(m RepairedMixedConicalAP242DAGFrameAppearance) ([]byte, error) {
	geometry, err := mixedConicalDAGAppearanceRepairClaims(m.geometry)
	if err != nil {
		return nil, err
	}
	// The inner geometry claim binds its staged source (the frame-only
	// correction), the final carrier and the original style and part IDs.
	return json.Marshal(struct {
		SourceSHA256 string
		Options      MixedConicalAP242DAGFrameAppearanceRepairOptions
		Frame        MixedConicalAP242FrameRepairEvidence
		Geometry     json.RawMessage
	}{rationalPosedRepairDigest([]byte(m.original)), m.options, m.frame, geometry})
}

// WriteRepairedMixedConicalAP242DAGFrameAppearanceJSON re-proves both stages
// and writes the unmodified source with its frame, material and paint claims.
func WriteRepairedMixedConicalAP242DAGFrameAppearanceJSON(ctx context.Context, w io.Writer, m RepairedMixedConicalAP242DAGFrameAppearance) error {
	if ctx == nil || w == nil || m.original == "" {
		return geom.ErrInvalid
	}
	replayed, err := RepairMixedConicalAP242DAGFrameAppearance(ctx, strings.NewReader(m.original), MixedConicalAP242DAGLimits{}, m.options)
	if err != nil {
		return err
	}
	claims, err := mixedConicalDAGFrameAppearanceRepairClaims(m)
	if err != nil {
		return err
	}
	want, err := mixedConicalDAGFrameAppearanceRepairClaims(replayed)
	if err != nil {
		return err
	}
	if !bytes.Equal(claims, want) {
		return geom.ErrInvalid
	}
	return writeRationalPosedRepairEnvelope(ctx, w, rationalPosedPCurveRepairEnvelope{
		Format: mixedConicalAP242DAGFrameAppearanceRepairFormat, Version: 1,
		Source: []byte(m.original), MaxDistance: m.options.Geometry.MaxDistance, Claims: claims,
	})
}

// ReadRepairedMixedConicalAP242DAGFrameAppearanceJSON reconstructs the exact
// source-owned frame, corrected local geometry and styles before trusting
// the stored claims.
func ReadRepairedMixedConicalAP242DAGFrameAppearanceJSON(ctx context.Context, r io.Reader, l MixedConicalAP242DAGRepairJSONLimits) (RepairedMixedConicalAP242DAGFrameAppearance, error) {
	var zero RepairedMixedConicalAP242DAGFrameAppearance
	q, err := readRationalPosedRepairEnvelope(ctx, r, IndependentMaterialJSONLimits{MaxBytes: l.MaxBytes, Source: l.Source.Source}, mixedConicalAP242DAGFrameAppearanceRepairFormat)
	if err != nil {
		return zero, err
	}
	var request struct {
		Options MixedConicalAP242DAGFrameAppearanceRepairOptions
	}
	if err := json.Unmarshal(q.Claims, &request); err != nil || q.MaxDistance != request.Options.Geometry.MaxDistance {
		return zero, geom.ErrInvalid
	}
	m, err := RepairMixedConicalAP242DAGFrameAppearance(ctx, bytes.NewReader(q.Source), l.Source, request.Options)
	if err != nil {
		return zero, err
	}
	claims, err := independentCompact(q.Claims)
	if err != nil {
		return zero, err
	}
	want, err := mixedConicalDAGFrameAppearanceRepairClaims(m)
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
