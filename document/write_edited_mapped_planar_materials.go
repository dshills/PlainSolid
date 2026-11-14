package document

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"reflect"
)

// WriteEditedMappedPlanarMaterialsSTEP exports edited root/occurrence names and
// local placements against retained source products. It accepts the flat node
// layout created by ReadMappedPlanarMaterialsSTEP, including after native reload.
// Every native local solid must still equal its original source topology and
// coordinates. Historical labels are not trusted as geometry evidence. Changed
// units, hierarchy, membership or geometry fail before writing. Keep source
// separately: the native document does not embed the complete STEP graph.
func (s State) WriteEditedMappedPlanarMaterialsSTEP(w io.Writer, source *step.File, l step.Limits) error {
	fail := func() error { return fmt.Errorf("document: unsupported edited mapped planar material") }
	if e := s.Validate(); e != nil {
		return e
	}
	var raw bytes.Buffer
	if e := step.WriteMappedPlanarMaterialsSource(&raw, source); e != nil {
		return e
	}
	a, e := step.ReadMappedPlanarMaterials(&raw, l)
	if e != nil {
		return e
	}
	if s.LengthUnit != a.LengthUnit || len(s.Planar) != len(a.Materials) || len(s.Assets)+len(s.Parametric)+len(s.Trimmed)+len(s.Features) != 0 {
		return fail()
	}
	root, ok := s.Nodes["step-mapped-root"]
	if !ok || root.Parent != "" || root.Asset != "" || root.Transform != geom.Identity() {
		return fail()
	}
	edits := []step.MappedPlanarOccurrence{}
	for _, mid := range a.MapOrder {
		m := a.Materials[mid]
		key := fmt.Sprintf("step-map-%d", mid)
		native, ok := s.Planar[key]
		if !ok || !reflect.DeepEqual(native.Vertices(), m.Body.Solid.Vertices()) || !reflect.DeepEqual(native.Edges(), m.Body.Solid.Edges()) || !reflect.DeepEqual(native.Faces(), m.Body.Solid.Faces()) {
			return fail()
		}
		for _, use := range m.Occurrences {
			n, ok := s.Nodes[fmt.Sprintf("step-item-%d", use.ItemID)]
			if !ok || n.Parent != "step-mapped-root" || n.Asset != key {
				return fail()
			}
			use.Name = n.Name
			use.Transform = n.Transform
			edits = append(edits, use)
		}
	}
	if len(s.Nodes) != len(edits)+1 {
		return fail()
	}
	return step.WriteEditedMappedPlanarMaterials(w, source, root.Name, edits, l)
}
