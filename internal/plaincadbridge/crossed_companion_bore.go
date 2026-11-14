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
	additional  []trimmed.CardinalThroughBore
	depth       [2]float64
	oppositeCap [2]bool
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
			var recipe crossedCompanionGeometry
			if blind[i] || blind[j] {
				p, ok := blindCrossedRecipe(base, []*cylinderBody{a, b}, []float64{spans[i], spans[j]}, []bool{blind[i], blind[j]})
				if !ok || p.radii[0] == p.radii[1] || p.partialSkew() {
					continue
				}
				recipe.crossedBoreGeometry, recipe.depth, recipe.oppositeCap = p.crossedBoreGeometry, p.depth, p.oppositeCap
			} else {
				p, ok := crossedBoreRecipe(base, []*cylinderBody{a, b}, []bool{false, false})
				if !ok || p.partialSkew() || p.radii[0] == p.radii[1] && p.offset != 0 {
					continue
				}
				recipe.crossedBoreGeometry = p
			}
			p := recipe.crossedBoreGeometry
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
			ctx := context.Background()
			var result trimmed.BoxCrossedBoresResult
			floors := make([]int, len(tools))
			var err error
			switch {
			case p.radii[0] == p.radii[1]:
				result, err = trimmed.BoxCrossedBoresWithDisjoint(ctx, p.frame, p.dimensions, p.crossing, p.radii[0], recipe.additional, 1e-9)
			case recipe.depth[0] != 0 || recipe.depth[1] != 0:
				var finite trimmed.FiniteSkewCrossedBoresResult
				finite, err = trimmed.BoxFiniteSkewCrossedBoresWithDisjoint(ctx, p.frame, p.dimensions, p.crossing, p.radii[0], p.radii[1], p.offset,
					recipe.depth[0], recipe.depth[1], recipe.oppositeCap[0], recipe.oppositeCap[1], recipe.additional, 1e-9)
				result = finite.BoxCrossedBoresResult
				for _, k := range []int{i, j} {
					if blind[k] {
						floors[k] = finite.ZFloorFace
						if tools[k].frame.ZAxis().Dot(p.frame.XAxis()) != 0 {
							floors[k] = finite.XFloorFace
						}
					}
				}
			default:
				result, err = trimmed.BoxSkewCrossedBoresWithDisjoint(ctx, p.frame, p.dimensions, p.crossing, p.radii[0], p.radii[1], p.offset, recipe.additional, 1e-9)
			}
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
