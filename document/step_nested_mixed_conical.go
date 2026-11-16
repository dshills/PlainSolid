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
	if ctx == nil || r == nil {
		return nil, geom.ErrInvalid
	}
	if err := validateMixedConicalDocumentLimits(limits, historyLimit); err != nil {
		return nil, err
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

func validateMixedConicalDocumentLimits(limits STEPAssemblyOptions, historyLimit int) error {
	if limits.MaxDepth < 0 || limits.MaxDepth > 256 || limits.MaxNodes < 0 || historyLimit < 0 {
		return geom.ErrInvalid
	}
	if limits.MaxDepth != 0 && limits.MaxDepth < 3 || limits.MaxNodes != 0 && limits.MaxNodes < 7 {
		return fmt.Errorf("document: mixed conical hierarchy exceeds the expansion limits: %w", geom.ErrConvergence)
	}
	return nil
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
	b, err := newMixedConicalDocumentBuilder(ctx, m.Assembly, m.Parts)
	if err != nil {
		return nil, err
	}
	rootKey := mixedConicalProductKey(root)
	b.addRoot(root, rootKey)
	for _, parent := range parents {
		b.addOccurrence(parent, mixedConicalOccurrenceKey(rootKey, parent.NAUOID), rootKey, "", parent.Matrix())
	}
	for _, instance := range m.Instances {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		parent := mixedConicalOccurrenceKey(rootKey, instance.Path[0])
		child := relationships[instance.Path[1]]
		b.addOccurrence(child, mixedConicalOccurrenceKey(parent, child.NAUOID), parent, mixedConicalProductKey(instance.ProductID), instance.Matrix())
	}
	return New(b.state, historyLimit)
}

type mixedConicalDocumentBuilder struct {
	state    State
	products map[int64]step.AssemblyProduct
}

func mixedConicalProductKey(id int64) string { return fmt.Sprintf("step-product-%d", id) }
func mixedConicalOccurrenceKey(parent string, id int64) string {
	return fmt.Sprintf("%s/occ-%d", parent, id)
}
func conicalIntAttr(x int64) Attribute  { return Attribute{Kind: "integer", Integer: x} }
func conicalStrAttr(x string) Attribute { return Attribute{Kind: "string", Text: x} }

func newMixedConicalDocumentBuilder(ctx context.Context, a step.Assembly, parts map[int64]step.MixedConicalAP242Part) (*mixedConicalDocumentBuilder, error) {
	b := &mixedConicalDocumentBuilder{
		state:    State{LengthUnit: a.LengthUnit, Nodes: map[string]Node{}, Trimmed: map[string]trimmed.Solid{}, Labels: map[string]Label{}},
		products: make(map[int64]step.AssemblyProduct, len(a.Products)),
	}
	for _, p := range a.Products {
		b.products[p.DefinitionID] = p
	}
	for id, p := range parts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := mixedConicalProductKey(id)
		var body, shell int64
		var family string
		var faces []int64
		switch {
		case p.Rounded != nil:
			v := p.Rounded
			b.state.Trimmed[key], body, shell, faces, family = v.Solid, v.SourceBodyID, v.SourceShellID, v.NativeFaceSourceIDs, "conical-rim-fillet-bore"
		case p.Chamfered != nil:
			v := p.Chamfered
			b.state.Trimmed[key], body, shell, faces, family = v.Solid, v.SourceBodyID, v.SourceShellID, v.NativeFaceSourceIDs, "eccentric-annular-chamfer"
		default:
			return nil, geom.ErrUnrepresentable
		}
		labelID := "source:" + key
		b.state.Labels[labelID] = Label{Target: AssetLabel, TargetID: key, Name: p.Product.Name, Attributes: map[string]Attribute{
			"step.source.definition": conicalIntAttr(id), "step.source.body": conicalIntAttr(body), "step.source.shell": conicalIntAttr(shell),
			"step.product.identifier": conicalStrAttr(p.Product.Identifier), "step.material.family": conicalStrAttr(family),
		}}
		for i, source := range faces {
			face := i
			b.state.Labels[fmt.Sprintf("source-face:%s:%d", key, i)] = Label{Target: FaceLabel, TargetID: key, Parent: labelID, Face: &face,
				Name: fmt.Sprintf("STEP face #%d", source), Attributes: map[string]Attribute{"step.source.face": conicalIntAttr(source)}}
		}
	}
	return b, nil
}

func (b *mixedConicalDocumentBuilder) addRoot(product int64, key string) {
	b.addNode(product, key, "", "", "", geom.Identity(), map[string]Attribute{})
}

func (b *mixedConicalDocumentBuilder) addNode(product int64, key, parent, asset, name string, local geom.Affine, attrs map[string]Attribute) {
	p := b.products[product]
	if name == "" {
		name = p.Name
	}
	attrs["step.source.definition"] = conicalIntAttr(product)
	attrs["step.product.identifier"] = conicalStrAttr(p.Identifier)
	attrs["step.source.path"] = conicalStrAttr(key)
	b.state.Nodes[key] = Node{Name: name, Parent: parent, Asset: asset, Transform: local}
	b.state.Labels["source-node:"+key] = Label{Name: name, Target: NodeLabel, TargetID: key, Attributes: attrs}
}

func (b *mixedConicalDocumentBuilder) addOccurrence(o step.ExactNAUOOccurrence, key, parent, asset string, world [3][4]*big.Rat) {
	local := o.Matrix()
	var display geom.Affine
	for i := range local {
		for j := range local[i] {
			display[i][j], _ = local[i][j].Float64()
		}
	}
	b.addNode(o.ChildID, key, parent, asset, o.Name, display, map[string]Attribute{
		"step.source.nauo": conicalIntAttr(o.NAUOID), "step.source.context-dependent-representation": conicalIntAttr(o.CDSRID),
		"step.source.relationship": conicalIntAttr(o.RelationshipID), "step.source.transformation": conicalIntAttr(o.TransformationID),
		"step.source.local-pose": conicalStrAttr(mixedConicalPoseText(local)), "step.source.world-pose": conicalStrAttr(mixedConicalPoseText(world)),
	})
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
