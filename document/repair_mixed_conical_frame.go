package document

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"reflect"
	"strings"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

const mixedConicalFrameRepairLabel = "source:step-frame-repair"

// RepairedMixedConicalAP242FrameDocument pairs an editable native document
// with the separate proof of its original rounded STEP occurrence.
type RepairedMixedConicalAP242FrameDocument struct {
	Document *Document
	Repair   step.RepairedMixedConicalAP242Frame
}

// ReadRoundedMixedConicalAP242FrameSTEP explicitly repairs one unstyled
// source frame, then projects its corrected exact DAG into a document.
func ReadRoundedMixedConicalAP242FrameSTEP(ctx context.Context, r io.Reader, l MixedConicalAP242DAGDocumentLimits, o step.MixedConicalAP242FrameRepairOptions, historyLimit int) (RepairedMixedConicalAP242FrameDocument, error) {
	var zero RepairedMixedConicalAP242FrameDocument
	if l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap || historyLimit < 0 {
		return zero, geom.ErrInvalid
	}
	m, err := step.RepairMixedConicalAP242Frame(ctx, r, l.Assembly, o)
	if err != nil {
		return zero, err
	}
	return materializeRepairedMixedConicalFrameDocument(ctx, m, l.MaxNodes, historyLimit)
}

// ReadRepairedMixedConicalAP242FrameReceipt re-proves the original source,
// corrected frame and motion budget before reconstructing the initial native
// document. A later edited native snapshot is saved independently.
func ReadRepairedMixedConicalAP242FrameReceipt(ctx context.Context, r io.Reader, l MixedConicalAP242DAGDocumentLimits, maxReceiptBytes int64, historyLimit int) (RepairedMixedConicalAP242FrameDocument, error) {
	var zero RepairedMixedConicalAP242FrameDocument
	if l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap || historyLimit < 0 {
		return zero, geom.ErrInvalid
	}
	m, err := step.ReadRepairedMixedConicalAP242FrameJSON(ctx, r,
		step.MixedConicalAP242DAGRepairJSONLimits{MaxBytes: maxReceiptBytes, Source: l.Assembly})
	if err != nil {
		return zero, err
	}
	return materializeRepairedMixedConicalFrameDocument(ctx, m, l.MaxNodes, historyLimit)
}

func materializeRepairedMixedConicalFrameDocument(ctx context.Context, m step.RepairedMixedConicalAP242Frame, maxNodes, historyLimit int) (RepairedMixedConicalAP242FrameDocument, error) {
	var zero RepairedMixedConicalAP242FrameDocument
	d, err := mixedConicalDAGDocument(ctx, m.Assembly(), maxNodes, 0)
	if err != nil {
		return zero, err
	}
	s, _ := d.Snapshot()
	if err := addConicalFrameRepairProvenance(&s, m.OriginalSTEP(), m.CorrectedModelSTEP(), m.Evidence()); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	d, err = New(s, historyLimit)
	if err != nil {
		return zero, err
	}
	return RepairedMixedConicalAP242FrameDocument{Document: d, Repair: m}, nil
}

func addConicalFrameRepairProvenance(s *State, original, corrected string, e step.MixedConicalAP242FrameRepairEvidence) error {
	digest := func(raw string) Attribute {
		return conicalStrAttr(fmt.Sprintf("%x", sha256.Sum256([]byte(raw))))
	}
	s.Labels[mixedConicalFrameRepairLabel] = Label{
		Name: "Original STEP occurrence repair", Target: DocumentLabel,
		Attributes: map[string]Attribute{
			"step.source.original-sha256": digest(original), "step.source.corrected-sha256": digest(corrected),
			"step.source.nauo": conicalIntAttr(e.OccurrenceID), "step.source.relationship": conicalIntAttr(e.RelationshipID),
			"step.source.context-dependent-representation": conicalIntAttr(e.CDSRID),
			"step.source.transformation":                   conicalIntAttr(e.TransformationID),
			"step.source.from-placement":                   conicalIntAttr(e.SourcePlacementID),
			"step.source.to-placement":                     conicalIntAttr(e.TargetPlacementID),
			"step.source.origin":                           conicalIntAttr(e.OriginID),
			"step.source.x-direction":                      conicalIntAttr(e.DirectionIDs[0]),
			"step.source.z-direction":                      conicalIntAttr(e.DirectionIDs[1]),
			"step.source.parent-definition":                conicalIntAttr(e.ParentID),
			"step.source.definition":                       conicalIntAttr(e.ChildID),
			"step.source.body":                             conicalIntAttr(e.BodyID),
			"step.repair.displacement-upper":               conicalStrAttr(e.DisplacementUpper),
		},
	}
	uses := 0
	for key, n := range s.Nodes {
		id := "source-node:" + key
		label := s.Labels[id]
		if label.Attributes["step.source.nauo"] != conicalIntAttr(e.OccurrenceID) {
			continue
		}
		if n.Asset != mixedConicalProductKey(e.ChildID) || label.Attributes["step.source.relationship"] != conicalIntAttr(e.RelationshipID) {
			return geom.ErrUnrepresentable
		}
		label.Parent = mixedConicalFrameRepairLabel
		s.Labels[id] = label
		uses++
	}
	if uses == 0 {
		return geom.ErrUnrepresentable
	}
	return nil
}

// WriteRepairedMixedConicalAP242FrameSTEP binds the native snapshot to its
// original repair, then uses the strict corrected carrier for placement and
// name edits. STEP recoloring requires a styled source owner.
func (s State) WriteRepairedMixedConicalAP242FrameSTEP(ctx context.Context, w io.Writer, repair step.RepairedMixedConicalAP242Frame, l MixedConicalAP242DAGDocumentLimits) error {
	if ctx == nil || w == nil || repair.CorrectedModelSTEP() == "" || l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap {
		return geom.ErrInvalid
	}
	d, err := materializeRepairedMixedConicalFrameDocument(ctx, repair, l.MaxNodes, 0)
	if err != nil {
		return err
	}
	before, _ := d.Document.Snapshot()
	for key, label := range before.Labels {
		if !reflect.DeepEqual(s.Labels[key], label) {
			return fmt.Errorf("document: STEP frame repair provenance changed for %q: %w", key, geom.ErrUnrepresentable)
		}
	}
	for key, label := range s.Labels {
		if strings.HasPrefix(key, "source-node:") {
			if _, ok := before.Labels[key]; !ok {
				return fmt.Errorf("document: unknown original STEP path %q: %w", key, geom.ErrUnrepresentable)
			}
		}
		if strings.HasPrefix(key, "step-style:") || key == mixedConicalAppearanceSourceLabel {
			return fmt.Errorf("document: repaired STEP frame source has no appearance owner: %w", geom.ErrUnrepresentable)
		}
		for attr := range label.Attributes {
			if strings.HasPrefix(attr, "step.srgb.") || attr == "step.source.styled-item" {
				return fmt.Errorf("document: repaired STEP frame source has no appearance owner: %w", geom.ErrUnrepresentable)
			}
		}
	}
	return s.WriteMixedConicalAP242DAGSTEP(ctx, w, strings.NewReader(repair.CorrectedModelSTEP()), l, geom.SurfacePatchOptions{})
}
