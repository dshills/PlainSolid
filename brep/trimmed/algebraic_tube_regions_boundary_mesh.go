package trimmed

import (
	"context"
	"plainsolid/geom"
)

// AlgebraicTubeRegionsBoundaryOptions bounds the whole network in addition to
// each loop's proof limits. Each exact loop is sampled once, even when adjacent
// nested material regions use it as both an outer boundary and a hole.
type AlgebraicTubeRegionsBoundaryOptions struct {
	Loop                                 AlgebraicTubeLensBoundaryOptions
	MaxLoops, MaxVertices, MaxOperations int
}
type AlgebraicTubeRegionBoundaryUse struct {
	Region, Chart, Loop, Piece int
	Hole, Reversed             bool
	Vertices                   []int
	UV                         [][2]*geom.ExactAlgebraicRoot
}
type CertifiedAlgebraicTubeRegionsBoundary struct {
	SourceLoops     []AlgebraicTubeRegionLoop
	Regions         []AlgebraicTubeRegion
	Vertices        []geom.Vec3
	Loops           []CertifiedAlgebraicTubeLensBoundary
	LoopVertices    [][]int // local display index -> global network index
	Uses            []AlgebraicTubeRegionBoundaryUse
	ExactOperations int // partition + display + owner work over every loop
}

// TessellateBoundaryCertified preserves exact loop containment and source
// ancestry from a sealed region collection. Every loop has one common model
// polyline and both exact chart owners. Distinct loops are never coordinate
// welded; coincident display vertices of distinct loops reject. The result
// does not certify that the polylines are mutually disjoint, that their display
// containment matches the source, or that the regions have been filled.
// Adjacent nested regions intentionally share the same zero-thickness rim.
func (r AlgebraicTubeRegions) TessellateBoundaryCertified(ctx context.Context, o AlgebraicTubeRegionsBoundaryOptions) (CertifiedAlgebraicTubeRegionsBoundary, error) {
	var empty CertifiedAlgebraicTubeRegionsBoundary
	if ctx == nil || r.section.runSource() == nil || len(r.loops) == 0 {
		return empty, geom.ErrInvalid
	}
	if o.MaxLoops == 0 {
		o.MaxLoops = 16
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 65536
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 128000000
	}
	if o.MaxLoops < 1 || o.MaxLoops > 64 || o.MaxVertices < 1 || o.MaxVertices > 1000000 || o.MaxOperations < 1 || o.MaxOperations > 1000000000 {
		return empty, geom.ErrInvalid
	}
	if len(r.loops) > o.MaxLoops {
		return empty, geom.ErrConvergence
	}
	out := CertifiedAlgebraicTubeRegionsBoundary{SourceLoops: r.Loops(), Regions: r.Regions()}
	used := map[geom.Vec3]bool{}
	for i, loop := range r.loops {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		remaining := o.MaxOperations - out.ExactOperations
		if remaining < 1 {
			return empty, geom.ErrConvergence
		}
		options := o.Loop
		if options.Run.Partition.MaxOperations == 0 {
			options.Run.Partition.MaxOperations = 8000000
		}
		if options.Run.Display.MaxOperations == 0 {
			options.Run.Display.MaxOperations = 8000000
		}
		if options.MaxOwnerOperations == 0 {
			options.MaxOwnerOperations = 16000000
		}
		// Three independent stages share a conservative equal quota.
		remaining /= 3
		if remaining < 1 {
			return empty, geom.ErrConvergence
		}
		vertices := o.MaxVertices - len(out.Vertices)
		if vertices < 1 {
			return empty, geom.ErrConvergence
		}
		if options.Run.Display.MaxSegments == 0 {
			options.Run.Display.MaxSegments = 4096
		}
		if options.Run.Display.MaxSegments > vertices {
			options.Run.Display.MaxSegments = vertices
		}
		options.Run.Partition.MaxOperations = capPositiveBudget(options.Run.Partition.MaxOperations, remaining)
		options.Run.Display.MaxOperations = capPositiveBudget(options.Run.Display.MaxOperations, remaining)
		options.MaxOwnerOperations = capPositiveBudget(options.MaxOwnerOperations, remaining)
		// This private view is used solely by the boundary routine: it is not a
		// newly sealed disk material and is never exposed as an AlgebraicTubeLens.
		view := AlgebraicTubeLens{section: r.section, faces: r.partitions[i].Faces, graphs: r.graphs, branch: loop.Branch}
		if loop.InteriorHeightSign < 0 {
			view.upper = 1
		}
		mesh, e := view.TessellateBoundaryCertified(ctx, options)
		if e != nil {
			return empty, e
		}
		out.ExactOperations += mesh.Run.Partition.ExactOperations + mesh.Run.ExactOperations + mesh.OwnerOperations
		if out.ExactOperations > o.MaxOperations || len(out.Vertices)+len(mesh.Run.Vertices) > o.MaxVertices {
			return empty, geom.ErrConvergence
		}
		ids := make([]int, len(mesh.Run.Vertices))
		for j, p := range mesh.Run.Vertices {
			if used[p] {
				return empty, geom.ErrDegenerate
			}
			used[p] = true
			ids[j] = len(out.Vertices)
			out.Vertices = append(out.Vertices, p)
		}
		out.Loops = append(out.Loops, mesh)
		out.LoopVertices = append(out.LoopVertices, ids)
	}
	for region, x := range r.regions {
		faces := r.RegionFaces(region)
		loops := append([]int{x.Outer}, x.Holes...)
		for chart, face := range faces {
			for index, loop := range loops {
				reversed := face.OuterReversed
				if index > 0 {
					reversed = face.HolesReversed[index-1]
				}
				reversed = reversed != face.Reversed
				for piece, owner := range out.Loops[loop].Owners[chart] {
					use := AlgebraicTubeRegionBoundaryUse{Region: region, Chart: chart, Loop: loop, Piece: piece, Hole: index > 0, Reversed: reversed}
					for j := range owner.Vertices {
						k := j
						if owner.Reversed != reversed {
							k = len(owner.Vertices) - 1 - j
						}
						use.Vertices = append(use.Vertices, out.LoopVertices[loop][owner.Vertices[k]])
						use.UV = append(use.UV, owner.UV[k])
					}
					out.Uses = append(out.Uses, use)
				}
			}
		}
	}
	return out, nil
}
