package document

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"strings"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// ReadNestedMixedConicalAP242STEP imports the verified nested conical AP242
// family into an editable document. Repeated occurrences refer to two shared
// native material assets. Source labels retain exact original placements and
// entity identities; editing a node changes its current document placement,
// independently of those original source labels.
func ReadNestedMixedConicalAP242STEP(ctx context.Context, r io.Reader, limits STEPAssemblyOptions, geometry geom.SurfacePatchOptions, historyLimit int) (*Document, error) {
	if ctx == nil || r == nil || limits.MaxDepth < 0 || limits.MaxDepth > 256 || limits.MaxNodes < 0 || historyLimit < 0 {
		return nil, geom.ErrInvalid
	}
	if limits.MaxDepth != 0 && limits.MaxDepth < 3 || limits.MaxNodes != 0 && limits.MaxNodes < 7 {
		return nil, fmt.Errorf("document: mixed conical hierarchy exceeds the expansion limits: %w", geom.ErrConvergence)
	}
	m, err := step.ReadNestedMixedConicalAP242Assembly(ctx, r, limits.Geometry, geometry)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return mixedConicalDocument(ctx, m, limits.MaxNodes, historyLimit)
}

func mixedConicalDocument(ctx context.Context, m step.NestedMixedConicalAP242Assembly, maxNodes, historyLimit int) (*Document, error) {
	root := m.Assembly.Roots[0]
	var parents []step.ExactNAUOOccurrence
	relationships := make(map[int64]step.ExactNAUOOccurrence, len(m.Occurrences))
	for _, o := range m.Occurrences {
		relationships[o.NAUOID] = o
		if o.ParentID == root {
			parents = append(parents, o)
		}
	}
	if maxNodes != 0 && 1+len(parents)+len(m.Instances) > maxNodes {
		return nil, fmt.Errorf("document: mixed conical hierarchy exceeds the node limit: %w", geom.ErrConvergence)
	}
	s := State{LengthUnit: m.Assembly.LengthUnit, Nodes: map[string]Node{}, Trimmed: map[string]trimmed.Solid{}, Labels: map[string]Label{}}
	products := make(map[int64]step.AssemblyProduct, len(m.Assembly.Products))
	for _, p := range m.Assembly.Products {
		products[p.DefinitionID] = p
	}
	productKey := func(id int64) string { return fmt.Sprintf("step-product-%d", id) }
	occurrenceKey := func(parent string, id int64) string { return fmt.Sprintf("%s/occ-%d", parent, id) }
	intAttr := func(x int64) Attribute { return Attribute{Kind: "integer", Integer: x} }
	strAttr := func(x string) Attribute { return Attribute{Kind: "string", Text: x} }
	for id, p := range m.Parts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := productKey(id)
		var body, shell int64
		var family string
		var faces []int64
		switch {
		case p.Rounded != nil:
			v := p.Rounded
			s.Trimmed[key], body, shell, faces, family = v.Solid, v.SourceBodyID, v.SourceShellID, v.NativeFaceSourceIDs, "conical-rim-fillet-bore"
		case p.Chamfered != nil:
			v := p.Chamfered
			s.Trimmed[key], body, shell, faces, family = v.Solid, v.SourceBodyID, v.SourceShellID, v.NativeFaceSourceIDs, "eccentric-annular-chamfer"
		default:
			return nil, geom.ErrUnrepresentable
		}
		labelID := "source:" + key
		s.Labels[labelID] = Label{Target: AssetLabel, TargetID: key, Name: p.Product.Name, Attributes: map[string]Attribute{
			"step.source.definition": intAttr(id), "step.source.body": intAttr(body), "step.source.shell": intAttr(shell),
			"step.product.identifier": strAttr(p.Product.Identifier), "step.material.family": strAttr(family),
		}}
		for i, source := range faces {
			face := i
			s.Labels[fmt.Sprintf("source-face:%s:%d", key, i)] = Label{Target: FaceLabel, TargetID: key, Parent: labelID, Face: &face,
				Name: fmt.Sprintf("STEP face #%d", source), Attributes: map[string]Attribute{"step.source.face": intAttr(source)}}
		}
	}
	rootKey := productKey(root)
	addNode := func(product int64, key, parent, asset, name string, local geom.Affine, attrs map[string]Attribute) {
		p := products[product]
		if name == "" {
			name = p.Name
		}
		attrs["step.source.definition"] = intAttr(product)
		attrs["step.product.identifier"] = strAttr(p.Identifier)
		attrs["step.source.path"] = strAttr(key)
		s.Nodes[key] = Node{Name: name, Parent: parent, Asset: asset, Transform: local}
		s.Labels["source-node:"+key] = Label{Name: name, Target: NodeLabel, TargetID: key, Attributes: attrs}
	}
	addOccurrence := func(o step.ExactNAUOOccurrence, key, parent, asset string, world [3][4]*big.Rat) {
		local := o.Matrix()
		var display geom.Affine
		for i := range local {
			for j := range local[i] {
				display[i][j], _ = local[i][j].Float64()
			}
		}
		addNode(o.ChildID, key, parent, asset, o.Name, display, map[string]Attribute{
			"step.source.nauo": intAttr(o.NAUOID), "step.source.context-dependent-representation": intAttr(o.CDSRID),
			"step.source.relationship": intAttr(o.RelationshipID), "step.source.transformation": intAttr(o.TransformationID),
			"step.source.local-pose": strAttr(mixedConicalPoseText(local)), "step.source.world-pose": strAttr(mixedConicalPoseText(world)),
		})
	}
	addNode(root, rootKey, "", "", "", geom.Identity(), map[string]Attribute{})
	for _, parent := range parents {
		addOccurrence(parent, occurrenceKey(rootKey, parent.NAUOID), rootKey, "", parent.Matrix())
	}
	for _, instance := range m.Instances {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		parent := occurrenceKey(rootKey, instance.Path[0])
		child := relationships[instance.Path[1]]
		addOccurrence(child, occurrenceKey(parent, child.NAUOID), parent, productKey(instance.ProductID), instance.Matrix())
	}
	return New(s, historyLimit)
}

// Source pose attributes store three semicolon-separated matrix rows with four
// reduced, comma-separated rational values per row.
func mixedConicalPoseText(m [3][4]*big.Rat) string {
	var b strings.Builder
	for i := range m {
		if i != 0 {
			b.WriteByte(';')
		}
		for j, q := range m[i] {
			if j != 0 {
				b.WriteByte(',')
			}
			b.WriteString(q.RatString())
		}
	}
	return b.String()
}
