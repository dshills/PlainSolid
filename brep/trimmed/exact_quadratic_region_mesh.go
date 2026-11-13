package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// ExactQuadraticRegionMeshOptions applies the existing global face, boundary,
// topology and span budgets. Composition bounds exact model knot partitioning
// independently for each native trim. No SplineUV reconstruction is performed.
type ExactQuadraticRegionMeshOptions struct {
	Face         MultiSpanCurvedTrimMeshOptions
	Composition  geom.SurfaceCurveCompositionOptions
	MaxInputBits int
}

// CertifiedExactQuadraticRegionMesh retains native exact source controls.
// LoopTrims contains UV-space polyline certificates on the identity chart,
// outer then holes normalized CCW. Boundary.Region.OriginalSource maps indices
// back to input; holes have reversed traversal. Exact model vertices and final
// error bounds reside in the embedded face mesh. Source area enclosures remain
// conservative; neither chord-fill equality nor source embedding is asserted.
type CertifiedExactQuadraticRegionMesh struct {
	CertifiedCurvedQuadraticTrimMesh
	LoopTrims [][]CertifiedExactQuadraticTrimPolyline
}

// TessellateExactQuadraticRegionCertified constructs a bounded certified image
// mesh of a CCW native outer loop and optional separated CW native holes.
// Fixed cuts use each input trim's original directed normalized parameter.
// Original homogeneous controls drive exact topology, knot composition and UV
// chord bounds directly. Joint source/polygon winding and boundary homotopy
// prove the fill correspondence; chart Lipschitz and exact triangle residuals
// transport it to model space including rounding. All errors are atomic.
func TessellateExactQuadraticRegionCertified(ctx context.Context, p *geom.SurfacePatch, outer []ExactQuadraticUV, holes [][]ExactQuadraticUV, cuts [][][]*big.Rat, o ExactQuadraticRegionMeshOptions) (CertifiedExactQuadraticRegionMesh, error) {
	empty := CertifiedExactQuadraticRegionMesh{}
	if ctx == nil || p == nil || len(holes) > 64 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	inputs := append([][]ExactQuadraticUV{outer}, holes...)
	// Empty SplineUV slices carry only loop lengths into the common algorithm;
	// every control/topology/composition branch reads native controls instead.
	shapes := make([][]SplineUV, len(inputs))
	count := 0
	for i, loop := range inputs {
		if len(loop) < 2 || len(loop) > brep.MaxHoledFaceVertices-count {
			return empty, geom.ErrInvalid
		}
		count += len(loop)
		shapes[i] = make([]SplineUV, len(loop))
		for _, trim := range loop {
			if trim.control[0][0] == nil {
				return empty, geom.ErrInvalid
			}
		}
	}
	result, e := tessellateControlledCutRegion(ctx, p, shapes[0], shapes[1:], inputs, cuts, RationalCutFaceMeshOptions{Face: o.Face, MaxInputBits: o.MaxInputBits}, o.Composition, false)
	if e != nil {
		return empty, e
	}
	return CertifiedExactQuadraticRegionMesh{CertifiedCurvedQuadraticTrimMesh: result.CertifiedCurvedQuadraticTrimMesh, LoopTrims: result.nativeLoopTrims}, nil
}
