package document

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"sort"
	"strconv"
)

// ReadWrappedMaterialAssemblySTEP imports the authoritative source graph. Local
// bodies become shared native assets; explicit product and body provenance is
// recorded in labels. No caller-supplied cached source IDs are trusted.
func ReadWrappedMaterialAssemblySTEP(r io.Reader, l step.Limits, o step.WrappedSurfaceOptions, limits STEPAssemblyOptions, historyLimit int) (*Document, error) {
	a, e := step.ReadWrappedMaterialAssembly(r, l, o)
	if e != nil {
		return nil, e
	}
	if limits.MaxNodes == 0 {
		limits.MaxNodes = 10000
	}
	if limits.MaxDepth == 0 {
		limits.MaxDepth = 64
	}
	if limits.MaxNodes < 1 || limits.MaxDepth < 1 || limits.MaxDepth > 256 {
		return nil, fmt.Errorf("document: assembly expansion limits")
	}
	m := step.WrappedMaterialModel{LengthUnit: a.Assembly.LengthUnit}
	assets := map[int64][]string{}
	products := map[int64]step.AssemblyProduct{}
	for _, p := range a.Assembly.Products {
		products[p.DefinitionID] = p
		for _, b := range a.ProductBodies[p.DefinitionID] {
			m.Bodies = append(m.Bodies, b)
			assets[p.DefinitionID] = append(assets[p.DefinitionID], fmt.Sprintf("body-%06d", len(m.Bodies)))
		}
	}
	d, e := FromWrappedMaterialsSTEP(m, historyLimit)
	if e != nil {
		return nil, e
	}
	s, _ := d.Snapshot()
	s.Nodes = map[string]Node{}
	children := map[int64][]step.AssemblyOccurrence{}
	for _, occ := range a.Assembly.Occurrences {
		children[occ.Parent] = append(children[occ.Parent], occ)
	}
	text := func(v string) Attribute { return Attribute{Kind: "string", Text: v} }
	var expand func(int64, string, string, string, geom.Affine, int) error
	expand = func(pid int64, id, parent, name string, tr geom.Affine, depth int) error {
		if depth > limits.MaxDepth || len(s.Nodes) >= limits.MaxNodes {
			return fmt.Errorf("document: mixed assembly expansion budget")
		}
		p := products[pid]
		if name == "" {
			name = p.Name
		}
		s.Nodes[id] = Node{Name: name, Parent: parent, Transform: tr}
		key := strconv.FormatInt(pid, 10)
		s.Labels["step.product:"+id] = Label{Name: p.Name, Target: NodeLabel, TargetID: id, Attributes: map[string]Attribute{"step.product.id": text(key), "step.product.identifier": text(p.Identifier), "step.product.name": text(p.Name)}}
		for j, asset := range assets[pid] {
			if len(s.Nodes) >= limits.MaxNodes {
				return fmt.Errorf("document: mixed assembly expansion budget")
			}
			member := fmt.Sprintf("%s/body-%d", id, j+1)
			name := s.Labels["step:"+asset].Name
			s.Nodes[member] = Node{Name: name, Parent: id, Asset: asset, Transform: geom.Identity()}
			s.Labels["step.member:"+member] = Label{Name: name, Target: NodeLabel, TargetID: member, Attributes: map[string]Attribute{"step.product.id": text(key), "step.body.asset": text(asset), "step.source.body.id": {Kind: "integer", Integer: a.ProductBodies[pid][j].SourceBodyID}}}
			label := s.Labels["step:"+asset]
			label.Attributes["step.product.id"] = text(key)
			label.Attributes["step.source.body.id"] = Attribute{Kind: "integer", Integer: a.ProductBodies[pid][j].SourceBodyID}
			s.Labels["step:"+asset] = label
		}
		for _, occ := range children[pid] {
			if e := expand(occ.Child, fmt.Sprintf("%s/occ-%d", id, occ.ID), id, occ.Name, occ.Transform, depth+1); e != nil {
				return e
			}
		}
		return nil
	}
	for _, root := range a.Assembly.Roots {
		if e := expand(root, fmt.Sprintf("product-%d", root), "", "", geom.Identity(), 1); e != nil {
			return nil, e
		}
	}
	return New(s, historyLimit)
}

// WriteWrappedMaterialAssemblySTEP exports imported product provenance. Shared
// product definitions, local asset identities and occurrence placements remain
// separate. All ten explicitly tagged wrapped material families have
// native extraction support. Structural edits that make reused products inconsistent,
// stale labels, moved body members and unsupported families are rejected.
func (s State) WriteWrappedMaterialAssemblySTEP(w io.Writer) error {
	if w == nil {
		return fmt.Errorf("document: nil mixed assembly writer")
	}
	if len(s.Features) > 0 {
		return fmt.Errorf("document: mixed assembly feature edits unsupported")
	}
	if e := s.Validate(); e != nil {
		return e
	}
	if len(s.Nodes) == 0 || len(s.Nodes) > 10000 {
		return fmt.Errorf("document: mixed assembly node budget")
	}
	get := func(l Label, k string) string {
		a := l.Attributes[k]
		if a.Kind == "string" {
			return a.Text
		}
		return ""
	}
	productNode := map[string]string{}
	members := map[string][]string{}
	occChildren := map[string][]string{}
	roots := []string{}
	for id, n := range s.Nodes {
		if label, ok := s.Labels["step.product:"+id]; ok {
			pid := get(label, "step.product.id")
			if label.Target != NodeLabel || label.TargetID != id || pid == "" || n.Asset != "" || !splineRigidPlacement(n.Transform) {
				return fmt.Errorf("document: stale mixed product node")
			}
			productNode[id] = pid
		} else if label, ok := s.Labels["step.member:"+id]; ok {
			if label.Target != NodeLabel || label.TargetID != id || n.Asset == "" || get(label, "step.body.asset") != n.Asset || n.Transform != geom.Identity() {
				return fmt.Errorf("document: stale or moved mixed body member")
			}
			members[n.Parent] = append(members[n.Parent], id)
		} else {
			return fmt.Errorf("document: missing mixed assembly provenance")
		}
	}
	for id := range productNode {
		n := s.Nodes[id]
		if n.Parent == "" {
			if n.Transform != geom.Identity() {
				return fmt.Errorf("document: moved assembly root")
			}
			roots = append(roots, id)
		} else {
			if productNode[n.Parent] == "" {
				return fmt.Errorf("document: product parent missing")
			}
			occChildren[n.Parent] = append(occChildren[n.Parent], id)
		}
	}
	for parent, list := range members {
		pid := productNode[parent]
		if pid == "" {
			return fmt.Errorf("document: body parent missing")
		}
		for _, id := range list {
			n := s.Nodes[id]
			label := s.Labels["step.member:"+id]
			asset := s.Labels["step:"+n.Asset]
			if get(label, "step.product.id") != pid || get(asset, "step.product.id") != pid || asset.Target != AssetLabel || asset.TargetID != n.Asset || n.Name != asset.Name || label.Attributes["step.source.body.id"].Kind != "integer" || label.Attributes["step.source.body.id"].Integer <= 0 || label.Attributes["step.source.body.id"] != asset.Attributes["step.source.body.id"] {
				return fmt.Errorf("document: stale product/body ownership")
			}
		}
	}
	ids := []string{}
	for id := range productNode {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	sort.Strings(roots)
	exportIDs := map[string]string{}
	exported := map[string]string{}
	for _, id := range ids {
		pid := productNode[id]
		key := get(s.Labels["step.product:"+id], "step.product.identifier")
		if key == "" {
			key = "product-" + pid
		}
		if old, ok := exported[key]; ok && old != pid {
			return fmt.Errorf("document: duplicate product identifier")
		}
		exportIDs[pid] = key
		exported[key] = pid
	}
	model := step.WrappedMaterialAssemblyModel{LengthUnit: s.LengthUnit}
	seen := map[string]string{}
	bodyCache := map[string]step.WrappedMaterialBody{}
	for _, id := range ids {
		pid := productNode[id]
		label := s.Labels["step.product:"+id]
		sort.Strings(members[id])
		sort.Strings(occChildren[id])
		// Repeated definitions must carry identical local assets and child occurrence
		// descriptions, irrespective of expanded native node IDs.
		signature := get(label, "step.product.identifier") + "\x00" + get(label, "step.product.name")
		for _, mid := range members[id] {
			n := s.Nodes[mid]
			signature += fmt.Sprintf("|body:%q:%q", n.Asset, n.Name)
		}
		for _, cid := range occChildren[id] {
			n := s.Nodes[cid]
			signature += fmt.Sprintf("|child:%q:%q:%v", productNode[cid], n.Name, n.Transform)
		}
		if old, ok := seen[pid]; ok {
			if old != signature {
				return fmt.Errorf("document: inconsistent reused product")
			}
			continue
		}
		seen[pid] = signature
		part := step.WrappedMaterialAssemblyPart{ID: exportIDs[pid], Name: get(label, "step.product.name")}
		for _, mid := range members[id] {
			n := s.Nodes[mid]
			b, ok := bodyCache[n.Asset]
			if !ok {
				only := State{LengthUnit: s.LengthUnit, Trimmed: s.Trimmed, Nodes: map[string]Node{"local": {Name: s.Labels["step:"+n.Asset].Attributes["step.body.name"].Text, Asset: n.Asset, Transform: geom.Identity()}}, Labels: map[string]Label{"step:" + n.Asset: s.Labels["step:"+n.Asset]}}
				var raw bytes.Buffer
				if e := only.WriteWrappedMaterialsSTEP(&raw); e != nil {
					return e
				}
				m, e := step.ReadWrappedMaterials(&raw, step.Limits{}, step.WrappedSurfaceOptions{})
				if e != nil {
					return e
				}
				if len(m.Bodies) != 1 {
					return fmt.Errorf("document: local body count")
				}
				b = m.Bodies[0]
				bodyCache[n.Asset] = b
			}
			part.Bodies = append(part.Bodies, b)
		}
		model.Products = append(model.Products, part)
		for j, cid := range occChildren[id] {
			n := s.Nodes[cid]
			model.Occurrences = append(model.Occurrences, step.AssemblyInstance{ID: fmt.Sprintf("%s-child-%d", pid, j), Name: n.Name, Parent: exportIDs[pid], Child: exportIDs[productNode[cid]], Transform: n.Transform})
		}
	}
	if len(roots) != 1 {
		return fmt.Errorf("document: mixed assembly requires one explicit root")
	}
	return step.WriteWrappedMaterialAssembly(w, model)
}
