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
	if a.analytic != nil && a.analytic.pockets != nil && len(tools) == 1 {
		tool, err := b.get(tools[0])
		if err != nil {
			return Shape{}, err
		}
		p, err := a.analytic.pockets.throughBore(tool.cylinder)
		if err != nil {
			return Shape{}, err
		}
		return b.putPocketStock(p)
	}
	if a.analytic != nil {
		return Shape{}, &failureError{"UNSUPPORTED", "cutMany does not support this analytic stock"}
	}
	inputs := make([]stored, len(tools))
	planar, circular := true, true
	for i, id := range tools {
		inputs[i], err = b.get(id)
		if err != nil {
			return Shape{}, err
		}
		c := inputs[i]
		if c.analytic != nil {
			return Shape{}, &failureError{"UNSUPPORTED", "cutMany does not support this analytic tool"}
		}
		planar = planar && c.planar()
		circular = circular && c.cylinder != nil
	}
	if a.treated != nil {
		return b.cutManyTreated(a, inputs)
	}
	if planar && a.planar() {
		solid := a.solid
		for _, tool := range inputs {
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
	if !circular || a.cylinder != nil {
		return Shape{}, &failureError{"UNSUPPORTED", "cutMany requires planar tools on planar stock or cylindrical tools on qualified stock"}
	}
	base := a.solid
	var cylinders []*cylinderBody
	if a.bore != nil {
		base = a.bore.base
		cylinders = append(cylinders, a.bore.cylinders...)
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
