package document

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"sort"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// FromSplineShellAssemblySTEP imports the explicit product graph into existing
// trimmed assets and occurrence nodes, without changing document schema. Source
// geometry is revalidated; supplied product descriptors must match that source.
// Solid fields are derived and are never accepted as proof substitutes.
func FromSplineShellAssemblySTEP(a step.SplineShellAssembly, o STEPAssemblyOptions, historyLimit int) (*Document, error) {
	maxNodes := o.MaxNodes
	if maxNodes == 0 {
		maxNodes = 10000
	}
	if maxNodes < 1 || len(a.Assembly.Products) > maxNodes || len(a.Assembly.Occurrences) > maxNodes {
		return nil, fmt.Errorf("document: spline assembly expansion limits")
	}
	for _, occ := range a.Assembly.Occurrences {
		if !splineRigidPlacement(occ.Transform) {
			return nil, fmt.Errorf("document: spline assembly requires rigid occurrences")
		}
	}
	maxPoints, maxFaces, maxUses, maxBodies := o.Geometry.MaxVertices, o.Geometry.MaxFaces, o.Geometry.MaxEdgeUses, o.Geometry.MaxSolids
	if maxPoints == 0 {
		maxPoints = 1000000
	}
	if maxFaces == 0 {
		maxFaces = 100000
	}
	if maxUses == 0 {
		maxUses = 1000000
	}
	if maxBodies == 0 {
		maxBodies = 1000
	}
	points, faces, bodies := 0, 0, 0
	seen := map[int64]bool{}
	for _, p := range a.Assembly.Products {
		source, e := a.Assembly.SplineShellBodies(p.DefinitionID, o.Geometry)
		if e != nil {
			return nil, e
		}
		supplied := a.ProductBodies[p.DefinitionID]
		if len(source) != len(supplied) {
			return nil, fmt.Errorf("document: spline assembly product/source mismatch")
		}
		for i, b := range source {
			bodies++
			faces += len(b.Faces)
			for _, f := range b.Faces {
				points += len(f.Geometry.Data().Poles)
			}
			if points > maxPoints || faces > maxFaces || faces > maxUses/4 || bodies > maxBodies {
				return nil, fmt.Errorf("document: spline assembly geometry limits")
			}
			other := supplied[i]
			if b.Name != other.Name || len(b.Faces) != len(other.Faces) {
				return nil, fmt.Errorf("document: spline assembly body/source mismatch")
			}
			for j, f := range b.Faces {
				g := other.Faces[j]
				if g.Geometry == nil || f.Name != g.Name || f.Domain != g.Domain || !reflect.DeepEqual(f.Geometry.Data(), g.Geometry.Data()) {
					return nil, fmt.Errorf("document: spline assembly support/source mismatch")
				}
			}
		}
		seen[p.DefinitionID] = true
	}
	for id := range a.ProductBodies {
		if !seen[id] {
			return nil, fmt.Errorf("document: unknown spline assembly product")
		}
	}
	doc, e := fromSTEPAssembly(a.Assembly, o, historyLimit, true)
	if e != nil {
		return nil, e
	}
	state, _ := doc.Snapshot()
	for def, bs := range a.ProductBodies {
		for i, b := range bs {
			id := fmt.Sprintf("product-%d-body-%d", def, i+1)
			label := state.Labels["step:"+id]
			label.Attributes["step.body.name"] = Attribute{Kind: "string", Text: b.Name}
			state.Labels["step:"+id] = label
		}
	}
	return New(state, historyLimit)
}

// WriteSplineShellAssemblySTEP exports the document tree as rigid named
// occurrences of certified local spline-shell assets. Repeated leaf assets stay
// shared. Features are regenerated first; no world-space flattening is performed.
func (s State) WriteSplineShellAssemblySTEP(w io.Writer, rootName string) error {
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
	if w == nil || len(s.Nodes) == 0 || len(s.Nodes) > 10000 {
		return fmt.Errorf("document: invalid spline assembly export")
	}
	if rootName == "" {
		rootName = "PlainSolid spline assembly"
	}
	ids := []string{}
	hasChildren := map[string]bool{}
	for id, n := range s.Nodes {
		ids = append(ids, id)
		hasChildren[n.Parent] = true
	}
	sort.Strings(ids)
	m := step.SplineShellAssemblyModel{LengthUnit: s.LengthUnit, Products: []step.SplineShellAssemblyPart{{ID: "assembly", Name: rootName}}}
	shared := map[string]bool{}
	bodies := map[string]step.SplineShellBody{}
	names := map[string]string{}
	for _, id := range ids {
		n := s.Nodes[id]
		if n.Asset == "" && !hasChildren[id] {
			return fmt.Errorf("document: empty spline assembly node")
		}
		if !splineRigidPlacement(n.Transform) {
			return fmt.Errorf("document: nonrigid spline assembly occurrence")
		}
		var local []step.SplineShellBody
		if n.Asset != "" {
			body, ok := bodies[n.Asset]
			if !ok {
				solid, exists := s.Trimmed[n.Asset]
				if !exists {
					return fmt.Errorf("document: spline assembly requires trimmed assets")
				}
				// The standalone adapter performs exact surface-map extraction and complete
				// embedding validation; identity placement retains all local coordinates.
				only := State{LengthUnit: s.LengthUnit, Trimmed: map[string]trimmed.Solid{n.Asset: solid}, Nodes: map[string]Node{"local": {Name: n.Asset, Asset: n.Asset, Transform: geom.Identity()}}}
				var raw bytes.Buffer
				if e := only.WriteSplineShellSTEP(&raw); e != nil {
					return e
				}
				model, e := step.ReadSplineShells(&raw, step.Limits{})
				if e != nil || len(model.Bodies) != 1 {
					return fmt.Errorf("document: local spline body extraction failed: %v", e)
				}
				body = model.Bodies[0]
				bodies[n.Asset] = body
				names[n.Asset] = n.Asset
				if label, exists := s.Labels["step:"+n.Asset]; exists && label.Target == AssetLabel && label.TargetID == n.Asset && label.Name != "" {
					names[n.Asset] = label.Name
					if attr, ok := label.Attributes["step.body.name"]; ok && attr.Kind == "string" {
						body.Name = attr.Text
						bodies[n.Asset] = body
					}
				}
			}
			local = []step.SplineShellBody{body}
		}
		product := "node:" + id
		name := n.Name
		if name == "" {
			name = id
		}
		if !hasChildren[id] {
			product = "asset:" + n.Asset
			if !shared[product] {
				m.Products = append(m.Products, step.SplineShellAssemblyPart{ID: product, Name: names[n.Asset], Bodies: local})
				shared[product] = true
			}
		} else {
			m.Products = append(m.Products, step.SplineShellAssemblyPart{ID: product, Name: name, Bodies: local})
		}
		parent := "assembly"
		if n.Parent != "" {
			parent = "node:" + n.Parent
		}
		m.Occurrences = append(m.Occurrences, step.AssemblyInstance{ID: id, Name: name, Parent: parent, Child: product, Transform: n.Transform})
	}
	return step.WriteSplineShellAssembly(w, m)
}
