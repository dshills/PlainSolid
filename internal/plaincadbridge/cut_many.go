package plaincadbridge

import (
	"context"
	"errors"
	"math"

	"plainsolid/csg/poly"
)

// cutMany publishes one result after the complete cutter set has succeeded.
// A redundant member is allowed; the set must collectively remove material.
func (b *Bridge) cutMany(target string, tools []string) (Shape, error) {
	if len(tools) == 0 {
		return Shape{}, errors.New("cutMany requires at least one tool")
	}
	if len(tools) > 32 {
		return Shape{}, &failureError{"LIMIT", "cutMany accepts at most 32 tools"}
	}
	a, err := b.get(target)
	if err != nil {
		return Shape{}, err
	}
	if a.tangent != nil {
		return Shape{}, unsupportedEdge("Boolean operations on side-tangent blind bores are outside the supported domain")
	}
	if len(tools) == 1 {
		c, e := b.get(tools[0])
		if e != nil {
			return Shape{}, e
		}
		if c.tangent != nil {
			return Shape{}, unsupportedEdge("Boolean operations on side-tangent blind bores are outside the supported domain")
		}
		if body, ok := tangentBlindBoreRecipe(a, c); ok {
			return b.putTangentBlindBore(body)
		}
		if a.planar() && c.cylinder != nil {
			if body, handled, err := b.cornerCylinderBoolean(a, c, "cut"); handled {
				return body, err
			}
		}
		if body, removed, handled, e := retainedPlanarBoolean(a, c, "cut"); handled {
			if e != nil {
				return Shape{}, e
			}
			if removed <= 0 {
				return Shape{}, unsupportedEdge("Boolean cut removed no volume")
			}
			return b.putAnalyticBody(body)
		}
	}

	if a.analytic != nil && a.analytic.pockets != nil && len(tools) == 1 {
		tool, err := b.get(tools[0])
		if err != nil {
			return Shape{}, err
		}
		if body, removed, handled, err := cutPocketCylinder(a, tool.cylinder); handled {
			if err != nil {
				return Shape{}, err
			}
			if removed <= 0 {
				return Shape{}, unsupportedEdge("Boolean cut removed no volume")
			}
			return b.putAnalyticBody(body)
		}
	}
	inputs := make([]stored, len(tools))
	planar, circular := true, true
	for i, id := range tools {
		inputs[i], err = b.get(id)
		if err != nil {
			return Shape{}, err
		}
		c := inputs[i]
		if c.tangent != nil {
			return Shape{}, unsupportedEdge("Boolean operations on side-tangent blind bores are outside the supported domain")
		}
		planar = planar && c.planar()
		circular = circular && c.cylinder != nil
	}
	if circular {
		if result, handled, err := b.cutSideNotchBores(a, inputs); handled {
			return result, err
		}
		if result, handled, err := b.cutCircularStockBores(a, inputs); handled {
			return result, err
		}
	}
	if result, _, handled, err := cutRoundedBores(a, inputs); handled {
		if err != nil {
			return Shape{}, err
		}
		return b.putAnalyticBody(result)
	}
	if result, _, handled, err := cutThroughProfiles(a, inputs); handled {
		if err != nil {
			return Shape{}, err
		}
		return b.putAnalyticBody(result)
	}
	if a.treated != nil {
		return b.cutManyTreated(a, inputs)
	}
	if planar {
		solid, supported, err := a.planarCutBoundary()
		if err != nil {
			return Shape{}, err
		}
		if !supported {
			return Shape{}, &failureError{"UNSUPPORTED", "cutMany does not support this stock with planar tools"}
		}
		for _, tool := range inputs {
			if pocket, ok := openPlanarPocket(solid, tool.solid); ok {
				solid = pocket.solid
				continue
			}
			solid, err = poly.Combine(context.Background(), solid, tool.solid, poly.Difference, poly.Options{})
			if err != nil {
				return Shape{}, err
			}
		}
		p, err := solid.MassProperties()
		if err != nil {
			return Shape{}, err
		}
		if p.Volume <= 0 || !removesMaterial(a.result.GeometryAssertions.Volume, p.Volume) {
			return Shape{}, errors.New("cutMany must retain solid material and collectively remove volume")
		}
		return b.put(solid)
	}
	if !circular || a.cylinder != nil || a.analytic != nil {
		return Shape{}, &failureError{"UNSUPPORTED", "cutMany requires planar tools on planar stock or cylindrical tools on qualified stock"}
	}
	base := a.solid
	var cylinders []*cylinderBody
	if a.bore != nil {
		base = a.bore.base
		cylinders = append(cylinders, a.bore.sourceCylinders()...)
	}
	for _, tool := range inputs {
		cylinders = append(cylinders, tool.cylinder)
	}
	body, err := newBore(base, cylinders)
	if err != nil {
		return Shape{}, err
	}
	volume, _, err := body.measures()
	if err != nil {
		return Shape{}, err
	}
	if volume <= 0 || !removesMaterial(a.result.GeometryAssertions.Volume, volume) {
		return Shape{}, errors.New("cutMany must retain solid material and collectively remove volume")
	}
	return b.putBore(body)
}

// Match the application cut contract, so arithmetic roundoff in an idempotent
// union cannot publish a spurious material-removal result.
func removesMaterial(before, after float64) bool {
	return before-after > math.Max(1e-9, math.Abs(before)*1e-10)
}
