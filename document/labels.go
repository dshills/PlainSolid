package document

import (
	"fmt"
	"math"
	"unicode/utf8"
)

// LabelTarget indicates whether a label belongs to the entire document,
// an occurrence node, an asset/feature definition, or an indexed asset face.
type LabelTarget string

const (
	DocumentLabel LabelTarget = "document"
	NodeLabel     LabelTarget = "node"
	AssetLabel    LabelTarget = "asset"
	FaceLabel     LabelTarget = "face"
)

// Attribute stores one typed application value. Kind selects "string",
// "number", "integer", or "boolean". Other fields must be zero.
type Attribute struct {
	Kind    string
	Text    string  `json:",omitempty"`
	Number  float64 `json:",omitempty"`
	Integer int64   `json:",omitempty"`
	Boolean bool    `json:",omitempty"`
}

// Label attaches a name and typed attributes to a stable document ID.
// TargetID is empty for DocumentLabel. Parent forms a metadata hierarchy
// independent of the occurrence hierarchy and may be empty. Face is used
// only for FaceLabel and identifies an index on the current asset definition.
type Label struct {
	Name       string
	Target     LabelTarget
	TargetID   string               `json:",omitempty"`
	Face       *int                 `json:",omitempty"`
	Parent     string               `json:",omitempty"`
	Attributes map[string]Attribute `json:",omitempty"`
}

func (l Label) clone() Label {
	if l.Face != nil {
		i := *l.Face
		l.Face = &i
	}
	if l.Attributes != nil {
		a := make(map[string]Attribute, len(l.Attributes))
		for k, v := range l.Attributes {
			a[k] = v
		}
		l.Attributes = a
	}
	return l
}

func (s State) validateLabels() error {
	if len(s.Labels) > 10000 {
		return fmt.Errorf("document: label limit exceeded")
	}
	valid := func(v string, n int) bool { return utf8.ValidString(v) && len(v) <= n }
	for id, l := range s.Labels {
		if id == "" || !valid(id, 1024) || !valid(l.Name, 4096) || len(l.Attributes) > 128 {
			return fmt.Errorf("document: invalid label %q", id)
		}
		switch l.Target {
		case DocumentLabel:
			if l.TargetID != "" {
				return fmt.Errorf("document: unexpected target ID for label %q", id)
			}
		case NodeLabel:
			if _, ok := s.Nodes[l.TargetID]; !ok {
				return fmt.Errorf("document: label %q references unknown node", id)
			}
		case AssetLabel:
			_, a := s.Assets[l.TargetID]
			_, b := s.Planar[l.TargetID]
			_, c := s.Parametric[l.TargetID]
			_, d := s.Trimmed[l.TargetID]
			_, e := s.Features[l.TargetID]
			if !a && !b && !c && !d && !e {
				return fmt.Errorf("document: label %q references unknown asset", id)
			}
		case FaceLabel:
			count := 0
			if a, ok := s.Planar[l.TargetID]; ok {
				count = len(a.Faces())
			}
			if a, ok := s.Parametric[l.TargetID]; ok {
				count = len(a.Faces())
			}
			if a, ok := s.Trimmed[l.TargetID]; ok {
				count = len(a.Faces())
			}
			if l.Face == nil || *l.Face < 0 || *l.Face >= count {
				return fmt.Errorf("document: invalid face target on label %q", id)
			}
		default:
			return fmt.Errorf("document: unknown label target %q", l.Target)
		}
		if l.Target != FaceLabel && l.Face != nil {
			return fmt.Errorf("document: unexpected face target on label %q", id)
		}
		if l.Parent != "" {
			if _, ok := s.Labels[l.Parent]; !ok {
				return fmt.Errorf("document: missing parent of label %q", id)
			}
		}
		for k, x := range l.Attributes {
			if k == "" || !valid(k, 1024) || !valid(x.Text, 65536) ||
				x.Kind != "string" && x.Kind != "number" && x.Kind != "integer" && x.Kind != "boolean" ||
				x.Kind != "string" && x.Text != "" || x.Kind != "number" && x.Number != 0 ||
				x.Kind != "integer" && x.Integer != 0 || x.Kind != "boolean" && x.Boolean ||
				math.IsNaN(x.Number) || math.IsInf(x.Number, 0) {
				return fmt.Errorf("document: invalid attribute %q on label %q", k, id)
			}
		}
	}
	mark := map[string]uint8{}
	for id := range s.Labels {
		p := id
		for p != "" && mark[p] == 0 {
			mark[p] = 1
			p = s.Labels[p].Parent
		}
		if p != "" && mark[p] == 1 {
			return fmt.Errorf("document: label cycle at %q", p)
		}
		for p = id; p != "" && mark[p] == 1; p = s.Labels[p].Parent {
			mark[p] = 2
		}
	}
	return nil
}
