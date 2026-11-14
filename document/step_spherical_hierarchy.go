package document

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// WriteSphericalHierarchySTEP exports edited occurrence positions and names
// using the original independently authenticated local STEP material. It
// preserves shared product relationships. Every expanded use of one original
// relationship must agree on edits; changed assets and topology are rejected.
// Both edited and unchanged transfers preserve the original product, geometry
// and relationship entity numbers.
func (s State) WriteSphericalHierarchySTEP(ctx context.Context, out io.Writer, original io.Reader, limits STEPAssemblyOptions, tolerance float64) error {
	if ctx == nil || out == nil || original == nil || len(s.Features) != 0 {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m, err := step.ReadIndependentSphericalHierarchy(ctx, original, limits.Geometry, tolerance)
	if err != nil {
		return err
	}
	d, err := materializeSphericalHierarchy(ctx, m, limits, 0)
	if err != nil {
		return err
	}
	baseline, _ := d.Snapshot()
	if s.LengthUnit != baseline.LengthUnit || len(s.Nodes) != len(baseline.Nodes) || len(s.Planar) != len(baseline.Planar) || len(s.Trimmed) != len(baseline.Trimmed) || len(s.Parametric) != 0 || len(s.Assets) != 0 {
		return geom.ErrUnrepresentable
	}
	for id, before := range baseline.Planar {
		after, ok := s.Planar[id]
		if !ok {
			return geom.ErrUnrepresentable
		}
		var a, b bytes.Buffer
		if err := exchange.WriteBRepJSON(&a, before); err != nil {
			return err
		}
		if err := exchange.WriteBRepJSON(&b, after); err != nil {
			return err
		}
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			return geom.ErrUnrepresentable
		}
	}
	for id, before := range baseline.Trimmed {
		after, ok := s.Trimmed[id]
		if !ok {
			return geom.ErrUnrepresentable
		}
		var a, b bytes.Buffer
		if err := trimmed.WriteJSON(&a, before); err != nil {
			return err
		}
		if err := trimmed.WriteJSON(&b, after); err != nil {
			return err
		}
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			return geom.ErrUnrepresentable
		}
	}
	type relationshipEdit struct {
		local geom.Affine
		name  string
	}
	edits := map[int64]relationshipEdit{}
	renamed := map[int64]bool{}
	for key, before := range baseline.Nodes {
		if err := ctx.Err(); err != nil {
			return err
		}
		after, ok := s.Nodes[key]
		if !ok || after.Parent != before.Parent || after.Asset != before.Asset {
			return geom.ErrUnrepresentable
		}
		if before.Parent == "" {
			if after.Transform != geom.Identity() || after.Name != before.Name {
				return geom.ErrUnrepresentable
			}
			continue
		}
		label := baseline.Labels["source-node:"+key]
		id := label.Attributes["step.source.nauo"].Integer
		if id <= 0 {
			return geom.ErrUnrepresentable
		}
		v := relationshipEdit{local: after.Transform, name: after.Name}
		if prev, seen := edits[id]; seen && prev != v {
			return fmt.Errorf("document: inconsistent changes to shared STEP occurrence %d: %w", id, geom.ErrUnrepresentable)
		}
		edits[id] = v
		if after.Name != before.Name {
			renamed[id] = true
		}
	}
	var changes []step.SphericalHierarchyOccurrenceEdit
	for _, rel := range m.Assembly.Occurrences {
		edit, ok := edits[rel.ID]
		if !ok {
			return geom.ErrUnrepresentable
		}
		if !renamed[rel.ID] {
			edit.name = rel.Name
		} else if edit.name == "" {
			// An empty source label is displayed as the product name on import.
			return geom.ErrUnrepresentable
		}
		if edit.local != rel.Transform || edit.name != rel.Name {
			changes = append(changes, step.SphericalHierarchyOccurrenceEdit{NAUOID: rel.ID, Name: edit.name, Transform: edit.local})
		}
	}
	return step.WriteIndependentSphericalHierarchyEdits(ctx, out, m.Assembly.Source, changes, limits.Geometry, tolerance)
}

// ReadSphericalHierarchySTEP materializes an independently proved signed-axis
// STEP hierarchy as editable document occurrences with shared native assets.
// Stable labels identify original definitions, local bodies, and NAUO paths.
// Source-bound round trips remain available through the separate STEP codec.
func ReadSphericalHierarchySTEP(ctx context.Context, r io.Reader, limits STEPAssemblyOptions, tolerance float64, historyLimit int) (*Document, error) {
	m, err := step.ReadIndependentSphericalHierarchy(ctx, r, limits.Geometry, tolerance)
	if err != nil {
		return nil, err
	}
	return materializeSphericalHierarchy(ctx, m, limits, historyLimit)
}

// ReadSphericalHierarchyNative reads the source-bound STEP hierarchy envelope
// and verifies it before materializing the editable occurrences.
func ReadSphericalHierarchyNative(ctx context.Context, r io.Reader, native step.IndependentMaterialJSONLimits, limits STEPAssemblyOptions, historyLimit int) (*Document, error) {
	m, err := step.ReadIndependentSphericalHierarchyJSON(ctx, r, native)
	if err != nil {
		return nil, err
	}
	return materializeSphericalHierarchy(ctx, m, limits, historyLimit)
}

func materializeSphericalHierarchy(ctx context.Context, m step.IndependentSphericalHierarchy, limits STEPAssemblyOptions, historyLimit int) (*Document, error) {
	if limits.MaxNodes == 0 {
		limits.MaxNodes = 10000
	}
	if limits.MaxDepth == 0 {
		limits.MaxDepth = 64
	}
	if limits.MaxNodes < 1 || limits.MaxDepth < 1 || limits.MaxDepth > 256 {
		return nil, fmt.Errorf("document: invalid hierarchy expansion limits")
	}
	s := State{LengthUnit: m.Assembly.LengthUnit, Planar: map[string]brep.Solid{}, Trimmed: map[string]trimmed.Solid{}, Nodes: map[string]Node{}, Labels: map[string]Label{}}
	intAttr := func(x int64) Attribute { return Attribute{Kind: "integer", Integer: x} }
	strAttr := func(x string) Attribute { return Attribute{Kind: "string", Text: x} }
	productKey := func(id int64) string { return fmt.Sprintf("step-product-%d", id) }
	products := make(map[int64]step.AssemblyProduct, len(m.Assembly.Products))
	for _, p := range m.Assembly.Products {
		products[p.DefinitionID] = p
	}
	for id, body := range m.Products {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := productKey(id)
		kind := "planar"
		switch {
		case body.Planar != nil:
			s.Planar[key] = body.Planar.Solid
		case body.Cylinder != nil:
			kind = "cylinder"
		case body.Torus != nil:
			kind = "torus"
		case body.Sphere != nil:
			kind = "sphere"
		default:
			return nil, geom.ErrUnrepresentable
		}
		if body.Planar == nil {
			s.Trimmed[key] = body.Solid
		}
		p := products[id]
		s.Labels["source:"+key] = Label{Name: body.Name, Target: AssetLabel, TargetID: key, Attributes: map[string]Attribute{
			"step.source.definition": intAttr(id), "step.source.body": intAttr(body.SourceBodyID), "step.source.representation": intAttr(body.SourceRepresentationID),
			"step.product.identifier": strAttr(p.Identifier), "step.product.name": strAttr(p.Name), "step.material.family": strAttr(kind),
		}}
	}
	children := map[int64][]step.AssemblyOccurrence{}
	for _, x := range m.Assembly.Occurrences {
		children[x.Parent] = append(children[x.Parent], x)
	}
	for _, list := range children {
		sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	}
	leafChecks := make(map[string]step.IndependentAnalyticDAGInstance, len(m.Instances))
	if len(m.Assembly.Roots) != 1 {
		return nil, geom.ErrUnrepresentable
	}
	root := m.Assembly.Roots[0]
	rootKey := productKey(root)
	occKey := func(parent string, id int64) string { return parent + "/occ-" + strconv.FormatInt(id, 10) }
	for _, leaf := range m.Instances {
		key := rootKey
		for _, id := range leaf.Path {
			key = occKey(key, id)
		}
		leafChecks[key] = leaf
	}
	checked := 0
	var walk func(int64, string, string, string, geom.Affine, int64, int) error
	walk = func(product int64, key, parent, name string, local geom.Affine, incoming int64, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > limits.MaxDepth || len(s.Nodes) >= limits.MaxNodes {
			return fmt.Errorf("document: hierarchy expansion limit exceeded: %w", geom.ErrConvergence)
		}
		p := products[product]
		if name == "" {
			name = p.Name
		}
		n := Node{Name: name, Parent: parent, Transform: local}
		attrs := map[string]Attribute{"step.source.definition": intAttr(product), "step.product.identifier": strAttr(p.Identifier), "step.source.path": strAttr(key)}
		if incoming != 0 {
			attrs["step.source.nauo"] = intAttr(incoming)
		}
		if body, ok := m.Products[product]; ok {
			n.Asset = productKey(product)
			attrs["step.source.body"] = intAttr(body.SourceBodyID)
		}
		s.Nodes[key] = n
		s.Labels["source-node:"+key] = Label{Name: name, Target: NodeLabel, TargetID: key, Attributes: attrs}
		if n.Asset != "" {
			leaf, ok := leafChecks[key]
			actual, err := s.WorldTransform(key)
			if !ok || leaf.ProductID != product || err != nil || actual != leaf.Transform {
				return geom.ErrUnrepresentable
			}
			checked++
		}
		for _, occ := range children[product] {
			if err := walk(occ.Child, occKey(key, occ.ID), key, occ.Name, occ.Transform, occ.ID, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root, rootKey, "", "", geom.Identity(), 0, 1); err != nil {
		return nil, err
	}
	if checked != len(leafChecks) {
		return nil, geom.ErrUnrepresentable
	}
	return New(s, historyLimit)
}
