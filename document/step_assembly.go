package document

import (
	"context"
	"fmt"
	"io"
	"sort"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// STEPAssemblyOptions bounds expansion of the product DAG into the document's
// tree of occurrences. Shared geometry is stored once per product body.
// Zero values select at most 10,000 nodes and 64 levels.
type STEPAssemblyOptions struct {
	Geometry           step.Limits
	MaxNodes, MaxDepth int
}

// FromSTEPAssembly materializes occurrences while preserving hierarchy,
// names, local placement and shared exact assets. Product geometry must be
// all planar or all supported analytic within each product. Unsupported or
// mixed geometry fails import.
func FromSTEPAssembly(a step.Assembly, o STEPAssemblyOptions, historyLimit int) (*Document, error) {
	return fromSTEPAssembly(a, o, historyLimit, false)
}
func fromSTEPAssembly(a step.Assembly, o STEPAssemblyOptions, historyLimit int, splineShells bool) (*Document, error) {
	if o.MaxNodes == 0 {
		o.MaxNodes = 10000
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 64
	}
	if o.MaxNodes < 1 || o.MaxDepth < 1 || o.MaxDepth > 256 || len(a.Products) == 0 ||
		len(a.Products) > o.MaxNodes || len(a.Occurrences) > o.MaxNodes {
		return nil, fmt.Errorf("document: invalid STEP assembly or expansion limits")
	}
	s := State{LengthUnit: a.LengthUnit, Planar: map[string]brep.Solid{}, Parametric: map[string]parametric.Solid{}, Trimmed: map[string]trimmed.Solid{}, Nodes: map[string]Node{}, Labels: map[string]Label{}}
	type body struct{ name, id string }
	type product struct {
		name, identifier, description string
		body                          []body
	}
	products := map[int64]product{}
	children := map[int64][]step.AssemblyOccurrence{}
	incoming := map[int64]int{}
	for _, p := range a.Products {
		if _, duplicate := products[p.DefinitionID]; duplicate || p.DefinitionID <= 0 {
			return nil, fmt.Errorf("document: invalid STEP product definition")
		}
		name := p.Name
		if name == "" {
			name = p.Identifier
		}
		products[p.DefinitionID] = product{name: name, identifier: p.Identifier, description: p.Description}
	}
	occurrences := map[int64]bool{}
	for _, x := range a.Occurrences {
		_, parent := products[x.Parent]
		_, child := products[x.Child]
		if !parent || !child || x.ID <= 0 || occurrences[x.ID] {
			return nil, fmt.Errorf("document: invalid STEP occurrence")
		}
		occurrences[x.ID] = true
		children[x.Parent] = append(children[x.Parent], x)
		incoming[x.Child]++
	}
	roots := append([]int64(nil), a.Roots...)
	sort.Slice(roots, func(i, j int) bool { return roots[i] < roots[j] })
	rootSet := map[int64]bool{}
	for _, id := range roots {
		if _, ok := products[id]; !ok || rootSet[id] || incoming[id] != 0 {
			return nil, fmt.Errorf("document: invalid STEP assembly root")
		}
		rootSet[id] = true
	}
	for id := range products {
		if incoming[id] == 0 && !rootSet[id] {
			return nil, fmt.Errorf("document: missing STEP assembly root")
		}
		list := children[id]
		sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	}
	// Validate the product graph before decoding geometry or expanding it.
	queue := append([]int64(nil), roots...)
	depth := map[int64]int{}
	for _, id := range roots {
		depth[id] = 1
	}
	for k := 0; k < len(queue); k++ {
		id := queue[k]
		for _, x := range children[id] {
			depth[x.Child] = max(depth[x.Child], depth[id]+1)
			if depth[x.Child] > o.MaxDepth {
				return nil, fmt.Errorf("document: STEP assembly depth limit exceeded")
			}
			incoming[x.Child]--
			if incoming[x.Child] == 0 {
				queue = append(queue, x.Child)
			}
		}
	}
	if len(queue) != len(products) {
		return nil, fmt.Errorf("document: cyclic STEP assembly")
	}
	maxAssets := o.MaxNodes
	for _, id := range queue {
		p := products[id]
		if splineShells {
			shells, e := a.SplineShellBodies(id, o.Geometry)
			if e != nil {
				return nil, e
			}
			if len(shells) > maxAssets {
				return nil, fmt.Errorf("document: STEP assembly asset limit exceeded")
			}
			for j, b := range shells {
				key := fmt.Sprintf("product-%d-body-%d", id, j+1)
				s.Trimmed[key] = b.Solid
				p.body = append(p.body, body{b.Name, key})
			}
		} else {
			planar, e := a.PlanarBodies(id, o.Geometry)
			if e == nil {
				if len(planar) > maxAssets {
					return nil, fmt.Errorf("document: STEP assembly asset limit exceeded")
				}
				for j, b := range planar {
					key := fmt.Sprintf("product-%d-body-%d", id, j+1)
					s.Planar[key] = b.Solid
					p.body = append(p.body, body{b.Name, key})
				}
			} else {
				analytic, analyticErr := a.AnalyticBodies(id, o.Geometry)
				if analyticErr != nil {
					return nil, fmt.Errorf("document: unsupported STEP product %d: planar: %w; analytic: %v", id, e, analyticErr)
				}
				if len(analytic) > maxAssets {
					return nil, fmt.Errorf("document: STEP assembly asset limit exceeded")
				}
				for j, b := range analytic {
					key := fmt.Sprintf("product-%d-body-%d", id, j+1)
					s.Parametric[key] = b.Solid
					p.body = append(p.body, body{b.Name, key})
				}
			}
		}
		maxAssets -= len(p.body)
		for _, b := range p.body {
			s.Labels["step:"+b.id] = Label{Name: p.name, Target: AssetLabel, TargetID: b.id, Attributes: map[string]Attribute{
				"step.product.id":          {Kind: "string", Text: p.identifier},
				"step.product.description": {Kind: "string", Text: p.description},
			}}
		}
		products[id] = p
	}
	type occurrence struct {
		product           int64
		key, parent, name string
		transform         geom.Affine
	}
	stack := make([]occurrence, 0, len(roots))
	for i := len(roots) - 1; i >= 0; i-- {
		id := roots[i]
		stack = append(stack, occurrence{product: id, key: fmt.Sprintf("root-%d", id), transform: geom.Identity()})
	}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(s.Nodes) >= o.MaxNodes {
			return nil, fmt.Errorf("document: STEP occurrence expansion limit exceeded")
		}
		p := products[x.product]
		name := x.name
		if name == "" {
			name = p.name
		}
		n := Node{Name: name, Parent: x.parent, Transform: x.transform}
		if len(p.body) == 1 {
			n.Asset = p.body[0].id
		}
		s.Nodes[x.key] = n
		if len(p.body) > 1 {
			if len(p.body) > o.MaxNodes-len(s.Nodes)-len(stack) {
				return nil, fmt.Errorf("document: STEP occurrence expansion limit exceeded")
			}
			for j, b := range p.body {
				key := fmt.Sprintf("%s/body-%d", x.key, j+1)
				s.Nodes[key] = Node{Name: b.name, Parent: x.key, Asset: b.id, Transform: geom.Identity()}
			}
		}
		list := children[x.product]
		if len(list) > o.MaxNodes-len(s.Nodes)-len(stack) {
			return nil, fmt.Errorf("document: STEP occurrence expansion limit exceeded")
		}
		for i := len(list) - 1; i >= 0; i-- {
			y := list[i]
			stack = append(stack, occurrence{y.Child, fmt.Sprintf("%s/occ-%d", x.key, y.ID), x.key, y.Name, y.Transform})
		}
	}
	return New(s, historyLimit)
}

// WriteSTEPAssembly exports a document tree as named AP214 occurrences with
// rigid placements and shared local planar assets. The document is placed
// under an explicit root product so multiple top-level nodes and their local
// transforms are retained. A node with children and geometry is exported as
// a product containing both its local body and child occurrences.
func (s State) WriteSTEPAssembly(w io.Writer, rootName string) error {
	if len(s.Features) > 0 {
		var e error
		s, e = s.EvaluateFeatures(context.Background())
		if e != nil {
			return e
		}
	}
	return s.writeSTEPAssemblyEvaluated(w, rootName)
}
func (s State) writeSTEPAssemblyEvaluated(w io.Writer, rootName string) error {
	if e := s.Validate(); e != nil {
		return e
	}
	if w == nil || len(s.Nodes) == 0 || len(s.Nodes) > 10000 {
		return fmt.Errorf("document: invalid STEP assembly export")
	}
	ids := make([]string, 0, len(s.Nodes))
	hasChildren := map[string]bool{}
	for id, n := range s.Nodes {
		ids = append(ids, id)
		hasChildren[n.Parent] = true
	}
	sort.Strings(ids)
	if rootName == "" {
		rootName = "PlainSolid assembly"
	}
	const rootID = "assembly"
	m := step.AssemblyModel{LengthUnit: s.LengthUnit, Products: []step.AssemblyPart{{ID: rootID, Name: rootName}}}
	shared := map[string]bool{}
	for _, id := range ids {
		n := s.Nodes[id]
		if n.Asset == "" && !hasChildren[id] {
			return fmt.Errorf("document: empty STEP assembly node %q", id)
		}
		var bodies []step.Body
		if n.Asset != "" {
			b, ok := s.Planar[n.Asset]
			if !ok {
				return fmt.Errorf("document: STEP assembly export requires a planar asset for node %q", id)
			}
			bodies = []step.Body{{Name: n.Asset, Solid: b}}
		}
		product := "node:" + id
		name := n.Name
		if name == "" {
			name = id
		}
		if !hasChildren[id] {
			product = "asset:" + n.Asset
			if !shared[product] {
				m.Products = append(m.Products, step.AssemblyPart{ID: product, Name: n.Asset, Bodies: bodies})
				shared[product] = true
			}
		} else {
			m.Products = append(m.Products, step.AssemblyPart{ID: product, Name: name, Bodies: bodies})
		}
		parent := rootID
		if n.Parent != "" {
			parent = "node:" + n.Parent
		}
		m.Occurrences = append(m.Occurrences, step.AssemblyInstance{ID: id, Name: name, Parent: parent, Child: product, Transform: n.Transform})
	}
	return step.WriteAssembly(w, m)
}
