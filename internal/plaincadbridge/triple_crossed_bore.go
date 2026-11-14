package plaincadbridge

import (
	"context"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type tripleCrossedGeometry struct {
	crossedBoreGeometry
	depth       float64
	oppositeCap bool
}

func newTripleCrossedBore(base brep.Solid, tools []*cylinderBody, cuts []trimmed.CylinderCut, spans []float64, blind []bool) (*boreBody, bool) {
	if len(tools) != 3 {
		return nil, false
	}
	var byAxis [3]int
	for i := range byAxis {
		byAxis[i] = -1
	}
	blindTool := -1
	for i, tool := range tools {
		if tool.radius != tools[0].radius || !cardinalFrame(tool.frame) {
			return nil, false
		}
		if blind[i] {
			if blindTool >= 0 {
				return nil, false
			}
			blindTool = i
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
	ctx := context.Background()
	g := tripleCrossedGeometry{crossedBoreGeometry: p}
	floors := make([]int, 3)
	var r trimmed.BoxCrossedBoresResult
	var err error
	if blindTool < 0 {
		r, err = trimmed.BoxTripleCrossedBores(ctx, p.frame, p.dimensions, p.crossing, p.radii[0], 1e-9)
	} else {
		tool := tools[blindTool]
		dot := tool.frame.ZAxis().Dot(p.frame.ZAxis())
		if dot != 1 && dot != -1 {
			return nil, false
		}
		g.depth, g.oppositeCap = spans[blindTool], dot == -1
		floor := p.crossing
		floor.Z = g.depth
		if g.oppositeCap {
			floor.Z = p.dimensions.Z - g.depth
		}
		authored := tool.frame.Origin().Add(tool.frame.ZAxis().Scale(tool.height))
		if p.frame.ToWorld(floor) != authored {
			return nil, false
		}
		r, err = trimmed.BoxBlindTripleCrossedBores(ctx, p.frame, p.dimensions, p.crossing, p.radii[0], g.depth, g.oppositeCap, 1e-9)
		floors[blindTool] = trimmed.BlindTripleCrossedBoreFloorFace
	}
	if err != nil {
		return nil, false
	}
	return &boreBody{solid: r.Solid, base: base, cylinders: tools, cuts: cuts, spans: spans, blind: blind, floors: floors, crossed: &r, tripleCrossed: &g}, true
}
