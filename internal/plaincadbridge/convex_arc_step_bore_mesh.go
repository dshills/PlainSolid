package plaincadbridge

import (
	"context"
	"math/big"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/mesh"
)

func tessellateStepLoopThroughHole(ctx context.Context, outer []geom.Vec3, hole trimmed.ExtrusionCircle, settings stepLoopMesh, levels [2]float64, maxTriangles int) (mesh.Mesh, error) {
	if !finite(hole.Radius) || hole.Radius <= 0 {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	center := rationalXY(hole.Center)
	radius := new(big.Rat).SetFloat64(hole.Radius)
	// The complete analytic bore and its rounding envelope must lie strictly
	// inside every chord of the rounded exterior boundary. This keeps the
	// exterior sagitta and bore sagitta bands disjoint on both cap planes.
	protect := new(big.Rat).Add(radius, settings.deflection)
	protect.Mul(protect, protect)
	for i, p := range outer {
		a := rationalXY(geom.Vec2{X: p.X, Y: p.Y})
		q := outer[(i+1)%len(outer)]
		edge := rationalXY(geom.Vec2{X: q.X, Y: q.Y}).sub(a)
		distance := edge.cross(center.sub(a))
		if distance.Sign() <= 0 || distance.Mul(distance, distance).Cmp(new(big.Rat).Mul(protect, edge.dot(edge))) <= 0 {
			return mesh.Mesh{}, parametric.ErrBudget
		}
	}
	settings.ring = nil
	settings.maxPoints = min(2048, maxTriangles/4) - len(outer)
	if settings.maxPoints < 4 {
		return mesh.Mesh{}, parametric.ErrBudget
	}
	zero, negative := new(big.Rat), new(big.Rat).Neg(radius)
	quarters := [4]rationalPoint2{{radius, zero}, {zero, radius}, {negative, zero}, {zero, negative}}
	for i, a := range quarters {
		if err := ctx.Err(); err != nil {
			return mesh.Mesh{}, err
		}
		err := settings.arc(center, a, quarters[(i+1)%len(quarters)], radius)
		if err != nil {
			return mesh.Mesh{}, err
		}
	}
	inner := make([]geom.Vec3, len(settings.ring))
	for i, q := range settings.ring {
		// Exterior is CCW. The bore needs the opposite winding to pair
		// directed cap edges with inward cylindrical strips.
		inner[len(inner)-1-i] = geom.Vec3{X: q.X, Y: q.Y}
	}
	return extrudeStepAnnulusMesh(ctx, outer, inner, levels, maxTriangles)
}

func extrudeStepAnnulusMesh(ctx context.Context, outer, inner []geom.Vec3, levels [2]float64, maxTriangles int) (mesh.Mesh, error) {
	n := len(outer) + len(inner)
	if n > maxTriangles/4 {
		return mesh.Mesh{}, parametric.ErrBudget
	}
	caps, err := brep.TriangulatePlanarRings(ctx, outer, [][]geom.Vec3{inner}, brep.PlanarTriangulationOptions{MaxVertices: 2048})
	if err != nil {
		return mesh.Mesh{}, err
	}
	if len(caps) != n {
		return mesh.Mesh{}, geom.ErrDomain
	}
	out := mesh.Mesh{Vertices: make([]geom.Vec3, 0, 2*n), Triangles: make([][3]int, 0, 4*n)}
	for _, z := range levels {
		for _, ring := range [][]geom.Vec3{outer, inner} {
			for _, q := range ring {
				q.Z = z
				out.Vertices = append(out.Vertices, q)
			}
		}
	}
	for _, t := range caps {
		out.Triangles = append(out.Triangles, [3]int{t[0], t[2], t[1]}, [3]int{n + t[0], n + t[1], n + t[2]})
	}
	offset := 0
	for _, ring := range [][]geom.Vec3{outer, inner} {
		for i := range ring {
			a, b := offset+i, offset+(i+1)%len(ring)
			out.Triangles = append(out.Triangles, [3]int{a, b, n + b}, [3]int{a, n + b, n + a})
		}
		offset += len(ring)
	}
	if err = out.ValidateSolid(); err != nil {
		return mesh.Mesh{}, err
	}
	return out, nil
}
