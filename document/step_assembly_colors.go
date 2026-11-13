package document

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/exchange/step"
	"reflect"
	"strings"
)

// FromColoredSTEPAssembly stores local product colors as asset/face labels while
// retaining shared assets and expanded occurrence hierarchy. Geometry in the
// supplied color model must exactly match its source assembly's local topology.
func FromColoredSTEPAssembly(a step.ColoredAssembly, o STEPAssemblyOptions, historyLimit int) (*Document, error) {
	d, e := FromSTEPAssembly(a.Assembly, o, historyLimit)
	if e != nil {
		return nil, e
	}
	s, _ := d.Snapshot()
	if len(a.ProductBodies) != len(a.Assembly.Products) {
		return nil, fmt.Errorf("document: incomplete colored assembly products")
	}
	for _, p := range a.Assembly.Products {
		bodies, ok := a.ProductBodies[p.DefinitionID]
		if !ok {
			return nil, fmt.Errorf("document: missing colored product")
		}
		expected, e := a.Assembly.PlanarBodies(p.DefinitionID, o.Geometry)
		if e != nil || len(expected) != len(bodies) {
			return nil, fmt.Errorf("document: mismatched colored product geometry")
		}
		for i, b := range bodies {
			original := expected[i].Solid
			if !reflect.DeepEqual(original.Vertices(), b.Body.Solid.Vertices()) || !reflect.DeepEqual(original.Edges(), b.Body.Solid.Edges()) || !reflect.DeepEqual(original.Faces(), b.Body.Solid.Faces()) {
				return nil, fmt.Errorf("document: colored body does not match source topology")
			}
			key := fmt.Sprintf("product-%d-body-%d", p.DefinitionID, i+1)
			if b.SurfaceColor != nil {
				if !validColor(*b.SurfaceColor) {
					return nil, fmt.Errorf("document: invalid colored assembly default")
				}
				s.Labels["step-style:"+key] = Label{Name: b.Body.Name, Target: AssetLabel, TargetID: key, Attributes: colorAttributes(*b.SurfaceColor)}
			}
			for index, c := range b.FaceColors {
				if !validColor(c) || index < 0 || index >= len(original.Faces()) {
					return nil, fmt.Errorf("document: invalid colored assembly face")
				}
				face := index
				s.Labels[fmt.Sprintf("step-style:%s:face-%d", key, index)] = Label{Name: b.Body.Name, Target: FaceLabel, TargetID: key, Face: &face, Attributes: colorAttributes(c)}
			}
		}
	}
	return New(s, historyLimit)
}

// WriteColoredSTEPAssembly evaluates features and exports product-local asset
// and face RGB labels while retaining shared geometry and rigid occurrences.
// Node-local style overrides, partial/conflicting colors and nonplanar assets fail.
func (s State) WriteColoredSTEPAssembly(w io.Writer, rootName string) error {
	if w == nil {
		return fmt.Errorf("document: nil colored assembly writer")
	}
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
	styles, overrides, e := s.stepColorLabels(true)
	if e != nil {
		return e
	}
	var raw bytes.Buffer
	if e := s.writeSTEPAssemblyEvaluated(&raw, rootName); e != nil {
		return e
	}
	a, e := step.ReadAssembly(&raw, step.Limits{})
	if e != nil {
		return e
	}
	model := step.ColoredAssemblyModel{LengthUnit: s.LengthUnit}
	identifiers := map[int64]string{}
	for _, p := range a.Products {
		identifiers[p.DefinitionID] = p.Identifier
		part := step.ColoredAssemblyPart{ID: p.Identifier, Name: p.Name}
		bodies, e := a.PlanarBodies(p.DefinitionID, step.Limits{})
		if e != nil {
			return e
		}
		asset := ""
		if strings.HasPrefix(p.Identifier, "asset:") {
			asset = strings.TrimPrefix(p.Identifier, "asset:")
		} else if strings.HasPrefix(p.Identifier, "node:") {
			node, ok := s.Nodes[strings.TrimPrefix(p.Identifier, "node:")]
			if !ok {
				return fmt.Errorf("document: missing staged assembly node")
			}
			asset = node.Asset
		}
		if len(bodies) > 0 && (asset == "" || len(bodies) != 1) {
			return fmt.Errorf("document: inconsistent staged assembly asset")
		}
		for _, b := range bodies {
			colored := step.FaceColoredBody{Body: b, FaceColors: overrides[asset]}
			if c, ok := styles[asset]; ok {
				colored.SurfaceColor = &c
			}
			part.Bodies = append(part.Bodies, colored)
		}
		model.Products = append(model.Products, part)
	}
	for _, o := range a.Occurrences {
		model.Occurrences = append(model.Occurrences, step.AssemblyInstance{ID: o.Identifier, Name: o.Name, Parent: identifiers[o.Parent], Child: identifiers[o.Child], Transform: o.Transform})
	}
	return step.WriteColoredAssembly(w, model)
}
