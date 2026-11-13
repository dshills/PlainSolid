package document

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"sort"
	"strconv"
)

// ReadSplineLensAssemblySTEP imports the authoritative source graph. Local
// bodies become shared native assets; explicit product and body provenance is
// recorded in labels. No caller-supplied cached source IDs are trusted.
func ReadSplineLensAssemblySTEP(r io.Reader, l step.Limits, o step.CurvedOvalOptions, limits STEPAssemblyOptions, historyLimit int) (*Document, error) {
	return readSplineLensAssemblySTEP(r, l, o, limits, historyLimit, false)
}
func readSplineLensAssemblySTEP(r io.Reader, l step.Limits, o step.CurvedOvalOptions, limits STEPAssemblyOptions, historyLimit int, partitioned bool, refined ...bool) (*Document, error) {
	readAssembly := step.ReadSplineLensAssembly
	if partitioned {
		readAssembly = step.ReadPartitionedSplineLensAssembly
	}
	if len(refined) > 1 && refined[1] {
		readAssembly = step.ReadRationalSplineLensAssembly
	} else if len(refined) > 0 && refined[0] {
		readAssembly = step.ReadRefinedSplineLensAssembly
	}
	a, e := readAssembly(r, l, o)
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
	assets := map[int64][]string{}
	products := map[int64]step.AssemblyProduct{}
	s := State{LengthUnit: a.Assembly.LengthUnit, Trimmed: map[string]trimmed.Solid{}, Labels: map[string]Label{}}
	for _, p := range a.Assembly.Products {
		products[p.DefinitionID] = p
		for _, body := range a.ProductBodies[p.DefinitionID] {
			var d *Document
			var e error
			if len(refined) > 1 && refined[1] {
				d, e = fromVerifiedRationalLens(step.RationalSplineLens{PartitionedSplineLens: step.PartitionedSplineLens{Body: body, Pieces: a.ProductPieces[p.DefinitionID]}, Curves: a.ProductRationalCurves[p.DefinitionID], Style: a.ProductStyles[p.DefinitionID]}, historyLimit)
			} else if len(refined) > 0 && refined[0] {
				d, e = fromVerifiedRefinedLens(step.RefinedSplineLens{PartitionedSplineLens: step.PartitionedSplineLens{Body: body, Pieces: a.ProductPieces[p.DefinitionID]}, Curves: a.ProductCurves[p.DefinitionID]}, historyLimit)
			} else if partitioned {
				d, e = fromVerifiedPartitionedLens(step.PartitionedSplineLens{Body: body, Pieces: a.ProductPieces[p.DefinitionID]}, historyLimit)
			} else {
				d, e = fromVerifiedSplineLens(body, historyLimit)
			}
			if e != nil {
				return nil, e
			}
			local, _ := d.Snapshot()
			id := fmt.Sprintf("body-%06d", len(s.Trimmed)+1)
			s.Trimmed[id] = local.Trimmed["body-000001"]
			label := local.Labels["step:body-000001"]
			label.TargetID = id
			s.Labels["step:"+id] = label
			assets[p.DefinitionID] = append(assets[p.DefinitionID], id)
		}
	}
	s.Nodes = map[string]Node{}
	children := map[int64][]step.AssemblyOccurrence{}
	for _, occ := range a.Assembly.Occurrences {
		children[occ.Parent] = append(children[occ.Parent], occ)
	}
	text := func(v string) Attribute { return Attribute{Kind: "string", Text: v} }
	var expand func(int64, string, string, string, geom.Affine, int) error
	expand = func(pid int64, id, parent, name string, tr geom.Affine, depth int) error {
		if depth > limits.MaxDepth || len(s.Nodes) >= limits.MaxNodes {
			return fmt.Errorf("document: lens assembly expansion budget")
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
				return fmt.Errorf("document: lens assembly expansion budget")
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

// WriteSplineLensAssemblySTEP exports imported product provenance. Shared
// product definitions, local asset identities and occurrence placements remain
// separate. The canonical elliptical lens has native extraction support. Structural edits that make reused products inconsistent,
// stale labels, moved body members and unsupported families are rejected.
func (s State) WriteSplineLensAssemblySTEP(w io.Writer) error {
	return s.writeSplineLensAssemblySTEP(w, false)
}
func (s State) writeSplineLensAssemblySTEP(w io.Writer, partitioned bool, refined ...bool) error {
	if w == nil {
		return fmt.Errorf("document: nil lens assembly writer")
	}
	if len(s.Features) > 0 {
		return fmt.Errorf("document: lens assembly feature edits unsupported")
	}
	if e := s.Validate(); e != nil {
		return e
	}
	if len(s.Assets) != 0 || len(s.Planar) != 0 || len(s.Parametric) != 0 {
		return fmt.Errorf("document: lens assembly contains unsupported assets")
	}
	if len(s.Nodes) == 0 || len(s.Nodes) > 10000 {
		return fmt.Errorf("document: lens assembly node budget")
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
				return fmt.Errorf("document: stale lens product node")
			}
			productNode[id] = pid
		} else if label, ok := s.Labels["step.member:"+id]; ok {
			if label.Target != NodeLabel || label.TargetID != id || n.Asset == "" || get(label, "step.body.asset") != n.Asset || n.Transform != geom.Identity() {
				return fmt.Errorf("document: stale or moved lens body member")
			}
			members[n.Parent] = append(members[n.Parent], id)
		} else {
			return fmt.Errorf("document: missing lens assembly provenance")
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
	model := step.SplineLensAssemblyModel{LengthUnit: s.LengthUnit}
	seen := map[string]string{}
	bodyCache := map[string]step.SplineLensBody{}
	curveCache := map[string][][3]*geom.BSpline{}
	rationalCache := map[string][][3]step.RationalLensCurve{}
	styleCache := map[string]step.LensStyle{}
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
		part := step.SplineLensAssemblyPart{ID: exportIDs[pid], Name: get(label, "step.product.name")}
		for _, mid := range members[id] {
			n := s.Nodes[mid]
			b, ok := bodyCache[n.Asset]
			if !ok {
				only := State{LengthUnit: s.LengthUnit, Trimmed: map[string]trimmed.Solid{n.Asset: s.Trimmed[n.Asset]}, Nodes: map[string]Node{"local": {Name: s.Labels["step:"+n.Asset].Attributes["step.body.name"].Text, Asset: n.Asset, Transform: geom.Identity()}}, Labels: map[string]Label{"step:" + n.Asset: s.Labels["step:"+n.Asset]}}
				var raw bytes.Buffer
				var exportErr error
				if len(refined) > 1 && refined[1] {
					exportErr = only.WriteRationalSplineLensSTEP(&raw)
				} else if len(refined) > 0 && refined[0] {
					exportErr = only.WriteRefinedSplineLensSTEP(&raw)
				} else {
					exportErr = only.writeSplineLensSTEP(&raw, partitioned)
				}
				if e := exportErr; e != nil {
					return e
				}
				var m step.SplineLensBody
				var e error
				if len(refined) > 1 && refined[1] {
					var part step.RationalSplineLens
					part, e = step.ReadRationalSplineLensBody(&raw, step.Limits{}, step.CurvedOvalOptions{})
					m = part.Body
					rationalCache[n.Asset] = part.Curves
					styleCache[n.Asset] = part.Style
				} else if len(refined) > 0 && refined[0] {
					var part step.RefinedSplineLens
					part, e = step.ReadRefinedSplineLensBody(&raw, step.Limits{}, step.CurvedOvalOptions{})
					m = part.Body
					curveCache[n.Asset] = part.Curves
				} else if partitioned {
					var part step.PartitionedSplineLens
					part, e = step.ReadPartitionedSplineLensBody(&raw, step.Limits{}, step.CurvedOvalOptions{})
					m = part.Body
				} else {
					m, e = step.ReadSplineLensBody(&raw, step.Limits{}, step.CurvedOvalOptions{})
				}
				if e != nil {
					return e
				}
				b = m
				bodyCache[n.Asset] = b
			}
			part.Bodies = append(part.Bodies, b)
			if len(refined) > 1 && refined[1] {
				part.RationalSeamCurves = rationalCache[n.Asset]
				part.Style = styleCache[n.Asset]
			} else if len(refined) > 0 && refined[0] {
				part.SeamCurves = curveCache[n.Asset]
			}
		}
		model.Products = append(model.Products, part)
		for j, cid := range occChildren[id] {
			n := s.Nodes[cid]
			model.Occurrences = append(model.Occurrences, step.AssemblyInstance{ID: fmt.Sprintf("%s-child-%d", pid, j), Name: n.Name, Parent: exportIDs[pid], Child: exportIDs[productNode[cid]], Transform: n.Transform})
		}
	}
	if len(bodyCache) != len(s.Trimmed) {
		return fmt.Errorf("document: unused lens assets")
	}
	if len(roots) != 1 {
		return fmt.Errorf("document: lens assembly requires one explicit root")
	}
	if len(refined) > 1 && refined[1] {
		return step.WriteRationalSplineLensAssembly(w, model)
	} else if len(refined) > 0 && refined[0] {
		return step.WriteRefinedSplineLensAssembly(w, model)
	}
	if partitioned {
		return step.WritePartitionedSplineLensAssembly(w, model)
	}
	return step.WriteSplineLensAssembly(w, model)
}
