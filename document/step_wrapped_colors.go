package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/exchange/step"
	"sort"
	"strconv"
	"strings"
)

const wrappedStyleAttribute = "step.wrapped.named-style"

type wrappedNamedStyle struct {
	SurfaceColor *step.RGB
	FaceColors   map[string]step.RGB
}

// ReadColoredWrappedMaterialAssemblySTEP preserves shared native assets and
// product-local named face styles in existing asset labels. Occurrence styles
// remain unsupported. Geometry and source ownership use the strict adapter.
func ReadColoredWrappedMaterialAssemblySTEP(r io.Reader, l step.Limits, o step.WrappedSurfaceOptions, limits STEPAssemblyOptions, historyLimit int) (*Document, error) {
	cap := l.MaxBytes
	if cap == 0 {
		cap = 64 << 20
	}
	if r == nil || cap < 1 || cap > 1<<30 {
		return nil, fmt.Errorf("document: invalid STEP input budget")
	}
	raw, e := io.ReadAll(io.LimitReader(r, cap+1))
	if e != nil {
		return nil, e
	}
	if int64(len(raw)) > cap {
		return nil, fmt.Errorf("document: STEP byte budget")
	}
	a, e := step.ReadColoredWrappedMaterialAssembly(bytes.NewReader(raw), l, o)
	if e != nil {
		return nil, e
	}
	d, e := ReadWrappedMaterialAssemblySTEP(bytes.NewReader(raw), l, o, limits, historyLimit)
	if e != nil {
		return nil, e
	}
	s, _ := d.Snapshot()
	index := 0
	for _, p := range a.Assembly.Products {
		for _, b := range a.ProductBodies[p.DefinitionID] {
			index++
			key := fmt.Sprintf("step:body-%06d", index)
			label, ok := s.Labels[key]
			if !ok {
				return nil, fmt.Errorf("document: missing style asset")
			}
			data, e := json.Marshal(wrappedNamedStyle{b.SurfaceColor, b.FaceColors})
			if e != nil {
				return nil, e
			}
			label.Attributes[wrappedStyleAttribute] = Attribute{Kind: "string", Text: string(data)}
			s.Labels[key] = label
		}
	}
	return New(s, historyLimit)
}

// WriteColoredWrappedMaterialAssemblySTEP retains exact asset/occurrence
// provenance and binds styles to unique regenerated face names. Renaming or
// deleting a styled face without updating its binding fails before output.
func (s State) WriteColoredWrappedMaterialAssemblySTEP(w io.Writer) error {
	if w == nil {
		return fmt.Errorf("document: nil writer")
	}
	var raw bytes.Buffer
	if e := s.WriteWrappedMaterialAssemblySTEP(&raw); e != nil {
		return e
	}
	a, e := step.ReadWrappedMaterialAssembly(&raw, step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		return e
	}
	// Reproduce the strict adapter's sorted member order per shared product.
	styles := map[string][]wrappedNamedStyle{}
	nodes := make([]string, 0, len(s.Nodes))
	for id := range s.Nodes {
		nodes = append(nodes, id)
	}
	sort.Strings(nodes)
	for _, id := range nodes {
		p, ok := s.Labels["step.product:"+id]
		if !ok {
			continue
		}
		identifier := p.Attributes["step.product.identifier"].Text
		if identifier == "" {
			identifier = p.Attributes["step.product.id"].Text
		}
		if _, ok := styles[identifier]; ok {
			continue
		}
		members := []string{}
		for mid, n := range s.Nodes {
			if n.Parent == id && n.Asset != "" {
				members = append(members, mid)
			}
		}
		sort.Strings(members)
		local := []wrappedNamedStyle{}
		for _, mid := range members {
			label := s.Labels["step:"+s.Nodes[mid].Asset]
			attr, ok := label.Attributes[wrappedStyleAttribute]
			var style wrappedNamedStyle
			if ok {
				if attr.Kind != "string" || len(attr.Text) > 1<<20 {
					return fmt.Errorf("document: invalid named style")
				}
				dec := json.NewDecoder(strings.NewReader(attr.Text))
				dec.DisallowUnknownFields()
				if e := dec.Decode(&style); e != nil {
					return e
				}
				var extra any
				if e := dec.Decode(&extra); e != io.EOF {
					return fmt.Errorf("document: trailing style data")
				}
			}
			local = append(local, style)
		}
		styles[identifier] = local
	}
	model := step.ColoredWrappedMaterialAssemblyModel{LengthUnit: a.Assembly.LengthUnit}
	ids := map[int64]string{}
	for _, p := range a.Assembly.Products {
		id := p.Identifier
		if id == "" {
			id = strconv.FormatInt(p.DefinitionID, 10)
		}
		ids[p.DefinitionID] = id
		colors, ok := styles[id]
		bodies := a.ProductBodies[p.DefinitionID]
		if !ok || len(colors) != len(bodies) {
			return fmt.Errorf("document: style product ownership mismatch")
		}
		part := step.ColoredWrappedMaterialAssemblyPart{ID: id, Name: p.Name}
		for i, b := range bodies {
			part.Bodies = append(part.Bodies, step.ColoredWrappedMaterialBody{Body: b, SurfaceColor: colors[i].SurfaceColor, FaceColors: colors[i].FaceColors})
		}
		model.Products = append(model.Products, part)
	}
	for _, v := range a.Assembly.Occurrences {
		model.Occurrences = append(model.Occurrences, step.AssemblyInstance{ID: strconv.FormatInt(v.ID, 10), Name: v.Name, Parent: ids[v.Parent], Child: ids[v.Child], Transform: v.Transform})
	}
	return step.WriteColoredWrappedMaterialAssembly(w, model)
}
