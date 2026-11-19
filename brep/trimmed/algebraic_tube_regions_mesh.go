//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

type AlgebraicTubeRegionsMeshOptions struct {
	MaxResidualOperations       int
	Boundary                    AlgebraicTubeRegionsBoundaryOptions
	Component                   AlgebraicTubeLensMeshOptions // Boundary is supplied by the global network; triangle limits are cumulative.
	Span                        MultiSpanTriangleBoundOptions
	MaxSpanCells, MaxSpanPieces int
	AffineCharts                bool
}
type CertifiedAlgebraicTubeRegionsMesh struct {
	ResidualOperations                                          int
	Boundary                                                    CertifiedAlgebraicTubeRegionsBoundary
	Mesh                                                        mesh.Mesh
	Components                                                  []CertifiedAlgebraicTubeLensMesh
	ComponentVertices                                           [][]int // Local component mesh -> common global mesh.
	OriginalRegion, OriginalChart                               []int
	SourceImageSquaredDistance, SignedDisplayVolume             *big.Rat
	Collision                                                   collision.Report
	TriangleChecks, Predicates, SpanCellChecks, SpanPieceChecks int
}

// TessellateDisjointClosedCertified fills every disjoint disk region from one
// freshly proved global boundary network. Shared seam vertices are assigned
// once before filling; components never regenerate or coordinate-weld them.
// Each face image and each closed component retain their existing certificate,
// followed by an exact contact check across the entire display collection.
// Piecewise rational supports use the explicitly bounded residual degree.
// Regions with holes or nested parents reject before meshing; this entry point
// does not promote a quartic annular section certificate to a fill certificate.
func (r AlgebraicTubeRegions) TessellateDisjointClosedCertified(ctx context.Context, o AlgebraicTubeRegionsMeshOptions) (CertifiedAlgebraicTubeRegionsMesh, error) {
	return r.tessellateDisjointClosedCertified(ctx, o, nil)
}

// prescribed is private, freshly certified against the same original source,
// and may differ only by the exact model isometry of its scratch graph view.
func (r AlgebraicTubeRegions) tessellateDisjointClosedCertified(ctx context.Context, o AlgebraicTubeRegionsMeshOptions, prescribed *CertifiedAlgebraicTubeRegionsBoundary) (CertifiedAlgebraicTubeRegionsMesh, error) {
	var empty CertifiedAlgebraicTubeRegionsMesh
	if ctx == nil || r.section.runSource() == nil || len(r.regions) == 0 || o.MaxResidualOperations < 0 || o.MaxResidualOperations > 1000000000 {
		return empty, geom.ErrInvalid
	}
	for _, x := range r.regions {
		if len(x.Holes) != 0 || r.loops[x.Outer].Parent != -1 {
			return empty, geom.ErrConvergence
		}
	}
	if o.Component.Interior.MaxTriangles == 0 {
		o.Component.Interior.MaxTriangles = 100000
	}
	if o.Component.Interior.MaxTriangleChecks == 0 {
		o.Component.Interior.MaxTriangleChecks = 100000
	}
	if o.Component.MaxPredicates == 0 {
		o.Component.MaxPredicates = 10000000
	}
	if o.Component.Interior.MaxTriangles < 1 || o.Component.Interior.MaxTriangles > 1000000 || o.Component.Interior.MaxTriangleChecks < 1 || o.Component.Interior.MaxTriangleChecks > 1000000 || o.Component.MaxPredicates < 1 || o.Component.MaxPredicates > 100000000 {
		return empty, geom.ErrInvalid
	}
	if o.MaxSpanCells == 0 {
		o.MaxSpanCells = 100000
	}
	if o.MaxSpanPieces == 0 {
		o.MaxSpanPieces = 100000
	}
	if o.Span.MaxCells == 0 {
		o.Span.MaxCells = 1024
	}
	if o.Span.MaxPieces == 0 {
		o.Span.MaxPieces = 8192
	}
	if o.Span.MaxInputBits == 0 {
		o.Span.MaxInputBits = 16384
	}
	if o.MaxSpanCells < 1 || o.MaxSpanCells > 1000000 || o.MaxSpanPieces < 1 || o.MaxSpanPieces > 1000000 || o.Span.MaxCells < 1 || o.Span.MaxCells > 65536 || o.Span.MaxPieces < 1 || o.Span.MaxPieces > 1000000 || o.Span.MaxInputBits < 1 || o.Span.MaxInputBits > 65536 {
		return empty, geom.ErrInvalid
	}
	if o.Boundary.Loop.Run.Display.Point.Tolerance == 0 {
		tol := o.Boundary.Loop.Run.Display.Deflection * 1e-6
		if tol > 0 {
			o.Boundary.Loop.Run.Display.Point.Tolerance = tol
		}
	}
	var network CertifiedAlgebraicTubeRegionsBoundary
	if prescribed != nil {
		network = *prescribed
	} else {
		var e error
		network, e = r.TessellateBoundaryCertified(ctx, o.Boundary)
		if e != nil {
			return empty, e
		}
	}
	out := CertifiedAlgebraicTubeRegionsMesh{Boundary: network, SourceImageSquaredDistance: new(big.Rat), SignedDisplayVolume: new(big.Rat)}
	out.Mesh.Vertices = append([]geom.Vec3(nil), network.Vertices...)
	work := multiSpanMeshWork{options: MultiSpanCurvedTrimMeshOptions{Span: o.Span, MaxSpanCells: o.MaxSpanCells, MaxSpanPieces: o.MaxSpanPieces, MaxResidualOperations: o.MaxResidualOperations}}
	for region, x := range r.regions {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		options := o.Component
		options.Boundary = o.Boundary.Loop
		options.Interior.MaxTriangleChecks -= out.TriangleChecks
		options.Interior.MaxTriangles -= len(out.Mesh.Triangles)
		options.MaxPredicates -= out.Predicates
		if options.Interior.MaxTriangleChecks < 1 || options.Interior.MaxTriangles < 1 || options.MaxPredicates < 1 {
			return empty, geom.ErrConvergence
		}
		lens := AlgebraicTubeLens{section: r.section, faces: r.partitions[x.Outer].Faces, graphs: r.graphs, upper: x.UpperChart, branch: r.loops[x.Outer].Branch}
		// Only this freshly generated, privately associated network reaches the
		// internal override; no caller-supplied mutable boundary is trusted.
		cellsBefore, piecesBefore, operationsBefore := work.cells, work.pieces, work.operations
		component, e := lens.tessellateClosedTubeLens(ctx, options, &work, o.AffineCharts, &network.Loops[x.Outer], nil)
		if e != nil {
			return empty, e
		}
		component.SpanCellChecks -= cellsBefore
		component.SpanPieceChecks -= piecesBefore
		component.ResidualOperations -= operationsBefore
		out.TriangleChecks += component.TriangleChecks
		out.Predicates += component.Predicates
		remap := make([]int, len(component.Mesh.Vertices))
		for i := range remap {
			remap[i] = -1
		}
		for localBoundary, localMesh := range component.BoundaryVertices {
			global := network.LoopVertices[x.Outer][localBoundary]
			if component.Mesh.Vertices[localMesh] != out.Mesh.Vertices[global] {
				return empty, geom.ErrConvergence
			}
			remap[localMesh] = global
		}
		for i, p := range component.Mesh.Vertices {
			if remap[i] < 0 {
				remap[i] = len(out.Mesh.Vertices)
				out.Mesh.Vertices = append(out.Mesh.Vertices, p)
			}
		}
		for i, tri := range component.Mesh.Triangles {
			out.Mesh.Triangles = append(out.Mesh.Triangles, [3]int{remap[tri[0]], remap[tri[1]], remap[tri[2]]})
			out.OriginalRegion = append(out.OriginalRegion, region)
			out.OriginalChart = append(out.OriginalChart, component.OriginalFace[i])
		}
		if component.SourceImageSquaredDistance.Cmp(out.SourceImageSquaredDistance) > 0 {
			out.SourceImageSquaredDistance.Set(component.SourceImageSquaredDistance)
		}
		out.SignedDisplayVolume.Add(out.SignedDisplayVolume, component.SignedDisplayVolume)
		out.Components = append(out.Components, component)
		out.ComponentVertices = append(out.ComponentVertices, remap)
	}
	if e := out.Mesh.ValidateSolid(); e != nil {
		return empty, e
	}
	report, e := collision.SelfIntersectionsBVHContext(ctx, out.Mesh, o.Component.Collision)
	if e != nil {
		return empty, e
	}
	if !report.Complete || len(report.Contacts) != 0 {
		return empty, geom.ErrConvergence
	}
	out.Collision = report
	out.SpanCellChecks, out.SpanPieceChecks = work.cells, work.pieces
	out.ResidualOperations = work.operations
	return out, nil
}
