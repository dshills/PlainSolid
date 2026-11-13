package document

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// FromSTEP creates a flat assembly from the independently represented planar
// bodies in a STEP model. File units are preserved; original names become node
// labels and generated asset IDs are deterministic.
func FromSTEP(model step.Model, historyLimit int) (*Document, error) {
	s := State{LengthUnit: model.LengthUnit, Planar: map[string]brep.Solid{}, Nodes: map[string]Node{}}
	for i, b := range model.Bodies {
		id := fmt.Sprintf("body-%06d", i+1)
		if len(b.Solid.Faces()) == 0 {
			return nil, fmt.Errorf("document: STEP body %d is empty", i)
		}
		s.Planar[id] = b.Solid
		s.Nodes[id] = Node{Name: b.Name, Asset: id, Transform: geom.Identity()}
	}
	return New(s, historyLimit)
}

// WritePlanarSTEP exports each placed planar instance as a separate body.
// Coordinate units are preserved. Hierarchy is flattened to world placements.
// Curved and mesh instances require a different exchange route.
func (s State) WritePlanarSTEP(w io.Writer, advanced bool) error {
	if len(s.Features) > 0 {
		m, e := s.EvaluateFeatures(context.Background())
		if e != nil {
			return e
		}
		s = m
	}
	if e := s.Validate(); e != nil {
		return e
	}
	transforms, e := s.worldTransforms()
	if e != nil {
		return e
	}
	ids := make([]string, 0, len(s.Nodes))
	for id, n := range s.Nodes {
		if n.Asset != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	m := step.Model{LengthUnit: s.LengthUnit}
	for _, id := range ids {
		n := s.Nodes[id]
		b, ok := s.Planar[n.Asset]
		if !ok {
			return fmt.Errorf("document: STEP planar export cannot represent instance %q", id)
		}
		tr := transforms[id]
		b, e = b.Transformed(tr)
		if e != nil {
			return e
		}
		props, e := b.MassProperties()
		if e != nil || props.Volume <= 0 {
			return fmt.Errorf("document: STEP body %q must be outward oriented", id)
		}
		name := n.Name
		if name == "" {
			name = id
		}
		m.Bodies = append(m.Bodies, step.Body{Name: name, Solid: b})
	}
	if advanced {
		return step.WriteAdvanced(w, m)
	}
	return step.Write(w, m)
}

// FromAnalyticSTEP creates a flat assembly of exact analytic STEP assets.
func FromAnalyticSTEP(model step.AnalyticModel, historyLimit int) (*Document, error) {
	s := State{LengthUnit: model.LengthUnit, Parametric: map[string]parametric.Solid{}, Nodes: map[string]Node{}}
	for i, b := range model.Bodies {
		if len(b.Solid.Faces()) == 0 {
			return nil, fmt.Errorf("document: analytic STEP body %d is empty", i)
		}
		id := fmt.Sprintf("body-%06d", i+1)
		s.Parametric[id] = b.Solid
		s.Nodes[id] = Node{Name: b.Name, Asset: id, Transform: geom.Identity()}
	}
	return New(s, historyLimit)
}

// FromTrimmedSTEP creates a flat assembly retaining exact trimmed topology
// and the STEP model's names and coordinate units.
func FromTrimmedSTEP(model step.TrimmedModel, historyLimit int) (*Document, error) {
	s := State{LengthUnit: model.LengthUnit, Trimmed: map[string]trimmed.Solid{}, Nodes: map[string]Node{}}
	for i, b := range model.Bodies {
		if len(b.Solid.Faces()) == 0 {
			return nil, fmt.Errorf("document: trimmed STEP body %d is empty", i)
		}
		id := fmt.Sprintf("body-%06d", i+1)
		s.Trimmed[id] = b.Solid
		s.Nodes[id] = Node{Name: b.Name, Asset: id, Transform: geom.Identity()}
	}
	return New(s, historyLimit)
}

// FromCavitySTEP imports oriented planar bodies including inward void shells.
func FromCavitySTEP(model step.CavityModel, historyLimit int) (*Document, error) {
	m := step.Model{LengthUnit: model.LengthUnit}
	for _, b := range model.Bodies {
		m.Bodies = append(m.Bodies, step.Body{Name: b.Name, Solid: b.Solid})
	}
	return FromSTEP(m, historyLimit)
}

// WriteCavitySTEP writes placed planar instances with one convex outer shell
// and at least one strictly contained, separated convex cavity. The
// STEP writer independently validates the shell relations.
func (s State) WriteCavitySTEP(w io.Writer) error {
	if len(s.Features) > 0 {
		m, e := s.EvaluateFeatures(context.Background())
		if e != nil {
			return e
		}
		s = m
	}
	if e := s.Validate(); e != nil {
		return e
	}
	world, e := s.worldTransforms()
	if e != nil {
		return e
	}
	ids := make([]string, 0, len(s.Nodes))
	for id, n := range s.Nodes {
		if n.Asset != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	m := step.CavityModel{LengthUnit: s.LengthUnit}
	for _, id := range ids {
		n := s.Nodes[id]
		b, ok := s.Planar[n.Asset]
		if !ok {
			return fmt.Errorf("document: cavity STEP needs planar asset for %q", id)
		}
		b, e = b.Transformed(world[id])
		if e != nil {
			return e
		}
		sh, e := b.Shells()
		if e != nil {
			return e
		}
		name := n.Name
		if name == "" {
			name = id
		}
		c := step.CavityBody{Name: name}
		haveOuter := false
		for _, q := range sh {
			v := q.Tessellate().SignedVolume()
			switch {
			case v > 0:
				if haveOuter {
					return fmt.Errorf("document: cavity STEP needs one outer shell for %q", id)
				}
				haveOuter = true
				c.Outer = q
			case v < 0:
				c.Voids = append(c.Voids, step.OrientedVoidShell{Shell: q.Reversed(), Reversed: true})
			default:
				return fmt.Errorf("document: unresolved signed volume for shell in %q", id)
			}
		}
		if !haveOuter || len(c.Voids) == 0 {
			return fmt.Errorf("document: cavity STEP needs an outer shell and a void for %q", id)
		}
		m.Bodies = append(m.Bodies, c)
	}
	return step.WriteCavities(w, m)
}

// FromColoredSTEP imports planar STEP bodies and stores whole-body sRGB
// styles as asset label attributes. Nil colors remain unspecified.
func FromColoredSTEP(model step.ColoredModel, historyLimit int) (*Document, error) {
	m := step.FaceColoredModel{LengthUnit: model.LengthUnit}
	for _, b := range model.Bodies {
		m.Bodies = append(m.Bodies, step.FaceColoredBody{Body: b.Body, SurfaceColor: b.SurfaceColor})
	}
	return FromFaceColoredSTEP(m, historyLimit)
}

// FromFaceColoredSTEP stores whole-body and face-specific STEP colors as
// separately targeted document labels.
func FromFaceColoredSTEP(model step.FaceColoredModel, historyLimit int) (*Document, error) {
	plain := step.Model{LengthUnit: model.LengthUnit}
	for _, b := range model.Bodies {
		plain.Bodies = append(plain.Bodies, b.Body)
	}
	d, e := FromSTEP(plain, historyLimit)
	if e != nil {
		return nil, e
	}
	s, _ := d.Snapshot()
	s.Labels = map[string]Label{}
	for i, b := range model.Bodies {
		id := fmt.Sprintf("body-%06d", i+1)
		if b.SurfaceColor != nil {
			if !validColor(*b.SurfaceColor) {
				return nil, fmt.Errorf("document: invalid STEP surface color")
			}
			s.Labels["step-style:"+id] = Label{Name: b.Body.Name, Target: AssetLabel, TargetID: id, Attributes: colorAttributes(*b.SurfaceColor)}
		}
		for face, c := range b.FaceColors {
			if !validColor(c) {
				return nil, fmt.Errorf("document: invalid STEP face color")
			}
			f := face
			s.Labels[fmt.Sprintf("step-style:%s:face-%d", id, face)] = Label{Name: b.Body.Name, Target: FaceLabel, TargetID: id, Face: &f, Attributes: colorAttributes(c)}
		}
	}
	return New(s, historyLimit)
}

func colorAttributes(c step.RGB) map[string]Attribute {
	return map[string]Attribute{
		"step.srgb.red":   {Kind: "number", Number: c.R},
		"step.srgb.green": {Kind: "number", Number: c.G},
		"step.srgb.blue":  {Kind: "number", Number: c.B},
	}
}

func validColor(c step.RGB) bool {
	for _, v := range [3]float64{c.R, c.G, c.B} {
		if v < 0 || v > 1 || math.IsNaN(v) {
			return false
		}
	}
	return true
}

// WriteColoredPlanarSTEP exports placed planar instances with their asset's
// whole-body sRGB style, using the step.srgb.red, green and blue label keys.
// Partial and conflicting color labels fail before writing.
func (s State) WriteColoredPlanarSTEP(w io.Writer) error {
	return s.writeColoredPlanarSTEP(w, false)
}

// WriteFaceColoredPlanarSTEP exports planar body colors and face overrides.
// Indexed FaceLabel targets are preserved by a document instance placement.
func (s State) WriteFaceColoredPlanarSTEP(w io.Writer) error {
	return s.writeColoredPlanarSTEP(w, true)
}

func (s State) writeColoredPlanarSTEP(w io.Writer, faces bool) error {
	if len(s.Features) > 0 {
		m, e := s.EvaluateFeatures(context.Background())
		if e != nil {
			return e
		}
		s = m
	}
	if e := s.Validate(); e != nil {
		return e
	}
	world, e := s.worldTransforms()
	if e != nil {
		return e
	}
	styles, overrides, e := s.stepColorLabels(faces)
	if e != nil {
		return e
	}
	ids := make([]string, 0, len(s.Nodes))
	for id, n := range s.Nodes {
		if n.Asset != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	m := step.FaceColoredModel{LengthUnit: s.LengthUnit}
	for _, id := range ids {
		n := s.Nodes[id]
		b, ok := s.Planar[n.Asset]
		if !ok {
			return fmt.Errorf("document: colored STEP requires planar asset for %q", id)
		}
		b, e = b.Transformed(world[id])
		if e != nil {
			return e
		}
		name := n.Name
		if name == "" {
			name = id
		}
		x := step.FaceColoredBody{Body: step.Body{Name: name, Solid: b}, FaceColors: overrides[n.Asset]}
		if c, ok := styles[n.Asset]; ok {
			x.SurfaceColor = &c
		}
		m.Bodies = append(m.Bodies, x)
	}
	return step.WriteFaceColoredPlanar(w, m)
}

func (s State) stepColorLabels(faces bool) (map[string]step.RGB, map[string]map[int]step.RGB, error) {
	styles := map[string]step.RGB{}
	overrides := map[string]map[int]step.RGB{}
	for id, l := range s.Labels {
		r, ar := l.Attributes["step.srgb.red"]
		g, ag := l.Attributes["step.srgb.green"]
		b, ab := l.Attributes["step.srgb.blue"]
		if !ar && !ag && !ab {
			continue
		}
		c := step.RGB{R: r.Number, G: g.Number, B: b.Number}
		if (l.Target != AssetLabel && (!faces || l.Target != FaceLabel)) || !ar || !ag || !ab || r.Kind != "number" || g.Kind != "number" || b.Kind != "number" || !validColor(c) {
			return nil, nil, fmt.Errorf("document: invalid STEP color label %q", id)
		}
		if l.Target == FaceLabel {
			m := overrides[l.TargetID]
			if m == nil {
				m = map[int]step.RGB{}
				overrides[l.TargetID] = m
			}
			if old, ok := m[*l.Face]; ok && old != c {
				return nil, nil, fmt.Errorf("document: conflicting STEP face colors for asset %q", l.TargetID)
			}
			m[*l.Face] = c
			continue
		}
		if old, ok := styles[l.TargetID]; ok && old != c {
			return nil, nil, fmt.Errorf("document: conflicting STEP colors for asset %q", l.TargetID)
		}
		styles[l.TargetID] = c
	}
	return styles, overrides, nil
}
