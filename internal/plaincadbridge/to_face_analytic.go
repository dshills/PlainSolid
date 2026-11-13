package plaincadbridge

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

var errToFaceCoverage = errors.New("brep: termination face does not cover the projected profile")

func (b *Bridge) toAnalyticFace(target stored, q request) (Shape, error) {
	if target.analytic == nil {
		return Shape{}, &failureError{"UNSUPPORTED", "to-face termination requires a qualified bounded target cap"}
	}
	native := target.analytic.solid
	face, err := target.analytic.planarFace(q.Origin.vec(), q.Normal.vec())
	if err != nil {
		return Shape{}, fmt.Errorf("to-face reference was lost or is ambiguous: %w", err)
	}
	frame, outer, holes, err := native.PlanarStraightFaceWithCircularHoles(face.Face)
	if err != nil {
		return Shape{}, &failureError{"UNSUPPORTED", "to-face termination requires a straight cap boundary with complete circular holes"}
	}
	direction := q.Direction.vec()
	unit, err := direction.Unit()
	axis := frame.ZAxis()
	if err != nil || unit != axis && unit != axis.Scale(-1) {
		return Shape{}, &failureError{"UNSUPPORTED", "analytic target cap requires a perpendicular sweep"}
	}
	// Prove the exact outer boundary with the existing planar algorithm.
	// The planar extrusion is a proof carrier for that one finite support.
	carrier, err := brep.Prism(outer, axis)
	if err != nil {
		return Shape{}, err
	}
	ctx := context.Background()
	if q.Op == "circleToFace" {
		if q.Center == nil || len(q.Holes) != 0 {
			return Shape{}, errors.New("circular to-face extrusion requires one circle")
		}
		center, radius := q.Center.vec(), float64(q.Radius)
		_, proof, err := trimmed.CylinderToFace(ctx, center, direction, radius, carrier, 0, geom.DiskPolygonOptions{})
		if err != nil {
			return Shape{}, fmt.Errorf("extrude to face failed: %w", err)
		}
		p := toFaceExactLocal(frame, center)
		for _, hole := range holes {
			if !toFaceDisjointDisks(p, hole.Center, radius, hole.Radius) {
				return Shape{}, errToFaceCoverage
			}
		}
		return b.circle(center, direction, radius, proof.Height)
	}
	source, sourceHoles := loop(q.Outer), make([][]geom.Vec3, len(q.Holes))
	count := len(source)
	for i, h := range q.Holes {
		count += len(h)
		sourceHoles[i] = loop(h)
	}
	if count > MaxProfileVertices {
		return Shape{}, &failureError{"LIMIT", "profile exceeds 128 vertices"}
	}
	result, err := brep.PrismToFace(ctx, source, sourceHoles, direction, carrier, 0)
	if err != nil {
		return Shape{}, fmt.Errorf("extrude to face failed: %w", err)
	}
	project := func(points []geom.Vec3) ([]geom.Vec3, error) {
		out := make([]geom.Vec3, len(points))
		for i, p := range points {
			local := toFaceExactLocal(frame, p)
			// The recognized frame is cardinal. Reconstruct in rational arithmetic
			// and reject a projected coordinate that cannot be stored losslessly.
			for j := 0; j < 3; j++ {
				v := new(big.Rat).SetFloat64(toFaceComponent(frame.Origin(), j))
				v.Add(v, new(big.Rat).Mul(local[0], new(big.Rat).SetFloat64(toFaceComponent(frame.XAxis(), j))))
				v.Add(v, new(big.Rat).Mul(local[1], new(big.Rat).SetFloat64(toFaceComponent(frame.YAxis(), j))))
				f, _ := v.Float64()
				back := new(big.Rat).SetFloat64(f)
				if back == nil || back.Cmp(v) != 0 {
					return nil, geom.ErrDomain
				}
				switch j {
				case 0:
					out[i].X = f
				case 1:
					out[i].Y = f
				case 2:
					out[i].Z = f
				}
			}
		}
		return out, nil
	}
	projected, err := project(source)
	if err != nil {
		return Shape{}, err
	}
	projectedHoles := make([][]geom.Vec3, len(sourceHoles))
	for i, h := range sourceHoles {
		projectedHoles[i], err = project(h)
		if err != nil {
			return Shape{}, err
		}
	}
	for _, h := range holes {
		center := frame.ToWorld(geom.Vec3{X: h.Center.X, Y: h.Center.Y})
		back := toFaceExactLocal(frame, center)
		if back[0].Cmp(new(big.Rat).SetFloat64(h.Center.X)) != 0 || back[1].Cmp(new(big.Rat).SetFloat64(h.Center.Y)) != 0 {
			return Shape{}, geom.ErrDomain
		}
		disk, err := geom.NewPlanarCircularProfile(center, axis, h.Radius)
		if err != nil {
			return Shape{}, err
		}
		coverage, err := geom.ClassifyDiskPlanarPolygon(ctx, disk, projected, projectedHoles, geom.DiskPolygonOptions{})
		if err != nil {
			return Shape{}, err
		}
		if !coverage.Complete || coverage.Relation != geom.DiskPolygonOutside {
			return Shape{}, errToFaceCoverage
		}
	}
	return b.put(result)
}

// Inputs are finite validated native geometry; retain exact represented differences.
func toFaceComponent(v geom.Vec3, i int) float64 {
	if i == 0 {
		return v.X
	}
	if i == 1 {
		return v.Y
	}
	return v.Z
}
func toFaceExactLocal(f geom.Frame, p geom.Vec3) [2]*big.Rat {
	var out [2]*big.Rat
	for i, axis := range []geom.Vec3{f.XAxis(), f.YAxis()} {
		out[i] = new(big.Rat)
		for j := 0; j < 3; j++ {
			d := new(big.Rat).Sub(new(big.Rat).SetFloat64(toFaceComponent(p, j)), new(big.Rat).SetFloat64(toFaceComponent(f.Origin(), j)))
			out[i].Add(out[i], new(big.Rat).Mul(d, new(big.Rat).SetFloat64(toFaceComponent(axis, j))))
		}
	}
	return out
}
func toFaceDisjointDisks(p [2]*big.Rat, c geom.Vec2, a, b float64) bool {
	d := new(big.Rat)
	for i, x := range []float64{c.X, c.Y} {
		q := new(big.Rat).Sub(p[i], new(big.Rat).SetFloat64(x))
		d.Add(d, new(big.Rat).Mul(q, q))
	}
	r := new(big.Rat).Add(new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(b))
	return d.Cmp(new(big.Rat).Mul(r, r)) > 0
}
