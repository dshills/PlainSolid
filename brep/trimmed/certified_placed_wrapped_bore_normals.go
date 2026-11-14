package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedPlacedWrappedBoreNormals separates the original canonical source
// certificate from its world-space display. Local vertex/triangle/source-owner
// indices also index Mesh. Placement is the exact interpretation of the source;
// no rounded replacement control net is constructed.
type CertifiedPlacedWrappedBoreNormals struct {
	Local     CertifiedWrappedBoreNormals
	Mesh      mesh.Mesh
	Placement geom.Affine
}

// TessellatePlacedWrappedCircularBoreNormalsCertified applies an immutable proper
// signed-coordinate rigid placement after independently certifying the canonical
// bore. Every resulting display coordinate must be exactly representable. A
// translation that would round even one vertex is rejected atomically, including
// tiny translations lost at large coordinates. Rotation, translation and source
// evaluation never use a tolerance-based equality.
//
// A determinant-one isometry preserves the Local angular and two-sided distance
// certificates, exact signed display volume, manifold incidence and absence of
// non-incidence contacts. Local remains in canonical coordinates; Mesh is world
// space. Additional work is linear in the existing bounded vertex count, with
// three exact represented-coordinate checks per vertex and cancellation checks.
func TessellatePlacedWrappedCircularBoreNormalsCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, pose placement.Rigid, angle float64, o CertifiedWrappedFrustumMeshOptions) (out CertifiedPlacedWrappedBoreNormals, err error) {
	if ctx == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	matrix, e := pose.Matrix()
	if e != nil {
		return out, e
	}
	local, e := TessellateWrappedCircularBoreNormalsCertified(ctx, outer, inner, angle, o)
	if e != nil {
		return out, e
	}
	world := mesh.Mesh{Vertices: make([]geom.Vec3, len(local.Mesh.Vertices)), Triangles: append([][3]int(nil), local.Mesh.Triangles...)}
	for i, p := range local.Mesh.Vertices {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		in := [3]float64{p.X, p.Y, p.Z}
		var xyz [3]float64
		for row := 0; row < 3; row++ {
			q := ratInset(matrix[row][3])
			for column := 0; column < 3; column++ {
				if matrix[row][column] != 0 {
					q.Add(q, new(big.Rat).Mul(ratInset(in[column]), ratInset(matrix[row][column])))
				}
			}
			xyz[row], e = geom.RepresentedExactScalar(q)
			if e != nil || !finite(xyz[row]) {
				if e != nil {
					return out, e
				}
				return out, geom.ErrConvergence
			}
		}
		world.Vertices[i] = geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
	}
	return CertifiedPlacedWrappedBoreNormals{Local: local, Mesh: world, Placement: matrix}, nil
}
