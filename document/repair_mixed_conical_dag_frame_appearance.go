package document

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// RepairedMixedConicalAP242DAGFrameAppearanceDocument pairs editable native
// geometry and paint with the separate original STEP repair proof.
type RepairedMixedConicalAP242DAGFrameAppearanceDocument struct {
	Document *Document
	Repair   step.RepairedMixedConicalAP242DAGFrameAppearance
}

// ReadRoundedMixedConicalAP242DAGFrameAppearanceSTEP explicitly repairs the
// original styled source frame and local solids. The document retains the
// original shared relationship paths, material ownership and paint owners.
func ReadRoundedMixedConicalAP242DAGFrameAppearanceSTEP(ctx context.Context, r io.Reader, l MixedConicalAP242DAGDocumentLimits, o step.MixedConicalAP242DAGFrameAppearanceRepairOptions, historyLimit int) (RepairedMixedConicalAP242DAGFrameAppearanceDocument, error) {
	var zero RepairedMixedConicalAP242DAGFrameAppearanceDocument
	if l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap || historyLimit < 0 {
		return zero, geom.ErrInvalid
	}
	m, err := step.RepairMixedConicalAP242DAGFrameAppearance(ctx, r, l.Assembly, o)
	if err != nil {
		return zero, err
	}
	return materializeRepairedMixedConicalFrameAppearanceDocument(ctx, m, l.MaxNodes, historyLimit)
}

// ReadRepairedMixedConicalAP242DAGFrameAppearanceReceipt re-proves both
// corrections and appearance from the original bytes before projecting
// the initial document. Edited native snapshots save separately.
func ReadRepairedMixedConicalAP242DAGFrameAppearanceReceipt(ctx context.Context, r io.Reader, l MixedConicalAP242DAGDocumentLimits, maxReceiptBytes int64, historyLimit int) (RepairedMixedConicalAP242DAGFrameAppearanceDocument, error) {
	var zero RepairedMixedConicalAP242DAGFrameAppearanceDocument
	if l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap || historyLimit < 0 {
		return zero, geom.ErrInvalid
	}
	m, err := step.ReadRepairedMixedConicalAP242DAGFrameAppearanceJSON(ctx, r,
		step.MixedConicalAP242DAGRepairJSONLimits{MaxBytes: maxReceiptBytes, Source: l.Assembly})
	if err != nil {
		return zero, err
	}
	return materializeRepairedMixedConicalFrameAppearanceDocument(ctx, m, l.MaxNodes, historyLimit)
}

func materializeRepairedMixedConicalFrameAppearanceDocument(ctx context.Context, m step.RepairedMixedConicalAP242DAGFrameAppearance, maxNodes, historyLimit int) (RepairedMixedConicalAP242DAGFrameAppearanceDocument, error) {
	var zero RepairedMixedConicalAP242DAGFrameAppearanceDocument
	d, err := mixedConicalDAGAppearanceDocument(ctx, m.Appearance(), maxNodes, 0)
	if err != nil {
		return zero, err
	}
	s, _ := d.Snapshot()
	if err := addConicalRepairProvenance(&s, m); err != nil {
		return zero, err
	}
	if err := addConicalFrameRepairProvenance(&s, m.OriginalSTEP(), m.CorrectedModelSTEP(), m.FrameEvidence()); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	d, err = New(s, historyLimit)
	if err != nil {
		return zero, err
	}
	return RepairedMixedConicalAP242DAGFrameAppearanceDocument{Document: d, Repair: m}, nil
}

// WriteRepairedMixedConicalAP242DAGFrameAppearanceSTEP binds the native
// snapshot's original source owners to both proofs and transfers supported
// edits to the corrected strict styled carrier.
func (s State) WriteRepairedMixedConicalAP242DAGFrameAppearanceSTEP(ctx context.Context, w io.Writer, repair step.RepairedMixedConicalAP242DAGFrameAppearance, l MixedConicalAP242DAGDocumentLimits) error {
	if ctx == nil || w == nil || repair.CorrectedModelSTEP() == "" || l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap {
		return geom.ErrInvalid
	}
	d, err := materializeRepairedMixedConicalFrameAppearanceDocument(ctx, repair, l.MaxNodes, 0)
	if err != nil {
		return err
	}
	before, _ := d.Document.Snapshot()
	sourceLabel := func(key string) bool {
		return strings.HasPrefix(key, "source:") || strings.HasPrefix(key, "source-node:") || strings.HasPrefix(key, "source-face:")
	}
	for key, label := range before.Labels {
		if sourceLabel(key) && !reflect.DeepEqual(s.Labels[key], label) {
			return fmt.Errorf("document: STEP repair provenance changed for %q: %w", key, geom.ErrUnrepresentable)
		}
	}
	for key := range s.Labels {
		if _, ok := before.Labels[key]; sourceLabel(key) && !ok {
			return fmt.Errorf("document: unknown original STEP owner %q: %w", key, geom.ErrUnrepresentable)
		}
	}
	return s.WriteMixedConicalAP242DAGAppearanceSTEP(ctx, w, strings.NewReader(repair.CorrectedModelSTEP()), l, geom.SurfacePatchOptions{})
}
