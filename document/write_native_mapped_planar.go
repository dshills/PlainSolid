package document

import (
	"fmt"
	"io"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"sort"
)

// WriteNativeMappedPlanarSTEP authors mapped STEP from a flat native document:
// one identity root without an asset, with every other node a direct planar
// instance. Two to eight local assets and at most sixteen instances are supported.
// Asset IDs become local product names; root and node names become representation
// and mapped-use names. Labels are not exported as STEP metadata. Every local
// asset must be used. No imported STEP source or provenance labels are required.
// Hierarchy, unevaluated features and nonplanar assets fail rather than flatten.
// Existing WriteSTEPAssembly remains the distinct NAUO product assembly route.
func (s State) WriteNativeMappedPlanarSTEP(w io.Writer, l step.Limits) error {
	fail := func() error { return fmt.Errorf("document: unsupported flat native mapped planar STEP") }
	if w == nil || len(s.Features)+len(s.Assets)+len(s.Parametric)+len(s.Trimmed) != 0 || len(s.Planar) < 2 || len(s.Planar) > 8 || len(s.Nodes) < 3 || len(s.Nodes) > 17 {
		return fail()
	}
	if e := s.Validate(); e != nil {
		return e
	}
	rootID := ""
	for id, n := range s.Nodes {
		if n.Parent == "" {
			if rootID != "" || n.Asset != "" || n.Transform != geom.Identity() {
				return fail()
			}
			rootID = id
		}
	}
	if rootID == "" {
		return fail()
	}
	keys := make([]string, 0, len(s.Planar))
	for key := range s.Planar {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	model := step.NativeMappedPlanarModel{Name: s.Nodes[rootID].Name, LengthUnit: s.LengthUnit}
	indices := map[string]int{}
	for i, key := range keys {
		indices[key] = i
		model.Products = append(model.Products, step.Body{Name: key, Solid: s.Planar[key]})
	}
	ids := make([]string, 0, len(s.Nodes)-1)
	for id := range s.Nodes {
		if id != rootID {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		n := s.Nodes[id]
		index, ok := indices[n.Asset]
		if !ok || n.Parent != rootID {
			return fail()
		}
		model.Uses = append(model.Uses, step.NativeMappedPlanarUse{Name: n.Name, Product: index, Transform: n.Transform})
	}
	return step.WriteNativeMappedPlanarMaterials(w, model, l)
}
