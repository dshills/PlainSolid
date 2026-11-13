package document

import (
	"context"
	"fmt"
	"io"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"sort"
)

// WriteWrappedFrustumSTEP emits each explicitly placed instance as a separate
// material body. Shared assets remain shared in the document; this export does
// not infer STEP assembly structure. Only exact translations are supported by
// the canonical embedding certificate, including parent translation chains.
func (s State) WriteWrappedFrustumSTEP(w io.Writer) error {
	if w == nil {
		return fmt.Errorf("document: nil frustum writer")
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
	model := step.WrappedFrustumModel{LengthUnit: s.LengthUnit}
	for _, id := range ids {
		node := s.Nodes[id]
		solid, ok := s.Trimmed[node.Asset]
		if !ok {
			return fmt.Errorf("document: frustum requires trimmed asset")
		}
		support, proof, e := solid.ValidateCanonicalWrappedFrustum(context.Background(), geom.SurfacePatchOptions{})
		if e != nil {
			return e
		}
		if !proof.Complete {
			return fmt.Errorf("document: unproved canonical frustum")
		}

		depth := 0
		for current := id; current != ""; {
			n := s.Nodes[current]
			a := n.Transform
			identity := geom.Identity()
			for row := 0; row < 3; row++ {
				for col := 0; col < 3; col++ {
					if a[row][col] != identity[row][col] {
						return fmt.Errorf("document: canonical frustum placement must be a translation")
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
		capNames := [2]string{}
		if label, ok := s.Labels["step:"+node.Asset]; ok {
			for k, key := range []string{"step.cap.lower.name", "step.cap.upper.name"} {
				if a, ok := label.Attributes[key]; ok && a.Kind == "string" {
					capNames[k] = a.Text
				}
			}
			if a, ok := label.Attributes["step.face.name"]; ok && a.Kind == "string" {
				faceName = a.Text
			}
		}
		model.Bodies = append(model.Bodies, step.WrappedFrustumBody{Name: name, CapNames: capNames, Face: step.WrappedSplineFace{Name: faceName, Geometry: support}})
	}
	return step.WriteWrappedFrusta(w, model)
}
