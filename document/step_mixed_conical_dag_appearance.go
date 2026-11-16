package document

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"maps"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

const mixedConicalAppearanceSourceLabel = "source:step-dag-appearance"

// ReadMixedConicalAP242DAGAppearanceSTEP projects the original styled DAG
// into a document with shared native geometry. The editable sRGB attributes
// are separate from the original STEP owners recorded on each style label.
func ReadMixedConicalAP242DAGAppearanceSTEP(ctx context.Context, r io.Reader, l MixedConicalAP242DAGDocumentLimits, geometry geom.SurfacePatchOptions, historyLimit int) (*Document, error) {
	if ctx == nil || r == nil || l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap || historyLimit < 0 {
		return nil, geom.ErrInvalid
	}
	a, err := step.ReadMixedConicalAP242DAGAppearance(ctx, r, l.Assembly, geometry)
	if err != nil {
		return nil, err
	}
	return mixedConicalDAGAppearanceDocument(ctx, a, l.MaxNodes, historyLimit)
}

func mixedConicalDAGAppearanceDocument(ctx context.Context, a step.MixedConicalAP242DAGAppearance, maxNodes, historyLimit int) (*Document, error) {
	d, err := mixedConicalDAGDocument(ctx, a.DAG, maxNodes, 0)
	if err != nil {
		return nil, err
	}
	s, _ := d.Snapshot()
	s.Labels[mixedConicalAppearanceSourceLabel] = Label{
		Name: "Original AP242 appearance", Target: DocumentLabel,
		Attributes: map[string]Attribute{
			"step.source.original-sha256": conicalStrAttr(fmt.Sprintf("%x", sha256.Sum256([]byte(a.OriginalSTEP())))),
		},
	}
	for _, style := range a.ProductStyles {
		asset := mixedConicalProductKey(style.DefinitionID)
		id := "source:" + asset
		s.Labels[mixedConicalDAGStyleKey(asset)] = mixedConicalStyleLabel(s.Labels[id], id, style)
	}
	for _, style := range a.FaceStyles {
		asset := mixedConicalProductKey(style.DefinitionID)
		id := fmt.Sprintf("source-face:%s:%d", asset, style.NativeFaceIndex)
		s.Labels[mixedConicalDAGFaceStyleKey(asset, style.NativeFaceIndex)] = mixedConicalStyleLabel(s.Labels[id], id, style)
	}
	use := a.PathOverride
	path := mixedConicalProductKey(a.DAG.Assembly.Roots[0])
	for _, id := range use.Path {
		path = mixedConicalOccurrenceKey(path, id)
	}
	c := use.Context
	parent := "source-node:" + path
	label := s.Labels[parent]
	label.Parent = parent
	label.Attributes = mixedConicalStyleAttributes(c.Color)
	attrs := label.Attributes
	attrs["step.source.path"] = conicalStrAttr(path)
	attrs["step.source.definition"] = s.Labels[parent].Attributes["step.source.definition"]
	for key, value := range map[string]int64{
		"body": c.BodyID, "styled-item": c.StyledItemID, "base-styled-item": c.BaseStyledItemID,
		"style-assignment": c.AssignmentID, "style-context": c.ContextRepresentationID,
		"style-definition-representation": c.DefinitionRepresentationID, "style-product-shape": c.ProductShapeID,
		"context-dependent-representation": c.CDSRID, "transformation": c.TransformationID,
		"style-placement": c.PlacementID, "prefix-shuo": use.PrefixSHUOID, "terminal-shuo": use.TerminalSHUOID,
		"outer-nauo": use.Path[0], "middle-nauo": use.Path[1], "nauo": use.Path[2],
	} {
		attrs["step.source."+key] = conicalIntAttr(value)
	}
	attrs["step.source.presentation"] = s.Labels[mixedConicalDAGStyleKey(s.Nodes[path].Asset)].Attributes["step.source.presentation"]
	s.Labels[mixedConicalDAGStyleKey(path)] = label
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return New(s, historyLimit)
}

func mixedConicalDAGStyleKey(owner string) string { return "step-style:" + owner }

func mixedConicalDAGFaceStyleKey(asset string, index int) string {
	return fmt.Sprintf("step-style:%s:face-%d", asset, index)
}

func mixedConicalStyleLabel(owner Label, parent string, s step.MixedConicalAP242SurfaceStyle) Label {
	owner.Parent = parent
	owner.Attributes = mixedConicalStyleAttributes(s.Color)
	for key, value := range map[string]int64{
		"definition": s.DefinitionID, "body": s.BodyID, "styled-item": s.StyledItemID,
		"presentation": s.PresentationID, "style-assignment": s.AssignmentID,
	} {
		owner.Attributes["step.source."+key] = conicalIntAttr(value)
	}
	if s.FaceID != 0 {
		owner.Attributes["step.source.face"] = conicalIntAttr(s.FaceID)
	}
	return owner
}

func mixedConicalStyleAttributes(c step.RGB) map[string]Attribute {
	a := colorAttributes(c)
	a["step.source.srgb.red"] = a["step.srgb.red"]
	a["step.source.srgb.green"] = a["step.srgb.green"]
	a["step.source.srgb.blue"] = a["step.srgb.blue"]
	return a
}

// MixedConicalAP242DAGSurfaceColor resolves editable paint for the given
// expanded occurrence and native face index. Its face fill takes precedence
// over a contextual whole-body fill on this path, then the product default.
func (s State) MixedConicalAP242DAGSurfaceColor(nodeID string, face int) (step.RGB, bool, error) {
	if _, ok := s.Labels[mixedConicalAppearanceSourceLabel]; !ok {
		return step.RGB{}, false, geom.ErrInvalid
	}
	n := s.Nodes[nodeID]
	solid, ok := s.Trimmed[n.Asset]
	if !ok || face < 0 || face >= len(solid.Faces()) {
		return step.RGB{}, false, geom.ErrInvalid
	}
	for _, key := range []string{mixedConicalDAGFaceStyleKey(n.Asset, face), mixedConicalDAGStyleKey(nodeID), mixedConicalDAGStyleKey(n.Asset)} {
		if label, ok := s.Labels[key]; ok {
			color, err := mixedConicalStyleColor(label.Attributes)
			return color, err == nil, err
		}
	}
	return step.RGB{}, false, nil
}

func mixedConicalStyleColor(attrs map[string]Attribute) (step.RGB, error) {
	r, g, b := attrs["step.srgb.red"], attrs["step.srgb.green"], attrs["step.srgb.blue"]
	color := step.RGB{R: r.Number, G: g.Number, B: b.Number}
	if r.Kind != "number" || g.Kind != "number" || b.Kind != "number" || !validColor(color) {
		return step.RGB{}, geom.ErrUnrepresentable
	}
	return color, nil
}

func mixedConicalDAGColorEdits(before, after State) ([]step.MixedConicalAP242SurfaceColorEdit, error) {
	if !identicalMixedConicalStyleLabels(before.Labels[mixedConicalAppearanceSourceLabel], after.Labels[mixedConicalAppearanceSourceLabel], false) {
		return nil, fmt.Errorf("document: STEP appearance source digest changed: %w", geom.ErrUnrepresentable)
	}
	var edits []step.MixedConicalAP242SurfaceColorEdit
	for key, original := range before.Labels {
		id := original.Attributes["step.source.styled-item"].Integer
		if id == 0 {
			continue
		}
		current := after.Labels[key]
		if !identicalMixedConicalStyleLabels(original, current, true) {
			return nil, fmt.Errorf("document: STEP style owner changed for %q: %w", key, geom.ErrUnrepresentable)
		}
		color, err := mixedConicalStyleColor(current.Attributes)
		if err != nil {
			return nil, err
		}
		if old, _ := mixedConicalStyleColor(original.Attributes); color != old {
			edits = append(edits, step.MixedConicalAP242SurfaceColorEdit{StyledItemID: id, Color: color})
		}
	}
	for key, label := range after.Labels {
		if before.Labels[key].Attributes["step.source.styled-item"].Integer != 0 {
			continue
		}
		for _, key := range []string{"step.srgb.red", "step.srgb.green", "step.srgb.blue", "step.source.styled-item"} {
			if _, exists := label.Attributes[key]; exists {
				return nil, fmt.Errorf("document: unsupported new STEP style owner: %w", geom.ErrUnrepresentable)
			}
		}
	}
	return edits, nil
}

func identicalMixedConicalStyleLabels(a, b Label, editableColor bool) bool {
	if a.Name != b.Name || a.Target != b.Target || a.TargetID != b.TargetID || a.Parent != b.Parent ||
		(a.Face == nil) != (b.Face == nil) || a.Face != nil && *a.Face != *b.Face {
		return false
	}
	if !editableColor {
		return maps.Equal(a.Attributes, b.Attributes)
	}
	if len(a.Attributes) != len(b.Attributes) {
		return false
	}
	for key, original := range a.Attributes {
		if key != "step.srgb.red" && key != "step.srgb.green" && key != "step.srgb.blue" && b.Attributes[key] != original {
			return false
		}
	}
	return true
}
