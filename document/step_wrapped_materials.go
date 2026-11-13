package document

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"sort"
)

// FromWrappedMaterialsSTEP imports the twelve explicitly tagged material families.
// Source graph IDs are consumed during validation, never stored as native IDs.
// Native assets and labels preserve each body's family and name.
func FromWrappedMaterialsSTEP(m step.WrappedMaterialModel, historyLimit int) (*Document, error) {
	var buf bytes.Buffer
	if e := step.WriteWrappedMaterials(&buf, m); e != nil {
		return nil, e
	}
	verified, e := step.ReadWrappedMaterials(&buf, step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		return nil, e
	}
	plain := step.TrimmedModel{LengthUnit: verified.LengthUnit}
	labels := map[string]Label{}
	for i, b := range verified.Bodies {
		var name, family string
		var solid trimmed.Solid
		attrs := map[string]Attribute{}
		add := func(k, v string) { attrs[k] = Attribute{Kind: "string", Text: v} }
		switch {
		case b.Toroid != nil:
			name, family, solid = b.Toroid.Name, "toroid", b.Toroid.Solid
			add("step.face.name", b.Toroid.Face.Name)
		case b.Cylinder != nil:
			name, family, solid = b.Cylinder.Name, "cylinder", b.Cylinder.Solid
			add("step.face.name", b.Cylinder.Face.Name)
			add("step.cap.lower.name", b.Cylinder.CapNames[0])
			add("step.cap.upper.name", b.Cylinder.CapNames[1])
		case b.Frustum != nil:
			name, family, solid = b.Frustum.Name, "frustum", b.Frustum.Solid
			add("step.face.name", b.Frustum.Face.Name)
			add("step.cap.lower.name", b.Frustum.CapNames[0])
			add("step.cap.upper.name", b.Frustum.CapNames[1])
		case b.Tube != nil:
			name, family, solid = b.Tube.Name, "tube", b.Tube.Solid
			add("step.outer.name", b.Tube.Outer.Name)
			add("step.inner.name", b.Tube.Inner.Name)
			add("step.cap.lower.name", b.Tube.CapNames[0])
			add("step.cap.upper.name", b.Tube.CapNames[1])
		case b.BlindBore != nil:
			name, family, solid = b.BlindBore.Name, "blind-bore", b.BlindBore.Solid
			add("step.outer.name", b.BlindBore.Outer.Name)
			add("step.inner.name", b.BlindBore.Inner.Name)
			add("step.cap.lower.name", b.BlindBore.CapNames[0])
			add("step.cap.upper.name", b.BlindBore.CapNames[1])
			add("step.floor.name", b.BlindBore.FloorName)
		case b.Shoulder != nil:
			name, family, solid = b.Shoulder.Name, "shoulder", b.Shoulder.Solid
			add("step.outer.name", b.Shoulder.Larger.Name)
			add("step.inner.name", b.Shoulder.Smaller.Name)
			add("step.cap.lower.name", b.Shoulder.LargerCapNames[0])
			add("step.cap.upper.name", b.Shoulder.LargerCapNames[1])
			add("step.floor.name", b.Shoulder.SmallerEndName)
		case b.Cavity != nil:
			name, family, solid = b.Cavity.Name, "cavity", b.Cavity.Solid
			add("step.outer.name", b.Cavity.Outer.Name)
			add("step.inner.name", b.Cavity.Inner.Name)
			add("step.cap.lower.name", b.Cavity.OuterCapNames[0])
			add("step.cap.upper.name", b.Cavity.OuterCapNames[1])
			add("step.inner.cap.lower.name", b.Cavity.InnerCapNames[0])
			add("step.inner.cap.upper.name", b.Cavity.InnerCapNames[1])
		case b.ToroidalVoid != nil:
			name, family, solid = b.ToroidalVoid.Name, "toroidal-void", b.ToroidalVoid.Solid
			add("step.outer.name", b.ToroidalVoid.Outer.Name)
			add("step.inner.name", b.ToroidalVoid.Inner.Name)
		case b.AnnularFrustum != nil:
			name, family, solid = b.AnnularFrustum.Name, "annular-frustum", b.AnnularFrustum.Solid
			add("step.cap.lower.name", b.AnnularFrustum.CapNames[0])
			add("step.cap.upper.name", b.AnnularFrustum.CapNames[1])
			for j, w := range b.AnnularFrustum.Outer.Walls {
				add(fmt.Sprintf("step.outer.wall.%d.name", j), w.Name)
			}
			for j, w := range b.AnnularFrustum.Inner.Walls {
				add(fmt.Sprintf("step.inner.wall.%d.name", j), w.Name)
			}
		case b.StationBore != nil:
			name, family, solid = b.StationBore.Name, "station-bore", b.StationBore.Solid
			add("step.cap.lower.name", b.StationBore.CapNames[0])
			add("step.cap.upper.name", b.StationBore.CapNames[1])
			for j, w := range b.StationBore.Outer.Walls {
				add(fmt.Sprintf("step.outer.wall.%d.name", j), w.Name)
			}
			for j, w := range b.StationBore.Inner.Walls {
				add(fmt.Sprintf("step.inner.wall.%d.name", j), w.Name)
			}
		case b.RimFillet != nil:
			name, family, solid = b.RimFillet.Name, "rim-fillet", b.RimFillet.Solid
			add("step.cap.lower.name", b.RimFillet.CapNames[0])
			add("step.cap.upper.name", b.RimFillet.CapNames[1])
			for j, w := range b.RimFillet.Walls {
				add(fmt.Sprintf("step.wall.%d.name", j), w.Name)
			}
		case b.StationLoft != nil:
			name, family, solid = b.StationLoft.Name, "station-loft", b.StationLoft.Solid
			add("step.cap.lower.name", b.StationLoft.CapNames[0])
			add("step.cap.upper.name", b.StationLoft.CapNames[1])
			for j, w := range b.StationLoft.Walls {
				add(fmt.Sprintf("step.wall.%d.name", j), w.Name)
			}
		default:
			return nil, fmt.Errorf("document: unsupported wrapped family")
		}
		plain.Bodies = append(plain.Bodies, step.TrimmedBody{Name: name, Solid: solid})
		id := fmt.Sprintf("body-%06d", i+1)
		add("step.wrapped.family", family)
		add("step.body.name", name)
		labels["step:"+id] = Label{Name: name, Target: AssetLabel, TargetID: id, Attributes: attrs}
	}
	d, e := FromTrimmedSTEP(plain, historyLimit)
	if e != nil {
		return nil, e
	}
	s, _ := d.Snapshot()
	s.Labels = labels
	return New(s, historyLimit)
}

// WriteWrappedMaterialsSTEP exports explicitly tagged toroid, cylinder, frustum,
// tube, blind-bore, cavity, shoulder, toroidal void, station-loft, station-bore, annular-frustum and rim-fillet assets.
// Unknown family tags return unsupported.
// Repeated assets remain shared in the native document; this export emits
// independent translated bodies, not a STEP assembly or a Boolean union.
func (s State) WriteWrappedMaterialsSTEP(w io.Writer) error {
	if w == nil {
		return fmt.Errorf("document: nil wrapped material writer")
	}
	var e error
	if len(s.Features) > 0 {
		s, e = s.EvaluateFeatures(context.Background())
		if e != nil {
			return e
		}
	}
	if e = s.Validate(); e != nil {
		return e
	}
	if len(s.Nodes) > 10000 {
		return fmt.Errorf("document: node budget")
	}
	ids := []string{}
	for id, n := range s.Nodes {
		if n.Asset != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	model := step.WrappedMaterialModel{LengthUnit: s.LengthUnit}
	for _, id := range ids {
		n := s.Nodes[id]
		label, ok := s.Labels["step:"+n.Asset]
		if !ok || label.Target != AssetLabel || label.TargetID != n.Asset {
			return fmt.Errorf("document: missing or stale wrapped asset label")
		}
		get := func(k string) string {
			a := label.Attributes[k]
			if a.Kind == "string" {
				return a.Text
			}
			return ""
		}
		family := get("step.wrapped.family")
		one := s
		one.Features = nil
		one.Nodes = map[string]Node{}
		for nodeID, node := range s.Nodes {
			if nodeID != id {
				node.Asset = ""
			}
			one.Nodes[nodeID] = node
		}
		for current, depth := id, 0; current != ""; depth++ {
			if depth >= 256 {
				return fmt.Errorf("document: placement depth budget")
			}
			current = s.Nodes[current].Parent
		}

		var raw bytes.Buffer
		switch family {
		case "toroid":
			if e = one.WriteWrappedToroidalSTEP(&raw); e != nil {
				return e
			}
			m, err := step.ReadWrappedToroidalBodies(&raw, step.Limits{}, step.WrappedSurfaceOptions{})
			if err != nil {
				return err
			}
			if len(m.Bodies) != 1 {
				return fmt.Errorf("document: toroid occurrence count")
			}
			model.Bodies = append(model.Bodies, step.WrappedMaterialBody{Toroid: &m.Bodies[0]})
		case "cylinder":
			if e = one.WriteWrappedCylindricalSTEP(&raw); e != nil {
				return e
			}
			m, err := step.ReadWrappedCylindricalBodies(&raw, step.Limits{}, step.WrappedSurfaceOptions{})
			if err != nil {
				return err
			}
			if len(m.Bodies) != 1 {
				return fmt.Errorf("document: cylinder occurrence count")
			}
			model.Bodies = append(model.Bodies, step.WrappedMaterialBody{Cylinder: &m.Bodies[0]})
		case "tube", "blind-bore", "cavity", "shoulder", "toroidal-void":
			b, err := s.wrappedBoreSTEPBody(id, family)
			if err != nil {
				return err
			}
			model.Bodies = append(model.Bodies, b)
		case "frustum":
			if e = one.WriteWrappedFrustumSTEP(&raw); e != nil {
				return e
			}
			m, err := step.ReadWrappedFrusta(&raw, step.Limits{}, step.WrappedSurfaceOptions{})
			if err != nil {
				return err
			}
			if len(m.Bodies) != 1 {
				return fmt.Errorf("document: frustum occurrence count")
			}
			model.Bodies = append(model.Bodies, step.WrappedMaterialBody{Frustum: &m.Bodies[0]})
		case "annular-frustum":
			body, err := s.wrappedAnnularFrustumSTEPBody(id)
			if err != nil {
				return err
			}
			model.Bodies = append(model.Bodies, step.WrappedMaterialBody{AnnularFrustum: &body})
		case "station-bore":
			body, err := s.wrappedStationBoreSTEPBody(id)
			if err != nil {
				return err
			}
			model.Bodies = append(model.Bodies, step.WrappedMaterialBody{StationBore: &body})
		case "rim-fillet":
			body, err := s.wrappedRimFilletSTEPBody(id)
			if err != nil {
				return err
			}
			model.Bodies = append(model.Bodies, step.WrappedMaterialBody{RimFillet: &body})
		case "station-loft":
			solid, ok := s.Trimmed[n.Asset]
			if !ok {
				return fmt.Errorf("document: loft requires trimmed asset")
			}
			name := n.Name
			if name == "" {
				name = id
			}
			b, err := step.WrappedStationLoftFromSolid(context.Background(), solid, name, s.LengthUnit, geom.SurfacePatchOptions{})
			if err != nil {
				return err
			}
			b.CapNames = [2]string{get("step.cap.lower.name"), get("step.cap.upper.name")}
			for j := range b.Walls {
				b.Walls[j].Name = get(fmt.Sprintf("step.wall.%d.name", j))
				support := b.Walls[j].Geometry
				for current := id; current != ""; {
					node := s.Nodes[current]
					a := node.Transform
					identity := geom.Identity()
					for r := 0; r < 3; r++ {
						for c := 0; c < 3; c++ {
							if a[r][c] != identity[r][c] {
								return fmt.Errorf("document: loft placement must be translation")
							}
						}
					}
					support, _, err = support.Transformed(context.Background(), a, geom.WrappedSurfaceOptions{})
					if err != nil {
						return err
					}
					current = node.Parent
				}
				b.Walls[j].Geometry = support
				proof, err := geom.CertifyWrappedFrustumEmbedding(context.Background(), support, geom.SurfacePatchOptions{})
				if err != nil || !proof.Complete {
					return fmt.Errorf("document: translated loft support proof: %v", err)
				}
				x, err := geom.RepresentedExactScalar(proof.CenterX)
				if err != nil {
					return err
				}
				y, err := geom.RepresentedExactScalar(proof.CenterY)
				if err != nil {
					return err
				}
				b.Center = geom.Vec2{X: x, Y: y}
				z, err := geom.RepresentedExactScalar(proof.LowerZ)
				if err != nil {
					return err
				}
				b.Stations[j].Z = z
				z, err = geom.RepresentedExactScalar(proof.UpperZ)
				if err != nil {
					return err
				}
				b.Stations[j+1].Z = z
			}
			model.Bodies = append(model.Bodies, step.WrappedMaterialBody{StationLoft: &b})
		default:
			return fmt.Errorf("document: wrapped family %q export unsupported", family)
		}
	}
	return step.WriteWrappedMaterials(w, model)
}
