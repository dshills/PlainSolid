package plaincadbridge

import (
	"context"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type tripleCrossedGeometry struct {
	crossedBoreGeometry
	depths   [3]float64
	opposite [3]bool
}

func newTripleCrossedBore(base brep.Solid, tools []*cylinderBody, cuts []trimmed.CylinderCut, spans []float64, blind []bool) (*boreBody, bool) {
	if len(tools) != 3 {
		return nil, false
	}
	var byAxis [3]int
	for i := range byAxis {
		byAxis[i] = -1
	}
	blindTool, blindCount := -1, 0
	for i, tool := range tools {
		if tool.radius != tools[0].radius || !cardinalFrame(tool.frame) {
			return nil, false
		}
		if blind[i] {
			blindTool = i
			blindCount++
		}
		for axis, world := range disjointBoreAxes {
			if tool.frame.ZAxis().Dot(world) != 0 {
				if byAxis[axis] >= 0 {
					return nil, false
				}
				byAxis[axis] = i
				break
			}
		}
	}
	if blindCount != 1 {
		blindTool = -1
	}
	x, y, z := byAxis[0], byAxis[1], byAxis[2]
	if blindTool == x {
		x, z = z, x
	} else if blindTool == y {
		y, z = z, y
	}
	p, ok := crossedBoreRecipe(base, []*cylinderBody{tools[x], tools[z]}, []bool{false, false})
	if !ok || p.offset != 0 {
		return nil, false
	}
	if blindTool >= 0 && tools[blindTool].frame.ZAxis().Dot(p.frame.ZAxis()) == 0 {
		old := p.frame
		frame, err := geom.NewFrame(old.ToWorld(geom.Vec3{Y: p.dimensions.Y}), old.ZAxis(), old.XAxis())
		if err != nil {
			return nil, false
		}
		p.frame = frame
		p.crossing = frame.ToLocal(old.ToWorld(p.crossing))
		p.dimensions = geom.Vec3{X: p.dimensions.Z, Y: p.dimensions.Y, Z: p.dimensions.X}
	}
	middle, ok := crossedCompanionSite(p.frame, tools[y])
	if !ok || middle.Axis != 1 || middle.Center != (geom.Vec3{X: p.crossing.X, Z: p.crossing.Z}) {
		return nil, false
	}
	g := tripleCrossedGeometry{crossedBoreGeometry: p}
	floors := make([]int, 3)
	nextFloor := trimmed.BlindTripleCrossedBoreFloorFace
	axisByGroup := [3]geom.Vec3{p.frame.XAxis(), p.frame.YAxis(), p.frame.ZAxis()}
	for _, axis := range [3]int{2, 0, 1} {
		for i, tool := range tools {
			dot := tool.frame.ZAxis().Dot(axisByGroup[axis])
			if !blind[i] || dot == 0 {
				continue
			}
			g.depths[axis], g.opposite[axis] = spans[i], dot == -1
			floor := coordinates(p.crossing)
			floor[axis] = g.depths[axis]
			if g.opposite[axis] {
				floor[axis] = coordinates(p.dimensions)[axis] - g.depths[axis]
			}
			authored := tool.frame.Origin().Add(tool.frame.ZAxis().Scale(tool.height))
			if p.frame.ToWorld(floor.vec()) != authored {
				return nil, false
			}
			floors[i] = nextFloor
			nextFloor++
			break
		}
	}
	r, err := trimmed.BoxFiniteTripleCrossedBores(context.Background(), p.frame, p.dimensions, p.crossing, p.radii[0], g.depths, g.opposite, 1e-9)
	if err != nil {
		return nil, false
	}
	return &boreBody{solid: r.Solid, base: base, cylinders: tools, cuts: cuts, spans: spans, blind: blind, floors: floors, crossed: &r, tripleCrossed: &g}, true
}
