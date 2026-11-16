package document

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	rigidplacement "plainsolid/brep/placement"
	"plainsolid/geom"
)

const (
	mixedConicalRotationLabelPrefix = "step-edit:conical-rotation:"
	mixedConicalRotationAttribute   = "step.edit.exact-rotation"
)

type mixedConicalRotationEdit struct {
	relationship int64
	exact        [3][3]*big.Rat
	display      [3][3]float64
}

// SetMixedConicalAP242OccurrenceRotation stages a proper exact rotation on
// one original conical STEP relationship. All expanded uses display the
// same local rotation. The exact intent survives native save/load and is
// checked against the original relationship when writing STEP.
func (s *State) SetMixedConicalAP242OccurrenceRotation(nauoID int64, rotation [3][3]*big.Rat) error {
	if s == nil || nauoID <= 0 || len(s.Nodes) > mixedConicalDAGDocumentNodeCap {
		return geom.ErrInvalid
	}
	display, err := projectMixedConicalRotation(rotation)
	if err != nil {
		return err
	}
	var uses []string
	var relationship int64
	for key, node := range s.Nodes {
		source := s.Labels["source-node:"+key]
		na := source.Attributes["step.source.nauo"]
		if na.Integer != nauoID {
			continue
		}
		rel := source.Attributes["step.source.relationship"]
		if node.Parent == "" || source.Target != NodeLabel || source.TargetID != key ||
			na.Kind != "integer" || rel.Kind != "integer" || rel.Integer <= 0 ||
			source.Attributes["step.source.path"] != conicalStrAttr(key) ||
			key != mixedConicalOccurrenceKey(node.Parent, nauoID) ||
			relationship != 0 && relationship != rel.Integer {
			return geom.ErrUnrepresentable
		}
		relationship = rel.Integer
		uses = append(uses, key)
	}
	if len(uses) == 0 {
		return geom.ErrUnrepresentable
	}
	labelID := mixedConicalRotationLabelKey(nauoID)
	s.Labels[labelID] = Label{
		Name: "Exact STEP occurrence rotation", Target: DocumentLabel,
		Attributes: map[string]Attribute{
			"step.source.nauo": conicalIntAttr(nauoID), "step.source.relationship": conicalIntAttr(relationship),
			mixedConicalRotationAttribute: conicalStrAttr(mixedConicalRotationText(rotation)),
		},
	}
	for _, key := range uses {
		node := s.Nodes[key]
		for i := range display {
			copy(node.Transform[i][:3], display[i][:])
		}
		s.Nodes[key] = node
	}
	return nil
}

func mixedConicalRotationLabelKey(id int64) string {
	return mixedConicalRotationLabelPrefix + strconv.FormatInt(id, 10)
}

func mixedConicalRotationText(rotation [3][3]*big.Rat) string {
	var values []string
	for _, row := range rotation {
		for _, q := range row {
			values = append(values, q.RatString())
		}
	}
	return strings.Join(values, ",")
}

func projectMixedConicalRotation(rotation [3][3]*big.Rat) ([3][3]float64, error) {
	var m [3][4]*big.Rat
	var display [3][3]float64
	for i, row := range rotation {
		copy(m[i][:3], row[:])
		m[i][3] = new(big.Rat)
	}
	if _, err := rigidplacement.NewRationalRigid(m, 256); err != nil {
		return display, geom.ErrUnrepresentable
	}
	for i, row := range rotation {
		for j, q := range row {
			display[i][j], _ = q.Float64()
		}
	}
	return display, nil
}

func readMixedConicalRotationEdits(labels map[string]Label) (map[int64]mixedConicalRotationEdit, error) {
	out := map[int64]mixedConicalRotationEdit{}
	for key, label := range labels {
		value, wanted := label.Attributes[mixedConicalRotationAttribute]
		if !wanted && !strings.HasPrefix(key, mixedConicalRotationLabelPrefix) {
			continue
		}
		nauo, rel := label.Attributes["step.source.nauo"], label.Attributes["step.source.relationship"]
		if len(out) >= 64 || key != mixedConicalRotationLabelKey(nauo.Integer) ||
			label.Target != DocumentLabel || label.TargetID != "" || label.Parent != "" || label.Face != nil ||
			nauo != conicalIntAttr(nauo.Integer) || nauo.Integer <= 0 || rel != conicalIntAttr(rel.Integer) || rel.Integer <= 0 ||
			value != conicalStrAttr(value.Text) || len(value.Text) > 1536 {
			return nil, fmt.Errorf("document: malformed exact STEP rotation label %q: %w", key, geom.ErrUnrepresentable)
		}
		var edit mixedConicalRotationEdit
		tokens := strings.Split(value.Text, ",")
		if len(tokens) != 9 {
			return nil, geom.ErrUnrepresentable
		}
		for i, token := range tokens {
			// Only reduced integer and fraction tokens belong in a native
			// rotation label. Exponents must never reach big.Rat parsing.
			if len(token) > 158 || strings.Trim(token, "0123456789-/") != "" {
				return nil, geom.ErrUnrepresentable
			}
			q, ok := new(big.Rat).SetString(token)
			if !ok || q.RatString() != token {
				return nil, geom.ErrUnrepresentable
			}
			edit.exact[i/3][i%3] = q
		}
		var err error
		edit.display, err = projectMixedConicalRotation(edit.exact)
		if err != nil {
			return nil, err
		}
		edit.relationship = rel.Integer
		out[nauo.Integer] = edit
	}
	return out, nil
}
