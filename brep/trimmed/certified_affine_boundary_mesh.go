package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedAffineBoundaryMesh is an exact conforming triangulation of a closed
// boundary with affine spline supports and straight polygon trims, including
// holes. OriginalFace maps each triangle to its original trimmed face.
// Every squared error is exactly zero, in both directions. This guarantee is
// established by exact chart/edge/embedding proof before planar triangulation;
// it is not inferred from a sampled deflection check.
type CertifiedAffineBoundaryMesh struct {
	Mesh           mesh.Mesh
	OriginalFace   []int
	SquaredErrors  []*big.Rat
	BothDirections bool
}

// TessellateAffineBoundaryCertified proves affine supports, exact zero-error straight trim
// agreement and embedding, then triangulates the same represented polygons.
// It rejects nonaffine/curved trims and any geometry needing silent rounding.
// maxTriangles defaults to1M, maximum8M. Proof options retain existing per-stage
// bounds; failure returns no partial mesh. Existing tessellation is unchanged.
func (s Solid) TessellateAffineBoundaryCertified(ctx context.Context, o AffineSplineShellBooleanOptions, maxTriangles int) (CertifiedAffineBoundaryMesh, error) {
	empty := CertifiedAffineBoundaryMesh{}
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	if maxTriangles == 0 {
		maxTriangles = 1000000
	}
	if maxTriangles < 1 || maxTriangles > 8000000 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	// Polygon triangulation without Steiner points has at most n+2h−2
	// triangles per face. Charge the conservative count before conversion.
	total := 0
	for _, f := range s.faces {
		n := len(f.Outer) + 2*len(f.Holes) - 2
		for _, h := range f.Holes {
			n += len(h)
		}
		if n < 1 || n > maxTriangles-total {
			return empty, geom.ErrConvergence
		}
		total += n
	}
	agreement, e := s.ValidateEdgeAgreement(ctx, o.Edges)
	if e != nil {
		return empty, e
	}
	if !agreement.Complete {
		return empty, geom.ErrConvergence
	}
	for _, use := range agreement.Uses {
		if use.Report.SquaredUpperBound == nil || use.Report.SquaredUpperBound.Sign() != 0 {
			return empty, geom.ErrConvergence
		}
	}
	planar, e := PlanarBoundaryFromAffineSplines(ctx, s, o)
	if e != nil {
		return empty, e
	}
	m := planar.Tessellate()
	if len(m.Triangles) > maxTriangles {
		return empty, geom.ErrConvergence
	}
	faces := planar.TriangleFaces()
	if len(faces) != len(m.Triangles) {
		return empty, geom.ErrConvergence
	}
	if e := m.ValidateSolid(); e != nil {
		return empty, e
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	result := CertifiedAffineBoundaryMesh{Mesh: m, OriginalFace: faces, BothDirections: true, SquaredErrors: make([]*big.Rat, len(faces))}
	for i := range result.SquaredErrors {
		result.SquaredErrors[i] = new(big.Rat)
	}
	return result, nil
}
