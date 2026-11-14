package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// This scratch view is justified only by an immutable Boolean's independently
// proved face loops and their nesting/disjointness, after the caller rebinds every whole source tube.
// It is never returned as a sealed original-world AlgebraicTubeRegions value.
func projectedBooleanAnnulusView(ctx context.Context, section AlgebraicTrimmedPairSection, branches [2]int, o AlgebraicTubeRegionOptions) (AlgebraicTubeRegions, error) {
	return projectedBooleanRegionsView(ctx, section, branches[:], true, o)
}

func projectedBooleanRegionsView(ctx context.Context, section AlgebraicTrimmedPairSection, branches []int, nested bool, o AlgebraicTubeRegionOptions) (AlgebraicTubeRegions, error) {
	var empty AlgebraicTubeRegions
	if o.MaxLoops < 0 || o.MaxLoops > 64 {
		return empty, geom.ErrInvalid
	}
	if o.MaxLoops == 0 {
		o.MaxLoops = 16
	}
	if len(branches) < 1 || len(branches) > o.MaxLoops {
		return empty, geom.ErrConvergence
	}
	if section.runSource() == nil || (section.runSource().projection != [2]int{1, 2} && section.runSource().projection != [2]int{0, 2}) {
		return empty, geom.ErrConvergence
	}
	if o.Exact.MaxOperations == 0 {
		o.Exact.MaxOperations = 128000000
	}
	if o.Exact.MaxCoefficientBits == 0 {
		o.Exact.MaxCoefficientBits = 16384
	}
	if o.Exact.MaxOperations < 1 || o.Exact.MaxCoefficientBits < 64 || o.Exact.MaxSubdivisions < 0 {
		return empty, geom.ErrInvalid
	}
	r := AlgebraicTubeRegions{section: section}
	for _, branch := range branches {
		p := o.Partition
		p.MaxOperations = capPositiveBudget(p.MaxOperations, o.Exact.MaxOperations-r.ExactOperations)
		if p.MaxOperations < 1 {
			return empty, geom.ErrConvergence
		}
		q, e := section.PartitionBothFacesByTubeRun(ctx, branch, 0, p)
		r.ExactOperations += q.ExactOperations
		if e != nil {
			return empty, e
		}
		sign := q.Faces[0].FragmentHeightSigns[0]
		if sign == 0 || sign != -q.Faces[1].FragmentHeightSigns[0] {
			return empty, geom.ErrConvergence
		}
		r.loops = append(r.loops, AlgebraicTubeRegionLoop{Branch: branch, Parent: -1, Winding: [2]int{q.Faces[0].CutWinding, q.Faces[1].CutWinding}, InteriorHeightSign: sign})
		r.partitions = append(r.partitions, q)
	}

	if len(r.loops) == 0 {
		return empty, geom.ErrConvergence
	}
	for i, l := range r.loops {
		upper := 0
		if l.InteriorHeightSign < 0 {
			upper = 1
		}
		r.regions = append(r.regions, AlgebraicTubeRegion{Outer: i, UpperChart: upper})
	}
	if nested {
		if len(r.loops) != 2 || r.loops[0].InteriorHeightSign != -r.loops[1].InteriorHeightSign {
			return empty, geom.ErrConvergence
		}
		// Original Boolean outer/inner ownership, not sampled containment.
		r.loops[1].Parent = 0
		r.regions[0].Holes = []int{1}
	}
	for chart := 0; chart < 2; chart++ {
		p := o.Graph
		p.MaxOperations = capPositiveBudget(p.MaxOperations, o.Exact.MaxOperations-r.ExactOperations)
		if p.MaxOperations < 1 {
			return empty, geom.ErrConvergence
		}
		support, patch, e := permuteTubeLensSupport(r.partitions[0].Faces[chart], [3]int{section.runSource().projection[0], section.runSource().projection[1], 3 - section.runSource().projection[0] - section.runSource().projection[1]}, p)
		if e != nil {
			return empty, e
		}
		r.ExactOperations += patch.ExactOperations()
		p.MaxOperations = capPositiveBudget(o.Graph.MaxOperations, o.Exact.MaxOperations-r.ExactOperations)
		if p.MaxOperations < 1 {
			return empty, geom.ErrConvergence
		}
		r.graphs[chart], e = geom.NewAffineXYGraph(ctx, patch, p)
		if e != nil {
			return empty, e
		}
		r.ExactOperations += r.graphs[chart].ExactOperations()
		for i := range r.partitions {
			r.partitions[i].Faces[chart].SourceSurface = support
			r.partitions[i].Faces[chart].Support, _ = support.(*geom.BSplineSurface)
		}
	}
	if r.ExactOperations > o.Exact.MaxOperations {
		return empty, geom.ErrConvergence
	}
	return r, nil
}

func tessellateProjectedBooleanAnnulus(ctx context.Context, r AlgebraicTubeRegions, o AlgebraicTubeRegionMeshOptions) (CertifiedAlgebraicTubeRegionMesh, error) {
	var empty CertifiedAlgebraicTubeRegionMesh
	work, e := newTubeLensSpanWork(o.Surface)
	if e != nil {
		return empty, e
	}
	if o.Footprint.Boundary.Loop.Run.Display.Point.Tolerance == 0 {
		o.Footprint.Boundary.Loop.Run.Display.Point.Tolerance = o.Footprint.Boundary.Loop.Run.Display.Deflection * 1e-6
	}
	original, e := r.TessellateBoundaryCertified(ctx, o.Footprint.Boundary)
	if e != nil {
		return empty, e
	}
	axes := r.section.runSource().projection
	order := [3]int{axes[0], axes[1], 3 - axes[0] - axes[1]}
	forward := func(p geom.Vec3) geom.Vec3 {
		v := [3]float64{p.X, p.Y, p.Z}
		return geom.Vec3{X: v[order[0]], Y: v[order[1]], Z: v[order[2]]}
	}
	inverse := func(p geom.Vec3) geom.Vec3 {
		var v [3]float64
		v[order[0]], v[order[1]], v[order[2]] = p.X, p.Y, p.Z
		return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}
	}
	// The temporary graph view uses canonical orientation. For odd permutation,
	// return owners in physical world traversal while retaining exact UV/model
	// sample identities. Reversal changes no point-correspondence certificate.
	if projectedAxisParity(axes) < 0 {
		for li := range original.Loops {
			for chart := range original.Loops[li].Owners {
				for i := range original.Loops[li].Owners[chart] {
					owner := &original.Loops[li].Owners[chart][i]
					owner.Reversed = !owner.Reversed
					for a, b := 0, len(owner.Vertices)-1; a < b; a, b = a+1, b-1 {
						owner.Vertices[a], owner.Vertices[b] = owner.Vertices[b], owner.Vertices[a]
						owner.UV[a], owner.UV[b] = owner.UV[b], owner.UV[a]
					}
				}
			}
		}
	}

	projected := original
	projected.Vertices = append([]geom.Vec3(nil), original.Vertices...)
	for i, p := range projected.Vertices {
		projected.Vertices[i] = forward(p)
	}
	projected.Loops = append([]CertifiedAlgebraicTubeLensBoundary(nil), original.Loops...)
	for i := range projected.Loops {
		projected.Loops[i].Run.Vertices = append([]geom.Vec3(nil), original.Loops[i].Run.Vertices...)
		for j, p := range projected.Loops[i].Run.Vertices {
			projected.Loops[i].Run.Vertices[j] = forward(p)
		}
	}
	footprint, e := triangulateTubeFootprintNetwork(ctx, projected, []int{0, 1}, o.Footprint)
	if e != nil {
		return empty, e
	}
	footprint.Boundary = original
	footprint.Region = -1
	fill := tubeRegionFillSeed{seed: CertifiedAffineQuadraticTrimMesh{Region: AffineQuadraticTrimRegionArea{FillSquaredDistance: new(big.Rat)}, Mesh: mesh.Mesh{Triangles: footprint.Mesh.Triangles}, BoundaryLoops: footprint.BoundaryLoops}, squared: footprint.SourceFillSquaredDistance, predicates: footprint.Predicates}
	for i, p := range footprint.Mesh.Vertices {
		fill.seed.ExactUV = append(fill.seed.ExactUV, [2]*big.Rat{ratInset(p.X), ratInset(p.Y)})
		fill.vertices = append(fill.vertices, projected.Vertices[footprint.GlobalBoundaryVertices[i]])
	}
	view := AlgebraicTubeLens{faces: r.partitions[0].Faces, graphs: r.graphs, upper: r.regions[0].UpperChart}
	options := o.Surface.Closed
	options.Boundary = o.Footprint.Boundary.Loop
	shell, e := view.tessellateClosedTubeLens(ctx, options, &work, true, nil, &fill)
	if e != nil {
		return empty, e
	}
	for i, p := range shell.Mesh.Vertices {
		shell.Mesh.Vertices[i] = inverse(p)
	}
	if projectedAxisParity(axes) < 0 {
		for i := range shell.Mesh.Triangles {
			t := &shell.Mesh.Triangles[i]
			t[1], t[2] = t[2], t[1]
		}
	}
	if e := shell.Mesh.ValidateSolid(); e != nil {
		return empty, e
	}
	out := CertifiedAlgebraicTubeRegionMesh{Footprint: footprint, Mesh: shell.Mesh, Region: -1, OriginalChart: shell.OriginalFace, GlobalBoundaryVertices: map[int]int{}, FaceResidualSquared: shell.FaceResidualSquared, FaceRefinementPasses: shell.FaceRefinementPasses, SourceImageSquaredDistance: shell.SourceImageSquaredDistance, SignedDisplayVolume: shell.SignedDisplayVolume, Collision: shell.Collision, TriangleChecks: shell.TriangleChecks, Predicates: shell.Predicates, SpanCellChecks: work.cells, SpanPieceChecks: work.pieces, ResidualOperations: work.operations}
	for seed, index := range shell.BoundaryVertices {
		out.GlobalBoundaryVertices[footprint.GlobalBoundaryVertices[seed]] = index
	}
	return out, nil
}
