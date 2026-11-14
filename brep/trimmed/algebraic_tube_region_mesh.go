package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

type AlgebraicTubeRegionMeshOptions struct {
	Footprint AlgebraicTubeFootprintMeshOptions
	Surface   MultiSpanTubeLensMeshOptions // MaxDegree>2 explicitly enables bounded higher-degree residuals.
}
type CertifiedAlgebraicTubeRegionMesh struct {
	Footprint                                                                       CertifiedAlgebraicTubeFootprint
	Mesh                                                                            mesh.Mesh
	Region                                                                          int
	OriginalChart                                                                   []int
	GlobalBoundaryVertices                                                          map[int]int
	FaceResidualSquared                                                             [2]*big.Rat
	FaceRefinementPasses                                                            [2]int
	SourceImageSquaredDistance, SignedDisplayVolume                                 *big.Rat
	Collision                                                                       collision.Report
	TriangleChecks, Predicates, SpanCellChecks, SpanPieceChecks, ResidualOperations int
}

// TessellateRegionClosedCertified fills one independently sealed material
// region, with its original outer and optional hole loops. The exact footprint
// proof transports source fill to the common represented polygon, then bounded
// rational chart residuals certify both graph images. Shared boundary displays
// and synchronized interior triangulation are preserved. Positive represented
// interior height, closed opposite incidence, manifold links, positive volume,
// and exact absence of display contacts are mandatory. Failures are atomic.
// Degree>2 requires explicit Surface.Span.MaxDegree (maximum6); this is a
// boundary-image distance certificate, not a certified material/volume error.
// Original float spline and exact homogeneous Bézier owner charts are retained
// without conversion of their control points.
func (r AlgebraicTubeRegions) TessellateRegionClosedCertified(ctx context.Context, region int, o AlgebraicTubeRegionMeshOptions) (CertifiedAlgebraicTubeRegionMesh, error) {
	var empty CertifiedAlgebraicTubeRegionMesh
	if ctx == nil || r.section.runSource() == nil || region < 0 || region >= len(r.regions) {
		return empty, geom.ErrInvalid
	}
	work, e := newTubeLensSpanWork(o.Surface)
	if e != nil {
		return empty, e
	}
	if o.Footprint.Boundary.Loop.Run.Display.Point.Tolerance == 0 {
		tol := o.Footprint.Boundary.Loop.Run.Display.Deflection * 1e-6
		if tol > 0 {
			o.Footprint.Boundary.Loop.Run.Display.Point.Tolerance = tol
		}
	}
	footprint, e := r.TessellateFootprintCertified(ctx, region, o.Footprint)
	if e != nil {
		return empty, e
	}
	fill := tubeRegionFillSeed{seed: CertifiedAffineQuadraticTrimMesh{Region: AffineQuadraticTrimRegionArea{FillSquaredDistance: new(big.Rat)}, Mesh: mesh.Mesh{Triangles: footprint.Mesh.Triangles}, BoundaryLoops: footprint.BoundaryLoops}, squared: footprint.SourceFillSquaredDistance, predicates: footprint.Predicates}
	for i, p := range footprint.Mesh.Vertices {
		fill.seed.ExactUV = append(fill.seed.ExactUV, [2]*big.Rat{ratInset(p.X), ratInset(p.Y)})
		fill.vertices = append(fill.vertices, footprint.Boundary.Vertices[footprint.GlobalBoundaryVertices[i]])
	}
	x := r.regions[region]
	view := AlgebraicTubeLens{section: r.section, faces: r.partitions[x.Outer].Faces, graphs: r.graphs, upper: x.UpperChart, branch: r.loops[x.Outer].Branch}
	options := o.Surface.Closed
	options.Boundary = o.Footprint.Boundary.Loop
	shell, e := view.tessellateClosedTubeLens(ctx, options, &work, o.Surface.AffineCharts, nil, &fill)
	if e != nil {
		return empty, e
	}
	result := CertifiedAlgebraicTubeRegionMesh{Footprint: footprint, Mesh: shell.Mesh, Region: region, OriginalChart: shell.OriginalFace, GlobalBoundaryVertices: map[int]int{}, FaceResidualSquared: shell.FaceResidualSquared, FaceRefinementPasses: shell.FaceRefinementPasses, SourceImageSquaredDistance: shell.SourceImageSquaredDistance, SignedDisplayVolume: shell.SignedDisplayVolume, Collision: shell.Collision, TriangleChecks: shell.TriangleChecks, Predicates: shell.Predicates, SpanCellChecks: work.cells, SpanPieceChecks: work.pieces, ResidualOperations: work.operations}
	for seed, index := range shell.BoundaryVertices {
		result.GlobalBoundaryVertices[footprint.GlobalBoundaryVertices[seed]] = index
	}
	return result, nil
}
