package plaincadbridge

import (
	"context"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type blindCrossedGeometry struct {
	crossedBoreGeometry
	depth        [2]float64
	oppositeCap  [2]bool
	clippedFloor bool
}

func blindCrossedRecipe(base brep.Solid, tools []*cylinderBody, spans []float64, blind []bool) (blindCrossedGeometry, bool) {
	var p blindCrossedGeometry
	if len(tools) != 2 || !blind[0] && !blind[1] || tools[0].radius != tools[1].radius {
		return p, false
	}
	crossed, ok := crossedBoreRecipe(base, tools, []bool{false, false})
	if !ok || crossed.offset != 0 {
		return p, false
	}
	if blind[0] != blind[1] {
		tool := tools[0]
		if blind[1] {
			tool = tools[1]
		}
		// A lone floor occupies local Z. Reflect the transverse chart
		// when exchanging X and Z to keep the frame right-handed.
		if tool.frame.ZAxis().Dot(crossed.frame.XAxis()) != 0 {
			old := crossed.frame
			f, err := geom.NewFrame(old.ToWorld(geom.Vec3{Y: crossed.dimensions.Y}), old.ZAxis(), old.XAxis())
			if err != nil {
				return p, false
			}
			crossed.frame = f
			x, y, z := crossed.dimensions.X, crossed.dimensions.Y, crossed.dimensions.Z
			crossed.dimensions = geom.Vec3{X: z, Y: y, Z: x}
			crossed.crossing = f.ToLocal(old.ToWorld(crossed.crossing))
		}
	}
	p.crossedBoreGeometry = crossed
	for i, tool := range tools {
		if !blind[i] {
			continue
		}
		axis := tool.frame.ZAxis()
		slot := 1
		direction := axis.Dot(p.frame.ZAxis())
		size := p.dimensions.Z
		if axis.Dot(p.frame.XAxis()) != 0 {
			slot, direction, size = 0, axis.Dot(p.frame.XAxis()), p.dimensions.X
		}
		if direction != 1 && direction != -1 || !finite(spans[i]) || spans[i] <= 0 {
			return blindCrossedGeometry{}, false
		}
		p.depth[slot], p.oppositeCap[slot] = spans[i], direction == -1
		level := spans[i]
		if p.oppositeCap[slot] {
			level = size - spans[i]
		}
		floor := p.crossing
		if slot == 0 {
			floor.X = level
		} else {
			floor.Z = level
		}
		authoredFloor := tool.frame.Origin().Add(axis.Scale(tool.height))
		if authoredFloor != p.frame.ToWorld(floor) {
			return blindCrossedGeometry{}, false
		}
	}
	if p.depth[0] == 0 {
		distance := p.depth[1] - p.crossing.Z
		if p.oppositeCap[1] {
			distance = p.depth[1] - (p.dimensions.Z - p.crossing.Z)
		}
		if distance > -p.radii[0] && distance < p.radii[0] {
			p.clippedFloor = true
			if p.oppositeCap[1] {
				old := p.frame
				frame, err := geom.NewFrame(old.ToWorld(geom.Vec3{X: p.dimensions.X, Z: p.dimensions.Z}), old.XAxis().Scale(-1), old.ZAxis().Scale(-1))
				if err != nil {
					return blindCrossedGeometry{}, false
				}
				p.frame, p.crossing = frame, frame.ToLocal(old.ToWorld(p.crossing))
				p.oppositeCap[1] = false
			}
		}
	}
	return p, true
}

func newBlindCrossedBore(base brep.Solid, tools []*cylinderBody, cuts []trimmed.CylinderCut, spans []float64, blind []bool) (*boreBody, bool, error) {
	p, ok := blindCrossedRecipe(base, tools, spans, blind)
	if !ok {
		return nil, false, nil
	}
	ctx := context.Background()
	var result trimmed.BoxCrossedBoresResult
	var err error
	switch {
	case p.clippedFloor:
		switch {
		case p.depth[1] < p.crossing.Z:
			result, err = trimmed.BoxBeforeCrossedBores(ctx, p.frame, p.dimensions, p.crossing, p.radii[0], p.depth[1], 1e-9)
		case p.depth[1] == p.crossing.Z:
			result, err = trimmed.BoxCenterCrossedBores(ctx, p.frame, p.dimensions, p.crossing, p.radii[0], 1e-9)
		default:
			result, err = trimmed.BoxClippedCrossedBores(ctx, p.frame, p.dimensions, p.crossing, p.radii[0], p.depth[1], 1e-9)
		}
	case p.depth[0] != 0:
		result, err = trimmed.BoxDoubleBlindCrossedBores(ctx, p.frame, p.dimensions, p.crossing, p.radii[0],
			p.depth[0], p.depth[1], p.oppositeCap[0], p.oppositeCap[1], 1e-9)
	default:
		result, err = trimmed.BoxBlindCrossedBores(ctx, p.frame, p.dimensions, p.crossing, p.radii[0],
			p.depth[1], p.oppositeCap[1], 1e-9)
	}
	if err != nil {
		return nil, true, err
	}
	floors := make([]int, len(blind))
	for i := range blind {
		if !blind[i] {
			continue
		}
		floors[i] = trimmed.BlindCrossedBoreFloorFace
		if tools[i].frame.ZAxis().Dot(p.frame.XAxis()) != 0 {
			floors[i] = trimmed.DoubleBlindCrossedBoreXFloorFace
		}
	}
	return &boreBody{
		solid: result.Solid, base: base, cylinders: tools, cuts: cuts,
		spans: spans, blind: blind, floors: floors, crossed: &result, blindCrossed: &p,
	}, true, nil
}

func blindCrossedFromSTEP(p step.CrossedBoreBody, unit float64) (*boreBody, error) {
	geometry := crossedBoreGeometry{frame: p.Frame, dimensions: p.Dimensions, crossing: p.Crossing, radii: [2]float64{p.Radius, p.Radius}}
	return finiteCrossedBoreFromSTEP(geometry, [2]float64{p.XBlindDepth, p.BlindDepth}, [2]bool{p.XOppositeCap, p.OppositeCap}, unit)
}
