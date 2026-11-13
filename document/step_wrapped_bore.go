package document

import (
	"context"
	"fmt"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (s State) wrappedBoreSTEPBody(id, family string) (step.WrappedMaterialBody, error) {
	fail := func() (step.WrappedMaterialBody, error) {
		return step.WrappedMaterialBody{}, fmt.Errorf("document: unsupported or inconsistent wrapped bore asset")
	}
	node := s.Nodes[id]
	solid, ok := s.Trimmed[node.Asset]
	if !ok {
		return fail()
	}
	var outer, inner *geom.WrappedSurfacePatch
	var e error
	switch family {
	case "shoulder":
		p, err := solid.ValidateCanonicalWrappedShoulder(context.Background(), geom.SurfacePatchOptions{})
		if err != nil {
			return step.WrappedMaterialBody{}, err
		}
		if !p.Complete {
			return fail()
		}
		outer, inner = p.First, p.Second
	case "toroidal-void":
		p, err := solid.ValidateCanonicalWrappedToroidalVoid(context.Background(), geom.SurfacePatchOptions{})
		if err != nil {
			return step.WrappedMaterialBody{}, err
		}
		if !p.Complete {
			return fail()
		}
		outer, inner = p.First, p.Second
	default:
		proof, err := solid.ValidateCanonicalWrappedBore(context.Background(), geom.SurfacePatchOptions{})
		if err != nil {
			return step.WrappedMaterialBody{}, err
		}
		if !proof.Complete || proof.Kind != family {
			return fail()
		}
		outer, inner = proof.Outer, proof.Inner
	}
	for current, depth := id, 0; current != ""; depth++ {
		if depth >= 256 {
			return fail()
		}
		n := s.Nodes[current]
		a := n.Transform
		identity := geom.Identity()
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				if a[r][c] != identity[r][c] {
					return fail()
				}
			}
		}
		outer, _, e = outer.Transformed(context.Background(), a, geom.WrappedSurfaceOptions{})
		if e != nil {
			return step.WrappedMaterialBody{}, e
		}
		inner, _, e = inner.Transformed(context.Background(), a, geom.WrappedSurfaceOptions{})
		if e != nil {
			return step.WrappedMaterialBody{}, e
		}
		current = n.Parent
	}
	label := s.Labels["step:"+node.Asset]
	get := func(k string) string {
		a := label.Attributes[k]
		if a.Kind == "string" {
			return a.Text
		}
		return ""
	}
	name := node.Name
	if name == "" {
		name = id
	}
	a, b := step.WrappedSplineFace{Name: get("step.outer.name"), Geometry: outer}, step.WrappedSplineFace{Name: get("step.inner.name"), Geometry: inner}
	caps := [2]string{get("step.cap.lower.name"), get("step.cap.upper.name")}
	switch family {
	case "shoulder":
		return step.WrappedMaterialBody{Shoulder: &step.WrappedShoulderBody{Name: name, Larger: a, Smaller: b, LargerCapNames: caps, SmallerEndName: get("step.floor.name"), LengthUnit: s.LengthUnit}}, nil
	case "toroidal-void":
		return step.WrappedMaterialBody{ToroidalVoid: &step.WrappedToroidalVoid{Name: name, Outer: a, Inner: b, LengthUnit: s.LengthUnit}}, nil
	case "tube":
		return step.WrappedMaterialBody{Tube: &step.WrappedTubeBody{Name: name, Outer: a, Inner: b, CapNames: caps, LengthUnit: s.LengthUnit}}, nil
	case "blind-bore":
		return step.WrappedMaterialBody{BlindBore: &step.WrappedBlindBoreBody{Name: name, Outer: a, Inner: b, CapNames: caps, FloorName: get("step.floor.name"), LengthUnit: s.LengthUnit}}, nil
	case "cavity":
		return step.WrappedMaterialBody{Cavity: &step.WrappedCavityBody{Name: name, Outer: a, Inner: b, OuterCapNames: caps, InnerCapNames: [2]string{get("step.inner.cap.lower.name"), get("step.inner.cap.upper.name")}, LengthUnit: s.LengthUnit}}, nil
	}
	return fail()
}
