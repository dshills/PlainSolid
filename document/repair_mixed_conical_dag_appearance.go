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

// RepairedMixedConicalAP242DAGAppearanceDocument pairs editable native
// geometry and paint with the separately replayable proof of the original
// rounded styled STEP. Document edits leave Repair unchanged.
type RepairedMixedConicalAP242DAGAppearanceDocument struct {
	Document *Document
	Repair   step.RepairedMixedConicalAP242DAGAppearance
}

// ReadRoundedMixedConicalAP242DAGAppearanceSTEP explicitly repairs the
// original styled source in file units before projecting it into a native
// document with shared geometry and original path-specific paint owners.
func ReadRoundedMixedConicalAP242DAGAppearanceSTEP(ctx context.Context, r io.Reader, l MixedConicalAP242DAGDocumentLimits, options step.MixedConicalAP242DAGRepairOptions, historyLimit int) (RepairedMixedConicalAP242DAGAppearanceDocument, error) {
	var zero RepairedMixedConicalAP242DAGAppearanceDocument
	if l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap || historyLimit < 0 {
		return zero, geom.ErrInvalid
	}
	m, err := step.RepairMixedConicalAP242DAGAppearance(ctx, r, l.Assembly, options)
	if err != nil {
		return zero, err
	}
	return materializeRepairedMixedConicalDAGAppearanceDocument(ctx, m, l.MaxNodes, historyLimit)
}

// ReadRepairedMixedConicalAP242DAGAppearanceReceipt re-proves the original
// source, material, occurrences and paint before reconstructing the initial
// document. An edited native snapshot can be saved independently.
func ReadRepairedMixedConicalAP242DAGAppearanceReceipt(ctx context.Context, r io.Reader, l MixedConicalAP242DAGDocumentLimits, maxReceiptBytes int64, historyLimit int) (RepairedMixedConicalAP242DAGAppearanceDocument, error) {
	var zero RepairedMixedConicalAP242DAGAppearanceDocument
	if l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap || historyLimit < 0 {
		return zero, geom.ErrInvalid
	}
	m, err := step.ReadRepairedMixedConicalAP242DAGAppearanceJSON(ctx, r,
		step.MixedConicalAP242DAGRepairJSONLimits{MaxBytes: maxReceiptBytes, Source: l.Assembly})
	if err != nil {
		return zero, err
	}
	return materializeRepairedMixedConicalDAGAppearanceDocument(ctx, m, l.MaxNodes, historyLimit)
}

func materializeRepairedMixedConicalDAGAppearanceDocument(ctx context.Context, m step.RepairedMixedConicalAP242DAGAppearance, maxNodes, historyLimit int) (RepairedMixedConicalAP242DAGAppearanceDocument, error) {
	var zero RepairedMixedConicalAP242DAGAppearanceDocument
	d, err := mixedConicalDAGAppearanceDocument(ctx, m.Appearance(), maxNodes, 0)
	if err != nil {
		return zero, err
	}
	d, err = conicalDocumentWithRepairProvenance(ctx, d, m, historyLimit)
	if err != nil {
		return zero, err
	}
	return RepairedMixedConicalAP242DAGAppearanceDocument{Document: d, Repair: m}, nil
}

// WriteRepairedMixedConicalAP242DAGAppearanceSTEP verifies the native
// snapshot's repair provenance against the supplied proof, then transfers
// edits to its corrected strict carrier. The original rounded STEP and
// source receipt remain available from the proof.
func (s State) WriteRepairedMixedConicalAP242DAGAppearanceSTEP(ctx context.Context, w io.Writer, repair step.RepairedMixedConicalAP242DAGAppearance, l MixedConicalAP242DAGDocumentLimits) error {
	if ctx == nil || w == nil || repair.CorrectedModelSTEP() == "" || l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap {
		return geom.ErrInvalid
	}
	d, err := materializeRepairedMixedConicalDAGAppearanceDocument(ctx, repair, l.MaxNodes, 0)
	if err != nil {
		return err
	}
	before, _ := d.Document.Snapshot()
	const repairLabel = "source:step-repair"
	if !reflect.DeepEqual(s.Labels[repairLabel], before.Labels[repairLabel]) {
		return fmt.Errorf("document: STEP repair source provenance changed: %w", geom.ErrUnrepresentable)
	}
	for _, p := range repair.PartRepairs() {
		key := "source:" + mixedConicalProductKey(p.DefinitionID)
		if !reflect.DeepEqual(s.Labels[key], before.Labels[key]) {
			return fmt.Errorf("document: STEP repair part provenance changed for %q: %w", key, geom.ErrUnrepresentable)
		}
	}
	return s.WriteMixedConicalAP242DAGAppearanceSTEP(ctx, w, strings.NewReader(repair.CorrectedModelSTEP()), l, geom.SurfacePatchOptions{})
}
