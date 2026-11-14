package plaincadbridge

import (
	"context"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func newTripleCrossedBore(base brep.Solid, tools []*cylinderBody, cuts []trimmed.CylinderCut, spans []float64, blind []bool) (*boreBody, bool) {
	if len(tools) != 3 {
		return nil, false
	}
	var byAxis [3]*cylinderBody
	for i, tool := range tools {
		if blind[i] || tool.radius != tools[0].radius || !cardinalFrame(tool.frame) {
			return nil, false
		}
		for axis, world := range disjointBoreAxes {
			if tool.frame.ZAxis().Dot(world) != 0 {
				if byAxis[axis] != nil {
					return nil, false
				}
				byAxis[axis] = tool
				break
			}
		}
	}
	p, ok := crossedBoreRecipe(base, []*cylinderBody{byAxis[0], byAxis[2]}, []bool{false, false})
	if !ok || p.offset != 0 {
		return nil, false
	}
	middle, ok := crossedCompanionSite(p.frame, byAxis[1])
	if !ok || middle.Axis != 1 || middle.Center != (geom.Vec3{X: p.crossing.X, Z: p.crossing.Z}) {
		return nil, false
	}
	r, err := trimmed.BoxTripleCrossedBores(context.Background(), p.frame, p.dimensions, p.crossing, p.radii[0], 1e-9)
	if err != nil {
		return nil, false
	}
	return &boreBody{solid: r.Solid, base: base, cylinders: tools, cuts: cuts, spans: spans, blind: blind, crossed: &r, tripleCrossed: &p}, true
}
