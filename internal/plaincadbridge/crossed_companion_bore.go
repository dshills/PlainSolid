package plaincadbridge

import (
	"context"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type crossedCompanionGeometry struct {
	crossedBoreGeometry
	additional []trimmed.CardinalThroughBore
}

func newCrossedCompanionBore(base brep.Solid, tools []*cylinderBody, cuts []trimmed.CylinderCut, spans []float64, blind []bool) (*boreBody, bool) {
	if len(tools) < 3 || len(tools) > 2+trimmed.MaxCrossedDisjointBores {
		return nil, false
	}
	for _, tool := range tools {
		if !cardinalFrame(tool.frame) {
			return nil, false
		}
	}
	for i, a := range tools {
		for j := i + 1; j < len(tools); j++ {
			b := tools[j]
			if a.frame.ZAxis().Dot(b.frame.ZAxis()) != 0 {
				continue
			}
			// A crossing with a blind bore had companions only where its
			// radii were unequal, and such a pair is cut by the general
			// Boolean alone (specs/general-boolean-quartic).
			if blind[i] || blind[j] {
				continue
			}
			var recipe crossedCompanionGeometry
			p, ok := crossedBoreRecipe(base, []*cylinderBody{a, b}, []bool{false, false})
			if !ok || p.skew() {
				continue
			}
			recipe.crossedBoreGeometry = p
			for k, tool := range tools {
				if k == i || k == j {
					continue
				}
				site, ok := crossedCompanionSite(p.frame, tool)
				if !ok || blind[k] {
					break
				}
				recipe.additional = append(recipe.additional, site)
			}
			if len(recipe.additional) != len(tools)-2 {
				continue
			}
			floors := make([]int, len(tools))
			result, err := trimmed.BoxCrossedBoresWithDisjoint(context.Background(), p.frame, p.dimensions, p.crossing, p.radii[0], recipe.additional, 1e-9)
			if err == nil {
				return &boreBody{solid: result.Solid, base: base, cylinders: tools, cuts: cuts, spans: spans, blind: blind, floors: floors,
					crossed: &result, crossedCompanions: &recipe}, true
			}
		}
	}
	return nil, false
}

func crossedCompanionSite(frame geom.Frame, tool *cylinderBody) (trimmed.CardinalThroughBore, bool) {
	center := coordinates(frame.ToLocal(tool.frame.Origin()))
	for axis, direction := range []geom.Vec3{frame.XAxis(), frame.YAxis(), frame.ZAxis()} {
		dot := direction.Dot(tool.frame.ZAxis())
		if dot != 1 && dot != -1 {
			continue
		}
		origin := geom.Vec3{X: center[0], Y: center[1], Z: center[2]}
		if frame.ToWorld(origin) != tool.frame.Origin() {
			return trimmed.CardinalThroughBore{}, false
		}
		center[axis] = 0
		return trimmed.CardinalThroughBore{Axis: axis, Center: geom.Vec3{X: center[0], Y: center[1], Z: center[2]}, Radius: tool.radius}, true
	}
	return trimmed.CardinalThroughBore{}, false
}

func crossedCompanionsFromSTEP(p step.CrossedBoreBody, unit float64) (*boreBody, error) {
	g := crossedBoreGeometry{frame: p.Frame, dimensions: p.Dimensions, crossing: p.Crossing, radii: [2]float64{p.Radius, p.Radius}}
	return crossedBoreToolsFromSTEP(g, [2]float64{}, [2]bool{}, p.Additional, unit)
}
