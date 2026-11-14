package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletBandRimSegment owns an indexed displayed chord on an original model
// edge. Parameters use that edge's increasing original range, not loop order.
type FilletBandRimSegment struct {
	Edge       int
	Vertices   [2]int
	Parameters [2]*big.Rat
}

// CertifiedFilletBandMesh certifies the entire original toroidal band in both
// directions, including represented rounding. All triangles have SourceFace.
// Rims[0] is the original flat cap rim, Rims[1] the retained cone rim.
// The U seam shares vertex IDs using the exact periodic source certificate.
// This open annular patch is not a closed-shell or global embedding proof.
type CertifiedFilletBandMesh struct {
	CertifiedKnotPatchMesh
	SourceFace   int
	Rims         [2][]FilletBandRimSegment
	SeamVertices []int
}

// TessellateFilletBandCertified uses immutable original construction proof,
// never caller-authored face IDs. Knot-cell residual bounds prove a two-sided
// correspondence for every triangle. Limits retain CertifiedPatchMeshOptions
// semantics and every failure returns no partial mesh or ancestry.
func (q FilletedFrustumMaterial) TessellateFilletBandCertified(ctx context.Context, end int, o CertifiedPatchMeshOptions) (CertifiedFilletBandMesh, error) {
	fail := func(e error) (CertifiedFilletBandMesh, error) { return CertifiedFilletBandMesh{}, e }
	if ctx == nil || q.slope == nil || end < 0 || end > 1 {
		return fail(geom.ErrInvalid)
	}
	id := q.fillets.Blends[end]
	face := q.solid.faces[id]
	w, ok := face.Surface.(*geom.WrappedSurfacePatch)
	if !ok {
		return fail(geom.ErrConvergence)
	}
	grid, e := TessellateWrappedKnotPatchCertified(ctx, w, o)
	if e != nil {
		return fail(e)
	}
	if !grid.BothDirections {
		return fail(geom.ErrConvergence)
	}
	nu, nv := len(grid.UParameters)-1, len(grid.VParameters)-1
	if nu < 4 || nv < 1 || len(grid.Mesh.Vertices) != nu*(nv+1) {
		return fail(geom.ErrConvergence)
	}
	out := CertifiedFilletBandMesh{CertifiedKnotPatchMesh: grid, SourceFace: id}
	for role := 0; role < 2; role++ {
		j := 0
		if role == 1 {
			j = nv
		}
		if end == 1 {
			j = nv - j
		}
		for i := 0; i < nu; i++ {
			a, b := grid.UParameters[i], grid.UParameters[i+1]
			quadrant := int(a)
			if quadrant < 0 || quadrant > 3 || b > float64(quadrant+1) {
				return fail(geom.ErrConvergence)
			}
			edge := q.fillets.Rims[end][role][quadrant]
			source := q.solid.edges[edge]
			if a < source.Range.Min || b > source.Range.Max {
				return fail(geom.ErrConvergence)
			}
			out.Rims[role] = append(out.Rims[role], FilletBandRimSegment{Edge: edge, Vertices: [2]int{i*(nv+1) + j, ((i+1)%nu)*(nv+1) + j}, Parameters: [2]*big.Rat{new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(b)}})
		}
	}
	for j := 0; j <= nv; j++ {
		out.SeamVertices = append(out.SeamVertices, j)
	}
	if face.Reversed {
		for i := range out.Mesh.Triangles {
			out.Mesh.Triangles[i][1], out.Mesh.Triangles[i][2] = out.Mesh.Triangles[i][2], out.Mesh.Triangles[i][1]
			out.UVTriangles[i][1], out.UVTriangles[i][2] = out.UVTriangles[i][2], out.UVTriangles[i][1]
		}
	}
	return out, nil
}
