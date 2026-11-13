package trimmed

import (
	"context"
	"fmt"
	"plainsolid/geom"
)

// AlgebraicTubeLensBoundaryOptions independently bounds construction of the
// shared display and exact owner chart identities. Neither stage returns a
// partial result as a certificate.
type AlgebraicTubeLensBoundaryOptions struct {
	Run                AlgebraicTubeRunMeshOptions
	Owners             geom.TubeAlgebraicGraphEndpointOptions
	MaxOwnerOperations int
}
type AlgebraicTubeLensBoundaryOwner struct {
	SourceChart, Piece int
	Reversed           bool
	Vertices           []int
	UV                 [][2]*geom.ExactAlgebraicRoot
}
type CertifiedAlgebraicTubeLensBoundary struct {
	Run             CertifiedAlgebraicTubeRun
	Owners          [2][]AlgebraicTubeLensBoundaryOwner
	OwnerOperations int
}

// TessellateBoundaryCertified produces a single common display loop with two
// oppositely incident chart owners. Each UV coordinate is an exact algebraic
// identity, not a rounded inverse projection. Endpoint bindings are rechecked
// against the immutable local tube; rational interior points use its exact
// graph-point certificate. This certifies the boundary approximation only,
// not a filled or embedded closed display shell.
func (l AlgebraicTubeLens) TessellateBoundaryCertified(ctx context.Context, o AlgebraicTubeLensBoundaryOptions) (CertifiedAlgebraicTubeLensBoundary, error) {
	var empty CertifiedAlgebraicTubeLensBoundary
	if ctx == nil || l.graphs[0] == nil {
		return empty, geom.ErrInvalid
	}
	faces := l.Faces()
	reverse := [2]bool{faces[0].Reversed != faces[0].CutReversed, faces[1].Reversed != faces[1].CutReversed}
	return tessellateTubeLensBoundary(ctx, l.section, l.branch, reverse, o)
}

func tessellateTubeLensBoundary(ctx context.Context, section AlgebraicTrimmedPairSection, branch int, reverse [2]bool, o AlgebraicTubeLensBoundaryOptions) (CertifiedAlgebraicTubeLensBoundary, error) {
	var empty CertifiedAlgebraicTubeLensBoundary
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	if o.MaxOwnerOperations == 0 {
		o.MaxOwnerOperations = 16000000
	}
	if o.MaxOwnerOperations < 1 || o.MaxOwnerOperations > 1000000000 {
		return empty, geom.ErrInvalid
	}
	if reverse[0] == reverse[1] {
		return empty, geom.ErrInvalid
	}
	network, e := section.TessellateTubeRunCertified(ctx, 0, branch, 0, o.Run)
	if e != nil {
		return empty, fmt.Errorf("common tube polyline: %w", e)
	}
	if !network.Partition.CutClosed {
		return empty, geom.ErrInvalid
	}
	result := CertifiedAlgebraicTubeLensBoundary{Run: network}
	for i, samples := range network.Samples {
		piece := network.Partition.Cut[i]
		params := make([][4]*geom.ExactAlgebraicRoot, len(samples))
		for j, sample := range samples {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			remaining := o.MaxOwnerOperations - result.OwnerOperations
			if remaining < 1 {
				return empty, geom.ErrConvergence
			}
			options := o.Owners
			if options.MaxOperations == 0 || options.MaxOperations > remaining {
				options.MaxOperations = remaining
			}
			if sample.Rational != nil {
				p, e := piece.Segment.EvaluateRationalProjectedGraph(ctx, sample.Rational, section.partitionSource.projection, options)
				result.OwnerOperations += p.ExactOperations
				if e != nil {
					return empty, e
				}
				if !p.Proven {
					return empty, geom.ErrConvergence
				}
				params[j] = p.Parameters
			} else {
				event := piece.StartPoint
				if j == len(samples)-1 {
					event = piece.EndPoint
				} else if j != 0 {
					return empty, geom.ErrInvalid
				}
				binding, e := piece.Segment.BindProjectedGraphEvent(ctx, event, section.partitionSource.projection, options)
				if binding != nil {
					result.OwnerOperations += binding.ExactOperations()
				}
				if e != nil {
					return empty, e
				}
				params[j], e = binding.Parameters()
				if e != nil {
					return empty, e
				}
			}
		}
		for chart := 0; chart < 2; chart++ {
			owner := AlgebraicTubeLensBoundaryOwner{SourceChart: chart, Piece: i, Reversed: reverse[chart]}
			for k := range samples {
				j := k
				if reverse[chart] {
					j = len(samples) - 1 - k
				}
				owner.Vertices = append(owner.Vertices, network.PieceVertices[i][j])
				owner.UV = append(owner.UV, [2]*geom.ExactAlgebraicRoot{params[j][2*chart], params[j][2*chart+1]})
			}
			result.Owners[chart] = append(result.Owners[chart], owner)
		}
	}
	return result, nil
}
