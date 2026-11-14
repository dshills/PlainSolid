package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// Called only with a freshly validated placement.Rigid matrix and a bounded
// certified mesh. No output shares mutable vertex or index storage with local.
func placeCertifiedMeshExactly(ctx context.Context, local mesh.Mesh, matrix geom.Affine) (mesh.Mesh, error) {
	var e error
	world := mesh.Mesh{Vertices: make([]geom.Vec3, len(local.Vertices)), Triangles: append([][3]int(nil), local.Triangles...)}
	for i, p := range local.Vertices {
		if e := ctx.Err(); e != nil {
			return mesh.Mesh{}, e
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
					return mesh.Mesh{}, e
				}
				return mesh.Mesh{}, geom.ErrConvergence
			}
		}
		world.Vertices[i] = geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
	}
	return world, nil
}
