package plaincadbridge

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	rigidplacement "plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

func (p rationalStepArcLoop) tessellate(ctx context.Context, options parametric.TessellationOptions) (mesh.Mesh, error) {
	localOptions := options
	localOptions.Deflection /= 2
	localOptions.Angle /= 2
	if localOptions.Angle*2 > options.Angle {
		localOptions.Angle = math.Nextafter(localOptions.Angle, 0)
	}
	localOptions.MaxTriangles = min(options.MaxTriangles, collision.MaxTriangles)
	if localOptions.MaxTriangles == 0 {
		localOptions.MaxTriangles = collision.MaxTriangles
	}
	if localOptions.Deflection <= 0 || localOptions.Angle <= 0 {
		return mesh.Mesh{}, parametric.ErrBudget
	}
	local := stepArcLoopSupport{descriptor: p.descriptor, exactPlacement: true}
	source, qualified, err := local.tessellate(localOptions)
	if err != nil {
		return mesh.Mesh{}, err
	}
	if !qualified {
		return mesh.Mesh{}, unsupportedEdge("rational-world STEP display requires the proved local convex extrusion mesh")
	}
	limits := rigidplacement.MeshRoundingOptions{MaxTriangles: localOptions.MaxTriangles}
	world, err := p.pose.RoundMesh(ctx, source, limits)
	if err != nil {
		return mesh.Mesh{}, err
	}
	distance := new(big.Rat).Add(new(big.Rat).SetFloat64(localOptions.Deflection), world.RoundingUpper)
	if distance.Cmp(new(big.Rat).SetFloat64(options.Deflection)) > 0 {
		return mesh.Mesh{}, parametric.ErrBudget
	}
	if err = verifyRationalStepArcFacetNormals(ctx, p.pose, source, world.Mesh, localOptions.Angle); err != nil {
		return mesh.Mesh{}, err
	}
	// Re-prove embedding and shell orientation on the represented world mesh.
	verified, err := rigidplacement.PrepareClosedDisplay(ctx, world.Mesh, nil, rigidplacement.ClosedDisplayOptions{Rounding: limits})
	if err != nil {
		return mesh.Mesh{}, err
	}
	shells := verified.Shells()
	if len(shells) != 1 || shells[0].Depth != 0 {
		return mesh.Mesh{}, geom.ErrDomain
	}
	return world.Mesh, nil
}

type rationalStepArcVector [3]*big.Rat

func rationalStepArcVectorFrom(v geom.Vec3) rationalStepArcVector {
	return rationalStepArcVector{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
}

func (a rationalStepArcVector) sub(b rationalStepArcVector) (q rationalStepArcVector) {
	for i := range q {
		q[i] = new(big.Rat).Sub(a[i], b[i])
	}
	return q
}

func (a rationalStepArcVector) cross(b rationalStepArcVector) (q rationalStepArcVector) {
	for i := range q {
		j, k := (i+1)%3, (i+2)%3
		q[i] = new(big.Rat).Sub(new(big.Rat).Mul(a[j], b[k]), new(big.Rat).Mul(a[k], b[j]))
	}
	return q
}

func (a rationalStepArcVector) dot(b rationalStepArcVector) *big.Rat {
	q := new(big.Rat)
	for i := range a {
		q.Add(q, new(big.Rat).Mul(a[i], b[i]))
	}
	return q
}

// RoundMesh retains indices. Compare each rounded facet to the exact image
// of its local normal. A positive dot and rational tangent bound verify
// orientation and the part of the angular allowance spent in world rounding.
func verifyRationalStepArcFacetNormals(ctx context.Context, pose rigidplacement.RationalRigid, local, world mesh.Mesh, angle float64) error {
	angleSquared := new(big.Rat).SetFloat64(angle)
	angleSquared.Mul(angleSquared, angleSquared)
	normal := func(points []geom.Vec3, tri [3]int) rationalStepArcVector {
		a, b, c := rationalStepArcVectorFrom(points[tri[0]]), rationalStepArcVectorFrom(points[tri[1]]), rationalStepArcVectorFrom(points[tri[2]])
		return b.sub(a).cross(c.sub(a))
	}
	for _, tri := range local.Triangles {
		exact, err := pose.TransformVector(ctx, normal(local.Vertices, tri), false, 4096)
		if err != nil {
			return err
		}
		a, b := rationalStepArcVector(exact), normal(world.Vertices, tri)
		dot := a.dot(b)
		if dot.Sign() <= 0 {
			return geom.ErrDomain
		}
		cross := a.cross(b)
		bound := new(big.Rat).Mul(dot, dot)
		bound.Mul(bound, angleSquared)
		if cross.dot(cross).Cmp(bound) > 0 {
			return parametric.ErrBudget
		}
	}
	return nil
}

func (p rationalStepArcLoop) roundDisplayEdges(ctx context.Context, positions []float64) error {
	for i := 0; i < len(positions); i += 3 {
		local := rationalStepArcVectorFrom(vector(positions[i : i+3]).vec())
		world, err := p.pose.TransformPoint(ctx, local, false, 4096)
		if err != nil {
			return err
		}
		for axis, q := range world {
			v, _ := q.Float64()
			if !finite(v) {
				return geom.ErrUnrepresentable
			}
			positions[i+axis] = v
		}
	}
	return nil
}
