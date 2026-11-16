package document

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// RepairedNestedMixedConicalDocument joins the editable native assembly
// to its separately replayable original STEP repair receipt. Editing the
// document does not change the receipt for the originally imported source.
type RepairedNestedMixedConicalDocument struct {
	Document *Document
	Repair   step.RepairedNestedMixedConicalAP242Assembly
}

// ReadRoundedNestedMixedConicalAP242STEP explicitly repairs the original
// AP242 source using a maximum displacement in its original file unit, and
// materializes its shared native parts and editable occurrence hierarchy.
func ReadRoundedNestedMixedConicalAP242STEP(ctx context.Context, r io.Reader, limits STEPAssemblyOptions, options step.NestedMixedConicalAP242RepairOptions, historyLimit int) (RepairedNestedMixedConicalDocument, error) {
	if err := validateMixedConicalDocumentLimits(limits, historyLimit); err != nil {
		return RepairedNestedMixedConicalDocument{}, err
	}
	m, err := step.RepairNestedMixedConicalAP242Assembly(ctx, r, limits.Geometry, options)
	if err != nil {
		return RepairedNestedMixedConicalDocument{}, err
	}
	return materializeRepairedMixedConicalDocument(ctx, m, limits, historyLimit)
}

// ReadRepairedNestedMixedConicalAP242Receipt re-proves a source-only repair
// receipt under the given limits before recreating the original document.
// An edited native document remains a separate snapshot.
func ReadRepairedNestedMixedConicalAP242Receipt(ctx context.Context, r io.Reader, limits STEPAssemblyOptions, maxReceiptBytes int64, historyLimit int) (RepairedNestedMixedConicalDocument, error) {
	if err := validateMixedConicalDocumentLimits(limits, historyLimit); err != nil {
		return RepairedNestedMixedConicalDocument{}, err
	}
	m, err := step.ReadRepairedNestedMixedConicalAP242JSON(ctx, r, step.IndependentMaterialJSONLimits{MaxBytes: maxReceiptBytes, Source: limits.Geometry})
	if err != nil {
		return RepairedNestedMixedConicalDocument{}, err
	}
	return materializeRepairedMixedConicalDocument(ctx, m, limits, historyLimit)
}

func materializeRepairedMixedConicalDocument(ctx context.Context, m step.RepairedNestedMixedConicalAP242Assembly, limits STEPAssemblyOptions, historyLimit int) (RepairedNestedMixedConicalDocument, error) {
	var zero RepairedNestedMixedConicalDocument
	d, err := mixedConicalDocument(ctx, m.Assembly(), limits.MaxNodes, 0)
	if err != nil {
		return zero, err
	}
	s, _ := d.Snapshot()
	str := func(x string) Attribute { return Attribute{Kind: "string", Text: x} }
	digest := func(x string) Attribute {
		q := sha256.Sum256([]byte(x))
		return str(hex.EncodeToString(q[:]))
	}
	s.Labels["source:step-repair"] = Label{Name: "Original STEP repair", Target: DocumentLabel, Attributes: map[string]Attribute{
		"step.source.original-sha256": digest(m.OriginalSTEP()), "step.source.corrected-sha256": digest(m.CorrectedModelSTEP()),
	}}
	for _, p := range m.PartRepairs() {
		key := fmt.Sprintf("source:step-product-%d", p.DefinitionID)
		label, ok := s.Labels[key]
		if !ok {
			return zero, geom.ErrUnrepresentable
		}
		label.Attributes["step.source.local-original-sha256"] = str(p.LocalSourceSHA256)
		label.Attributes["step.source.local-corrected-sha256"] = str(p.LocalCorrectedSHA256)
		label.Attributes["step.source.native-sha256"] = str(p.NativeSHA256)
		label.Parent = "source:step-repair"
		s.Labels[key] = label
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	d, err = New(s, historyLimit)
	if err != nil {
		return zero, err
	}
	return RepairedNestedMixedConicalDocument{Document: d, Repair: m}, nil
}
