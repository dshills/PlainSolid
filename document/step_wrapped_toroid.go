package document

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"sort"
)

// FromWrappedToroidalSTEP imports named certified wrapped toroids as local
// trimmed assets. Authoritative supports are re-proved; cached Solid and
// Embedding fields are not trusted. The existing native schema stores seams.
func FromWrappedToroidalSTEP(m step.WrappedToroidalModel, historyLimit int) (*Document, error) {
	var staged bytes.Buffer
	if e := step.WriteWrappedToroidalBodies(&staged, m); e != nil {
		return nil, e
	}
	verified, e := step.ReadWrappedToroidalBodies(&staged, step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		return nil, e
	}
	plain := step.TrimmedModel{LengthUnit: verified.LengthUnit}
	for _, b := range verified.Bodies {
		plain.Bodies = append(plain.Bodies, step.TrimmedBody{Name: b.Name, Solid: b.Solid})
	}
	d, e := FromTrimmedSTEP(plain, historyLimit)
	if e != nil {
		return nil, e
	}
	s, _ := d.Snapshot()
	s.Labels = map[string]Label{}
	for i, b := range verified.Bodies {
		id := fmt.Sprintf("body-%06d", i+1)
		s.Labels["step:"+id] = Label{Name: b.Name, Target: AssetLabel, TargetID: id, Attributes: map[string]Attribute{"step.body.name": {Kind: "string", Text: b.Name}, "step.face.name": {Kind: "string", Text: b.Face.Name}}}
	}
	return New(s, historyLimit)
}

// WriteWrappedToroidalSTEP emits each explicitly placed instance as a separate
// material body. Shared assets remain shared in the document; this export does
// not infer STEP assembly structure. Only exact translations are supported by
// the canonical embedding certificate, including parent translation chains.
func (s State) WriteWrappedToroidalSTEP(w io.Writer) error {
	if w == nil {
		return fmt.Errorf("document: nil toroidal writer")
	}
	if len(s.Features) > 0 {
		var e error
		s, e = s.EvaluateFeatures(context.Background())
		if e != nil {
			return e
		}
	}
	if e := s.Validate(); e != nil {
		return e
	}
	if len(s.Nodes) > 10000 {
		return fmt.Errorf("document: node budget")
	}
	ids := []string{}
	for id, n := range s.Nodes {
		if n.Asset != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	model := step.WrappedToroidalModel{LengthUnit: s.LengthUnit}
	validated := map[string]bool{}
	for _, id := range ids {
		node := s.Nodes[id]
		solid, ok := s.Trimmed[node.Asset]
		if !ok {
			return fmt.Errorf("document: toroid requires trimmed asset")
		}
		if !validated[node.Asset] {
			proof, e := solid.ValidateDisjointSplineComponents(context.Background(), trimmed.SplineRectangleEmbeddingOptions{})
			if e != nil {
				return e
			}
			if len(proof) != 1 || proof[0].Toroid == nil {
				return fmt.Errorf("document: asset is not one certified toroid")
			}
			validated[node.Asset] = true
		}
		faces := solid.Faces()
		if len(faces) != 1 {
			return fmt.Errorf("document: one toroidal face required")
		}
		support, ok := faces[0].Surface.(*geom.WrappedSurfacePatch)
		if !ok {
			return fmt.Errorf("document: wrapped support required")
		}
		depth := 0
		for current := id; current != ""; {
			n := s.Nodes[current]
			a := n.Transform
			identity := geom.Identity()
			for row := 0; row < 3; row++ {
				for col := 0; col < 3; col++ {
					if a[row][col] != identity[row][col] {
						return fmt.Errorf("document: canonical toroid placement must be a translation")
					}
				}
			}
			depth++
			if depth > 256 {
				return fmt.Errorf("document: placement depth budget")
			}
			var e error
			support, _, e = support.Transformed(context.Background(), a, geom.WrappedSurfaceOptions{})
			if e != nil {
				return e
			}
			current = n.Parent
		}
		name := node.Name
		if name == "" {
			name = id
		}
		faceName := ""
		if label, ok := s.Labels["step:"+node.Asset]; ok {
			if a, ok := label.Attributes["step.face.name"]; ok && a.Kind == "string" {
				faceName = a.Text
			}
		}
		model.Bodies = append(model.Bodies, step.WrappedToroidalBody{Name: name, Face: step.WrappedSplineFace{Name: faceName, Geometry: support}})
	}
	return step.WriteWrappedToroidalBodies(w, model)
}
