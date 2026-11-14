package document

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/brep"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"reflect"
)

// ReadMappedPlanarMaterialsSTEP imports distinct exact local representation maps
// into shared native assets and explicit occurrence nodes. It does not invent
// STEP product definitions or convert mapped uses into an authored NAUO graph.
func ReadMappedPlanarMaterialsSTEP(r io.Reader, o STEPAssemblyOptions, historyLimit int) (*Document, error) {
	a, e := step.ReadMappedPlanarMaterials(r, o.Geometry)
	if e != nil {
		return nil, e
	}
	return FromMappedPlanarMaterialsSTEP(a, o, historyLimit)
}

// FromMappedPlanarMaterialsSTEP revalidates the authoritative complete source and
// binds all cached geometry/placement fields before creating any document. Source
// map/body/face/surface/item IDs survive as native labels. The source-only STEP
// writer remains separate; labels are provenance, not a recipe for changed data.
func FromMappedPlanarMaterialsSTEP(a step.MappedPlanarMaterials, o STEPAssemblyOptions, historyLimit int) (*Document, error) {
	fail := func() (*Document, error) {
		return nil, fmt.Errorf("document: unsupported or stale mapped STEP material")
	}
	if o.MaxNodes == 0 {
		o.MaxNodes = 10000
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 64
	}
	if o.MaxNodes < 1 || o.MaxDepth < 2 || o.MaxDepth > 256 || a.Source == nil {
		return fail()
	}
	var raw bytes.Buffer
	if e := step.WriteMappedPlanarMaterialsSource(&raw, a.Source); e != nil {
		return nil, e
	}
	fresh, e := step.ReadMappedPlanarMaterials(&raw, o.Geometry)
	if e != nil {
		return nil, e
	}
	if a.Name != fresh.Name || a.LengthUnit != fresh.LengthUnit || !reflect.DeepEqual(a.MapOrder, fresh.MapOrder) || len(a.Materials) != len(fresh.Materials) {
		return fail()
	}
	for id, want := range fresh.Materials {
		got, ok := a.Materials[id]
		if !ok || got.Name != want.Name || got.LengthUnit != want.LengthUnit || got.SourceBodyID != want.SourceBodyID || got.MapID != want.MapID || got.RepresentationID != want.RepresentationID || got.Body.Name != want.Body.Name || !reflect.DeepEqual(got.Occurrences, want.Occurrences) || !reflect.DeepEqual(got.Body.Solid.Vertices(), want.Body.Solid.Vertices()) || !reflect.DeepEqual(got.Body.Solid.Edges(), want.Body.Solid.Edges()) || !reflect.DeepEqual(got.Body.Solid.Faces(), want.Body.Solid.Faces()) {
			return fail()
		}
	}
	s := State{LengthUnit: fresh.LengthUnit, Planar: map[string]brep.Solid{}, Nodes: map[string]Node{}, Labels: map[string]Label{}}
	root := "step-mapped-root"
	s.Nodes[root] = Node{Name: fresh.Name, Transform: geom.Identity()}
	attr := func(v int64) Attribute { return Attribute{Kind: "integer", Integer: v} }
	s.Labels["step-mapped-document"] = Label{Target: DocumentLabel, Name: fresh.Name, Attributes: map[string]Attribute{"step.representation.kind": {Kind: "string", Text: "mapped-planar-materials"}}}
	for _, mid := range fresh.MapOrder {
		m := fresh.Materials[mid]
		key := fmt.Sprintf("step-map-%d", mid)
		s.Planar[key] = m.Body.Solid
		s.Labels["source:"+key] = Label{Name: m.Body.Name, Target: AssetLabel, TargetID: key, Attributes: map[string]Attribute{"step.source.map": attr(mid), "step.source.body": attr(m.SourceBodyID)}}
		// readBody retains CLOSED_SHELL face order, so each source face maps to the
		// same native face index. No surface-name matching or nearest-face guess.
		body, ok := fresh.Source.Entities[m.SourceBodyID]
		if !ok || len(body.Records) != 1 || len(body.Records[0].Args) != 2 {
			return fail()
		}
		sid := body.Records[0].Args[1].Ref
		shell, ok := fresh.Source.Entities[sid]
		if !ok || len(shell.Records) != 1 || shell.Records[0].Type != "CLOSED_SHELL" || len(shell.Records[0].Args) != 2 || shell.Records[0].Args[1].Kind != step.Aggregate {
			return fail()
		}
		faces := shell.Records[0].Args[1].Items
		if len(faces) != len(m.Body.Solid.Faces()) {
			return fail()
		}
		for i, v := range faces {
			face, ok := fresh.Source.Entities[v.Ref]
			if !ok || v.Kind != step.Reference || len(face.Records) != 1 || face.Records[0].Type != "ADVANCED_FACE" || len(face.Records[0].Args) != 4 {
				return fail()
			}
			index := i
			s.Labels[fmt.Sprintf("source:%s:face-%d", key, i)] = Label{Target: FaceLabel, TargetID: key, Face: &index, Attributes: map[string]Attribute{"step.source.map": attr(mid), "step.source.body": attr(m.SourceBodyID), "step.source.face": attr(v.Ref), "step.source.surface": attr(face.Records[0].Args[2].Ref)}}
		}
		for _, use := range m.Occurrences {
			if len(s.Nodes) >= o.MaxNodes {
				return fail()
			}
			node := fmt.Sprintf("step-item-%d", use.ItemID)
			if _, duplicate := s.Nodes[node]; duplicate {
				return fail()
			}
			s.Nodes[node] = Node{Name: use.Name, Parent: root, Asset: key, Transform: use.Transform}
			s.Labels["source:"+node] = Label{Name: use.Name, Target: NodeLabel, TargetID: node, Attributes: map[string]Attribute{"step.source.map": attr(mid), "step.source.item": attr(use.ItemID), "step.source.target": attr(use.TargetID)}}
		}
	}
	return New(s, historyLimit)
}
