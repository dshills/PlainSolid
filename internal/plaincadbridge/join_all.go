package plaincadbridge

import (
	"context"
	"errors"
	"math"
	"math/big"

	"plainsolid/brep"
	"plainsolid/csg/poly"
)

func (b *Bridge) joinAll(ids []string, toolID string) (Shape, error) {
	if len(ids) < 2 || len(ids) > MaxHandles {
		return Shape{}, errors.New("joinAll requires at least two target bodies within the handle limit")
	}
	tool, err := b.get(toolID)
	if err != nil {
		return Shape{}, err
	}
	ctx := context.Background()
	if b.general {
		return b.generalJoinAll(ctx, ids, tool)
	}
	inputs := make([]stored, len(ids))
	for i, id := range ids {
		inputs[i], err = b.get(id)
		if err != nil {
			return Shape{}, err
		}
		if !inputs[i].planar() || (!tool.planar() && tool.cylinder == nil) {
			return Shape{}, &failureError{"UNSUPPORTED", "scoped join requires planar targets and a planar or containing circular tool"}
		}
	}

	disconnected := errors.New("Boolean join requires connected solids. Connect every selected target with the tool.")
	if tool.cylinder != nil {
		// Each target must be fully contained or provably separated. Partial
		// intersections require a curved Boolean and are outside this path.
		separated := false
		for _, target := range inputs {
			if boundsSeparated(target.result.Bounds, tool.result.Bounds) {
				separated = true
			} else if !cylinderContainsPlanar(tool.cylinder, target.solid) {
				return Shape{}, &failureError{"UNSUPPORTED", "scoped cylindrical join requires complete target containment"}
			}
		}
		if separated {
			return Shape{}, disconnected
		}
	}

	// Intermediate target unions may be disconnected. They never consume a
	// handle. Only the final connected result can be published.
	base := inputs[0].solid
	for _, target := range inputs[1:] {
		base, err = planarUnion(ctx, base, target.solid)
		if err != nil {
			return Shape{}, err
		}
	}
	before, err := base.MassProperties()
	if err != nil {
		return Shape{}, err
	}
	if tool.cylinder != nil {
		if !removesMaterial(tool.result.GeometryAssertions.Volume, before.Volume) {
			return Shape{}, errors.New("Boolean join added no volume beyond the selected target bodies.")
		}
		return b.publish(tool)
	}

	solid, err := planarUnion(ctx, base, tool.solid)
	if err != nil {
		return Shape{}, err
	}
	result, err := provePlanar(solid)
	if err != nil {
		return Shape{}, err
	}
	after := result.GeometryAssertions
	if after.SolidCount != 1 {
		return Shape{}, disconnected
	}
	if !removesMaterial(after.Volume, before.Volume) {
		return Shape{}, errors.New("Boolean join added no volume beyond the selected target bodies.")
	}
	tolerance := math.Max(1e-9, math.Abs(before.Volume)*1e-10)
	if after.Volume > before.Volume+tool.result.GeometryAssertions.Volume+tolerance {
		return Shape{}, errors.New("Boolean join produced an inconsistent solid volume.")
	}
	return b.publish(stored{solid: solid, result: result})
}

func planarUnion(ctx context.Context, a, b brep.Solid) (brep.Solid, error) {
	if box, ok := joinedRectangularStock(a, b); ok {
		return box, nil
	}
	return poly.Combine(ctx, a, b, poly.Union, poly.Options{})
}

// A cylinder is convex, so exact containment of all planar boundary vertices
// proves containment of the represented material. A signed cardinal frame
// allows exact squared radii and axial bounds without normalized products.
func cylinderContainsPlanar(c *cylinderBody, solid brep.Solid) bool {
	if !cardinalFrame(c.frame) {
		return false
	}
	origin, axis := coordinates(c.frame.Origin()), coordinates(c.frame.ZAxis())
	height := new(big.Rat).SetFloat64(c.height)
	radius := new(big.Rat).SetFloat64(c.radius)
	radiusSquared := new(big.Rat).Mul(radius, radius)
	for _, v := range solid.Vertices() {
		distanceSquared := new(big.Rat)
		for i, coordinate := range coordinates(v) {
			delta := new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate), new(big.Rat).SetFloat64(origin[i]))
			if axis[i] == 0 {
				distanceSquared.Add(distanceSquared, delta.Mul(delta, delta))
				continue
			}
			if axis[i] < 0 {
				delta.Neg(delta)
			}
			if delta.Sign() < 0 || delta.Cmp(height) > 0 {
				return false
			}
		}
		if distanceSquared.Cmp(radiusSquared) > 0 {
			return false
		}
	}
	return true
}
