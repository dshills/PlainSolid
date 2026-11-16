package document

import (
	"context"
	"io"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// RepairedMixedConicalAP242DAGDocument retains editable occurrences and
// an independent proof of the original rounded source. After editing the
// document, use Repair.CorrectedModelSTEP as the source for STEP writeback.
type RepairedMixedConicalAP242DAGDocument struct {
	Document *Document
	Repair   step.RepairedMixedConicalAP242DAG
}

// ReadRoundedMixedConicalAP242DAGSTEP explicitly repairs the original bounded
// DAG in file length units, then creates the editable, shared native document.
func ReadRoundedMixedConicalAP242DAGSTEP(ctx context.Context, r io.Reader, l MixedConicalAP242DAGDocumentLimits, repair step.MixedConicalAP242DAGRepairOptions, historyLimit int) (RepairedMixedConicalAP242DAGDocument, error) {
	var zero RepairedMixedConicalAP242DAGDocument
	if l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap || historyLimit < 0 {
		return zero, geom.ErrInvalid
	}
	m, err := step.RepairMixedConicalAP242DAG(ctx, r, l.Assembly, repair)
	if err != nil {
		return zero, err
	}
	return materializeRepairedMixedConicalDAGDocument(ctx, m, l.MaxNodes, historyLimit)
}

// ReadRepairedMixedConicalAP242DAGReceipt re-proves the rounded source,
// its edits and every occurrence under the caller's decoding and expansion
// limits before returning the initial document and its receipt.
func ReadRepairedMixedConicalAP242DAGReceipt(ctx context.Context, r io.Reader, l MixedConicalAP242DAGDocumentLimits, maxReceiptBytes int64, historyLimit int) (RepairedMixedConicalAP242DAGDocument, error) {
	var zero RepairedMixedConicalAP242DAGDocument
	if l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap || historyLimit < 0 {
		return zero, geom.ErrInvalid
	}
	m, err := step.ReadRepairedMixedConicalAP242DAGJSON(ctx, r, step.MixedConicalAP242DAGRepairJSONLimits{MaxBytes: maxReceiptBytes, Source: l.Assembly})
	if err != nil {
		return zero, err
	}
	return materializeRepairedMixedConicalDAGDocument(ctx, m, l.MaxNodes, historyLimit)
}

func materializeRepairedMixedConicalDAGDocument(ctx context.Context, m step.RepairedMixedConicalAP242DAG, maxNodes, historyLimit int) (RepairedMixedConicalAP242DAGDocument, error) {
	var zero RepairedMixedConicalAP242DAGDocument
	d, err := mixedConicalDAGDocument(ctx, m.Assembly(), maxNodes, 0)
	if err != nil {
		return zero, err
	}
	d, err = conicalDocumentWithRepairProvenance(ctx, d, m, historyLimit)
	if err != nil {
		return zero, err
	}
	return RepairedMixedConicalAP242DAGDocument{Document: d, Repair: m}, nil
}
