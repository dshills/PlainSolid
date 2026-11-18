//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// CoincidentTubeSourceRange identifies the part of one original boundary
// coedge that corresponds exactly to one interval of the imprint. Both roots
// use original source parameter units and follow section traversal.
type CoincidentTubeSourceRange struct {
	Segment, Interval int
	Source            SplineTubeTrimSource
	Start, End        *geom.ExactAlgebraicRoot
}

// CoincidentTubeRunImprint records an existing edge on one face and the
// corresponding partition of the opposite face.
type CoincidentTubeRunImprint struct {
	BoundaryChart   int
	BoundaryRanges  []CoincidentTubeSourceRange
	Partition       AlgebraicTubeFacePartition
	ExactOperations int
}

// ImprintCoincidentTrimRun splits the face whose interior contains the run.
// On the other face the whole run must follow proved original trim spans.
// The result retains each exact source-edge range so both uses can be planned
// for a later shell edit. Existing topology is not mutated by this proof.
func (r AlgebraicTrimmedPairSection) ImprintCoincidentTrimRun(ctx context.Context, boundaryChart, branch, run int,
	o AlgebraicTubeFacePartitionOptions) (out CoincidentTubeRunImprint, err error) {
	s := r.coincidentSource
	if ctx == nil || s == nil || boundaryChart < 0 || boundaryChart > 1 || branch < 0 || branch >= len(s.branches) ||
		run < 0 || run >= len(s.branches[branch].Runs) {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 16000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 0 {
		return out, geom.ErrInvalid
	}
	defer func() {
		if err != nil {
			out = CoincidentTubeRunImprint{ExactOperations: out.ExactOperations}
		}
	}()
	b := s.branches[branch]
	out.BoundaryChart = boundaryChart
	for _, item := range b.Runs[run].Intervals {
		clip := b.Clips[item.Segment]
		iv := clip.Intervals[item.Interval]
		i := iv.CoincidentTrim[boundaryChart]
		if !iv.Retained || i < 0 || iv.Locations[boundaryChart] != trimvalidate.Boundary || iv.Locations[1-boundaryChart] != trimvalidate.Inside {
			return out, geom.ErrConvergence
		}
		span := clip.FaceTrims[boundaryChart].Coincidences[i]
		source := span.Ends[0].Source
		x := CoincidentTubeSourceRange{Segment: item.Segment, Interval: item.Interval, Source: source}
		for k, endpoint := range []int{iv.Start, iv.End} {
			child := o.ExactCurvePlaneEventOptions
			child.MaxOperations -= out.ExactOperations
			if child.MaxOperations < 1 {
				return out, geom.ErrConvergence
			}
			root, used, e := span.proof.ParameterAtFreeRoot(ctx, clip.Events[endpoint].Free, child)
			out.ExactOperations += used
			if e != nil {
				return out, e
			}
			if source.ParameterStart != 0 || source.ParameterEnd != 1 {
				child.MaxOperations -= used
				if child.MaxOperations < 1 {
					return out, geom.ErrConvergence
				}
				start, end := ratInset(source.ParameterStart), ratInset(source.ParameterEnd)
				root, used, e = root.AffineMap(ctx, start, new(big.Rat).Sub(end, start), child)
				out.ExactOperations += used
				if e != nil {
					return out, e
				}
			}
			if k == 0 {
				x.Start = root
			} else {
				x.End = root
			}
		}
		out.BoundaryRanges = append(out.BoundaryRanges, x)
	}
	o.MaxOperations -= out.ExactOperations
	if o.MaxOperations < 1 {
		return out, geom.ErrConvergence
	}
	interior := AlgebraicTrimmedPairSection{exactPartitionSource: s}
	out.Partition, err = interior.PartitionFaceByTubeRun(ctx, 1-boundaryChart, branch, run, o)
	out.ExactOperations += out.Partition.ExactOperations
	return out, err
}
