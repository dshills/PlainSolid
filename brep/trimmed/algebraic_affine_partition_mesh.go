package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// CertifiedAffinePartitionBoundary adds one certified model chord to an exact
// two-fragment partition. It is NOT a triangle fill certificate. Affine support
// proves that the true new cut is a straight model segment; maximum endpoint
// displacement therefore bounds both directions under common interpolation.
type CertifiedAffinePartitionBoundary struct {
	Boundary        CertifiedAlgebraicBoundaryMesh
	Partition       AlgebraicFacePartition
	Cut             [2]int
	CutSquaredError *big.Rat
	Fragments       [2][]int // Closed vertex rings; final start vertex is not repeated.
}

// TessellateAffinePartitionBoundaryCertified rederives the exact hole-free
// partition and every source boundary owner. Distinct algebraic vertices retain
// their topology IDs. The support must be exactly affine; curved supports reject.
// This certifies each boundary piece, not embeddedness or equivalence of fills.
func (s AlgebraicSubdivision) TessellateAffinePartitionBoundaryCertified(ctx context.Context, face int, line [3]*big.Rat, o AlgebraicBoundaryMeshOptions) (CertifiedAffinePartitionBoundary, error) {
	var empty CertifiedAffinePartitionBoundary
	partition, e := s.PartitionFaceByAlgebraicUVLine(ctx, face, line, o.Proof)
	if e != nil {
		return empty, e
	}
	patch, e := exactFaceSupportPatch(s.source.faces[face], o.Proof.Composition.SurfacePatchOptions)
	if e != nil {
		return empty, e
	}
	chart, e := geom.NewAffineSurfaceChart(ctx, patch, geom.AffineSurfaceChartOptions{MaxOperations: o.Proof.Composition.MaxOperations, MaxCoefficientBits: o.Proof.Composition.MaxCoefficientBits})
	if e != nil {
		return empty, e
	}
	boundary, e := s.TessellateBoundaryCertified(ctx, o)
	if e != nil {
		return empty, e
	}
	result := CertifiedAffinePartitionBoundary{Boundary: boundary, Partition: partition, CutSquaredError: new(big.Rat)}
	for side, id := range partition.Vertices {
		result.Cut[side] = boundary.TopologyVertices[id]
		display := exactBoundPoint(boundary.Vertices[result.Cut[side]])
		bounds := partition.CutUV[side]
		if !bounds.HasBounds {
			return empty, geom.ErrConvergence
		}
		for _, u := range []*big.Rat{bounds.Coordinates[0].Lower, bounds.Coordinates[0].Upper} {
			for _, v := range []*big.Rat{bounds.Coordinates[1].Lower, bounds.Coordinates[1].Upper} {
				point, e := chart.EvalExact(u, v)
				if e != nil {
					return empty, e
				}
				squared := new(big.Rat)
				for axis := range point {
					d := new(big.Rat).Sub(point[axis], display[axis])
					squared.Add(squared, new(big.Rat).Mul(d, d))
				}
				if squared.Cmp(result.CutSquaredError) > 0 {
					result.CutSquaredError.Set(squared)
				}
			}
		}
	}
	deflection := o.Display.Deflection
	if deflection == 0 {
		deflection = 1e-6
	}
	d := ratInset(deflection)
	if result.CutSquaredError.Cmp(new(big.Rat).Mul(d, d)) > 0 {
		return empty, geom.ErrConvergence
	}
	for fi, fragment := range partition.Fragments {
		var ring []int
		for _, use := range fragment {
			path := []int{result.Cut[0], result.Cut[1]}
			if use.BoundaryEdge >= 0 {
				path = boundary.Edges[use.BoundaryEdge]
			}
			path = append([]int(nil), path...)
			if use.Reversed {
				for a, b := 0, len(path)-1; a < b; a, b = a+1, b-1 {
					path[a], path[b] = path[b], path[a]
				}
			}
			if len(ring) > 0 {
				if ring[len(ring)-1] != path[0] {
					return empty, geom.ErrConvergence
				}
				path = path[1:]
			}
			ring = append(ring, path...)
		}
		if len(ring) < 4 || ring[0] != ring[len(ring)-1] {
			return empty, geom.ErrConvergence
		}
		result.Fragments[fi] = ring[:len(ring)-1]
	}
	return result, nil
}
